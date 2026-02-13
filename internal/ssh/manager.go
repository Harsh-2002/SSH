package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

// Manager manages multiple SSH connections for a single session.
type Manager struct {
	connections map[string]*Client
	primary     string
	keyManager  *KeyManager
	mu          sync.RWMutex
	aliasLocks  map[string]*sync.Mutex
}

// NewManager creates a new SSH connection manager.
func NewManager(keyPath string) *Manager {
	mgr := &Manager{
		connections: make(map[string]*Client),
		keyManager:  NewKeyManager(keyPath),
		aliasLocks:  make(map[string]*sync.Mutex),
	}

	if err := mgr.keyManager.EnsureKey(); err != nil {
		log.Printf("[Manager] Warning: %v", err)
	}

	return mgr
}

// getAliasLock returns a per-alias lock.
func (m *Manager) getAliasLock(alias string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()

	if lock, ok := m.aliasLocks[alias]; ok {
		return lock
	}

	lock := &sync.Mutex{}
	m.aliasLocks[alias] = lock
	return lock
}

// generateAlias creates a unique alias and reserves it.
func (m *Manager) generateAlias(username, host string) string {
	base := fmt.Sprintf("%s@%s", username, host)

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.connections[base]; !exists {
		m.connections[base] = nil // Reserve
		return base
	}

	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, exists := m.connections[candidate]; !exists {
			m.connections[candidate] = nil // Reserve
			return candidate
		}
	}

	// Fallback (might collide but highly unlikely to fill 100 slots)
	final := fmt.Sprintf("%s-%d", base, 100)
	m.connections[final] = nil
	return final
}

// ConnectOptions contains options for SSH connection.
type ConnectOptions struct {
	Host           string
	Port           int
	Username       string
	Password       string
	PrivateKeyPath string
	Alias          string
	Via            string
}

