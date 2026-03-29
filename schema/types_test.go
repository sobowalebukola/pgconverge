package schema

import (
	"os"
	"testing"
)

func TestGetPort_Default(t *testing.T) {
	n := Node{Name: "test", Port: 0}
	if got := n.GetPort(); got != 5432 {
		t.Errorf("GetPort() = %d, want 5432", got)
	}
}

func TestGetPort_Custom(t *testing.T) {
	n := Node{Name: "test", Port: 5555}
	if got := n.GetPort(); got != 5555 {
		t.Errorf("GetPort() = %d, want 5555", got)
	}
}

func TestConnectionString(t *testing.T) {
	n := Node{
		Name:     "node_a",
		Host:     "localhost",
		Port:     5433,
		User:     "postgres",
		Database: "mydb",
		Password: "secret",
	}
	got := n.ConnectionString()
	expected := "host=localhost port=5433 dbname=mydb user=postgres password=secret sslmode=disable"
	if got != expected {
		t.Errorf("ConnectionString() = %q, want %q", got, expected)
	}
}

func TestConnectionString_DefaultPort(t *testing.T) {
	n := Node{
		Name:     "node_a",
		Host:     "db.example.com",
		User:     "admin",
		Database: "store",
		Password: "pass",
	}
	got := n.ConnectionString()
	expected := "host=db.example.com port=5432 dbname=store user=admin password=pass sslmode=disable"
	if got != expected {
		t.Errorf("ConnectionString() = %q, want %q", got, expected)
	}
}

func TestResolvePassword_FromEnv(t *testing.T) {
	n := Node{Name: "node_a", Password: "json_pass"}

	os.Setenv("PGCONVERGE_NODE_A_PASSWORD", "env_pass")
	defer os.Unsetenv("PGCONVERGE_NODE_A_PASSWORD")

	got := n.ResolvePassword()
	if got != "env_pass" {
		t.Errorf("ResolvePassword() = %q, want %q", got, "env_pass")
	}
}

func TestResolvePassword_FallbackToJSON(t *testing.T) {
	n := Node{Name: "node_b", Password: "json_pass"}

	// Ensure env var is not set
	os.Unsetenv("PGCONVERGE_NODE_B_PASSWORD")

	got := n.ResolvePassword()
	if got != "json_pass" {
		t.Errorf("ResolvePassword() = %q, want %q", got, "json_pass")
	}
}

func TestConnectionString_UsesEnvPassword(t *testing.T) {
	n := Node{
		Name:     "node_c",
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Database: "db",
		Password: "old_pass",
	}

	os.Setenv("PGCONVERGE_NODE_C_PASSWORD", "env_secret")
	defer os.Unsetenv("PGCONVERGE_NODE_C_PASSWORD")

	got := n.ConnectionString()
	expected := "host=localhost port=5432 dbname=db user=postgres password=env_secret sslmode=disable"
	if got != expected {
		t.Errorf("ConnectionString() = %q, want %q", got, expected)
	}
}
