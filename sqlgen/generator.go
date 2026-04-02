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
	var sb strings.Builder

	sb.WriteString(`CREATE EXTENSION IF NOT EXISTS "pgcrypto";` + "\n")

	if anyCRDTEnabled(tables) {
		writeHLCInfrastructure(&sb)
	}

	for _, table := range tables {
		writeTableDDL(&sb, table)
		writeTableIndexes(&sb, table)
		writeReplicaIdentity(&sb, table)

		if table.CRDT != nil && table.CRDT.Enabled {
			writeHLCTriggers(&sb, table)
		} else {
			writeLWWTriggers(&sb, table)
		}
	}

	return sb.String()
}

// anyCRDTEnabled checks if any table has CRDT enabled.
func anyCRDTEnabled(tables map[string]schema.Table) bool {
	for _, table := range tables {
		if table.CRDT != nil && table.CRDT.Enabled {
			return true
		}
	}
	return false
}

// writeHLCInfrastructure emits the shared HLC schema, state table, and advance function.
func writeHLCInfrastructure(sb *strings.Builder) {
	sb.WriteString(`
-- ============================================================
-- HLC (Hybrid Logical Clock) infrastructure for CRDT support
-- ============================================================
CREATE SCHEMA IF NOT EXISTS _pgconverge;

CREATE TABLE IF NOT EXISTS _pgconverge.hlc_state (
    node_name VARCHAR(50) PRIMARY KEY,
    hlc_ts BIGINT NOT NULL DEFAULT 0,
    hlc_counter INTEGER NOT NULL DEFAULT 0
);

CREATE OR REPLACE FUNCTION _pgconverge.advance_hlc(
    remote_ts BIGINT DEFAULT 0,
    remote_counter INTEGER DEFAULT 0
) RETURNS TABLE(new_ts BIGINT, new_counter INTEGER) AS $$
DECLARE
    local_ts BIGINT;
    local_counter INTEGER;
    wall_ts BIGINT;
    node_id VARCHAR(50);
BEGIN
    node_id := coalesce(current_setting('pgconverge.node_name', true), 'unknown');
    wall_ts := (EXTRACT(EPOCH FROM clock_timestamp()) * 1000000)::BIGINT;

    SELECT h.hlc_ts, h.hlc_counter INTO local_ts, local_counter
    FROM _pgconverge.hlc_state h
    WHERE h.node_name = node_id
    FOR UPDATE;

    IF NOT FOUND THEN
        local_ts := 0;
        local_counter := 0;
    END IF;

    -- HLC advance algorithm
    IF wall_ts > local_ts AND wall_ts > remote_ts THEN
        new_ts := wall_ts;
        new_counter := 0;
    ELSIF local_ts > remote_ts THEN
        new_ts := local_ts;
        new_counter := local_counter + 1;
    ELSIF remote_ts > local_ts THEN
        new_ts := remote_ts;
        new_counter := remote_counter + 1;
    ELSE
        new_ts := local_ts;
        new_counter := GREATEST(local_counter, remote_counter) + 1;
    END IF;

    INSERT INTO _pgconverge.hlc_state (node_name, hlc_ts, hlc_counter)
    VALUES (node_id, new_ts, new_counter)
    ON CONFLICT (node_name) DO UPDATE
    SET hlc_ts = EXCLUDED.hlc_ts, hlc_counter = EXCLUDED.hlc_counter;

    RETURN NEXT;
END;
$$ LANGUAGE plpgsql;
`)
}

