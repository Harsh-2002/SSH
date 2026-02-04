package tools

import (
	"context"
	"fmt"

	"ssh-mcp/internal/ssh"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerNetworkTools registers network and search tools.
func registerNetworkTools(s *server.MCPServer, pool *ssh.Pool) {
	// net_stat
	s.AddTool(
		mcp.NewTool("net_stat",
			mcp.WithDescription("Check listening ports (ss/netstat)"),
			mcp.WithNumber("port", mcp.Description("Filter by specific port")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createNetStatHandler(pool),
	)

	// search_files
	s.AddTool(
		mcp.NewTool("search_files",
			mcp.WithDescription("Find files using POSIX find"),
			mcp.WithString("pattern", mcp.Required(), mcp.Description("File name pattern (supports wildcards)")),
			mcp.WithString("path", mcp.Description("Search path (default: /)")),
			mcp.WithNumber("max_depth", mcp.Description("Maximum directory depth")),
			mcp.WithString("type", mcp.Description("File type: f (file), d (directory)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createSearchFilesHandler(pool),
	)

	// search_text
	s.AddTool(
		mcp.NewTool("search_text",
			mcp.WithDescription("Search text in files using grep"),
			mcp.WithString("pattern", mcp.Required(), mcp.Description("Search pattern")),
			mcp.WithString("path", mcp.Required(), mcp.Description("File or directory path")),
			mcp.WithBoolean("recursive", mcp.Description("Search recursively")),
			mcp.WithBoolean("ignore_case", mcp.Description("Case-insensitive search")),
			mcp.WithNumber("context", mcp.Description("Lines of context around matches")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createSearchTextHandler(pool),
	)

	// package_manage
	s.AddTool(
		mcp.NewTool("package_manage",
			mcp.WithDescription("Install/remove/check packages (apt, apk, dnf, yum)"),
			mcp.WithString("action", mcp.Required(), mcp.Description("Action: install, remove, check, list")),
			mcp.WithString("package", mcp.Description("Package name (required for install/remove/check)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createPackageManageHandler(pool),
	)
}

func createNetStatHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		port := req.GetInt("port", 0)
		target := req.GetString("target", "primary")

		var cmd string
		if port > 0 {
			cmd = fmt.Sprintf("ss -tlnp 2>/dev/null | grep ':%d ' || netstat -tlnp 2>/dev/null | grep ':%d '", port, port)
		} else {
			cmd = "ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null"
		}

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createSearchFilesHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		pattern, _ := req.RequireString("pattern")
		path := req.GetString("path", "/")
		maxDepth := req.GetInt("max_depth", 0)
		fileType := req.GetString("type", "")
		target := req.GetString("target", "primary")

		cmd := fmt.Sprintf("find %s", shellQuote(path))

		if maxDepth > 0 {
			cmd += fmt.Sprintf(" -maxdepth %d", maxDepth)
		}

		if fileType == "f" || fileType == "d" {
			cmd += fmt.Sprintf(" -type %s", fileType)
		}

		cmd += fmt.Sprintf(" -name %s 2>/dev/null | head -100", shellQuote(pattern))

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createSearchTextHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		pattern, _ := req.RequireString("pattern")
		path, _ := req.RequireString("path")
		recursive := req.GetBool("recursive", false)
		ignoreCase := req.GetBool("ignore_case", false)
		ctxLines := req.GetInt("context", 0)
		target := req.GetString("target", "primary")

		cmd := "grep"
		if recursive {
			cmd += " -r"
		}
		if ignoreCase {
			cmd += " -i"
		}
		if ctxLines > 0 {
			cmd += fmt.Sprintf(" -C %d", ctxLines)
		}
		cmd += fmt.Sprintf(" -n %s %s 2>/dev/null | head -100", shellQuote(pattern), shellQuote(path))

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createPackageManageHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		action, _ := req.RequireString("action")
		pkg := req.GetString("package", "")
		target := req.GetString("target", "primary")

		if action != "list" && pkg == "" {
			return mcp.NewToolResultError("Package name required for this action"), nil
		}

		detectCmd := `
if command -v apt-get >/dev/null 2>&1; then echo "apt"
elif command -v apk >/dev/null 2>&1; then echo "apk"
elif command -v dnf >/dev/null 2>&1; then echo "dnf"
elif command -v yum >/dev/null 2>&1; then echo "yum"
else echo "unknown"
fi`

		pkgMgr, err := mgr.Execute(ctx, detectCmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		pkgMgr = trimOutput(pkgMgr)

		var cmd string
		switch pkgMgr {
		case "apt":
			switch action {
			case "install":
				cmd = fmt.Sprintf("apt-get update && apt-get install -y %s", shellQuote(pkg))
			case "remove":
				cmd = fmt.Sprintf("apt-get remove -y %s", shellQuote(pkg))
			case "check":
				cmd = fmt.Sprintf("dpkg -s %s 2>/dev/null", shellQuote(pkg))
			case "list":
				cmd = "dpkg -l | head -50"
			}
		case "apk":
			switch action {
			case "install":
				cmd = fmt.Sprintf("apk add %s", shellQuote(pkg))
			case "remove":
				cmd = fmt.Sprintf("apk del %s", shellQuote(pkg))
			case "check":
				cmd = fmt.Sprintf("apk info %s 2>/dev/null", shellQuote(pkg))
			case "list":
				cmd = "apk list --installed | head -50"
			}
		case "dnf", "yum":
			switch action {
			case "install":
				cmd = fmt.Sprintf("%s install -y %s", pkgMgr, shellQuote(pkg))
			case "remove":
				cmd = fmt.Sprintf("%s remove -y %s", pkgMgr, shellQuote(pkg))
			case "check":
				cmd = fmt.Sprintf("rpm -qi %s 2>/dev/null", shellQuote(pkg))
			case "list":
				cmd = fmt.Sprintf("%s list installed 2>/dev/null | head -50", pkgMgr)
			}
		default:
			return mcp.NewToolResultError("No supported package manager found"), nil
		}

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}
