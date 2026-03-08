package cli

import (
	"fmt"

	"github.com/sobowalebukola/pgconverge/compose"
	"github.com/sobowalebukola/pgconverge/sqlgen"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(generateCmd)
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate SQL schema and docker-compose.yml",
	Long:  `Generate the SQL schema file from schema.json and docker-compose.yml from nodes.json.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Generate SQL from JSON schema
		if err := sqlgen.Generate("schema.json", SchemaFile); err != nil {
			return fmt.Errorf("failed to generate SQL: %w", err)
		}
		fmt.Printf("SQL generated in %s\n", SchemaFile)

		// Generate Docker Compose yml file from available nodes
		if err := compose.Generate(NodesFile, "docker-compose.yml"); err != nil {
			return fmt.Errorf("failed to generate docker-compose.yml: %w", err)
		}
		fmt.Println("docker-compose.yml generated")

		return nil
	},
}
