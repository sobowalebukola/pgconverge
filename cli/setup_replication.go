package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var setupReplicationNode string

func init() {
	setupReplicationCmd.Flags().StringVar(&setupReplicationNode, "node", "", "Set up replication for specific node only")
	rootCmd.AddCommand(setupReplicationCmd)
}

var setupReplicationCmd = &cobra.Command{
	Use:   "setup-replication",
	Short: "Set up bidirectional replication",
	Long:  `Set up bidirectional logical replication between all nodes or for a specific node.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := NewDBManager()
		if err != nil {
			return err
		}
		defer manager.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		var results []struct {
			NodeName string
			Success  bool
			Message  string
		}

		if setupReplicationNode != "" {
			// Set up replication for specific node
			fmt.Printf("Setting up replication for %s...\n", setupReplicationNode)
			rawResults := manager.SetupReplicationForNode(ctx, setupReplicationNode)
			for _, r := range rawResults {
				results = append(results, struct {
					NodeName string
					Success  bool
					Message  string
				}{r.NodeName, r.Success, r.Message})
			}
		} else {
			// Set up replication for all nodes
			fmt.Println("Setting up bidirectional replication between all nodes...")
			rawResults := manager.SetupBidirectionalReplication(ctx)
			for _, r := range rawResults {
				results = append(results, struct {
					NodeName string
					Success  bool
					Message  string
				}{r.NodeName, r.Success, r.Message})
			}
		}

		// Display results
		fmt.Println("\nResults:")
		fmt.Println("========")
		hasErrors := false
		for _, result := range results {
			status := "OK"
			if !result.Success {
				status = "FAILED"
				hasErrors = true
			}
			fmt.Printf("  [%s] %s: %s\n", status, result.NodeName, result.Message)
		}

		if hasErrors {
			return fmt.Errorf("some replication setup operations failed")
		}

		fmt.Println("\nReplication setup completed successfully!")
		return nil
	},
}
