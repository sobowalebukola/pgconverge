package db

import (
	"context"
	"fmt"

	"github.com/sobowalebukola/pgconverge/schema"
)

// ReplicationResult represents the result of a replication operation.
type ReplicationResult struct {
	NodeName string
	Success  bool
	Message  string
	Error    error
}

// CreatePublication creates a publication on a node.
func (m *DBManager) CreatePublication(ctx context.Context, node *schema.Node) error {
	pool, err := m.Connect(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", node.Name, err)
	}

	pubName := fmt.Sprintf("pub_%s", node.Name)
	query := fmt.Sprintf(`
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = '%s') THEN
				CREATE PUBLICATION %s FOR ALL TABLES;
			END IF;
		END
		$$;
	`, pubName, pubName)

	_, err = pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create publication on %s: %w", node.Name, err)
	}

	return nil
}

// CreateReplicationSlot creates a replication slot on a publisher node.
func (m *DBManager) CreateReplicationSlot(ctx context.Context, publisherNode *schema.Node, slotName string) error {
	pool, err := m.Connect(ctx, publisherNode)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", publisherNode.Name, err)
	}

	query := fmt.Sprintf(`
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = '%s') THEN
				PERFORM pg_create_logical_replication_slot('%s', 'pgoutput');
			END IF;
		END
		$$;
	`, slotName, slotName)

	_, err = pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create replication slot on %s: %w", publisherNode.Name, err)
	}

	return nil
}

// CreateSubscription creates a subscription from subscriber to publisher.
func (m *DBManager) CreateSubscription(ctx context.Context, subscriberNode, publisherNode *schema.Node) error {
	pool, err := m.Connect(ctx, subscriberNode)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", subscriberNode.Name, err)
	}

	pubName := fmt.Sprintf("pub_%s", publisherNode.Name)
	subName := fmt.Sprintf("sub_%s_from_%s", subscriberNode.Name, publisherNode.Name)

	// First, create the replication slot on the publisher
	if err := m.CreateReplicationSlot(ctx, publisherNode, subName); err != nil {
		return fmt.Errorf("failed to create replication slot: %w", err)
	}

	// Then create the subscription on the subscriber
	query := fmt.Sprintf(`
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_subscription WHERE subname = '%s') THEN
				CREATE SUBSCRIPTION %s
				CONNECTION '%s'
				PUBLICATION %s
				WITH (create_slot = false, slot_name = '%s', enabled = true, copy_data = false, origin = 'none');
			END IF;
		END
		$$;
	`, subName, subName, publisherNode.ConnectionString(), pubName, subName)

	_, err = pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create subscription on %s: %w", subscriberNode.Name, err)
	}

	return nil
}

// SetupBidirectionalReplication sets up bidirectional replication between all nodes.
func (m *DBManager) SetupBidirectionalReplication(ctx context.Context) []ReplicationResult {
	var results []ReplicationResult

	// Step 1: Create publications on all nodes
	for i := range m.nodes {
		node := &m.nodes[i]
		err := m.CreatePublication(ctx, node)
		result := ReplicationResult{
			NodeName: node.Name,
			Success:  err == nil,
		}
		if err != nil {
			result.Error = err
			result.Message = fmt.Sprintf("Failed to create publication: %v", err)
		} else {
			result.Message = fmt.Sprintf("Created publication pub_%s", node.Name)
		}
		results = append(results, result)
	}

	// Step 2: Create subscriptions between all pairs
	for i := range m.nodes {
		subscriber := &m.nodes[i]
		for j := range m.nodes {
			if i == j {
				continue // Skip self-subscription
			}
			publisher := &m.nodes[j]

			err := m.CreateSubscription(ctx, subscriber, publisher)
			result := ReplicationResult{
				NodeName: subscriber.Name,
				Success:  err == nil,
			}
			if err != nil {
				result.Error = err
				result.Message = fmt.Sprintf("Failed to subscribe %s to %s: %v", subscriber.Name, publisher.Name, err)
			} else {
				result.Message = fmt.Sprintf("%s subscribed to %s", subscriber.Name, publisher.Name)
			}
			results = append(results, result)
		}
	}

	return results
}

// SetupReplicationForNode sets up replication for a specific node.
func (m *DBManager) SetupReplicationForNode(ctx context.Context, nodeName string) []ReplicationResult {
	var results []ReplicationResult

	node := m.GetNode(nodeName)
	if node == nil {
		return []ReplicationResult{{
			NodeName: nodeName,
			Success:  false,
			Error:    fmt.Errorf("node %s not found", nodeName),
			Message:  fmt.Sprintf("Node %s not found", nodeName),
		}}
	}

	// Create publication for this node
	err := m.CreatePublication(ctx, node)
	result := ReplicationResult{
		NodeName: node.Name,
		Success:  err == nil,
	}
	if err != nil {
		result.Error = err
		result.Message = fmt.Sprintf("Failed to create publication: %v", err)
	} else {
		result.Message = fmt.Sprintf("Created publication pub_%s", node.Name)
	}
	results = append(results, result)

	// Subscribe this node to all other nodes
	for i := range m.nodes {
		publisher := &m.nodes[i]
		if publisher.Name == nodeName {
			continue
		}

		err := m.CreateSubscription(ctx, node, publisher)
		result := ReplicationResult{
			NodeName: node.Name,
			Success:  err == nil,
		}
		if err != nil {
			result.Error = err
			result.Message = fmt.Sprintf("Failed to subscribe to %s: %v", publisher.Name, err)
		} else {
			result.Message = fmt.Sprintf("Subscribed to %s", publisher.Name)
		}
		results = append(results, result)
	}

	// Subscribe all other nodes to this node
	for i := range m.nodes {
		subscriber := &m.nodes[i]
		if subscriber.Name == nodeName {
			continue
		}

		err := m.CreateSubscription(ctx, subscriber, node)
		result := ReplicationResult{
			NodeName: subscriber.Name,
			Success:  err == nil,
		}
		if err != nil {
			result.Error = err
			result.Message = fmt.Sprintf("Failed to subscribe %s to %s: %v", subscriber.Name, node.Name, err)
		} else {
			result.Message = fmt.Sprintf("%s subscribed to %s", subscriber.Name, node.Name)
		}
		results = append(results, result)
	}

	return results
}