// Connect establishes an SSH connection and returns the alias.
func (m *Manager) Connect(ctx context.Context, opts ConnectOptions) (alias string, err error) {
	if opts.Port == 0 {
		opts.Port = 22
	}

	var reserved bool
	if opts.Alias == "" {
		opts.Alias = m.generateAlias(opts.Username, opts.Host)
		reserved = true
	}

	if opts.Via == opts.Alias {
		return "", errors.New("'via' cannot be the same as 'alias'")
	}

	m.mu.Lock()
	existing, exists := m.connections[opts.Alias]
	if exists {
		if existing != nil {
			m.mu.Unlock()
			if existing.creds.Host == opts.Host && existing.creds.Username == opts.Username {
				return opts.Alias, nil
			}
			return "", fmt.Errorf("alias '%s' already exists for %s@%s", opts.Alias, existing.creds.Username, existing.creds.Host)
		}
		// Existing is nil (reserved)
		if !reserved {
			m.mu.Unlock()
			return "", fmt.Errorf("alias '%s' is currently connecting/reserved", opts.Alias)
		}
		// It's our reservation, proceed
	} else {
		// New explicit alias
		m.connections[opts.Alias] = nil // Reserve
	}
	m.mu.Unlock()

	// Defer cleanup of reservation on error
	defer func() {
		if err != nil {
			m.mu.Lock()
			// Only remove if it's still nil (failed to connect)
			if c, ok := m.connections[opts.Alias]; ok && c == nil {
				delete(m.connections, opts.Alias)
			}
			m.mu.Unlock()
		}
	}()

	creds := Credentials{
		Host:     opts.Host,
		Port:     opts.Port,
		Username: opts.Username,
		Password: opts.Password,
		Via:      opts.Via,
	}

	if opts.PrivateKeyPath != "" {
		keyBytes, err := os.ReadFile(opts.PrivateKeyPath)
		if err != nil {
			return "", fmt.Errorf("failed to read private key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return "", fmt.Errorf("failed to parse private key: %w", err)
		}
		creds.PrivateKey = signer
	} else if opts.Password == "" {
		signer, err := m.keyManager.LoadPrivateKey()
		if err != nil {
			return "", fmt.Errorf("no auth provided and system key unavailable: %w", err)
		}
		creds.PrivateKey = signer
		log.Printf("[Manager] Using system key for %s", opts.Alias)
	}

	var jumpClient *Client
	if opts.Via != "" {
		m.mu.RLock()
		jumpClient = m.connections[opts.Via]
		m.mu.RUnlock()
		if jumpClient == nil {
			return "", fmt.Errorf("jump host '%s' not connected", opts.Via)
		}
	}

	client, err := NewClient(opts.Alias, creds, jumpClient)
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	m.connections[opts.Alias] = client
	if m.primary == "" {
		m.primary = opts.Alias
	}
	m.mu.Unlock()

	return opts.Alias, nil
}

// Disconnect closes one or all connections.
func (m *Manager) Disconnect(alias string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if alias == "" {
		count := 0
		for a, client := range m.connections {
			if client != nil {
				client.Close()
				count++
			}
			delete(m.connections, a)
		}
		m.primary = ""
		return fmt.Sprintf("Disconnected all (%d) connections", count), nil
	}

	client, ok := m.connections[alias]
	if !ok {
		return "", fmt.Errorf("no connection with alias '%s'", alias)
	}

	if client != nil {
		client.Close()
	}
	delete(m.connections, alias)

	if m.primary == alias {
		m.primary = ""
		for a, c := range m.connections {
			if c != nil {
				m.primary = a
				break
			}
		}
	}

	return fmt.Sprintf("Disconnected '%s'", alias), nil
}

// resolveTarget returns the target alias.
func (m *Manager) resolveTarget(target string) (string, error) {
	if target != "" && target != "primary" {
		m.mu.RLock()
		_, ok := m.connections[target]
		m.mu.RUnlock()
		if !ok {
			return "", fmt.Errorf("no connection with alias '%s'", target)
		}
		return target, nil
	}

	m.mu.RLock()
	primary := m.primary
	m.mu.RUnlock()

	if primary == "" {
		return "", errors.New("no active connection")
	}
	return primary, nil
}

// Run executes a command on the target connection.
func (m *Manager) Run(ctx context.Context, cmd, target string) (*RunResult, error) {
	alias, err := m.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	lock := m.getAliasLock(alias)
	lock.Lock()
	defer lock.Unlock()

	m.mu.RLock()
	client := m.connections[alias]
	m.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("connection '%s' not found", alias)
	}

	result, err := client.Run(ctx, cmd)
	if err != nil {
		if isConnectionError(err) {
			log.Printf("[Manager] Connection lost for %s, reconnecting...", alias)
			if reconnErr := client.Reconnect(m.getJumpClient(client.creds.Via)); reconnErr != nil {
				return nil, fmt.Errorf("reconnect failed: %w", reconnErr)
			}
			return client.Run(ctx, cmd)
		}
		return nil, err
	}

	return result, nil
}

// getJumpClient returns the jump client.
func (m *Manager) getJumpClient(via string) *Client {
	if via == "" {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[via]
}

// isConnectionError checks if error indicates lost connection.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "connection refused")
}

// Execute runs a command and returns formatted output.
func (m *Manager) Execute(ctx context.Context, cmd, target string) (string, error) {
	result, err := m.Run(ctx, cmd, target)
	if err != nil {
		return "", err
	}

	var output strings.Builder
	if result.Stdout != "" {
		output.WriteString(result.Stdout)
	}
	if result.Stderr != "" {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString(result.Stderr)
	}

	if output.Len() == 0 {
		return "(No output)", nil
	}

	if result.ExitCode != 0 {
		output.WriteString(fmt.Sprintf("\n[Exit Code: %d]", result.ExitCode))
	}

	// Truncate if too long
	const maxBytes = 51200
	outputStr := output.String()
	if len(outputStr) > maxBytes {
		outputStr = outputStr[:maxBytes] + "\n... [Output truncated]"
	}

	return outputStr, nil
}

