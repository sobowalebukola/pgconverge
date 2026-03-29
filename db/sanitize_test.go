package db

import (
	"testing"
)

func TestValidateIdentifier(t *testing.T) {
	valid := []string{
		"node_a",
		"Node1",
		"_private",
		"abc",
		"A",
		"node_a_b_c",
	}
	for _, name := range valid {
		if err := validateIdentifier(name); err != nil {
			t.Errorf("validateIdentifier(%q) returned error: %v", name, err)
		}
	}

	invalid := []string{
		"",
		"1node",
		"node-a",
		"node a",
		"node;DROP TABLE",
		"node'name",
		`node"name`,
		"node.name",
		"pub_$(whoami)",
	}
	for _, name := range invalid {
		if err := validateIdentifier(name); err == nil {
			t.Errorf("validateIdentifier(%q) should have returned error", name)
		}
	}
}

func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"pub_node_a", `"pub_node_a"`},
		{"simple", `"simple"`},
		{`has"quote`, `"has""quote"`},
		{`a""b`, `"a""""b"`},
	}
	for _, tc := range tests {
		got := quoteIdentifier(tc.input)
		if got != tc.expected {
			t.Errorf("quoteIdentifier(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestEscapeLiteral(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"it's", "it''s"},
		{"a'b'c", "a''b''c"},
		{"no quotes", "no quotes"},
		{"''", "''''"},
	}
	for _, tc := range tests {
		got := escapeLiteral(tc.input)
		if got != tc.expected {
			t.Errorf("escapeLiteral(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
