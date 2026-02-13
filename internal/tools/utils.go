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

// sedEscapeLiteral escapes a literal string for use in a sed s/pattern/ context.
// Escapes: / \ & . * [ ] ^ $ and newlines.
func sedEscapeLiteral(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`/`, `\/`,
		`&`, `\&`,
		`.`, `\.`,
		`*`, `\*`,
		`[`, `\[`,
		`]`, `\]`,
		`^`, `\^`,
		`$`, `\$`,
		"\n", `\n`,
	)
	return replacer.Replace(s)
}

// sedEscapePattern escapes a regex pattern for use in sed, only escaping the delimiter.
// The pattern is passed as-is for regex matching, only / and newlines are escaped.
func sedEscapePattern(s string) string {
	replacer := strings.NewReplacer(
		`/`, `\/`,
		"\n", `\n`,
	)
	return replacer.Replace(s)
}

// sedEscapeReplacement escapes a replacement string for sed s//replacement/ context.
// Only escapes: / \ & and newlines (these have special meaning in sed replacements).
func sedEscapeReplacement(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`/`, `\/`,
		`&`, `\&`,
		"\n", `\n`,
	)
	return replacer.Replace(s)
}

// sedEscapeInsertText escapes text for sed i\ or a\ commands.
// Newlines need to be escaped with backslash continuation for multi-line inserts.
func sedEscapeInsertText(s string) string {
	return strings.ReplaceAll(s, "\n", `\n`)
}
