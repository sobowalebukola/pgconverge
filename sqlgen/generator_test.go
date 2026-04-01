package sqlgen

import (
	"strings"
	"testing"

	"github.com/sobowalebukola/pgconverge/schema"
)

func TestGenerateSQL_ContainsPgcrypto(t *testing.T) {
	tables := map[string]schema.Table{}
	sql := GenerateSQL(tables)
	if !strings.Contains(sql, `CREATE EXTENSION IF NOT EXISTS "pgcrypto"`) {
		t.Error("expected pgcrypto extension in output")
	}
}

func TestGenerateSQL_BasicTable(t *testing.T) {
	tables := map[string]schema.Table{
		"users": {
			Name: "users",
			Columns: map[string]schema.Column{
				"name": {Name: "name", DataType: "VARCHAR(100)"},
			},
			Constraints: schema.Constraints{
				Primary: []string{},
			},
		},
	}

	sql := GenerateSQL(tables)

	if !strings.Contains(sql, `CREATE TABLE IF NOT EXISTS "users"`) {
		t.Error("expected CREATE TABLE statement")
	}
	if !strings.Contains(sql, `"name" VARCHAR(100)`) {
		t.Error("expected name column")
	}
	if !strings.Contains(sql, `"updated_at" TIMESTAMP DEFAULT now()`) {
		t.Error("expected updated_at column")
	}
	if !strings.Contains(sql, `"origin_node" VARCHAR(50)`) {
		t.Error("expected origin_node column")
	}
}

func TestGenerateSQL_SerialPrimaryKeyBecomesUUID(t *testing.T) {
	tables := map[string]schema.Table{
		"items": {
			Name: "items",
			Columns: map[string]schema.Column{
				"id": {Name: "id", DataType: "serial"},
			},
			Constraints: schema.Constraints{
				Primary: []string{"id"},
			},
		},
	}

	sql := GenerateSQL(tables)

	if !strings.Contains(sql, `"id" uuid DEFAULT gen_random_uuid()`) {
		t.Error("expected serial primary key to be converted to UUID")
	}
	if strings.Contains(sql, `"id" serial`) {
		t.Error("serial type should have been replaced with uuid")
	}
}

func TestGenerateSQL_ReplicaIdentityFull(t *testing.T) {
	tables := map[string]schema.Table{
		"orders": {
			Name: "orders",
			Columns: map[string]schema.Column{
				"total": {Name: "total", DataType: "numeric"},
			},
		},
	}

	sql := GenerateSQL(tables)

	if !strings.Contains(sql, `ALTER TABLE "orders" REPLICA IDENTITY FULL`) {
		t.Error("expected REPLICA IDENTITY FULL with quoted table name")
	}
}

func TestGenerateSQL_ConflictResolutionNoOneSecondWindow(t *testing.T) {
	tables := map[string]schema.Table{
		"data": {
			Name: "data",
			Columns: map[string]schema.Column{
				"value": {Name: "value", DataType: "text"},
			},
		},
	}

	sql := GenerateSQL(tables)

	if strings.Contains(sql, "interval '1 second'") {
		t.Error("conflict resolution should NOT have the 1-second window")
	}
	if !strings.Contains(sql, "IF NEW.updated_at > OLD.updated_at THEN") {
		t.Error("expected strict last-write-wins comparison")
	}
}

func TestGenerateSQL_Indexes(t *testing.T) {
	tables := map[string]schema.Table{
		"users": {
			Name: "users",
			Columns: map[string]schema.Column{
				"email": {Name: "email", DataType: "VARCHAR(255)"},
			},
			Indexes: [][]string{{"email"}},
		},
	}

	sql := GenerateSQL(tables)

	if !strings.Contains(sql, `CREATE INDEX IF NOT EXISTS "users_email_idx"`) {
		t.Error("expected index creation")
	}
}

func TestGenerateSQL_UniqueConstraint(t *testing.T) {
	tables := map[string]schema.Table{
		"users": {
			Name: "users",
			Columns: map[string]schema.Column{
				"email": {Name: "email", DataType: "VARCHAR(255)"},
			},
			Constraints: schema.Constraints{
				Unique: [][]string{{"email"}},
			},
		},
	}

	sql := GenerateSQL(tables)

	if !strings.Contains(sql, `UNIQUE ("email")`) {
		t.Error("expected unique constraint")
	}
}

