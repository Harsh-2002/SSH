// Package tools provides MCP tool implementations.
// validate.go contains server-side (Go-native) file syntax validators.
// Zero remote host dependencies — all parsing happens on the MCP server.
package tools

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// ValidationResult holds the outcome of a syntax check.
type ValidationResult struct {
	Valid    bool
	FileType string
	Errors   []string
}

// FormatResult returns a human-readable summary.
func (v *ValidationResult) FormatResult(path string) string {
	if v.Valid {
		return fmt.Sprintf("✓ Valid %s — %s", strings.ToUpper(v.FileType), path)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("✗ INVALID %s — %s\n", strings.ToUpper(v.FileType), path))
	for _, e := range v.Errors {
		b.WriteString("  " + e + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// ValidateContent validates raw file content server-side based on the detected file type.
// Returns nil if the file type is not recognized (no validation possible).
func ValidateContent(content, fileType string) *ValidationResult {
	switch fileType {
	case "json":
		return validateJSON(content)
	case "yaml":
		return validateYAML(content)
	case "toml":
		return validateTOML(content)
	case "xml":
		return validateXML(content)
	case "ini":
		return validateINI(content)
	case "env":
		return validateENV(content)
	case "dockerfile":
		return validateDockerfile(content)
	default:
		return nil
	}
}

// --- JSON ---

func validateJSON(content string) *ValidationResult {
	r := &ValidationResult{FileType: "json"}
	var v interface{}
	if err := json.Unmarshal([]byte(content), &v); err != nil {
		r.Errors = append(r.Errors, err.Error())
		return r
	}
	r.Valid = true
	return r
}

// --- YAML ---

func validateYAML(content string) *ValidationResult {
	r := &ValidationResult{FileType: "yaml"}
	// Decode all documents (multi-doc YAML support)
	dec := yaml.NewDecoder(strings.NewReader(content))
	for {
		var v interface{}
		err := dec.Decode(&v)
		if err == io.EOF {
			break
		}
		if err != nil {
			r.Errors = append(r.Errors, err.Error())
			return r
		}
	}
	r.Valid = true
	return r
}

// --- TOML ---

func validateTOML(content string) *ValidationResult {
	r := &ValidationResult{FileType: "toml"}
	var v interface{}
	if _, err := toml.Decode(content, &v); err != nil {
		r.Errors = append(r.Errors, err.Error())
		return r
	}
	r.Valid = true
	return r
}

// --- XML ---

func validateXML(content string) *ValidationResult {
	r := &ValidationResult{FileType: "xml"}
	dec := xml.NewDecoder(strings.NewReader(content))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			r.Errors = append(r.Errors, err.Error())
			return r
		}
	}
	r.Valid = true
	return r
}

// --- INI / .conf / .cfg ---
// Simple validator: checks section headers [section] and key=value pairs.
// Allows comments (# and ;) and blank lines.

func validateINI(content string) *ValidationResult {
	r := &ValidationResult{FileType: "ini"}
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Blank or comment
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				r.Errors = append(r.Errors, fmt.Sprintf("line %d: unclosed section header: %s", lineNum, line))
			}
			continue
		}

		// Key=value (allow key = value, key: value)
		if strings.ContainsAny(line, "=:") {
			continue
		}

		r.Errors = append(r.Errors, fmt.Sprintf("line %d: invalid syntax: %s", lineNum, line))
	}

	r.Valid = len(r.Errors) == 0
	return r
}

// --- .env / dotenv ---
// Validates KEY=VALUE format. Allows comments (#) and blank lines.
// Keys must start with a letter or underscore.

func validateENV(content string) *ValidationResult {
	r := &ValidationResult{FileType: "env"}
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Must contain = and key must start with letter/underscore
		eqIdx := strings.Index(line, "=")
		if eqIdx <= 0 {
			r.Errors = append(r.Errors, fmt.Sprintf("line %d: missing KEY=VALUE format: %s", lineNum, line))
			continue
		}

		key := line[:eqIdx]
		key = strings.TrimSpace(key)
		// Remove "export " prefix if present
		key = strings.TrimPrefix(key, "export ")
		key = strings.TrimSpace(key)

		if key == "" {
			r.Errors = append(r.Errors, fmt.Sprintf("line %d: empty key", lineNum))
			continue
		}

		firstChar := key[0]
		if !((firstChar >= 'A' && firstChar <= 'Z') || (firstChar >= 'a' && firstChar <= 'z') || firstChar == '_') {
			r.Errors = append(r.Errors, fmt.Sprintf("line %d: key must start with letter or underscore: %s", lineNum, key))
		}
	}

	r.Valid = len(r.Errors) == 0
	return r
}

// --- Dockerfile ---
// Validates that each non-comment, non-continuation line starts with a known instruction.

var dockerfileInstructions = map[string]bool{
	"FROM": true, "RUN": true, "CMD": true, "LABEL": true,
	"EXPOSE": true, "ENV": true, "ADD": true, "COPY": true,
	"ENTRYPOINT": true, "VOLUME": true, "USER": true, "WORKDIR": true,
	"ARG": true, "ONBUILD": true, "STOPSIGNAL": true, "HEALTHCHECK": true,
	"SHELL": true, "MAINTAINER": true,
}

func validateDockerfile(content string) *ValidationResult {
	r := &ValidationResult{FileType: "dockerfile"}
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	continuation := false

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Handle line continuation from previous line
		if continuation {
			continuation = strings.HasSuffix(trimmed, "\\")
			continue
		}

		// Skip blank lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check if line continues
		continuation = strings.HasSuffix(trimmed, "\\")

		// Extract instruction (first word)
		parts := strings.Fields(trimmed)
		if len(parts) == 0 {
			continue
		}

		instruction := strings.ToUpper(parts[0])
		// Handle parser directives (# syntax=..., # escape=...)
		if strings.HasPrefix(instruction, "#") {
			continue
		}

		if !dockerfileInstructions[instruction] {
			r.Errors = append(r.Errors, fmt.Sprintf("line %d: unknown instruction: %s", lineNum, parts[0]))
		}
	}

	// Check that FROM is present
	hasFrom := false
	scanner2 := bufio.NewScanner(strings.NewReader(content))
	for scanner2.Scan() {
		line := strings.TrimSpace(scanner2.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) > 0 && strings.ToUpper(parts[0]) == "FROM" {
			hasFrom = true
			break
		}
	}
	if !hasFrom && strings.TrimSpace(content) != "" {
		r.Errors = append(r.Errors, "missing FROM instruction")
	}

	r.Valid = len(r.Errors) == 0
	return r
}
