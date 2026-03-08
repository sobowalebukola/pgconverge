// Package db provides database connection and management for PostgreSQL nodes.
package db

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sobowalebukola/pgconverge/schema"
)

// DBManager manages database connections to multiple PostgreSQL nodes.
type DBManager struct {
	pools map[string]*pgxpool.Pool
	nodes []schema.Node
	mu    sync.RWMutex
}

// NewDBManager creates a new DBManager with the given nodes.
func NewDBManager(nodes []schema.Node) *DBManager {
	return &DBManager{
		pools: make(map[string]*pgxpool.Pool),
		nodes: nodes,
	}
}

// GetPool returns a connection pool for the specified node.
func (m *DBManager) GetPool(ctx context.Context, nodeName string) (*pgxpool.Pool, error) {
	m.mu.RLock()
	if pool, ok := m.pools[nodeName]; ok {
		m.mu.RUnlock()
		return pool, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if pool, ok := m.pools[nodeName]; ok {
		return pool, nil
	}

	// Find the node
	var node *schema.Node
	for i := range m.nodes {
		if m.nodes[i].Name == nodeName {
			node = &m.nodes[i]
			break
		}
	}

	if node == nil {
		return nil, fmt.Errorf("node %s not found", nodeName)
	}

	pool, err := pgxpool.New(ctx, node.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to create pool for %s: %w", nodeName, err)
	}

	m.pools[nodeName] = pool
	return pool, nil
}

// Connect establishes a connection to the specified node.
func (m *DBManager) Connect(ctx context.Context, node *schema.Node) (*pgxpool.Pool, error) {
	m.mu.RLock()
	if pool, ok := m.pools[node.Name]; ok {
		m.mu.RUnlock()
		return pool, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if pool, ok := m.pools[node.Name]; ok {
		return pool, nil
	}

	pool, err := pgxpool.New(ctx, node.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", node.Name, err)
	}

	m.pools[node.Name] = pool
	return pool, nil
}

// GetNodes returns all configured nodes.
func (m *DBManager) GetNodes() []schema.Node {
	return m.nodes
}

// GetNode returns a node by name, or nil if not found.
func (m *DBManager) GetNode(name string) *schema.Node {
	for i := range m.nodes {
		if m.nodes[i].Name == name {
			return &m.nodes[i]
		}
	}
	return nil
}

// Close closes all connection pools.
func (m *DBManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pool := range m.pools {
		pool.Close()
	}
	m.pools = make(map[string]*pgxpool.Pool)
}

// Ping tests connectivity to a node.
func (m *DBManager) Ping(ctx context.Context, nodeName string) error {
	pool, err := m.GetPool(ctx, nodeName)
	if err != nil {
		return err
	}
	return pool.Ping(ctx)
}