func TestGenerateSQL_DefaultValue(t *testing.T) {
	tables := map[string]schema.Table{
		"config": {
			Name: "config",
			Columns: map[string]schema.Column{
				"active": {Name: "active", DataType: "boolean", Default: "true"},
			},
		},
	}

	sql := GenerateSQL(tables)

	if !strings.Contains(sql, `"active" boolean DEFAULT true`) {
		t.Error("expected column with default value")
	}
}

func TestGenerateSQL_TriggerFunctions(t *testing.T) {
	tables := map[string]schema.Table{
		"events": {
			Name: "events",
			Columns: map[string]schema.Column{
				"data": {Name: "data", DataType: "jsonb"},
			},
		},
	}

	sql := GenerateSQL(tables)

	if !strings.Contains(sql, "events_set_updated_at()") {
		t.Error("expected set_updated_at function")
	}
	if !strings.Contains(sql, "events_resolve_conflict()") {
		t.Error("expected resolve_conflict function")
	}
	if !strings.Contains(sql, "set_updated_at_events") {
		t.Error("expected set_updated_at trigger")
	}
	if !strings.Contains(sql, "conflict_resolution_events") {
		t.Error("expected conflict_resolution trigger")
	}
}

func TestGenerateSQL_ConflictTriggerEnableAlways(t *testing.T) {
	tables := map[string]schema.Table{
		"orders": {
			Name: "orders",
			Columns: map[string]schema.Column{
				"total": {Name: "total", DataType: "numeric"},
			},
		},
	}

	sql := GenerateSQL(tables)

	if !strings.Contains(sql, `ENABLE ALWAYS TRIGGER conflict_resolution_orders`) {
		t.Error("conflict resolution trigger must use ENABLE ALWAYS to fire during replication")
	}
	if strings.Contains(sql, `ENABLE ALWAYS TRIGGER set_updated_at_orders`) {
		t.Error("set_updated_at trigger should NOT use ENABLE ALWAYS")
	}
}

// --- CRDT / HLC Tests ---

func TestGenerateSQL_NoCRDT_NoHLCInfrastructure(t *testing.T) {
	tables := map[string]schema.Table{
		"items": {
			Name: "items",
			Columns: map[string]schema.Column{
				"name": {Name: "name", DataType: "text"},
			},
		},
	}

	sql := GenerateSQL(tables)

	if strings.Contains(sql, "_pgconverge") {
		t.Error("HLC infrastructure should NOT be generated when no table has CRDT enabled")
	}
	if strings.Contains(sql, "_hlc_ts") {
		t.Error("HLC columns should NOT be present on non-CRDT tables")
	}
}

func TestGenerateSQL_CRDT_HLCInfrastructure(t *testing.T) {
	tables := map[string]schema.Table{
		"events": {
			Name: "events",
			Columns: map[string]schema.Column{
				"data": {Name: "data", DataType: "jsonb"},
			},
			CRDT: &schema.CRDTConfig{Enabled: true},
		},
	}

	sql := GenerateSQL(tables)

	if !strings.Contains(sql, "CREATE SCHEMA IF NOT EXISTS _pgconverge") {
		t.Error("expected _pgconverge schema")
	}
	if !strings.Contains(sql, "_pgconverge.hlc_state") {
		t.Error("expected hlc_state table")
	}
	if !strings.Contains(sql, "_pgconverge.advance_hlc") {
		t.Error("expected advance_hlc function")
	}
}

func TestGenerateSQL_CRDT_HLCColumns(t *testing.T) {
	tables := map[string]schema.Table{
		"orders": {
			Name: "orders",
			Columns: map[string]schema.Column{
				"total": {Name: "total", DataType: "numeric"},
			},
			CRDT: &schema.CRDTConfig{Enabled: true},
		},
	}

	sql := GenerateSQL(tables)

	if !strings.Contains(sql, `"_hlc_ts" BIGINT DEFAULT 0`) {
		t.Error("expected _hlc_ts column")
	}
	if !strings.Contains(sql, `"_hlc_counter" INTEGER DEFAULT 0`) {
		t.Error("expected _hlc_counter column")
	}
	if !strings.Contains(sql, `"_hlc_node" VARCHAR(50) DEFAULT ''`) {
		t.Error("expected _hlc_node column")
	}
	// Should still have updated_at for readability
	if !strings.Contains(sql, `"updated_at" TIMESTAMP DEFAULT now()`) {
		t.Error("CRDT tables should still have updated_at for readability")
	}
}

