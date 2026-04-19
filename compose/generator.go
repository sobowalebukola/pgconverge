// Package compose provides Docker Compose file generation for PostgreSQL nodes.
package compose

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sobowalebukola/pgconverge/schema"
	"github.com/sobowalebukola/pgconverge/util"
	"gopkg.in/yaml.v3"
)

var EntrypointScript []byte

// GenerateComposeMap generates a Docker Compose configuration map from nodes.
func GenerateComposeMap(nodes []schema.Node) map[string]interface{} {
	compose := map[string]interface{}{
		"services": map[string]interface{}{},
		"volumes":  map[string]interface{}{},
	}
	services := compose["services"].(map[string]interface{})
	volumes := compose["volumes"].(map[string]interface{})

	// Build extra_hosts list only for external nodes (they need IP resolution)
	// Docker containers on the same network resolve each other by container name
	extraHosts := []string{}
	for _, node := range nodes {
		if node.External {
			// External nodes need explicit host mapping
			extraHosts = append(extraHosts, fmt.Sprintf("%s:%s", node.Name, node.Host))
		}
	}

	for _, node := range nodes {
		// Skip external nodes - they're not managed by Docker
		if node.External {
			continue
		}

		// Determine the port mapping
		hostPort := util.GetPort(node.Name)
		containerPort := 5432
		if node.Port != 0 {
			containerPort = node.Port
		}

		service := map[string]interface{}{
			"image":          "postgres:16",
			"container_name": node.Name,
			"environment": map[string]string{
				"POSTGRES_USER":     node.User,
				"POSTGRES_PASSWORD": node.Password,
				"POSTGRES_DB":       node.Database,
			},
			"ports": []string{fmt.Sprintf("%d:%d", hostPort, containerPort)},
			"volumes": []string{
				fmt.Sprintf("%s_data:/var/lib/postgresql/data", node.Name),
				"./:/scripts",
			},
			"entrypoint": []string{"/scripts/entrypoint.sh", node.Name},
		}

		// Only add extra_hosts if there are external nodes
		if len(extraHosts) > 0 {
			service["extra_hosts"] = extraHosts
		}

		services[node.Name] = service
		volumes[node.Name+"_data"] = map[string]interface{}{}
	}

	return compose
}

// Generate reads nodes from a file and writes docker-compose.yml.
func Generate(nodesFile, outputFile string) error {
	nodesBytes, err := os.ReadFile(nodesFile)
	if err != nil {
		return fmt.Errorf("failed to read nodes file: %w", err)
	}

	var nodes []schema.Node
	if err := json.Unmarshal(nodesBytes, &nodes); err != nil {
		return fmt.Errorf("failed to parse nodes file: %w", err)
	}

	compose := GenerateComposeMap(nodes)

	yamlData, err := yaml.Marshal(compose)
	if err != nil {
		return fmt.Errorf("failed to marshal compose: %w", err)
	}

	if err := os.WriteFile(outputFile, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write compose file: %w", err)
	}

	return nil
}
