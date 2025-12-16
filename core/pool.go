package core

import (
	"bastion/config"
	"bastion/models"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHConnectionPool maintains SSH connections
type SSHConnectionPool struct {
	pool map[string]*ssh.Client
	mu   sync.RWMutex
}

var Pool *SSHConnectionPool

func init() {
	Pool = &SSHConnectionPool{
		pool: make(map[string]*ssh.Client),
	}
}

// getChainKey builds a unique key for a bastion chain
func (p *SSHConnectionPool) getChainKey(bastions []models.Bastion) string {
	names := make([]string, len(bastions))
	for i, b := range bastions {
		names[i] = b.Name
	}
	return strings.Join(names, "->")
}

// GetConnection returns an existing SSH chain or creates one
func (p *SSHConnectionPool) GetConnection(bastions []models.Bastion) (*ssh.Client, error) {
	if len(bastions) == 0 {
		return nil, fmt.Errorf("empty bastion chain")
	}

	key := p.getChainKey(bastions)

	// First try read lock for existing connection
	p.mu.RLock()
	conn, exists := p.pool[key]
	p.mu.RUnlock()

	if exists {
		// Health-check outside lock to avoid holding read lock too long
		if p.isConnectionHealthy(conn) {
			return conn, nil
		}
		// Unhealthy connection; rebuild
	}

	// Rebuild connection with write lock
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check in case another goroutine rebuilt it
	if conn, exists := p.pool[key]; exists {
		// Re-verify health
		if p.isConnectionHealthy(conn) {
			return conn, nil
		}
		// Connection is dead; clean up
		log.Printf("Connection for chain %s is closed, cleaning up", key)
		conn.Close()
		delete(p.pool, key)
	}

	// Create new connection
	log.Printf("Creating new SSH tunnel chain for: %s", key)
	conn, err := p.createChain(bastions)
	if err != nil {
		return nil, fmt.Errorf("failed to establish SSH chain: %w", err)
	}

	p.pool[key] = conn
	return conn, nil
}

// isConnectionHealthy verifies the connection via a lightweight session
func (p *SSHConnectionPool) isConnectionHealthy(conn *ssh.Client) bool {
	session, err := conn.NewSession()
	if err != nil {
		return false
	}
	session.Close()
	return true
}

// createChain builds the SSH chain with retry logic
func (p *SSHConnectionPool) createChain(bastions []models.Bastion) (*ssh.Client, error) {
	var conn *ssh.Client
	var err error
	maxRetries := 3
	retryDelay := 2 * time.Second

	for i, b := range bastions {
		sshConfig := &ssh.ClientConfig{
			User:            b.Username,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         time.Duration(config.Settings.SSHConnectTimeout) * time.Second,
		}

		// Configure auth methods
		if b.PkeyPath != "" {
			key, err := loadPrivateKey(b.PkeyPath, b.PkeyPassphrase)
			if err != nil {
				log.Printf("Failed to load key for %s: %v", b.Name, err)
			} else {
				sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(key))
			}
		}

		if b.Password != "" {
			sshConfig.Auth = append(sshConfig.Auth, ssh.Password(b.Password))
		}

		// Ensure at least one auth method
		if len(sshConfig.Auth) == 0 {
			if conn != nil {
				conn.Close()
			}
			return nil, fmt.Errorf("no authentication method configured for %s", b.Name)
		}

		addr := fmt.Sprintf("%s:%d", b.Host, b.Port)

		// Retry logic
		var lastErr error
		for retry := 0; retry < maxRetries; retry++ {
			if retry > 0 {
				log.Printf("Retrying connection to %s (attempt %d/%d)", b.Name, retry+1, maxRetries)
				time.Sleep(retryDelay)
			}

			if i == 0 {
				// First hop: connect directly
				conn, err = ssh.Dial("tcp", addr, sshConfig)
				if err == nil {
					break
				}
				lastErr = err
			} else {
				// Subsequent hops: tunnel through previous connection
				netConn, err := conn.Dial("tcp", addr)
				if err != nil {
					lastErr = err
					continue
				}

				ncc, chans, reqs, err := ssh.NewClientConn(netConn, addr, sshConfig)
				if err != nil {
					netConn.Close()
					lastErr = err
					continue
				}

				conn = ssh.NewClient(ncc, chans, reqs)
				break
			}
		}

		if lastErr != nil {
			if conn != nil {
				conn.Close()
			}
			return nil, fmt.Errorf("failed to connect to %s after %d attempts: %w", b.Name, maxRetries, lastErr)
		}

		log.Printf("Connected to bastion: %s", b.Name)
	}

	return conn, nil
}

// loadPrivateKey loads a private key (optionally encrypted)
func loadPrivateKey(path, passphrase string) (ssh.Signer, error) {
	// Expand ~ to user home
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = strings.Replace(path, "~", home, 1)
		}
	}

	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var signer ssh.Signer
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}

	return signer, err
}

// CloseAll closes all pooled connections
func (p *SSHConnectionPool) CloseAll() {
	p.mu.Lock()
	// Copy connections to avoid slow operations under lock
	connsToClose := make(map[string]*ssh.Client, len(p.pool))
	for key, conn := range p.pool {
		connsToClose[key] = conn
	}
	// Clear pool
	p.pool = make(map[string]*ssh.Client)
	p.mu.Unlock()

	// Close connections outside lock to avoid blocking others
	for key, conn := range connsToClose {
		log.Printf("Closing connection: %s", key)
		if err := conn.Close(); err != nil {
			log.Printf("Error closing connection %s: %v", key, err)
		}
	}
}

// RemoveConnection removes a specific connection
func (p *SSHConnectionPool) RemoveConnection(bastions []models.Bastion) {
	if len(bastions) == 0 {
		return
	}

	key := p.getChainKey(bastions)

	p.mu.Lock()
	conn, exists := p.pool[key]
	if exists {
		delete(p.pool, key)
	}
	p.mu.Unlock()

	// Close connection outside the lock
	if exists {
		log.Printf("Removing connection: %s", key)
		conn.Close()
	}
}
