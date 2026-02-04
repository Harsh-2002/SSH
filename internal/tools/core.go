package tools

import (
	"context"
	"fmt"
	"log"
	"time"

	"ssh-mcp/internal/ssh"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerCoreTools registers core SSH tools.
func registerCoreTools(s *server.MCPServer, pool *ssh.Pool) {
	// connect
	s.AddTool(
		mcp.NewTool("connect",
			mcp.WithDescription("Establish an SSH connection to a remote host"),
			mcp.WithString("host", mcp.Required(), mcp.Description("Hostname or IP address")),
			mcp.WithString("username", mcp.Required(), mcp.Description("SSH username")),
			mcp.WithNumber("port", mcp.Description("SSH port (default: 22)")),
			mcp.WithString("password", mcp.Description("SSH password (optional if using key)")),
			mcp.WithString("private_key_path", mcp.Description("Path to private key file")),
			mcp.WithString("alias", mcp.Description("Connection alias (auto-generated if not provided)")),
			mcp.WithString("via", mcp.Description("Jump host alias for tunneling")),
		),
		createConnectHandler(pool),
	)

	// disconnect
	s.AddTool(
		mcp.NewTool("disconnect",
			mcp.WithDescription("Close an SSH connection"),
			mcp.WithString("alias", mcp.Description("Connection alias to disconnect (all if empty)")),
		),
		createDisconnectHandler(pool),
	)

	// run
	s.AddTool(
		mcp.NewTool("run",
			mcp.WithDescription("Execute a shell command on the remote host. Use timeout for long-running tasks."),
			mcp.WithString("command", mcp.Required(), mcp.Description("Shell command to execute")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
			mcp.WithNumber("timeout", mcp.Description("Command timeout in seconds (default: 120)")),
		),
		createRunHandler(pool),
	)

	// identity
	s.AddTool(
		mcp.NewTool("identity",
			mcp.WithDescription("Get the server's public SSH key for authorized_keys"),
		),
		createIdentityHandler(pool),
	)

	// info
	s.AddTool(
		mcp.NewTool("info",
			mcp.WithDescription("Get remote system information (OS, kernel, hostname)"),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createInfoHandler(pool),
	)
}

func createConnectHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		host, _ := req.RequireString("host")
		username, _ := req.RequireString("username")
		port := req.GetInt("port", 22)
		password := req.GetString("password", "")
		keyPath := req.GetString("private_key_path", "")
		alias := req.GetString("alias", "")
		via := req.GetString("via", "")

		opts := ssh.ConnectOptions{
			Host:           host,
			Port:           port,
			Username:       username,
			Password:       password,
			PrivateKeyPath: keyPath,
			Alias:          alias,
			Via:            via,
		}

		resultAlias, err := mgr.Connect(ctx, opts)
		if err != nil {
			log.Printf("[Tool:connect] Error: %v", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Connected to %s@%s (alias: %s)", username, host, resultAlias)), nil
	}
}

func createDisconnectHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		alias := req.GetString("alias", "")
		msg, err := mgr.Disconnect(alias)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(msg), nil
	}
}

func createRunHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		command, _ := req.RequireString("command")
		target := req.GetString("target", "primary")
		timeout := req.GetInt("timeout", 120) // Default 120s like Python

		// Create context with timeout
		if timeout > 0 {
			timeoutDuration := time.Duration(timeout) * time.Second
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeoutDuration)
			defer cancel()
		}

		output, err := mgr.Execute(ctx, command, target)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return mcp.NewToolResultError(fmt.Sprintf("Command timed out after %ds", timeout)), nil
			}
			log.Printf("[Tool:run] Error: %v", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createIdentityHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		pubKey, err := mgr.GetPublicKey()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(pubKey), nil
	}
}

func createInfoHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		target := req.GetString("target", "primary")

		cmd := `echo "Hostname: $(hostname)"; echo "OS: $(cat /etc/os-release 2>/dev/null | grep PRETTY_NAME | cut -d'"' -f2 || uname -s)"; echo "Kernel: $(uname -r)"; echo "Arch: $(uname -m)"; echo "Shell: $SHELL"`
		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

// getManager retrieves the SSH manager for the current session.
// Strategy:
// 1. Global mode: Single shared manager (-global flag)
// 2. Session-based: Pool by MCP session ID (default)
// Note: Header-based pooling (X-Session-Key) is available via GetByHeader
// but requires custom HTTP middleware to extract headers.
func getManager(ctx context.Context, pool *ssh.Pool) *ssh.Manager {
	session := server.ClientSessionFromContext(ctx)
	if session == nil {
		return nil
	}

	// Use session ID-based pooling
	return pool.Get(session.SessionID())
}

