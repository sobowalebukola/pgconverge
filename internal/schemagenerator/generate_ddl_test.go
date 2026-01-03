package schemagenerator

import (
	"strings"
	"testing"

	schema "pgconverge/internal"
)

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected SQL to contain:\n%s\n\nbut got:\n%s", want, got)
	}
}

func TestGenerateSQL_BasicTable(t *testing.T) {
	tables := map[string]schema.Table{
		"users": {
			Name: "users",
			Columns: map[string]schema.Column{
				"id":    {Name: "id", DataType: "serial"},
				"email": {Name: "email", DataType: "varchar(255)"},
			},
			Constraints: schema.Constraints{
				Primary: []string{"id"},
			},
		},
	}

	sql := GenerateSQL(tables)

	assertContains(t, sql, `CREATE TABLE IF NOT EXISTS "users"`)
	assertContains(t, sql, `"id" uuid DEFAULT gen_random_uuid()`)
	assertContains(t, sql, `"email" varchar(255)`)
	assertContains(t, sql, `"updated_at" TIMESTAMP DEFAULT now()`)
	assertContains(t, sql, `REPLICA IDENTITY FULL`)
	assertContains(t, sql, `CREATE OR REPLACE FUNCTION users_resolve_conflict`)
	assertContains(t, sql, `conflict_resolution_users`)
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

	assertContains(t, sql, `"id" uuid DEFAULT gen_random_uuid()`)
}

func TestGenerateSQL_DefaultValue(t *testing.T) {
	tables := map[string]schema.Table{
		"posts": {
			Name: "posts",
			Columns: map[string]schema.Column{
				"published": {Name: "published", DataType: "boolean", Default: "false"},
			},
		},
	}

	sql := GenerateSQL(tables)

	assertContains(t, sql, `"published" boolean DEFAULT false`)
}
