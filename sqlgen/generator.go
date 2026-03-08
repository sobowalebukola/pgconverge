// Package sqlgen provides SQL schema generation for PostgreSQL.
package sqlgen

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sobowalebukola/pgconverge/schema"
	"github.com/sobowalebukola/pgconverge/util"
)

// GenerateSQL generates SQL DDL from table definitions.
func GenerateSQL(tables map[string]schema.Table) string {
	var sqlBuilder strings.Builder

	sqlBuilder.WriteString(`CREATE EXTENSION IF NOT EXISTS "pgcrypto";` + "\n")

	for _, table := range tables {
		cols := []string{}

		for _, col := range table.Columns {
			def := ""

			if col.DataType == "serial" && util.Contains(table.Constraints.Primary, col.Name) {
				cols = append(cols,
					fmt.Sprintf(`"%s" uuid DEFAULT gen_random_uuid()`, col.Name),
				)
				continue
			}

			if col.Default != "" {
				def = fmt.Sprintf(" DEFAULT %s", col.Default)
			}

			cols = append(cols,
				fmt.Sprintf(`"%s" %s%s`, col.Name, col.DataType, def),
			)
		}

		cols = append(cols, `"updated_at" TIMESTAMP DEFAULT now()`)
		cols = append(cols, `"origin_node" VARCHAR(50)`)

		constraints := []string{}
		if len(table.Constraints.Primary) > 0 {
			constraints = append(constraints,
				fmt.Sprintf("PRIMARY KEY (%s)", util.QuoteCols(table.Constraints.Primary)),
			)
		}

		for _, uniq := range table.Constraints.Unique {
			constraints = append(constraints,
				fmt.Sprintf("UNIQUE (%s)", util.QuoteCols(uniq)),
			)
		}

		for _, fk := range table.Constraints.ForeignKeys {
			constraints = append(constraints, fmt.Sprintf(
				"FOREIGN KEY (%s) REFERENCES %s(%s)",
				util.QuoteCols(fk.Columns),
				fk.References["table"][0],
				strings.Join(fk.References["columns"], ", "),
			))
		}

		// CREATE TABLE
		sqlBuilder.WriteString(fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS "%s" (%s);`,
			table.Name,
			strings.Join(append(cols, constraints...), ",\n  "),
		))

		// Indexes
		for _, idxCols := range table.Indexes {
			idxName := fmt.Sprintf("%s_%s_idx", table.Name, strings.Join(idxCols, "_"))
			sqlBuilder.WriteString(fmt.Sprintf(
				`CREATE INDEX IF NOT EXISTS "%s" ON "%s" (%s);`,
				idxName,
				table.Name,
				util.QuoteCols(idxCols),
			))
		}

		// Conflict resolution & triggers
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

CREATE OR REPLACE TRIGGER set_updated_at_%s
BEFORE INSERT OR UPDATE ON %s
FOR EACH ROW EXECUTE FUNCTION %s_set_updated_at();

-- 3. Resolve conflicts based on timestamp
CREATE OR REPLACE FUNCTION %s_resolve_conflict() RETURNS TRIGGER AS $$
BEGIN
	IF TG_OP = 'UPDATE' THEN
		IF NEW.updated_at > OLD.updated_at + interval '1 second' THEN
			RETURN NEW;
		ELSE
			RETURN NULL;
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
			table.Name,
		))
	}

	return sqlBuilder.String()
}

// Generate reads schema from a file and writes SQL to output file.
func Generate(schemaFile, outputFile string) error {
	schemaBytes, err := os.ReadFile(schemaFile)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	var tables map[string]schema.Table
	if err := json.Unmarshal(schemaBytes, &tables); err != nil {
		return fmt.Errorf("failed to parse schema file: %w", err)
	}

	sql := GenerateSQL(tables)

	if err := os.WriteFile(outputFile, []byte(sql), 0644); err != nil {
		return fmt.Errorf("failed to write SQL file: %w", err)
	}

	return nil
}
