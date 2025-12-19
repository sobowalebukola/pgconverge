package main

import (
	"fmt"
	compose "jsonddl/internal/dockerComposeGenerator"
	generator "jsonddl/internal/sqlGenerator"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <current_node_name>")
		os.Exit(1)
	}
	// Generate SQL from JSON schema
	generator.SchemaGenerator()

	// Generate Docker Compose yml file from available nodes
	compose.ComposeGenerator()
}
