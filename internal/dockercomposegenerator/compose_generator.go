package dockercomposegenerator

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	schema "pgconverge/internal"

	"gopkg.in/yaml.v3"
)

func ComposeGenerator() {
	nodesBytes, err := os.ReadFile("nodes.json")
	if err != nil {
		log.Fatal(err)
	}
	var nodes []schema.Node
	if err := json.Unmarshal(nodesBytes, &nodes); err != nil {
		log.Fatal(err)
	}

	compose := GenerateComposeMap(nodes)

	yamlData, _ := yaml.Marshal(compose)
	if err := os.WriteFile("docker-compose.yml", yamlData, 0644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("docker-compose.yml generated")
}
