package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sobowalebukola/pgconverge/compose"
	"github.com/sobowalebukola/pgconverge/sqlgen"
	"github.com/spf13/cobra"
)

var (
	generateSchemaIn string
	generateSQLOut   string
)

func init() {
	generateCmd.Flags().StringVarP(&generateSchemaIn, "schema", "s", "schema.json", "Path to input schema JSON file")
	generateCmd.Flags().StringVarP(&generateSQLOut, "out", "o", "generated.sql", "Path to output SQL file")
	rootCmd.AddCommand(generateCmd)
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate SQL schema and docker-compose.yml",
	Long:  `Generate the SQL schema file from schema.json and docker-compose.yml from nodes.json.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := assertDistinctPaths(generateSchemaIn, generateSQLOut); err != nil {
			return err
		}

		if err := sqlgen.Generate(generateSchemaIn, generateSQLOut); err != nil {
			return fmt.Errorf("failed to generate SQL: %w", err)
		}
		fmt.Printf("SQL generated in %s\n", generateSQLOut)

		if err := compose.Generate(NodesFile, "docker-compose.yml"); err != nil {
			return fmt.Errorf("failed to generate docker-compose.yml: %w", err)
		}
		fmt.Println("docker-compose.yml generated")

		if err := os.WriteFile("entrypoint.sh", compose.EntrypointScript, 0755); err != nil {
			return fmt.Errorf("failed to write entrypoint.sh: %w", err)
		}
		fmt.Println("entrypoint.sh generated")

		return nil
	},
}

// assertDistinctPaths refuses to run when the input and output resolve to the
// same file — writing SQL over the JSON input destroys the source of truth.
func assertDistinctPaths(in, out string) error {
	absIn, err := filepath.Abs(in)
	if err != nil {
		return fmt.Errorf("failed to resolve --schema path: %w", err)
	}
	absOut, err := filepath.Abs(out)
	if err != nil {
		return fmt.Errorf("failed to resolve --out path: %w", err)
	}
	if absIn == absOut {
		return fmt.Errorf("--schema and --out must point to different files (both resolve to %s)", absIn)
	}
	return nil
}
