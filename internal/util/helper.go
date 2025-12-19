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

func Contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func QuoteCols(cols []string) string {
	for i := range cols {
		cols[i] = fmt.Sprintf(`"%s"`, cols[i])
	}
	return strings.Join(cols, ", ")
}

func GetPort(name string) int {
	if port, ok := portMap[name]; ok {
		return port
	}
	portCounter++
	port := basePort + portCounter
	portMap[name] = port

	return port
}
