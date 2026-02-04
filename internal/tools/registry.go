// Package tools provides MCP tool implementations.
package tools

import (
	"ssh-mcp/internal/ssh"

	"github.com/mark3labs/mcp-go/server"
)

// RegisterAll registers all MCP tools.
func RegisterAll(s *server.MCPServer, pool *ssh.Pool) {
	registerCoreTools(s, pool)
	registerFileTools(s, pool)
	registerMonitoringTools(s, pool)
	registerDockerTools(s, pool)
	registerNetworkTools(s, pool)
	registerDBTools(s, pool)
	registerVoIPTools(s, pool)
}

