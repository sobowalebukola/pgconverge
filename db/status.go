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
	err = pool.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err == nil {
		status.Version = version
	}

	// Get wal_level
	var walLevel string
	err = pool.QueryRow(ctx, "SHOW wal_level").Scan(&walLevel)
	if err == nil {
		status.WalLevel = walLevel
	}

	// Get publications
	rows, err := pool.Query(ctx, "SELECT pubname, puballtables FROM pg_publication")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pub PublicationInfo
			if err := rows.Scan(&pub.Name, &pub.AllTables); err == nil {
				status.Publications = append(status.Publications, pub)
			}
		}
	}

	// Get subscriptions
	rows, err = pool.Query(ctx, "SELECT subname, subenabled, subslotname FROM pg_subscription")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var sub SubscriptionInfo
			if err := rows.Scan(&sub.Name, &sub.Enabled, &sub.SlotName); err == nil {
				status.Subscriptions = append(status.Subscriptions, sub)
			}
		}
	}

	// Get replication slots
	rows, err = pool.Query(ctx, "SELECT slot_name, slot_type, active FROM pg_replication_slots")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var slot ReplicationSlotInfo
			if err := rows.Scan(&slot.Name, &slot.SlotType, &slot.Active); err == nil {
				status.ReplicationSlots = append(status.ReplicationSlots, slot)
			}
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
