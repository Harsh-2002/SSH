package tools

import (
	"context"
	"fmt"
	"log"

	"ssh-mcp/internal/ssh"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerDockerTools registers Docker-related tools.
func registerDockerTools(s *server.MCPServer, pool *ssh.Pool) {
	// docker_ps
	s.AddTool(
		mcp.NewTool("docker_ps",
			mcp.WithDescription("List Docker containers"),
			mcp.WithBoolean("all", mcp.Description("Show all containers (default: only running)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createDockerPsHandler(pool),
	)

	// docker_logs
	s.AddTool(
		mcp.NewTool("docker_logs",
			mcp.WithDescription("Get logs from a Docker container"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name or ID")),
			mcp.WithNumber("lines", mcp.Description("Number of lines (default: 50)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createDockerLogsHandler(pool),
	)

	// docker_op
	s.AddTool(
		mcp.NewTool("docker_op",
			mcp.WithDescription("Start, stop, or restart a Docker container"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name or ID")),
			mcp.WithString("action", mcp.Required(), mcp.Description("Action: start, stop, restart")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createDockerOpHandler(pool),
	)

	// docker_ip
	s.AddTool(
		mcp.NewTool("docker_ip",
			mcp.WithDescription("Get IP address(es) of a Docker container"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createDockerIPHandler(pool),
	)

	// docker_find_by_ip
	s.AddTool(
		mcp.NewTool("docker_find_by_ip",
			mcp.WithDescription("Find which Docker container has a specific IP"),
			mcp.WithString("ip", mcp.Required(), mcp.Description("IP address to search")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createDockerFindByIPHandler(pool),
	)

	// docker_networks
	s.AddTool(
		mcp.NewTool("docker_networks",
			mcp.WithDescription("List all Docker networks and their containers"),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createDockerNetworksHandler(pool),
	)

	// docker_cp_from
	s.AddTool(
		mcp.NewTool("docker_cp_from",
			mcp.WithDescription("Copy file from Docker container to host"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name")),
			mcp.WithString("container_path", mcp.Required(), mcp.Description("Path inside container")),
			mcp.WithString("host_path", mcp.Required(), mcp.Description("Destination path on host")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createDockerCpFromHandler(pool),
	)

	// docker_cp_to
	s.AddTool(
		mcp.NewTool("docker_cp_to",
			mcp.WithDescription("Copy file from host to Docker container"),
			mcp.WithString("host_path", mcp.Required(), mcp.Description("Source path on host")),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name")),
			mcp.WithString("container_path", mcp.Required(), mcp.Description("Destination path inside container")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createDockerCpToHandler(pool),
	)
}

func createDockerPsHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		all := req.GetBool("all", false)
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		flag := ""
		if all {
			flag = "-a"
		}

		cmd := fmt.Sprintf("docker ps %s --format 'table {{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}'", flag)
		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createDockerLogsHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		lines := req.GetInt("lines", 50)
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cmd := fmt.Sprintf("docker logs --tail %d %s 2>&1", lines, shellQuote(container))
		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createDockerOpHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		action, _ := req.RequireString("action")
		target := req.GetString("target", "primary")

		if action != "start" && action != "stop" && action != "restart" {
			return mcp.NewToolResultError("Invalid action. Use: start, stop, restart"), nil
		}

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cmd := fmt.Sprintf("docker %s %s 2>&1", shellQuote(action), shellQuote(container))
		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			log.Printf("[Tool:docker_op] Error: %v", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("%s: %s\n%s", action, container, output)), nil
	}
}

func createDockerIPHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cmd := fmt.Sprintf("docker inspect --format '{{range $net, $conf := .NetworkSettings.Networks}}{{$net}}:{{$conf.IPAddress}}|{{end}}' %s 2>/dev/null", shellQuote(container))
		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Container: %s\nNetworks: %s", container, output)), nil
	}
}

func createDockerFindByIPHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		ip, _ := req.RequireString("ip")
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cmd := fmt.Sprintf(`docker ps -q | xargs -I {} docker inspect --format '{{.Name}}|{{range $net, $conf := .NetworkSettings.Networks}}{{$net}}:{{$conf.IPAddress}},{{end}}' {} 2>/dev/null | grep %s`, shellQuote(ip))
		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("No container found with IP: %s", ip)), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createDockerNetworksHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cmd := "docker network ls --format '{{.Name}} ({{.Driver}})'"
		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createDockerCpFromHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		containerPath, _ := req.RequireString("container_path")
		hostPath, _ := req.RequireString("host_path")
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cmd := fmt.Sprintf("docker cp %s:%s %s 2>&1", shellQuote(container), shellQuote(containerPath), shellQuote(hostPath))
		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if containsString(output, "Error") || containsString(output, "No such") {
			return mcp.NewToolResultError(output), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Copied %s:%s to %s", container, containerPath, hostPath)), nil
	}
}

func createDockerCpToHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		hostPath, _ := req.RequireString("host_path")
		container, _ := req.RequireString("container")
		containerPath, _ := req.RequireString("container_path")
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cmd := fmt.Sprintf("docker cp %s %s:%s 2>&1", shellQuote(hostPath), shellQuote(container), shellQuote(containerPath))
		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if containsString(output, "Error") || containsString(output, "No such") {
			return mcp.NewToolResultError(output), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Copied %s to %s:%s", hostPath, container, containerPath)), nil
	}
}

func checkDockerAvailable(ctx context.Context, mgr *ssh.Manager, target string) error {
	output, err := mgr.Execute(ctx, "command -v docker >/dev/null 2>&1 && echo 'ok' || echo 'missing'", target)
	if err != nil {
		return err
	}
	if !containsString(output, "ok") {
		return fmt.Errorf("docker command not found on target")
	}
	return nil
}
