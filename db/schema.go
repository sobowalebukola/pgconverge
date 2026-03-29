package db

import (
	"context"
	"fmt"
	"os"

	"github.com/sobowalebukola/pgconverge/schema"
)

// ApplySchema applies SQL schema to a node within a transaction.
func (m *DBManager) ApplySchema(ctx context.Context, node *schema.Node, schemaSQL string) error {
	pool, err := m.Connect(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", node.Name, err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction on %s: %w", node.Name, err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("failed to apply schema to %s: %w", node.Name, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit schema on %s: %w", node.Name, err)
	}

	return nil
}

// ApplySchemaToAll applies SQL schema to all nodes.
func (m *DBManager) ApplySchemaToAll(ctx context.Context, schemaSQL string) map[string]error {
	errors := make(map[string]error)

	for _, node := range m.nodes {
		if err := m.ApplySchema(ctx, &node, schemaSQL); err != nil {
			errors[node.Name] = err
		}
	}

	return errors
}

// ApplySchemaFromFile applies schema from a file to a node.
func (m *DBManager) ApplySchemaFromFile(ctx context.Context, node *schema.Node, filePath string) error {
	schemaSQL, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	return m.ApplySchema(ctx, node, string(schemaSQL))
}

// ApplySchemaFromFileToAll applies schema from a file to all nodes.
func (m *DBManager) ApplySchemaFromFileToAll(ctx context.Context, filePath string) map[string]error {
	schemaSQL, err := os.ReadFile(filePath)
	if err != nil {
		errors := make(map[string]error)
		for _, node := range m.nodes {
			errors[node.Name] = fmt.Errorf("failed to read schema file: %w", err)
		}
		return errors
	}

	return m.ApplySchemaToAll(ctx, string(schemaSQL))
}
