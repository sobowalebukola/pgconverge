// Package cli provides the command-line interface for pgconverge.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sobowalebukola/pgconverge/db"
	"github.com/sobowalebukola/pgconverge/schema"
	"github.com/spf13/cobra"
)

var (
	// NodesFile is the path to the nodes configuration file.
	NodesFile string
	// SchemaFile is the path to the schema SQL file.
	SchemaFile string

	rootCmd = &cobra.Command{
		Use:   "pgconverge",
		Short: "PostgreSQL multi-master replication tool",
		Long: `pgconverge is a tool for setting up and managing bidirectional
PostgreSQL logical replication across multiple nodes.`,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&NodesFile, "nodes", "n", "nodes.json", "Path to nodes configuration file")
	rootCmd.PersistentFlags().StringVarP(&SchemaFile, "schema", "s", "generated.sql", "Path to schema SQL file")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// LoadNodes loads nodes from the configured nodes file.
func LoadNodes() ([]schema.Node, error) {
	return LoadNodesFromFile(NodesFile)
}

// LoadNodesFromFile loads nodes from a specific file.
func LoadNodesFromFile(path string) ([]schema.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read nodes file: %w", err)
	}

	var nodes []schema.Node
	if err := json.Unmarshal(data, &nodes); err != nil {
		return nil, fmt.Errorf("failed to parse nodes file: %w", err)
	}

	return nodes, nil
}

// NewDBManager creates a new DBManager from the configured nodes file.
func NewDBManager() (*db.DBManager, error) {
	nodes, err := LoadNodes()
	if err != nil {
		return nil, err
	}
	return db.NewDBManager(nodes), nil
}
