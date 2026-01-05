package main

import (
	"flag"
	"pgconverge/internal/dockercomposegenerator"
	"pgconverge/internal/schemagenerator"
)

func main() {
	schemaFile := flag.String("schema", "schema.json", "Path to schema JSON")
	nodesFile := flag.String("nodes", "nodes.json", "Path to nodes JSON")
	sqlOut := flag.String("out-sql", "generated.sql", "Output SQL file")
	composeOut := flag.String("out-compose", "docker-compose.yml", "Output Docker Compose file")
	onlySQL := flag.Bool("sql-only", false, "Generate only SQL")
	onlyCompose := flag.Bool("compose-only", false, "Generate only Docker Compose")
	flag.Parse()

	if !*onlyCompose {
		schemagenerator.SchemaGenerator(*schemaFile, *sqlOut)
	}

	if !*onlySQL {
		dockercomposegenerator.ComposeGenerator(*nodesFile, *composeOut)
	}
}
