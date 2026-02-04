// Package main is the entry point for the SSH MCP server.
// Supports stdio (for local MCP hosts) and Streamable HTTP transports.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ssh-mcp/internal/ssh"
	"ssh-mcp/internal/tools"

	"github.com/mark3labs/mcp-go/server"
)

const (
	serverName    = "ssh-mcp"
	serverVersion = "2.0.0"
	
	// Defaults
	defaultMode  = "http"
	defaultPort  = "8000"
	defaultDebug = "false"
)

func main() {
	// Configuration Precedence: Flag > Env > Default
	
	// Helper to get env with fallback
	getEnv := func(key, fallback string) string {
		if value, exists := os.LookupEnv(key); exists {
			return value
		}
		return fallback
	}

	// Initialize flags with Env/Default values
	modeEnv := getEnv("SSH_MCP_MODE", defaultMode)
	portEnv := getEnv("PORT", defaultPort)
	debugEnv := getEnv("SSH_MCP_DEBUG", defaultDebug) == "true"
	globalEnv := getEnv("SSH_MCP_GLOBAL", "false") == "true"

	// Define flags (overrides envs)
	mode := flag.String("mode", modeEnv, "Transport mode: stdio or http")
	port := flag.String("port", portEnv, "HTTP server port (http mode only)")
	debug := flag.Bool("debug", debugEnv, "Enable debug logging")
	globalState := flag.Bool("global", globalEnv, "Use single shared SSH manager for all sessions")
	flag.Parse()

	// Configure logging
	if *debug {
		log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	} else {
		log.SetFlags(log.LstdFlags)
	}

	log.Printf("Starting %s v%s (mode=%s, port=%s, global=%v)", serverName, serverVersion, *mode, *port, *globalState)

	// Initialize SSH Pool
	pool := ssh.NewPool(*globalState)

	// Create MCP Server
	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
		server.WithHooks(createSessionHooks(pool)),
	)

	// Register all tools
	tools.RegisterAll(mcpServer, pool)

	// Start server
	switch *mode {
	case "stdio":
		runStdio(mcpServer)
	case "http":
		runHTTP(mcpServer, *port)
	default:
		log.Fatalf("Unknown mode: %s. Use 'stdio' or 'http'.", *mode)
	}
}

// createSessionHooks sets up session lifecycle hooks.
func createSessionHooks(pool *ssh.Pool) *server.Hooks {
	hooks := &server.Hooks{}

	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		sessionID := session.SessionID()
		log.Printf("[Session] Started: %s", sessionID)
		pool.CreateSession(sessionID)
	})

	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		sessionID := session.SessionID()
		log.Printf("[Session] Ended: %s", sessionID)
		pool.DestroySession(sessionID)
	})

	return hooks
}

// runStdio runs the server in stdio mode.
func runStdio(s *server.MCPServer) {
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Stdio server error: %v", err)
	}
}

// runHTTP runs the server in Streamable HTTP mode with graceful shutdown.
func runHTTP(s *server.MCPServer, port string) {
	httpServer := server.NewStreamableHTTPServer(s)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[HTTP] Listening on :%s", port)
		if err := httpServer.Start(":" + port); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-sigChan
	log.Println("[HTTP] Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("[HTTP] Shutdown error: %v", err)
	}

	log.Println("[HTTP] Server stopped")
}
