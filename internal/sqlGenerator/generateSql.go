package sqlgenerator

import (
	"encoding/json"
	"fmt"
	schema "jsonddl/internal"
	helper "jsonddl/internal/util"
	"log"
	"os"
	"strings"
)

func SchemaGenerator() {

	schemaBytes, err := os.ReadFile("schema.json")
	if err != nil {
		log.Fatal(err)
	}
	var tables map[string]schema.Table
	if err := json.Unmarshal(schemaBytes, &tables); err != nil {
		log.Fatal(err)
	}

	var sqlBuilder strings.Builder

	for _, table := range tables {
		cols := []string{}

		for _, col := range table.Columns {
			def := ""

			// Auto-upgrade SERIAL primary keys to UUID
			if col.DataType == "serial" && helper.Contains(table.Constraints.Primary, col.Name) {
				cols = append(
					cols,
					fmt.Sprintf(`"%s" uuid DEFAULT gen_random_uuid()`, col.Name),
				)
				continue
			}

			if col.Default != "" {
				def = fmt.Sprintf(" DEFAULT %s", col.Default)
			}

			cols = append(
				cols,
				fmt.Sprintf(`"%s" %s%s`, col.Name, col.DataType, def),
			)
		}

		// add updated_at
		cols = append(cols, `"updated_at" TIMESTAMP DEFAULT now()`)

		cols = append(cols, `"origin_node" VARCHAR(50)`)

		constraints := []string{}
		if len(table.Constraints.Primary) > 0 {
			constraints = append(constraints, fmt.Sprintf("PRIMARY KEY (%s)", helper.QuoteCols(table.Constraints.Primary)))
		}
		for _, uniq := range table.Constraints.Unique {
			constraints = append(constraints, fmt.Sprintf("UNIQUE (%s)", helper.QuoteCols(uniq)))
		}

		for _, fk := range table.Constraints.ForeignKeys {
			constraints = append(constraints, fmt.Sprintf(
				"FOREIGN KEY (%s) REFERENCES %s(%s)",
				helper.QuoteCols(fk.Columns),
				fk.References["table"][0],                    // first table name
				strings.Join(fk.References["columns"], ", "), // join columns
			))
		}

		sqlBuilder.WriteString(`CREATE EXTENSION IF NOT EXISTS "pgcrypto";`)

		sqlBuilder.WriteString(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS "%s" (%s);`, table.Name, strings.Join(append(cols, constraints...), ",\n  ")))

		// Indexes
		for _, idxCols := range table.Indexes {
			idxName := fmt.Sprintf("%s_%s_idx", table.Name, strings.Join(idxCols, "_"))
			sqlBuilder.WriteString(fmt.Sprintf(`CREATE INDEX IF NOT EXISTS "%s" ON "%s" (%s);`,
				idxName, table.Name, helper.QuoteCols(idxCols)))
		}

		sqlBuilder.WriteString(fmt.Sprintf(`
				-- 1. Set replica identity
				ALTER TABLE %s REPLICA IDENTITY FULL;

				-- 2. Auto-update timestamp on changes
				CREATE OR REPLACE FUNCTION %s_set_updated_at() RETURNS TRIGGER AS $$
				BEGIN
					NEW.updated_at = NOW();
					RETURN NEW;
				END;
				$$ LANGUAGE plpgsql;

				CREATE TRIGGER set_updated_at_%s
				BEFORE INSERT OR UPDATE ON %s
				FOR EACH ROW EXECUTE FUNCTION %s_set_updated_at();

				-- 3. Resolve conflicts based on timestamp
				CREATE OR REPLACE FUNCTION %s_resolve_conflict() RETURNS TRIGGER AS $$
				BEGIN
					IF TG_OP = 'UPDATE' THEN
						-- Only apply if newer (with small tolerance for clock skew)
						IF NEW.updated_at > OLD.updated_at + interval '1 second' THEN
							RETURN NEW;
						ELSE
							RETURN NULL;  -- Skip, keep existing
						END IF;
					END IF;
					RETURN NEW;
				END;
				$$ LANGUAGE plpgsql;

				CREATE OR REPLACE TRIGGER conflict_resolution_%s 
				BEFORE UPDATE ON %s
				FOR EACH ROW EXECUTE FUNCTION %s_resolve_conflict();
`,
			table.Name,
			table.Name,
			table.Name,
			table.Name,
			table.Name,
			table.Name,
			table.Name,
			table.Name,
			table.Name))

	}

	// Write SQL file
	if err := os.WriteFile("generated.sql", []byte(sqlBuilder.String()), 0644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("SQL generated in generated.sql")

}
