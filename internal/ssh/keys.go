// Package ssh provides SSH connection management for the MCP server.
package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

const (
	// ProductionKeyPath is the Docker/production location for SSH keys.
	ProductionKeyPath = "/data/id_ed25519"
	// DevKeyPath is the local development location for SSH keys.
	DevKeyPath = "./data/id_ed25519"
)

// KeyManager handles SSH key generation and loading.
type KeyManager struct {
	keyPath string
}

// NewKeyManager creates a new KeyManager.
// Automatically detects environment: uses /data for production (Docker),
// ./data for local development.
func NewKeyManager(keyPath string) *KeyManager {
	if keyPath == "" {
		keyPath = getDefaultKeyPath()
	}
	return &KeyManager{keyPath: keyPath}
}

// getDefaultKeyPath returns the appropriate default path based on environment.
func getDefaultKeyPath() string {
	// Check if /data exists (production/Docker environment)
	if stat, err := os.Stat("/data"); err == nil && stat.IsDir() {
		// /data exists - this is production mode, MUST be writable
		return ProductionKeyPath
	}
	// /data doesn't exist - local development mode
	return DevKeyPath
}

// EnsureKey ensures the system key exists, generating if necessary.
func (km *KeyManager) EnsureKey() error {
	keyDir := filepath.Dir(km.keyPath)

	// Check if directory exists
	stat, err := os.Stat(keyDir)
	if os.IsNotExist(err) {
		// Directory doesn't exist
		if keyDir == "/data" {
			// Production mode - FAIL if /data doesn't exist
			return fmt.Errorf("production key directory /data does not exist - ensure volume is mounted")
		}
		// Development mode - create directory
		if err := os.MkdirAll(keyDir, 0700); err != nil {
			return fmt.Errorf("failed to create key directory %s: %w", keyDir, err)
		}
		log.Printf("[KEY] Created key directory: %s", keyDir)
	} else if err != nil {
		return fmt.Errorf("failed to access key directory %s: %w", keyDir, err)
	} else if !stat.IsDir() {
		return fmt.Errorf("key path %s exists but is not a directory", keyDir)
	}

	// Test write permissions by attempting to create a temp file
	testFile := filepath.Join(keyDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		return fmt.Errorf("key directory %s is not writable: %w", keyDir, err)
	}
	os.Remove(testFile)

	if _, err := os.Stat(km.keyPath); os.IsNotExist(err) {
		log.Printf("[KEY] Generating new Ed25519 key pair at %s", km.keyPath)
		return km.generateKey()
	}

	log.Printf("[KEY] Using existing key at %s", km.keyPath)
	return nil
}

// generateKey creates a new Ed25519 key pair.
func (km *KeyManager) generateKey() error {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}

	privKeyBytes, err := ssh.MarshalPrivateKey(privKey, "ssh-mcp")
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := os.WriteFile(km.keyPath, pem.EncodeToMemory(privKeyBytes), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("failed to create SSH public key: %w", err)
	}

	// Add "SSH-MCP" comment to public key for identification
	pubKeyBytes := []byte(fmt.Sprintf("%s %s SSH-MCP\n", 
		sshPubKey.Type(), 
		base64.StdEncoding.EncodeToString(sshPubKey.Marshal())))
	
	if err := os.WriteFile(km.keyPath+".pub", pubKeyBytes, 0644); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	log.Println("[KEY] System key generated successfully")
	return nil
}

// LoadPrivateKey loads the private key from disk.
func (km *KeyManager) LoadPrivateKey() (ssh.Signer, error) {
	keyBytes, err := os.ReadFile(km.keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return signer, nil
}

// GetPublicKey returns the public key string.
func (km *KeyManager) GetPublicKey() (string, error) {
	pubKeyBytes, err := os.ReadFile(km.keyPath + ".pub")
	if err != nil {
		return "", fmt.Errorf("failed to read public key: %w", err)
	}
	return string(pubKeyBytes), nil
}
