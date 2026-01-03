package schemagenerator

import (
	"fmt"
	schema "pgconverge/internal"
	helper "pgconverge/internal/util"
	"strings"
)

func GenerateSQL(tables map[string]schema.Table) string {
	var sqlBuilder strings.Builder

	sqlBuilder.WriteString(`CREATE EXTENSION IF NOT EXISTS "pgcrypto";` + "\n")

	for _, table := range tables {
		cols := []string{}

		for _, col := range table.Columns {
			def := ""

			if col.DataType == "serial" && helper.Contains(table.Constraints.Primary, col.Name) {
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
				fmt.Sprintf("PRIMARY KEY (%s)", helper.QuoteCols(table.Constraints.Primary)),
			)
		}

		for _, uniq := range table.Constraints.Unique {
			constraints = append(constraints,
				fmt.Sprintf("UNIQUE (%s)", helper.QuoteCols(uniq)),
			)
		}

		for _, fk := range table.Constraints.ForeignKeys {
			constraints = append(constraints, fmt.Sprintf(
				"FOREIGN KEY (%s) REFERENCES %s(%s)",
				helper.QuoteCols(fk.Columns),
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
				helper.QuoteCols(idxCols),
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
