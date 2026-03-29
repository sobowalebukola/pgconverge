package db

import (
	"fmt"
	"regexp"
	"strings"
)

var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validateIdentifier checks that a string is safe to use as a SQL identifier.
func validateIdentifier(name string) error {
	if !validIdentifier.MatchString(name) {
		return fmt.Errorf("invalid SQL identifier %q: must match [a-zA-Z_][a-zA-Z0-9_]*", name)
	}
	return nil
}

// quoteIdentifier double-quotes a SQL identifier, escaping embedded double quotes.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// escapeLiteral escapes a string for use inside a SQL single-quoted literal.
func escapeLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
