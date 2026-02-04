package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Client represents a single SSH connection with state tracking.
type Client struct {
	alias string
	conn  *ssh.Client
	sftp  *sftp.Client
	cwd   string
	mu    sync.Mutex
	creds Credentials
}

// Credentials holds SSH connection parameters.
type Credentials struct {
	Host       string
	Port       int
	Username   string
	Password   string
	PrivateKey ssh.Signer
	Via        string
}

// NewClient creates a new SSH client.
func NewClient(alias string, creds Credentials, jumpClient *Client) (*Client, error) {
	client := &Client{
		alias: alias,
		creds: creds,
		cwd:   "",
	}

	if err := client.connect(jumpClient); err != nil {
		return nil, err
	}

	return client, nil
}

// connect establishes the SSH connection.
func (c *Client) connect(jumpClient *Client) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	config := &ssh.ClientConfig{
		User:            c.creds.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	var authMethods []ssh.AuthMethod
	if c.creds.PrivateKey != nil {
		authMethods = append(authMethods, ssh.PublicKeys(c.creds.PrivateKey))
	}
	if c.creds.Password != "" {
		authMethods = append(authMethods, ssh.Password(c.creds.Password))
	}
	if len(authMethods) == 0 {
		return errors.New("no authentication method provided (key or password required)")
	}
	config.Auth = authMethods

	addr := fmt.Sprintf("%s:%d", c.creds.Host, c.creds.Port)

	var conn *ssh.Client
	var err error

	if jumpClient != nil {
		jumpConn := jumpClient.conn
		if jumpConn == nil {
			return errors.New("jump host not connected")
		}

		netConn, err := jumpConn.Dial("tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to dial through jump host: %w", err)
		}

		ncc, chans, reqs, err := ssh.NewClientConn(netConn, addr, config)
		if err != nil {
			netConn.Close()
			return fmt.Errorf("failed to create client connection through jump: %w", err)
		}

		conn = ssh.NewClient(ncc, chans, reqs)
	} else {
		conn, err = ssh.Dial("tcp", addr, config)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
	}

	c.conn = conn

	output, err := c.runRaw("pwd")
	if err != nil {
		log.Printf("[SSH] Warning: failed to get initial CWD: %v", err)
		c.cwd = "~"
	} else {
		c.cwd = strings.TrimSpace(output)
	}

	log.Printf("[SSH] Connected to %s@%s (alias: %s)", c.creds.Username, c.creds.Host, c.alias)
	return nil
}

// runRaw executes a command without CWD handling.
func (c *Client) runRaw(cmd string) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	return string(output), err
}

// Run executes a command with CWD tracking.
func (c *Client) Run(ctx context.Context, cmd string) (*RunResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, errors.New("not connected")
	}

	delimiter := fmt.Sprintf("___MCP_PWD_%d___", time.Now().UnixNano())
	wrappedCmd := fmt.Sprintf(
		`cd %q && %s; __EXIT__=$?; echo ""; echo "%s"; pwd; exit $__EXIT__`,
		c.cwd, cmd, delimiter,
	)

	session, err := c.conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()

	if err := session.Start(wrappedCmd); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	type readResult struct {
		stdout []byte
		stderr []byte
	}
	resultChan := make(chan readResult, 1)

	go func() {
		stdoutBytes, _ := io.ReadAll(stdout)
		stderrBytes, _ := io.ReadAll(stderr)
		resultChan <- readResult{stdout: stdoutBytes, stderr: stderrBytes}
	}()

	var res readResult
	select {
	case <-ctx.Done():
		session.Signal(ssh.SIGKILL)
		return nil, ctx.Err()
	case res = <-resultChan:
	}

	var exitCode int
	if err := session.Wait(); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, fmt.Errorf("command failed: %w", err)
		}
	}

	stdoutStr := string(res.stdout)
	cleanOutput := stdoutStr
	if idx := strings.Index(stdoutStr, delimiter); idx != -1 {
		cleanOutput = stdoutStr[:idx]
		remaining := strings.TrimSpace(stdoutStr[idx+len(delimiter):])
		if remaining != "" {
			c.cwd = remaining
		}
	}

	return &RunResult{
		Stdout:   strings.TrimSpace(cleanOutput),
		Stderr:   strings.TrimSpace(string(res.stderr)),
		ExitCode: exitCode,
		CWD:      c.cwd,
	}, nil
}

// RunResult contains command execution result.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	CWD      string
}

// SFTP returns the SFTP client.
func (c *Client) SFTP() (*sftp.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, errors.New("not connected")
	}

	if c.sftp != nil {
		return c.sftp, nil
	}

	sftpClient, err := sftp.NewClient(c.conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	c.sftp = sftpClient
	return c.sftp, nil
}

// Close closes the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sftp != nil {
		c.sftp.Close()
		c.sftp = nil
	}

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}

	return nil
}

// Alias returns the connection alias.
func (c *Client) Alias() string {
	return c.alias
}

// CWD returns the current working directory.
func (c *Client) CWD() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cwd
}

// Reconnect attempts to reconnect.
func (c *Client) Reconnect(jumpClient *Client) error {
	log.Printf("[SSH] Reconnecting %s...", c.alias)
	return c.connect(jumpClient)
}

// IsConnected returns connection status.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}
