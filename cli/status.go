package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check status of all nodes",
	Long:  `Check connectivity and replication status of all configured PostgreSQL nodes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := NewDBManager()
		if err != nil {
			return err
		}
		defer manager.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		statuses := manager.GetAllNodeStatuses(ctx)

		fmt.Println("Node Status:")
		fmt.Println("============")

		for _, status := range statuses {
			fmt.Printf("\n%s (%s:%d)\n", status.Name, status.Host, status.Port)
			if status.External {
				fmt.Println("  Type: External")
			} else {
				fmt.Println("  Type: Docker")
			}

			if status.Reachable {
				fmt.Println("  Status: Online")
				fmt.Printf("  WAL Level: %s\n", status.WalLevel)

				if len(status.Publications) > 0 {
					fmt.Println("  Publications:")
					for _, pub := range status.Publications {
						fmt.Printf("    - %s (all_tables: %v)\n", pub.Name, pub.AllTables)
					}
				}

				if len(status.Subscriptions) > 0 {
					fmt.Println("  Subscriptions:")
					for _, sub := range status.Subscriptions {
						fmt.Printf("    - %s (enabled: %v, slot: %s)\n", sub.Name, sub.Enabled, sub.SlotName)
					}
				}

				if len(status.ReplicationSlots) > 0 {
					fmt.Println("  Replication Slots:")
					for _, slot := range status.ReplicationSlots {
						fmt.Printf("    - %s (type: %s, active: %v)\n", slot.Name, slot.SlotType, slot.Active)
					}
				}
			} else {
				fmt.Println("  Status: Offline")
				if status.Error != nil {
					fmt.Printf("  Error: %v\n", status.Error)
				}
			}
		}

		// Check overall health
		fmt.Println("\n\nReplication Health:")
		fmt.Println("===================")
		healthy, issues := manager.CheckReplicationHealth(ctx)
		if healthy {
			fmt.Println("All nodes are healthy and properly configured.")
		} else {
			fmt.Println("Issues detected:")
			for _, issue := range issues {
				fmt.Printf("  - %s\n", issue)
			}
		}

		return nil
	},
}
