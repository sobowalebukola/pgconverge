package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var applySchemaNode string

func init() {
	applySchemaCmd.Flags().StringVar(&applySchemaNode, "node", "", "Apply schema to specific node only")
	rootCmd.AddCommand(applySchemaCmd)
}

var applySchemaCmd = &cobra.Command{
	Use:   "apply-schema",
	Short: "Apply schema to nodes",
	Long:  `Apply the generated SQL schema to all nodes or a specific node.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := NewDBManager()
		if err != nil {
			return err
		}
		defer manager.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if applySchemaNode != "" {
			// Apply to specific node
			node := manager.GetNode(applySchemaNode)
			if node == nil {
				return fmt.Errorf("node %s not found", applySchemaNode)
			}

			fmt.Printf("Applying schema to %s...\n", node.Name)
			if err := manager.ApplySchemaFromFile(ctx, node, SchemaFile); err != nil {
				return fmt.Errorf("failed to apply schema to %s: %w", node.Name, err)
			}
			fmt.Printf("Schema applied successfully to %s\n", node.Name)
		} else {
			// Apply to all nodes
			fmt.Println("Applying schema to all nodes...")
			errors := manager.ApplySchemaFromFileToAll(ctx, SchemaFile)

			hasErrors := false
			for nodeName, err := range errors {
				if err != nil {
					hasErrors = true
					fmt.Printf("  %s: FAILED - %v\n", nodeName, err)
				}
			}

			if hasErrors {
				return fmt.Errorf("some nodes failed to apply schema")
			}

			for _, node := range manager.GetNodes() {
				if errors[node.Name] == nil {
					fmt.Printf("  %s: OK\n", node.Name)
				}
			}
			fmt.Println("Schema applied successfully to all nodes")
		}

		return nil
	},
}
