package tools

import (
	"context"
	"fmt"
	"log"

	"ssh-mcp/internal/ssh"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerMonitoringTools registers monitoring and diagnostics tools.
func registerMonitoringTools(s *server.MCPServer, pool *ssh.Pool) {
	// usage
	s.AddTool(
		mcp.NewTool("usage",
			mcp.WithDescription("Get CPU/RAM/Disk usage summary"),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createUsageHandler(pool),
	)

	// ps
	s.AddTool(
		mcp.NewTool("ps",
			mcp.WithDescription("List top processes sorted by CPU or memory"),
			mcp.WithString("sort_by", mcp.Description("Sort by 'cpu' or 'mem' (default: cpu)")),
			mcp.WithNumber("limit", mcp.Description("Number of processes to show (default: 10)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createPsHandler(pool),
	)

	// logs
	s.AddTool(
		mcp.NewTool("logs",
			mcp.WithDescription("Read the tail of a log file"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Path to log file")),
			mcp.WithNumber("lines", mcp.Description("Number of lines to read (default: 50, max: 500)")),
			mcp.WithString("grep", mcp.Description("Optional filter pattern")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createLogsHandler(pool),
	)

	// journal_read
	s.AddTool(
		mcp.NewTool("journal_read",
			mcp.WithDescription("Read system logs (journalctl/syslog)"),
			mcp.WithString("service", mcp.Description("Service name to filter (e.g., nginx, sshd)")),
			mcp.WithString("since", mcp.Description("Time filter (e.g., '1 hour ago')")),
			mcp.WithNumber("lines", mcp.Description("Number of lines (default: 100, max: 500)")),
			mcp.WithString("priority", mcp.Description("Log priority: emerg, alert, crit, err, warning, notice, info, debug")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createJournalReadHandler(pool),
	)

	// dmesg_read
	s.AddTool(
		mcp.NewTool("dmesg_read",
			mcp.WithDescription("Read kernel ring buffer (dmesg)"),
			mcp.WithString("grep", mcp.Description("Optional pattern to filter messages")),
			mcp.WithNumber("lines", mcp.Description("Number of lines (default: 100)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createDmesgReadHandler(pool),
	)

	// diagnose_system
	s.AddTool(
		mcp.NewTool("diagnose_system",
			mcp.WithDescription("One-click SRE health check: load, OOM, disk, failed services"),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createDiagnoseHandler(pool),
	)

	// list_services
	s.AddTool(
		mcp.NewTool("list_services",
			mcp.WithDescription("List system services (systemd/OpenRC)"),
			mcp.WithBoolean("failed_only", mcp.Description("Show only failed services")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createListServicesHandler(pool),
	)
}

func createUsageHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		target := req.GetString("target", "primary")

		cmd := `
echo "=== LOAD AVERAGE ==="
cat /proc/loadavg 2>/dev/null

echo ""
echo "=== MEMORY ==="
free -h 2>/dev/null || cat /proc/meminfo 2>/dev/null | head -5

echo ""
echo "=== DISK ==="
df -h / 2>/dev/null
`
		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createPsHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		sortBy := req.GetString("sort_by", "cpu")
		limit := req.GetInt("limit", 10)
		target := req.GetString("target", "primary")

		if limit > 50 {
			limit = 50
		}

		sortFlag := "-%cpu"
		if sortBy == "mem" {
			sortFlag = "-%mem"
		}

		cmd := fmt.Sprintf("ps -eo pid,user,%%cpu,%%mem,comm --sort=%s | head -n %d", sortFlag, limit+1)
		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createLogsHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		path, _ := req.RequireString("path")
		lines := req.GetInt("lines", 50)
		grep := req.GetString("grep", "")
		target := req.GetString("target", "primary")

		if lines > 500 {
			lines = 500
		}

		cmd := fmt.Sprintf("tail -n %d %s", lines, shellQuote(path))
		if grep != "" {
			cmd += fmt.Sprintf(" | grep %s", shellQuote(grep))
		}

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createJournalReadHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		service := req.GetString("service", "")
		since := req.GetString("since", "")
		lines := req.GetInt("lines", 100)
		priority := req.GetString("priority", "")
		target := req.GetString("target", "primary")

		if lines > 500 {
			lines = 500
		}

		checkCmd := "command -v journalctl >/dev/null 2>&1 && echo 'systemd' || echo 'syslog'"
		checkOutput, err := mgr.Execute(ctx, checkCmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var cmd string
		if containsString(checkOutput, "systemd") {
			cmd = "journalctl --no-pager"
			if service != "" {
				cmd += fmt.Sprintf(" -u %s", shellQuote(service))
			}
			if since != "" {
				cmd += fmt.Sprintf(" --since %s", shellQuote(since))
			}
			if priority != "" {
				cmd += fmt.Sprintf(" -p %s", shellQuote(priority))
			}
			cmd += fmt.Sprintf(" -n %d 2>/dev/null", lines)
		} else {
			cmd = fmt.Sprintf("cat /var/log/syslog /var/log/messages 2>/dev/null | tail -n %d", lines)
			if service != "" {
				cmd += fmt.Sprintf(" | grep -i %s", shellQuote(service))
			}
		}

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createDmesgReadHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		grep := req.GetString("grep", "")
		lines := req.GetInt("lines", 100)
		target := req.GetString("target", "primary")

		if lines > 500 {
			lines = 500
		}

		cmd := "dmesg --time-format iso 2>/dev/null || dmesg 2>/dev/null"
		if grep != "" {
			cmd += fmt.Sprintf(" | grep -i %s", shellQuote(grep))
		}
		cmd += fmt.Sprintf(" | tail -n %d", lines)

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createDiagnoseHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		target := req.GetString("target", "primary")

		cmd := `
echo "=== SYSTEM HEALTH DIAGNOSTIC ==="
echo ""

echo "--- LOAD AVERAGE ---"
LOAD=$(cat /proc/loadavg 2>/dev/null | awk '{print $1}')
CPUS=$(nproc 2>/dev/null || echo 1)
echo "Load (1min): $LOAD (CPUs: $CPUS)"
if [ $(echo "$LOAD > $((CPUS * 2))" | bc 2>/dev/null) = 1 ] 2>/dev/null; then
  echo "WARNING: High load detected!"
fi
echo ""

echo "--- TOP CPU CONSUMERS ---"
ps -eo pid,user,%cpu,%mem,comm --sort=-%cpu 2>/dev/null | head -n 6
echo ""

echo "--- OOM EVENTS ---"
OOM=$(dmesg 2>/dev/null | grep -i 'out of memory' | tail -n 3)
if [ -n "$OOM" ]; then
  echo "$OOM"
  echo "WARNING: OOM events found!"
else
  echo "No OOM events in dmesg"
fi
echo ""

echo "--- DISK PRESSURE (>90%) ---"
df -P 2>/dev/null | awk 'NR>1 && int($5)>=90 {print $5, $6}'
if [ $(df -P 2>/dev/null | awk 'NR>1 && int($5)>=90' | wc -l) -eq 0 ]; then
  echo "No partitions over 90%"
fi
echo ""

echo "--- FAILED SERVICES ---"
if command -v systemctl >/dev/null 2>&1; then
  FAILED=$(systemctl --failed --no-legend --no-pager 2>/dev/null | head -n 5)
  if [ -n "$FAILED" ]; then
    echo "$FAILED"
  else
    echo "No failed services"
  fi
elif command -v rc-status >/dev/null 2>&1; then
  rc-status --crashed 2>/dev/null | head -n 5 || echo "No crashed services"
else
  echo "Init system not detected"
fi

echo ""
echo "=== END DIAGNOSTIC ==="
`
		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			log.Printf("[Tool:diagnose] Error: %v", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createListServicesHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		failedOnly := req.GetBool("failed_only", false)
		target := req.GetString("target", "primary")

		var cmd string
		if failedOnly {
			cmd = "systemctl --failed --no-pager 2>/dev/null || rc-status --crashed 2>/dev/null"
		} else {
			cmd = "systemctl list-units --type=service --no-pager 2>/dev/null | head -50 || rc-status 2>/dev/null"
		}

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}
