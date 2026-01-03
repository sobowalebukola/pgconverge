package dockercomposegenerator

import (
	"testing"

	schema "pgconverge/internal"
)

func TestGenerateComposeMap(t *testing.T) {
	nodes := []schema.Node{
		{
			Name:     "db1",
			User:     "user1",
			Password: "pass1",
			Database: "db1",
		},
		{
			Name:     "db2",
			User:     "user2",
			Password: "pass2",
			Database: "db2",
		},
	}

	compose := GenerateComposeMap(nodes)

	services := compose["services"].(map[string]interface{})
	volumes := compose["volumes"].(map[string]interface{})

	// Check services exist
	if _, ok := services["db1"]; !ok {
		t.Fatal("service db1 missing")
	}
	if _, ok := services["db2"]; !ok {
		t.Fatal("service db2 missing")
	}

	// Check environment for db1
	env := services["db1"].(map[string]interface{})["environment"].(map[string]string)
	if env["POSTGRES_USER"] != "user1" {
		t.Fatal("db1 user incorrect")
	}
	if env["POSTGRES_PASSWORD"] != "pass1" {
		t.Fatal("db1 password incorrect")
	}
	if env["POSTGRES_DB"] != "db1" {
		t.Fatal("db1 database incorrect")
	}

	// Check volumes
	if _, ok := volumes["db1_data"]; !ok {
		t.Fatal("db1_data volume missing")
	}
	if _, ok := volumes["db2_data"]; !ok {
		t.Fatal("db2_data volume missing")
	}
}
