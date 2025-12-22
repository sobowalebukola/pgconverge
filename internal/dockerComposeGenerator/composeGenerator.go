package dockercomposegenerator

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	schema "pgconverge/internal"
	helper "pgconverge/internal/util"

	"gopkg.in/yaml.v3"
)

func ComposeGenerator() {

	// --- Load nodes ---
	nodesBytes, err := os.ReadFile("nodes.json")
	if err != nil {
		log.Fatal(err)
	}
	var nodes []schema.Node
	if err := json.Unmarshal(nodesBytes, &nodes); err != nil {
		log.Fatal(err)
	}

	compose := map[string]interface{}{
		"version":  "3.9",
		"services": map[string]interface{}{},
		"volumes":  map[string]interface{}{},
	}
	services := compose["services"].(map[string]interface{})
	volumes := compose["volumes"].(map[string]interface{})

	for _, node := range nodes {
		services[node.Name] = map[string]interface{}{
			"image":          "postgres:16",
			"container_name": node.Name,
			"environment": map[string]string{
				"POSTGRES_USER":     node.User,
				"POSTGRES_PASSWORD": node.Password,
				"POSTGRES_DB":       node.Database,
			},
			"ports": []string{fmt.Sprintf("%d:5432", helper.GetPort(node.Name))},
			"volumes": []string{
				fmt.Sprintf("%s_data:/var/lib/postgresql/data", node.Name),
				"./:/scripts",
			},
			"entrypoint": []string{"/scripts/entrypoint.sh", node.Name},
		}
		volumes[node.Name+"_data"] = map[string]interface{}{}
	}

	yamlData, _ := yaml.Marshal(compose)
	if err := os.WriteFile("docker-compose.yml", yamlData, 0644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("docker-compose.yml generated")
}
