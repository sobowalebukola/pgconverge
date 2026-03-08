// Package schema provides types for PostgreSQL schema and node configuration.
package schema

import "fmt"

// Column represents a database column definition.
type Column struct {
	Name     string `json:"name"`
	DataType string `json:"data_type"`
	Default  string `json:"default,omitempty"`
}

// ForeignKey represents a foreign key constraint.
type ForeignKey struct {
	Columns    []string            `json:"columns"`
	References map[string][]string `json:"references"`
}

// Constraints represents table constraints.
type Constraints struct {
	Primary     []string     `json:"primary"`
	Unique      [][]string   `json:"unique"`
	ForeignKeys []ForeignKey `json:"foreign_keys"`
}

// Table represents a database table definition.
type Table struct {
	Name        string            `json:"name"`
	Columns     map[string]Column `json:"columns"`
	Constraints Constraints       `json:"constraints"`
	Indexes     [][]string        `json:"indexes"`
}

// Node represents a PostgreSQL node configuration.
type Node struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port,omitempty"`
	User     string `json:"user"`
	Database string `json:"db"`
	Password string `json:"password"`
	External bool   `json:"external,omitempty"`
}

// GetPort returns the port number, defaulting to 5432 if not specified.
func (n *Node) GetPort() int {
	if n.Port == 0 {
		return 5432
	}
	return n.Port
}

// ConnectionString returns a PostgreSQL connection string for the node.
func (n *Node) ConnectionString() string {
	return fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
		n.Host, n.GetPort(), n.Database, n.User, n.Password)
}