// resolvePath resolves a path to an absolute path using the connection's CWD.
// No path restrictions — the connected user's OS permissions are the only boundary.
func (m *Manager) resolvePath(path, alias string) string {
	m.mu.RLock()
	client := m.connections[alias]
	m.mu.RUnlock()

	cwd := "/"
	if client != nil {
		cwd = client.CWD()
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	return filepath.Clean(path)
}

// IsRoot checks whether the connection for the given target alias is logged in as root.
func (m *Manager) IsRoot(target string) bool {
	alias, err := m.resolveTarget(target)
	if err != nil {
		return false
	}

	m.mu.RLock()
	client := m.connections[alias]
	m.mu.RUnlock()

	if client == nil {
		return false
	}
	return client.creds.Username == "root"
}

// SudoPrefix returns "sudo " if the connection user is not root, empty string otherwise.
// This allows tools to automatically elevate privileges when needed.
func (m *Manager) SudoPrefix(target string) string {
	if m.IsRoot(target) {
		return ""
	}
	return "sudo "
}

// ReadFile reads a file.
func (m *Manager) ReadFile(ctx context.Context, path, target string) (string, error) {
	alias, err := m.resolveTarget(target)
	if err != nil {
		return "", err
	}

	resolved := m.resolvePath(path, alias)

	lock := m.getAliasLock(alias)
	lock.Lock()
	defer lock.Unlock()

	m.mu.RLock()
	client := m.connections[alias]
	m.mu.RUnlock()

	sftpClient, err := client.SFTP()
	if err != nil {
		return "", err
	}

	file, err := sftpClient.Open(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// WriteFile writes content to a file.
func (m *Manager) WriteFile(ctx context.Context, path, content, target string) error {
	alias, err := m.resolveTarget(target)
	if err != nil {
		return err
	}

	resolved := m.resolvePath(path, alias)

	lock := m.getAliasLock(alias)
	lock.Lock()
	defer lock.Unlock()

	m.mu.RLock()
	client := m.connections[alias]
	m.mu.RUnlock()

	sftpClient, err := client.SFTP()
	if err != nil {
		return err
	}

	file, err := sftpClient.Create(resolved)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = file.Write([]byte(content))
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ListDir lists directory contents.
func (m *Manager) ListDir(ctx context.Context, path, target string) ([]FileInfo, error) {
	alias, err := m.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	resolved := m.resolvePath(path, alias)

	lock := m.getAliasLock(alias)
	lock.Lock()
	defer lock.Unlock()

	m.mu.RLock()
	client := m.connections[alias]
	m.mu.RUnlock()

	sftpClient, err := client.SFTP()
	if err != nil {
		return nil, err
	}

	entries, err := sftpClient.ReadDir(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	var files []FileInfo
	for _, entry := range entries {
		ftype := "file"
		if entry.IsDir() {
			ftype = "dir"
		}
		files = append(files, FileInfo{
			Name:        entry.Name(),
			Type:        ftype,
			Size:        entry.Size(),
			Permissions: entry.Mode().String(),
		})
	}

	return files, nil
}

// FileInfo represents file metadata.
type FileInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Size        int64  `json:"size"`
	Permissions string `json:"permissions"`
}

// GetPublicKey returns the system's public SSH key.
func (m *Manager) GetPublicKey() (string, error) {
	return m.keyManager.GetPublicKey()
}

// ListConnections returns all active connection aliases.
func (m *Manager) ListConnections() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	aliases := make([]string, 0, len(m.connections))
	for a := range m.connections {
		aliases = append(aliases, a)
	}
	return aliases
}

// Close closes all connections.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.connections {
		if client != nil {
			client.Close()
		}
	}
	m.connections = make(map[string]*Client)
	m.primary = ""
}
