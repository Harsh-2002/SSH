package tools

import (
	"strings"
)

// shellQuote quotes a string for safe shell use.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// Simple single-quote escaping
	escaped := strings.ReplaceAll(s, "'", "'\"'\"'")
	return "'" + escaped + "'"
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}

// trimOutput trims whitespace from output.
func trimOutput(s string) string {
	return strings.TrimSpace(s)
}
