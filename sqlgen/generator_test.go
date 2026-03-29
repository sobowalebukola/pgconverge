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
	// set_updated_at should NOT be ENABLE ALWAYS (must not re-stamp replicated rows)
	if strings.Contains(sql, `ENABLE ALWAYS TRIGGER set_updated_at_orders`) {
		t.Error("set_updated_at trigger should NOT use ENABLE ALWAYS")
	}
}
