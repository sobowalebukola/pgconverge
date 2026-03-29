package db

import (
	"context"
	"fmt"

	"github.com/sobowalebukola/pgconverge/schema"
)

// NodeStatus represents the status of a PostgreSQL node.
type NodeStatus struct {
	Name             string
	Host             string
	Port             int
	External         bool
	Reachable        bool
	Error            error
	Warnings         []string
	Version          string
	WalLevel         string
	Publications     []PublicationInfo
	Subscriptions    []SubscriptionInfo
	ReplicationSlots []ReplicationSlotInfo
}

// PublicationInfo represents a PostgreSQL publication.
type PublicationInfo struct {
	Name      string
	AllTables bool
}

// SubscriptionInfo represents a PostgreSQL subscription.
type SubscriptionInfo struct {
	Name     string
	Enabled  bool
	SlotName string
}

// ReplicationSlotInfo represents a PostgreSQL replication slot.
type ReplicationSlotInfo struct {
	Name     string
	SlotType string
	Active   bool
}

// GetNodeStatus returns the status of a specific node.
func (m *DBManager) GetNodeStatus(ctx context.Context, node *schema.Node) NodeStatus {
	status := NodeStatus{
		Name:     node.Name,
		Host:     node.Host,
		Port:     node.GetPort(),
		External: node.External,
	}

	pool, err := m.Connect(ctx, node)
	if err != nil {
		status.Reachable = false
		status.Error = err
		return status
	}

	// Check connectivity
	if err := pool.Ping(ctx); err != nil {
		status.Reachable = false
		status.Error = err
		return status
	}
	status.Reachable = true

	// Get PostgreSQL version
	var version string
	if err := pool.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		status.Warnings = append(status.Warnings, fmt.Sprintf("failed to query version: %v", err))
	} else {
		status.Version = version
	}

	// Get wal_level
	var walLevel string
	if err := pool.QueryRow(ctx, "SHOW wal_level").Scan(&walLevel); err != nil {
		status.Warnings = append(status.Warnings, fmt.Sprintf("failed to query wal_level: %v", err))
	} else {
		status.WalLevel = walLevel
	}

	// Get publications
	pubRows, err := pool.Query(ctx, "SELECT pubname, puballtables FROM pg_publication")
	if err != nil {
		status.Warnings = append(status.Warnings, fmt.Sprintf("failed to query publications: %v", err))
	} else {
		defer pubRows.Close()
		for pubRows.Next() {
			var pub PublicationInfo
			if err := pubRows.Scan(&pub.Name, &pub.AllTables); err != nil {
				status.Warnings = append(status.Warnings, fmt.Sprintf("failed to scan publication: %v", err))
				continue
			}
			status.Publications = append(status.Publications, pub)
		}
		if err := pubRows.Err(); err != nil {
			status.Warnings = append(status.Warnings, fmt.Sprintf("error iterating publications: %v", err))
		}
	}

	// Get subscriptions
	subRows, err := pool.Query(ctx, "SELECT subname, subenabled, subslotname FROM pg_subscription")
	if err != nil {
		status.Warnings = append(status.Warnings, fmt.Sprintf("failed to query subscriptions: %v", err))
	} else {
		defer subRows.Close()
		for subRows.Next() {
			var sub SubscriptionInfo
			if err := subRows.Scan(&sub.Name, &sub.Enabled, &sub.SlotName); err != nil {
				status.Warnings = append(status.Warnings, fmt.Sprintf("failed to scan subscription: %v", err))
				continue
			}
			status.Subscriptions = append(status.Subscriptions, sub)
		}
		if err := subRows.Err(); err != nil {
			status.Warnings = append(status.Warnings, fmt.Sprintf("error iterating subscriptions: %v", err))
		}
	}

	// Get replication slots
	slotRows, err := pool.Query(ctx, "SELECT slot_name, slot_type, active FROM pg_replication_slots")
	if err != nil {
		status.Warnings = append(status.Warnings, fmt.Sprintf("failed to query replication slots: %v", err))
	} else {
		defer slotRows.Close()
		for slotRows.Next() {
			var slot ReplicationSlotInfo
			if err := slotRows.Scan(&slot.Name, &slot.SlotType, &slot.Active); err != nil {
				status.Warnings = append(status.Warnings, fmt.Sprintf("failed to scan replication slot: %v", err))
				continue
			}
			status.ReplicationSlots = append(status.ReplicationSlots, slot)
		}
		if err := slotRows.Err(); err != nil {
			status.Warnings = append(status.Warnings, fmt.Sprintf("error iterating replication slots: %v", err))
		}
	}

	return status
}

// GetAllNodeStatuses returns the status of all nodes.
func (m *DBManager) GetAllNodeStatuses(ctx context.Context) []NodeStatus {
	var statuses []NodeStatus
	for i := range m.nodes {
		status := m.GetNodeStatus(ctx, &m.nodes[i])
		statuses = append(statuses, status)
	}
	return statuses
}

// CheckReplicationHealth checks the health of replication across all nodes.
func (m *DBManager) CheckReplicationHealth(ctx context.Context) (healthy bool, issues []string) {
	healthy = true
	statuses := m.GetAllNodeStatuses(ctx)

	for _, status := range statuses {
		if !status.Reachable {
			healthy = false
			issues = append(issues, fmt.Sprintf("Node %s is not reachable: %v", status.Name, status.Error))
			continue
		}

		if len(status.Warnings) > 0 {
			for _, w := range status.Warnings {
				issues = append(issues, fmt.Sprintf("Node %s: %s", status.Name, w))
			}
		}

		if status.WalLevel != "logical" {
			healthy = false
			issues = append(issues, fmt.Sprintf("Node %s has wal_level=%s (should be 'logical')", status.Name, status.WalLevel))
		}

		// Check if node has a publication
		hasPublication := false
		expectedPubName := fmt.Sprintf("pub_%s", status.Name)
		for _, pub := range status.Publications {
			if pub.Name == expectedPubName {
				hasPublication = true
				break
			}
		}
		if !hasPublication {
			issues = append(issues, fmt.Sprintf("Node %s is missing publication %s", status.Name, expectedPubName))
		}

		// Check if node has subscriptions to all other nodes
		expectedSubCount := len(m.nodes) - 1
		if len(status.Subscriptions) < expectedSubCount {
			issues = append(issues, fmt.Sprintf("Node %s has %d subscriptions (expected %d)", status.Name, len(status.Subscriptions), expectedSubCount))
		}
	}

	return healthy, issues
}
