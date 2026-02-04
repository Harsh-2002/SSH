package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
			mcp.WithDescription("Write content to a remote file"),
			mcp.WithString("path", mcp.Required(), mcp.Description("File path to write")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Content to write")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createWriteHandler(pool),
	)

	// edit
	s.AddTool(
		mcp.NewTool("edit",
			mcp.WithDescription("Safely replace text in a file"),
			mcp.WithString("path", mcp.Required(), mcp.Description("File path to edit")),
			mcp.WithString("old_text", mcp.Required(), mcp.Description("Text to find and replace")),
			mcp.WithString("new_text", mcp.Required(), mcp.Description("Replacement text")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createEditHandler(pool),
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
		target := req.GetString("target", "primary")

		if err := mgr.WriteFile(ctx, path, content, target); err != nil {
			log.Printf("[Tool:write] Error: %v", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path)), nil
	}
}

func createEditHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		path, _ := req.RequireString("path")
		oldText, _ := req.RequireString("old_text")
		newText, _ := req.RequireString("new_text")
		target := req.GetString("target", "primary")

		content, err := mgr.ReadFile(ctx, path, target)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
		}

		count := strings.Count(content, oldText)
		if count == 0 {
			return mcp.NewToolResultError("Could not find exact match for old_text in file"), nil
		}
		if count > 1 {
			return mcp.NewToolResultError(fmt.Sprintf("Found %d occurrences of old_text. Please provide more context to be unique.", count)), nil
		}

		newContent := strings.Replace(content, oldText, newText, 1)

		if err := mgr.WriteFile(ctx, path, newContent, target); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to write file: %v", err)), nil
		}

		return mcp.NewToolResultText("File updated successfully"), nil
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
