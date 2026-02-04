// Package main is the entry point for the SSH MCP server.
// Supports stdio (for local MCP hosts) and Streamable HTTP transports.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"ssh-mcp/internal/ssh"
	"ssh-mcp/internal/tools"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/server"
)

const (
	serverName    = "ssh-mcp"
	
	// Defaults
	defaultMode  = "http"
	defaultPort  = "8000"
	defaultDebug = "false"
	defaultGlobal = "false"
)

// UUIDv7SessionManager generates time-ordered UUIDv7 session IDs
// for professional, sortable session identification.
// Includes automatic cleanup of old terminated sessions to prevent memory leaks.
type UUIDv7SessionManager struct {
	mu         sync.RWMutex
	terminated map[string]time.Time // sessionID -> termination time
}

func NewUUIDv7SessionManager() *UUIDv7SessionManager {
	mgr := &UUIDv7SessionManager{
		terminated: make(map[string]time.Time),
	}
	// Cleanup old terminated sessions every 10 minutes
	go mgr.cleanupLoop()
	return mgr
}

// cleanupLoop removes terminated sessions older than 1 hour.
func (m *UUIDv7SessionManager) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		cutoff := time.Now().Add(-1 * time.Hour)
		for id, terminatedAt := range m.terminated {
			if terminatedAt.Before(cutoff) {
				delete(m.terminated, id)
			}
		}
		m.mu.Unlock()
	}
}

func (m *UUIDv7SessionManager) Generate() string {
	// UUIDv7 includes timestamp for natural sorting and debugging
	return uuid.Must(uuid.NewV7()).String()
}

func (m *UUIDv7SessionManager) Validate(sessionID string) (bool, error) {
	// Validate format
	if _, err := uuid.Parse(sessionID); err != nil {
		return false, err
	}
	
	// Check if terminated
	m.mu.RLock()
	_, isTerminated := m.terminated[sessionID]
	m.mu.RUnlock()
	
	return isTerminated, nil
}

func (m *UUIDv7SessionManager) Terminate(sessionID string) (bool, error) {
	// Validate format first
	if _, err := uuid.Parse(sessionID); err != nil {
		return false, err
	}
	
	m.mu.Lock()
	m.terminated[sessionID] = time.Now()
	m.mu.Unlock()
	
	log.Printf("[SESSION] Terminated: %s", sessionID)
	return false, nil // isNotAllowed=false (we allow termination)
}

// Injected at build time
var commitSHA = "dev"

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
	
	log.Printf("Starting %s (commit=%s, mode=%s, port=%s, global=%v)", serverName, commitSHA, *mode, *port, *globalState)

	// Initialize SSH Pool
	pool := ssh.NewPool(*globalState)

	// Create MCP Server
	mcpServer := server.NewMCPServer(
		serverName,
		commitSHA,
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
		runHTTP(mcpServer, *port, pool)
	default:
		log.Fatalf("Unknown mode: %s. Use 'stdio' or 'http'.", *mode)
	}
}

// createSessionHooks sets up session lifecycle hooks.
// IMPORTANT: When X-Session-Key is present, we use header-based pooling instead of session-based.
// This prevents duplicate managers and ensures connection reuse across MCP session restarts.
func createSessionHooks(pool *ssh.Pool) *server.Hooks {
	hooks := &server.Hooks{}

	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		sessionID := session.SessionID()
		
		// Check if this request has X-Session-Key - if so, use header-based pooling
		if sessionKey, ok := ctx.Value(ssh.SessionKeyContextKey).(string); ok && sessionKey != "" {
			// Touch the header-based manager to keep it alive
			pool.TouchHeader(sessionKey)
			log.Printf("[Session] Started: %s (using header pool: %s)", sessionID, sessionKey)
			return // Don't create session-based manager
		}
		
		// No header - create session-based manager
		log.Printf("[Session] Started: %s (session pool)", sessionID)
		pool.CreateSession(sessionID)
	})

	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		sessionID := session.SessionID()
		
		// If using header-based pooling, release active count (managed by timeout)
		if sessionKey, ok := ctx.Value(ssh.SessionKeyContextKey).(string); ok && sessionKey != "" {
			pool.ReleaseHeader(sessionKey)
			log.Printf("[Session] Ended: %s (header pool: %s retained)", sessionID, sessionKey)
			return
		}
		
		log.Printf("[Session] Ended: %s (session pool destroyed)", sessionID)
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

const sessionKeyHeader = "X-Session-Key"

// runHTTP runs the server in Streamable HTTP mode with graceful shutdown.
// 
// PRODUCTION SECURITY NOTICE:
// This implementation requires additional security layers for production use:
// - TLS/HTTPS: Use WithTLSCert() or run behind a reverse proxy with TLS
// - Authentication: Validate X-Session-Key against authorized keys
// - Authorization: Implement per-user access controls
// - Rate Limiting: Add request throttling
// - Audit Logging: Track all tool invocations with user context
func runHTTP(s *server.MCPServer, port string, pool *ssh.Pool) {
	// Use StreamableHTTPServer with UUIDv7 session IDs and security middleware
	httpSrv := server.NewStreamableHTTPServer(s,
		// Use time-ordered UUIDv7 for professional session IDs
		server.WithSessionIdManager(NewUUIDv7SessionManager()),
		
		// Extract X-Session-Key for session persistence
		server.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			sessionKey := r.Header.Get(sessionKeyHeader)
			if sessionKey != "" {
				// TODO PRODUCTION: Validate sessionKey against authorized keys here
				log.Printf("[SECURITY] Session key received: %s from %s", sessionKey, r.RemoteAddr)
				return context.WithValue(ctx, ssh.SessionKeyContextKey, sessionKey)
			}
			log.Printf("[SECURITY] No session key provided from %s", r.RemoteAddr)
			return ctx
		}),
	)
	
	mux := http.NewServeMux()
	
	// Register the streamable HTTP handler at /mcp
	// This handles both POST requests and GET (SSE) connections
	mux.Handle("/mcp", httpSrv)

	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[HTTP] Listening on :%s/mcp", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-sigChan
	log.Println("[HTTP] Shutting down...")

	// Close SSH pool first (closes all SSH connections) - instant
	pool.Close()

	// Force close HTTP server - don't wait for SSE streams
	// SSE clients will reconnect automatically anyway
	if err := httpServer.Close(); err != nil {
		log.Printf("[HTTP] Close error: %v", err)
	}

	log.Println("[HTTP] Server stopped")
}