// writeTableDDL emits the CREATE TABLE statement with appropriate metadata columns.
func writeTableDDL(sb *strings.Builder, table schema.Table) {
	cols := []string{}

	for _, col := range table.Columns {
		if col.DataType == "serial" && util.Contains(table.Constraints.Primary, col.Name) {
			cols = append(cols,
				fmt.Sprintf(`"%s" uuid DEFAULT gen_random_uuid()`, col.Name),
			)
			continue
		}

		def := ""
		if col.Default != "" {
			def = fmt.Sprintf(" DEFAULT %s", col.Default)
		}
		cols = append(cols,
			fmt.Sprintf(`"%s" %s%s`, col.Name, col.DataType, def),
		)
	}

	// Common metadata columns
	cols = append(cols, `"updated_at" TIMESTAMP DEFAULT now()`)
	cols = append(cols, `"origin_node" VARCHAR(50)`)

	// HLC columns for CRDT-enabled tables
	if table.CRDT != nil && table.CRDT.Enabled {
		cols = append(cols, `"_hlc_ts" BIGINT DEFAULT 0`)
		cols = append(cols, `"_hlc_counter" INTEGER DEFAULT 0`)
		cols = append(cols, `"_hlc_node" VARCHAR(50) DEFAULT ''`)
	}

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

	sb.WriteString(fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS "%s" (%s);`,
		table.Name,
		strings.Join(append(cols, constraints...), ",\n  "),
	))
}

// writeTableIndexes emits CREATE INDEX statements.
func writeTableIndexes(sb *strings.Builder, table schema.Table) {
	for _, idxCols := range table.Indexes {
		idxName := fmt.Sprintf("%s_%s_idx", table.Name, strings.Join(idxCols, "_"))
		sb.WriteString(fmt.Sprintf(
			`CREATE INDEX IF NOT EXISTS "%s" ON "%s" (%s);`,
			idxName,
			table.Name,
			util.QuoteCols(idxCols),
		))
	}
}

// writeReplicaIdentity emits ALTER TABLE ... REPLICA IDENTITY FULL.
func writeReplicaIdentity(sb *strings.Builder, table schema.Table) {
	sb.WriteString(fmt.Sprintf("\nALTER TABLE \"%s\" REPLICA IDENTITY FULL;\n", table.Name))
}

// writeLWWTriggers emits timestamp-based last-write-wins triggers (non-CRDT fallback).
func writeLWWTriggers(sb *strings.Builder, table schema.Table) {
	q := fmt.Sprintf(`"%s"`, table.Name)
	sb.WriteString(fmt.Sprintf(`
CREATE OR REPLACE FUNCTION %s_set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER set_updated_at_%s
BEFORE INSERT OR UPDATE ON %s
FOR EACH ROW EXECUTE FUNCTION %s_set_updated_at();

CREATE OR REPLACE FUNCTION %s_resolve_conflict() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF NEW.updated_at > OLD.updated_at THEN
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

ALTER TABLE %s ENABLE ALWAYS TRIGGER conflict_resolution_%s;
`,
		table.Name,
		table.Name, q, table.Name,
		table.Name,
		table.Name, q, table.Name,
		q, table.Name,
	))
}

// writeHLCTriggers emits HLC-based conflict resolution triggers for CRDT-enabled tables.
func writeHLCTriggers(sb *strings.Builder, table schema.Table) {
	q := fmt.Sprintf(`"%s"`, table.Name)
	sb.WriteString(fmt.Sprintf(`
-- HLC stamping: only fires on local writes (normal trigger, skipped by replication workers)
CREATE OR REPLACE FUNCTION %s_stamp_hlc() RETURNS TRIGGER AS $$
DECLARE
    hlc RECORD;
BEGIN
    SELECT * INTO hlc FROM _pgconverge.advance_hlc(0, 0);
    NEW._hlc_ts := hlc.new_ts;
    NEW._hlc_counter := hlc.new_counter;
    NEW._hlc_node := coalesce(current_setting('pgconverge.node_name', true), 'unknown');
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER a_stamp_hlc_%s
BEFORE INSERT OR UPDATE ON %s
FOR EACH ROW EXECUTE FUNCTION %s_stamp_hlc();

-- HLC conflict resolution: fires on ALL writes including replicated (ENABLE ALWAYS)
-- Trigger name prefixed with "z_" to ensure it fires AFTER the "a_stamp_hlc" trigger
-- (PostgreSQL executes BEFORE triggers in alphabetical order)
CREATE OR REPLACE FUNCTION %s_resolve_conflict() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        -- Advance local HLC to track incoming timestamp (maintains causal ordering)
        PERFORM _pgconverge.advance_hlc(NEW._hlc_ts, NEW._hlc_counter);

        -- Total ordering via HLC tuple: (ts, counter, node_name)
        IF (NEW._hlc_ts, NEW._hlc_counter, NEW._hlc_node) > (OLD._hlc_ts, OLD._hlc_counter, OLD._hlc_node) THEN
            RETURN NEW;
        ELSE
            RETURN NULL;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER z_resolve_conflict_%s
BEFORE UPDATE ON %s
FOR EACH ROW EXECUTE FUNCTION %s_resolve_conflict();

ALTER TABLE %s ENABLE ALWAYS TRIGGER z_resolve_conflict_%s;
`,
		table.Name,
		table.Name, q, table.Name,
		table.Name,
		table.Name, q, table.Name,
		q, table.Name,
	))
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
