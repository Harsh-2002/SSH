package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"ssh-mcp/internal/ssh"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerFileTools registers file operation tools.
func registerFileTools(s *server.MCPServer, pool *ssh.Pool) {
	// read
	s.AddTool(
		mcp.NewTool("read",
			mcp.WithDescription("Read the contents of a remote file"),
			mcp.WithString("path", mcp.Required(), mcp.Description("File path to read")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createReadHandler(pool),
	)

	// write
	s.AddTool(
		mcp.NewTool("write",
			mcp.WithDescription("Write content to a remote file. Validates syntax BEFORE writing for known file types (JSON, YAML, TOML, XML, INI, Dockerfile). Validation is server-side with zero remote dependencies. Set skip_validate=true to bypass."),
			mcp.WithString("path", mcp.Required(), mcp.Description("File path to write")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Content to write")),
			mcp.WithBoolean("skip_validate", mcp.Description("Skip syntax validation before write (default: false)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createWriteHandler(pool),
	)

	// edit — sed-like file editing tool
	s.AddTool(
		mcp.NewTool("edit",
			mcp.WithDescription(`Powerful sed-like file editor. Supports multiple operations on any file type (YAML, JSON, conf, etc).

Operations (set via 'operation' parameter):
  replace     — Find and replace text (default). Exact literal match.
  regex       — Regex find and replace (sed-style). Use capture groups \1, \2, etc.
  insert      — Insert text at a specific line number (pushes existing content down).
  append      — Append text after a line matching a pattern, or at end of file if no pattern.
  prepend     — Prepend text before a line matching a pattern, or at start of file if no pattern.
  delete      — Delete lines matching a pattern or a line range.
  replace_line — Replace entire line(s) matching a pattern with new text.

Examples:
  operation=replace, old_text="port: 80", new_text="port: 443"
  operation=regex, pattern="timeout:\\s*\\d+", replacement="timeout: 30"
  operation=insert, line=5, content="new line here"
  operation=append, pattern="\\[section\\]", content="key = value"
  operation=delete, pattern="^#.*comment"
  operation=delete, start_line=10, end_line=15
  operation=replace_line, pattern="^server_name.*", content="server_name example.com;"
`),
			mcp.WithString("path", mcp.Required(), mcp.Description("File path to edit")),
			mcp.WithString("operation", mcp.Description("Operation: replace, regex, insert, append, prepend, delete, replace_line (default: replace)")),
			// For replace operation
			mcp.WithString("old_text", mcp.Description("Text to find (for 'replace' operation)")),
			mcp.WithString("new_text", mcp.Description("Replacement text (for 'replace' operation)")),
			// For regex operation
			mcp.WithString("pattern", mcp.Description("Regex pattern (for regex/append/prepend/delete/replace_line operations)")),
			mcp.WithString("replacement", mcp.Description("Replacement string with \\1 \\2 backrefs (for 'regex' operation)")),
			// For insert/append/prepend/replace_line
			mcp.WithString("content", mcp.Description("Content to insert/append/prepend/replace_line")),
			mcp.WithNumber("line", mcp.Description("Line number for 'insert' operation (1-based)")),
			// For delete range
			mcp.WithNumber("start_line", mcp.Description("Start line for range delete (1-based, inclusive)")),
			mcp.WithNumber("end_line", mcp.Description("End line for range delete (1-based, inclusive)")),
			// For replace/regex: control how many matches
			mcp.WithBoolean("global", mcp.Description("Replace all occurrences (default: false for replace, true for regex)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createEditHandler(pool),
	)
	// validate
	s.AddTool(
		mcp.NewTool("validate",
			mcp.WithDescription(`Validate file syntax server-side (zero remote host dependencies). Auto-detects type from extension.

Supported formats:
  .json                    — JSON syntax
  .yaml, .yml              — YAML syntax (multi-document)
  .toml                    — TOML syntax
  .xml, .svg, .xhtml       — XML well-formedness
  .ini, .cfg, .conf        — INI key=value structure
  .env                     — Dotenv KEY=VALUE format
  Dockerfile               — Instruction validation

All validation runs on the MCP server using Go parsers. No python3, jq, or other tools needed on the remote host.`),
			mcp.WithString("path", mcp.Required(), mcp.Description("File path to validate")),
			mcp.WithString("type", mcp.Description("Force file type: json, yaml, toml, xml, ini, env, dockerfile (auto-detected from extension if omitted)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createValidateHandler(pool),
	)

	// list_dir
	s.AddTool(
		mcp.NewTool("list_dir",
			mcp.WithDescription("List contents of a remote directory"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Directory path to list")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createListDirHandler(pool),
	)

	// sync
	s.AddTool(
		mcp.NewTool("sync",
			mcp.WithDescription("Stream a file directly between two remote nodes"),
			mcp.WithString("source_node", mcp.Required(), mcp.Description("Source connection alias")),
			mcp.WithString("source_path", mcp.Required(), mcp.Description("Source file path")),
			mcp.WithString("dest_node", mcp.Required(), mcp.Description("Destination connection alias")),
			mcp.WithString("dest_path", mcp.Required(), mcp.Description("Destination file path")),
		),
		createSyncHandler(pool),
	)
}

func createReadHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		path, _ := req.RequireString("path")
		target := req.GetString("target", "primary")

		content, err := mgr.ReadFile(ctx, path, target)
		if err != nil {
			log.Printf("[Tool:read] Error: %v", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(content), nil
	}
}

func createWriteHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		path, _ := req.RequireString("path")
		content, _ := req.RequireString("content")
		skipValidate := req.GetBool("skip_validate", false)
		target := req.GetString("target", "primary")

		// Validate BEFORE writing — catch errors before they hit the file
		if !skipValidate {
			fileType := detectFileType(path)
			if fileType != "" {
				result := ValidateContent(content, fileType)
				if result != nil && !result.Valid {
					return mcp.NewToolResultError(fmt.Sprintf(
						"Syntax validation failed — file NOT written.\n%s\n\nFix the errors above or set skip_validate=true to force write.",
						result.FormatResult(path))), nil
				}
			}
		}

		if err := mgr.WriteFile(ctx, path, content, target); err != nil {
			log.Printf("[Tool:write] Error: %v", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		msg := fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path)

		// Report validation status
		if !skipValidate {
			fileType := detectFileType(path)
			if fileType != "" {
				msg += fmt.Sprintf("\n✓ Syntax (%s): OK", fileType)
			}
		}

		return mcp.NewToolResultText(msg), nil
	}
}

func createEditHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		path, _ := req.RequireString("path")
		operation := req.GetString("operation", "replace")
		target := req.GetString("target", "primary")

		// Build a sed command based on the operation type.
		// We use sed for maximum compatibility with any file type on any remote system.
		var cmd string

		switch operation {
		case "replace":
			oldText := req.GetString("old_text", "")
			newText := req.GetString("new_text", "")
			if oldText == "" {
				return mcp.NewToolResultError("'old_text' is required for replace operation"), nil
			}
			// Use sed with literal string replacement via escaped special chars
			// We pipe through sed to handle special characters properly
			globalFlag := ""
			if req.GetBool("global", false) {
				globalFlag = "g"
			}
			cmd = fmt.Sprintf("sed -i 's/%s/%s/%s' %s 2>&1",
				sedEscapeLiteral(oldText), sedEscapeReplacement(newText), globalFlag, shellQuote(path))

		case "regex":
			pattern := req.GetString("pattern", "")
			replacement := req.GetString("replacement", "")
			if pattern == "" {
				return mcp.NewToolResultError("'pattern' is required for regex operation"), nil
			}
			globalFlag := "g" // regex defaults to global
			if !req.GetBool("global", true) {
				globalFlag = ""
			}
			cmd = fmt.Sprintf("sed -i -E 's/%s/%s/%s' %s 2>&1",
				sedEscapePattern(pattern), sedEscapeReplacement(replacement), globalFlag, shellQuote(path))

		case "insert":
			lineNum := req.GetInt("line", 0)
			content := req.GetString("content", "")
			if lineNum <= 0 {
				return mcp.NewToolResultError("'line' (positive integer) is required for insert operation"), nil
			}
			if content == "" {
				return mcp.NewToolResultError("'content' is required for insert operation"), nil
			}
			cmd = fmt.Sprintf("sed -i '%di\\%s' %s 2>&1",
				lineNum, sedEscapeInsertText(content), shellQuote(path))

		case "append":
			content := req.GetString("content", "")
			pattern := req.GetString("pattern", "")
			if content == "" {
				return mcp.NewToolResultError("'content' is required for append operation"), nil
			}
			if pattern != "" {
				// Append after line matching pattern
				cmd = fmt.Sprintf("sed -i '/%s/a\\%s' %s 2>&1",
					sedEscapePattern(pattern), sedEscapeInsertText(content), shellQuote(path))
			} else {
				// Append at end of file
				cmd = fmt.Sprintf("printf '\\n%%s' %s >> %s 2>&1",
					shellQuote(content), shellQuote(path))
			}

		case "prepend":
			content := req.GetString("content", "")
			pattern := req.GetString("pattern", "")
			if content == "" {
				return mcp.NewToolResultError("'content' is required for prepend operation"), nil
			}
			if pattern != "" {
				// Insert before line matching pattern
				cmd = fmt.Sprintf("sed -i '/%s/i\\%s' %s 2>&1",
					sedEscapePattern(pattern), sedEscapeInsertText(content), shellQuote(path))
			} else {
				// Prepend at start of file
				cmd = fmt.Sprintf("sed -i '1i\\%s' %s 2>&1",
					sedEscapeInsertText(content), shellQuote(path))
			}

		case "delete":
			pattern := req.GetString("pattern", "")
			startLine := req.GetInt("start_line", 0)
			endLine := req.GetInt("end_line", 0)

			if pattern != "" {
				// Delete lines matching pattern
				cmd = fmt.Sprintf("sed -i '/%s/d' %s 2>&1",
					sedEscapePattern(pattern), shellQuote(path))
			} else if startLine > 0 && endLine > 0 {
				// Delete line range
				cmd = fmt.Sprintf("sed -i '%d,%dd' %s 2>&1",
					startLine, endLine, shellQuote(path))
			} else if startLine > 0 {
				// Delete single line
				cmd = fmt.Sprintf("sed -i '%dd' %s 2>&1",
					startLine, shellQuote(path))
			} else {
				return mcp.NewToolResultError("'pattern' or 'start_line' is required for delete operation"), nil
			}

		case "replace_line":
			pattern := req.GetString("pattern", "")
			content := req.GetString("content", "")
			if pattern == "" {
				return mcp.NewToolResultError("'pattern' is required for replace_line operation"), nil
			}
			cmd = fmt.Sprintf("sed -i -E 's/%s/%s/' %s 2>&1",
				sedEscapePattern(pattern), sedEscapeReplacement(content), shellQuote(path))

		default:
			return mcp.NewToolResultError(fmt.Sprintf(
				"Unknown operation: '%s'. Supported: replace, regex, insert, append, prepend, delete, replace_line", operation)), nil
		}

		log.Printf("[Tool:edit] %s on %s: %s", operation, path, cmd)

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			log.Printf("[Tool:edit] Error: %v", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		// sed typically produces no output on success
		msg := ""
		if output == "(No output)" || strings.TrimSpace(output) == "" {
			msg = fmt.Sprintf("Successfully applied '%s' operation to %s", operation, path)
		} else {
			msg = output
		}

		// Validate AFTER edit — read back the file and check syntax server-side
		fileType := detectFileType(path)
		if fileType != "" {
			updated, readErr := mgr.ReadFile(ctx, path, target)
			if readErr == nil {
				result := ValidateContent(updated, fileType)
				if result != nil {
					if result.Valid {
						msg += fmt.Sprintf("\n✓ Syntax (%s): OK", fileType)
					} else {
						msg += fmt.Sprintf("\n\n⚠ Syntax (%s): BROKEN after edit\n%s",
							fileType, result.FormatResult(path))
					}
				}
			}
		}

		return mcp.NewToolResultText(msg), nil
	}
}

func createListDirHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		path, _ := req.RequireString("path")
		target := req.GetString("target", "primary")

		files, err := mgr.ListDir(ctx, path, target)
		if err != nil {
			log.Printf("[Tool:list_dir] Error: %v", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonBytes, err := json.MarshalIndent(files, "", "  ")
		if err != nil {
			return mcp.NewToolResultError("Failed to format directory listing"), nil
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

func createSyncHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		sourceNode, _ := req.RequireString("source_node")
		sourcePath, _ := req.RequireString("source_path")
		destNode, _ := req.RequireString("dest_node")
		destPath, _ := req.RequireString("dest_path")

		content, err := mgr.ReadFile(ctx, sourcePath, sourceNode)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read from source: %v", err)), nil
		}

		if err := mgr.WriteFile(ctx, destPath, content, destNode); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to write to destination: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Successfully synced %d bytes from %s to %s", len(content), sourceNode, destNode)), nil
	}
}

func createValidateHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		path, _ := req.RequireString("path")
		forceType := req.GetString("type", "")
		target := req.GetString("target", "primary")

		fileType := forceType
		if fileType == "" {
			fileType = detectFileType(path)
		}
		if fileType == "" {
			return mcp.NewToolResultError(fmt.Sprintf(
				"Cannot detect file type for '%s'. Use the 'type' parameter to specify: json, yaml, toml, xml, ini, env, dockerfile", path)), nil
		}

		// Read file content via SFTP
		content, err := mgr.ReadFile(ctx, path, target)
		if err != nil {
			log.Printf("[Tool:validate] Error reading file: %v", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Validate server-side with Go parsers
		result := ValidateContent(content, fileType)
		if result == nil {
			return mcp.NewToolResultError(fmt.Sprintf("No server-side validator for type '%s'", fileType)), nil
		}

		log.Printf("[Tool:validate] %s %s: valid=%v", fileType, path, result.Valid)
		return mcp.NewToolResultText(result.FormatResult(path)), nil
	}
}

// detectFileType determines the file type from its extension or name.
// fileTypePatterns maps glob-style patterns to file types.
// Matched in order — first match wins.
var fileTypePatterns = []struct {
	pattern  string // matched against lowercase basename
	fileType string
}{
	// Extension-based patterns
	{"*.json", "json"},
	{"*.yaml", "yaml"},
	{"*.yml", "yaml"},
	{"*.toml", "toml"},
	{"*.xml", "xml"},
	{"*.xsl", "xml"},
	{"*.xslt", "xml"},
	{"*.svg", "xml"},
	{"*.xhtml", "xml"},
	{"*.plist", "xml"},
	{"*.ini", "ini"},
	{"*.cfg", "ini"},
	{"*.conf", "ini"},
	{"*.env", "env"},

	// Name-based patterns (Dockerfile variants, dotenv)
	{"dockerfile*", "dockerfile"},
	{".env*", "env"},
}

func detectFileType(path string) string {
	lower := strings.ToLower(path)

	// Extract basename
	base := lower
	if idx := strings.LastIndex(lower, "/"); idx >= 0 {
		base = lower[idx+1:]
	}

	// Match against glob patterns
	for _, p := range fileTypePatterns {
		if matched, _ := filepath.Match(p.pattern, base); matched {
			return p.fileType
		}
	}

	return ""
}


