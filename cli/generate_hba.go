package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var authMethod string

func init() {
	generateHBACmd.Flags().StringVar(&authMethod, "auth-method", "scram-sha-256", "Authentication method (scram-sha-256, md5)")
	rootCmd.AddCommand(generateHBACmd)
}

var generateHBACmd = &cobra.Command{
	Use:   "generate-hba",
	Short: "Generate pg_hba.conf entries for all nodes",
	Long: `Generate the pg_hba.conf entries needed for each node to accept
replication connections from all other nodes in the cluster.

The output shows which lines to add to each node's pg_hba.conf file.
After applying, reload PostgreSQL with: SELECT pg_reload_conf();`,
	RunE: func(cmd *cobra.Command, args []string) error {
		nodes, err := LoadNodes()
		if err != nil {
			return fmt.Errorf("failed to load nodes: %w", err)
		}

		if len(nodes) < 2 {
			return fmt.Errorf("need at least 2 nodes, found %d", len(nodes))
		}

		for i, node := range nodes {
			fmt.Printf("# --- %s (%s) — add to pg_hba.conf ---\n", node.Name, node.Host)
			fmt.Println("# TYPE  DATABASE        USER            ADDRESS                 METHOD")

			for j, peer := range nodes {
				if i == j {
					continue
				}
				fmt.Printf("host    %-15s %-15s %-23s %s\n",
					node.Database, node.User, peer.Host+"/32", authMethod)
			}
			fmt.Println()
		}

		fmt.Println("# After applying, reload PostgreSQL on each node:")
		fmt.Println("#   psql -c \"SELECT pg_reload_conf();\"")

		return nil
	},
}
