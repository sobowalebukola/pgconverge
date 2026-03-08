// Package util provides utility functions for pgconverge.
package util

import (
	"fmt"
	"strings"
)

var (
	basePort    = 5432
	portCounter = 0
	portMap     = make(map[string]int)
)

// Contains checks if a string slice contains a value.
func Contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// QuoteCols quotes column names and joins them with commas.
func QuoteCols(cols []string) string {
	quoted := make([]string, len(cols))
	for i := range cols {
		quoted[i] = fmt.Sprintf(`"%s"`, cols[i])
	}
	return strings.Join(quoted, ", ")
}

// GetPort returns a unique port for a node name.
func GetPort(name string) int {
	if port, ok := portMap[name]; ok {
		return port
	}
	portCounter++
	port := basePort + portCounter
	portMap[name] = port

	return port
}

// ResetPorts resets the port counter (useful for testing).
func ResetPorts() {
	portCounter = 0
	portMap = make(map[string]int)
}
