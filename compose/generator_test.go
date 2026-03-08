package compose

import (
	"strings"
	"testing"

	"github.com/sobowalebukola/pgconverge/schema"
	"github.com/sobowalebukola/pgconverge/util"
)

func TestGenerateComposeMap(t *testing.T) {
	util.ResetPorts()
	nodes := []schema.Node{
		{
			Name:     "db1",
			Host:     "192.168.1.10",
			User:     "user1",
			Password: "pass1",
			Database: "db1",
		},
		{
			Name:     "db2",
			Host:     "192.168.1.11",
			User:     "user2",
			Password: "pass2",
			Database: "db2",
		},
	}

	compose := GenerateComposeMap(nodes)

	services := compose["services"].(map[string]interface{})
	volumes := compose["volumes"].(map[string]interface{})

	if _, ok := services["db1"]; !ok {
		t.Fatal("service db1 missing")
	}
	if _, ok := services["db2"]; !ok {
		t.Fatal("service db2 missing")
	}

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

	if _, ok := volumes["db1_data"]; !ok {
		t.Fatal("db1_data volume missing")
	}
	if _, ok := volumes["db2_data"]; !ok {
		t.Fatal("db2_data volume missing")
	}
}

func TestGenerateComposeMap_ExternalNodeSkipped(t *testing.T) {
	util.ResetPorts()
	nodes := []schema.Node{
		{
			Name:     "docker_node",
			Host:     "docker_node",
			User:     "postgres",
			Password: "postgres",
			Database: "db",
		},
		{
			Name:     "external_node",
			Host:     "db.example.com",
			Port:     5432,
			User:     "postgres",
			Password: "postgres",
			Database: "db",
			External: true,
		},
	}

	compose := GenerateComposeMap(nodes)
	services := compose["services"].(map[string]interface{})
	volumes := compose["volumes"].(map[string]interface{})

	if len(services) != 1 {
		t.Errorf("expected 1 service, got %d", len(services))
	}

	if _, ok := services["docker_node"]; !ok {
		t.Error("docker_node should be in services")
	}

	if _, ok := services["external_node"]; ok {
		t.Error("external_node should NOT be in services")
	}

	if len(volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(volumes))
	}
}

func TestGenerateComposeMap_ExtraHostsForExternal(t *testing.T) {
	util.ResetPorts()
	nodes := []schema.Node{
		{
			Name:     "docker_node",
			Host:     "docker_node",
			User:     "user1",
			Password: "pass1",
			Database: "db1",
		},
		{
			Name:     "external_db",
			Host:     "10.0.0.2",
			User:     "user2",
			Password: "pass2",
			Database: "db2",
			External: true,
		},
	}

	compose := GenerateComposeMap(nodes)
	services := compose["services"].(map[string]interface{})

	dockerService := services["docker_node"].(map[string]interface{})
	extraHosts := dockerService["extra_hosts"].([]string)

	if len(extraHosts) != 1 {
		t.Errorf("expected 1 extra_hosts entry, got %d", len(extraHosts))
	}

	if extraHosts[0] != "external_db:10.0.0.2" {
		t.Errorf("expected extra_host 'external_db:10.0.0.2', got %s", extraHosts[0])
	}
}

func TestGenerateComposeMap_NoExtraHostsForDockerOnly(t *testing.T) {
	util.ResetPorts()
	nodes := []schema.Node{
		{
			Name:     "node1",
			Host:     "node1",
			User:     "user1",
			Password: "pass1",
			Database: "db1",
		},
		{
			Name:     "node2",
			Host:     "node2",
			User:     "user2",
			Password: "pass2",
			Database: "db2",
		},
	}

	compose := GenerateComposeMap(nodes)
	services := compose["services"].(map[string]interface{})

	node1Service := services["node1"].(map[string]interface{})
	if _, ok := node1Service["extra_hosts"]; ok {
		t.Error("extra_hosts should not be present when there are no external nodes")
	}
}

func TestGenerateComposeMap_PortMapping(t *testing.T) {
	util.ResetPorts()
	nodes := []schema.Node{
		{
			Name:     "port_test_node",
			Host:     "localhost",
			User:     "postgres",
			Password: "postgres",
			Database: "mydb",
		},
	}

	compose := GenerateComposeMap(nodes)
	services := compose["services"].(map[string]interface{})
	service := services["port_test_node"].(map[string]interface{})
	ports := service["ports"].([]string)

	if len(ports) != 1 {
		t.Errorf("expected 1 port mapping, got %d", len(ports))
	}

	if !strings.HasSuffix(ports[0], ":5432") {
		t.Errorf("expected port mapping to end with :5432, got %s", ports[0])
	}
}

func TestGenerateComposeMap_Entrypoint(t *testing.T) {
	util.ResetPorts()
	nodes := []schema.Node{
		{
			Name:     "mynode",
			Host:     "localhost",
			User:     "postgres",
			Password: "postgres",
			Database: "mydb",
		},
	}

	compose := GenerateComposeMap(nodes)
	services := compose["services"].(map[string]interface{})
	service := services["mynode"].(map[string]interface{})
	entrypoint := service["entrypoint"].([]string)

	if len(entrypoint) != 2 {
		t.Errorf("expected 2 entrypoint args, got %d", len(entrypoint))
	}

	if entrypoint[0] != "/scripts/entrypoint.sh" {
		t.Errorf("expected entrypoint script '/scripts/entrypoint.sh', got %s", entrypoint[0])
	}

	if entrypoint[1] != "mynode" {
		t.Errorf("expected node name 'mynode' as second arg, got %s", entrypoint[1])
	}
}
