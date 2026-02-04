// Package ssh provides SSH connection management for the MCP server.
package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

const (
	// DefaultKeyPath is the standard location for the system SSH key.
	DefaultKeyPath = "/data/id_ed25519"
)

// KeyManager handles SSH key generation and loading.
type KeyManager struct {
	keyPath string
}

// NewKeyManager creates a new KeyManager.
func NewKeyManager(keyPath string) *KeyManager {
	if keyPath == "" {
		keyPath = DefaultKeyPath
	}
	return &KeyManager{keyPath: keyPath}
}

// EnsureKey ensures the system key exists, generating if necessary.
func (km *KeyManager) EnsureKey() error {
	keyDir := filepath.Dir(km.keyPath)

	if _, err := os.Stat(keyDir); os.IsNotExist(err) {
		return fmt.Errorf("key directory does not exist: %s. Mount a volume to /data", keyDir)
	}

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

	pubKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)
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
