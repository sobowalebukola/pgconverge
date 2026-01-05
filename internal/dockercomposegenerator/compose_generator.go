package dockercomposegenerator

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	schema "pgconverge/internal"

	"gopkg.in/yaml.v3"
)

func ComposeGenerator(filePath, outFile string) {
	nodesBytes, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal(err)
	}
	var nodes []schema.Node
	if err := json.Unmarshal(nodesBytes, &nodes); err != nil {
		log.Fatal(err)
	}

	compose := GenerateComposeMap(nodes)

	yamlData, _ := yaml.Marshal(compose)
	if err := os.WriteFile(outFile, yamlData, 0644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("docker-compose.yml generated in", outFile)
}
