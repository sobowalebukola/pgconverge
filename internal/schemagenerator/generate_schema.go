package schemagenerator

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	schema "pgconverge/internal"
)

func SchemaGenerator() {
	schemaBytes, err := os.ReadFile("schema.json")
	if err != nil {
		log.Fatal(err)
	}

	var tables map[string]schema.Table
	if err := json.Unmarshal(schemaBytes, &tables); err != nil {
		log.Fatal(err)
	}

	sql := GenerateSQL(tables)

	if err := os.WriteFile("generated.sql", []byte(sql), 0644); err != nil {
		log.Fatal(err)
	}

	fmt.Println("SQL generated in generated.sql")
}