func TestGenerateSQL_CRDT_HLCTriggers(t *testing.T) {
	tables := map[string]schema.Table{
		"orders": {
			Name: "orders",
			Columns: map[string]schema.Column{
				"total": {Name: "total", DataType: "numeric"},
			},
			CRDT: &schema.CRDTConfig{Enabled: true},
		},
	}

	sql := GenerateSQL(tables)

	// HLC stamping trigger (local writes only)
	if !strings.Contains(sql, "orders_stamp_hlc()") {
		t.Error("expected HLC stamping function")
	}
	if !strings.Contains(sql, "stamp_hlc_orders") {
		t.Error("expected HLC stamping trigger")
	}

	// Should NOT have the old set_updated_at trigger
	if strings.Contains(sql, "orders_set_updated_at()") {
		t.Error("CRDT tables should NOT have set_updated_at, should use stamp_hlc instead")
	}

	// Conflict resolution uses HLC comparison
	if !strings.Contains(sql, "NEW._hlc_ts") {
		t.Error("expected HLC-based conflict resolution")
	}
	if !strings.Contains(sql, "advance_hlc(NEW._hlc_ts, NEW._hlc_counter)") {
		t.Error("expected conflict trigger to advance local HLC on incoming writes")
	}

	// ENABLE ALWAYS on conflict trigger
	if !strings.Contains(sql, "ENABLE ALWAYS TRIGGER conflict_resolution_orders") {
		t.Error("conflict resolution trigger must use ENABLE ALWAYS")
	}

	// Stamping trigger should NOT be ENABLE ALWAYS
	if strings.Contains(sql, "ENABLE ALWAYS TRIGGER stamp_hlc_orders") {
		t.Error("HLC stamping trigger should NOT use ENABLE ALWAYS")
	}
}

func TestGenerateSQL_CRDT_HLCTupleComparison(t *testing.T) {
	tables := map[string]schema.Table{
		"data": {
			Name: "data",
			Columns: map[string]schema.Column{
				"value": {Name: "value", DataType: "text"},
			},
			CRDT: &schema.CRDTConfig{Enabled: true},
		},
	}

	sql := GenerateSQL(tables)

	// Must compare full HLC tuple (ts, counter, node) for total ordering
	if !strings.Contains(sql, "(NEW._hlc_ts, NEW._hlc_counter, NEW._hlc_node) > (OLD._hlc_ts, OLD._hlc_counter, OLD._hlc_node)") {
		t.Error("expected HLC tuple comparison for total ordering")
	}

	// Should NOT use timestamp-based comparison
	if strings.Contains(sql, "NEW.updated_at > OLD.updated_at") {
		t.Error("CRDT tables should NOT use timestamp-based conflict resolution")
	}
}

func TestGenerateSQL_MixedCRDTAndNonCRDT(t *testing.T) {
	tables := map[string]schema.Table{
		"crdt_table": {
			Name: "crdt_table",
			Columns: map[string]schema.Column{
				"data": {Name: "data", DataType: "text"},
			},
			CRDT: &schema.CRDTConfig{Enabled: true},
		},
		"normal_table": {
			Name: "normal_table",
			Columns: map[string]schema.Column{
				"info": {Name: "info", DataType: "text"},
			},
		},
	}

	sql := GenerateSQL(tables)

	// HLC infrastructure should be present (at least one CRDT table)
	if !strings.Contains(sql, "_pgconverge.hlc_state") {
		t.Error("HLC infrastructure should be present when at least one table uses CRDT")
	}

	// CRDT table uses HLC triggers
	if !strings.Contains(sql, "crdt_table_stamp_hlc") {
		t.Error("CRDT table should have HLC stamping trigger")
	}

	// Normal table uses LWW triggers
	if !strings.Contains(sql, "normal_table_set_updated_at") {
		t.Error("non-CRDT table should have set_updated_at trigger")
	}
}

func TestAnyCRDTEnabled(t *testing.T) {
	noCRDT := map[string]schema.Table{
		"a": {Name: "a"},
		"b": {Name: "b"},
	}
	if anyCRDTEnabled(noCRDT) {
		t.Error("expected false when no tables have CRDT")
	}

	withCRDT := map[string]schema.Table{
		"a": {Name: "a"},
		"b": {Name: "b", CRDT: &schema.CRDTConfig{Enabled: true}},
	}
	if !anyCRDTEnabled(withCRDT) {
		t.Error("expected true when at least one table has CRDT")
	}

	disabledCRDT := map[string]schema.Table{
		"a": {Name: "a", CRDT: &schema.CRDTConfig{Enabled: false}},
	}
	if anyCRDTEnabled(disabledCRDT) {
		t.Error("expected false when CRDT is present but disabled")
	}
}
