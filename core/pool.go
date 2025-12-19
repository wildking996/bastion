package core

import (
	"bastion/config"
	"bastion/models"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

type sshClient interface {
	Dial(network, addr string) (net.Conn, error)
	SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error)
	Close() error
}

type pooledSSHClient struct {
	client          sshClient
	createdAt       time.Time
	lastUsedAt      time.Time
	lastKeepaliveAt time.Time
	activeConnCount int
}

// SSHConnectionPool maintains reusable SSH connections keyed by bastion chain.
type SSHConnectionPool struct {
	mu          sync.Mutex
	pool        map[string]*pooledSSHClient
	createChain func([]models.Bastion) (sshClient, error)

	housekeepingOnce sync.Once
	stopOnce         sync.Once
	stopCh           chan struct{}

	keepaliveFailuresTotal uint64
	idleClosedTotal        uint64
}

var Pool *SSHConnectionPool

func init() {
	Pool = NewSSHConnectionPool()
}

// NewSSHConnectionPool constructs a pool instance.
func NewSSHConnectionPool() *SSHConnectionPool {
	p := &SSHConnectionPool{
		pool:   make(map[string]*pooledSSHClient),
		stopCh: make(chan struct{}),
	}
	p.createChain = p.createSSHChain
	return p
}

// StartHousekeeping starts periodic keepalive probing and idle connection sweeping.
// It is safe to call multiple times.
func (p *SSHConnectionPool) StartHousekeeping() {
	p.housekeepingOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-p.stopCh:
					return
				case <-ticker.C:
					p.housekeep(time.Now())
				}
			}
		}()
	})
}

func (p *SSHConnectionPool) stopHousekeeping() {
	p.stopOnce.Do(func() {
		close(p.stopCh)
	})
}

// getChainKey builds a unique key for a bastion chain.
func (p *SSHConnectionPool) getChainKey(bastions []models.Bastion) string {
	names := make([]string, len(bastions))
	for i, b := range bastions {
		names[i] = b.Name
	}
	return strings.Join(names, "->")
}

// Dial opens a tunneled TCP connection through the bastion chain using the pooled SSH client.
// The returned net.Conn tracks active usage so the pool can safely reclaim idle clients.
func (p *SSHConnectionPool) Dial(bastions []models.Bastion, network, addr string) (net.Conn, error) {
	key := p.getChainKey(bastions)

	entry, err := p.getOrCreateHealthy(key, bastions)
	if err != nil {
		return nil, err
	}

	p.incActive(key, entry, time.Now())
	conn, dialErr := entry.client.Dial(network, addr)
	if dialErr != nil {
		p.decActive(key, entry, time.Now())
		return nil, dialErr
	}

	return &pooledConn{
		Conn: conn,
		release: func() {
			p.decActive(key, entry, time.Now())
		},
	}, nil
}

// GetConnection returns an SSH chain client, creating it if needed.
// Prefer Dial for forwarding paths so the pool can track active usage.
func (p *SSHConnectionPool) GetConnection(bastions []models.Bastion) (sshClient, error) {
	key := p.getChainKey(bastions)
	entry, err := p.getOrCreateHealthy(key, bastions)
	if err != nil {
		return nil, err
	}
	return entry.client, nil
}

func (p *SSHConnectionPool) getOrCreateHealthy(key string, bastions []models.Bastion) (*pooledSSHClient, error) {
	if len(bastions) == 0 {
		return nil, fmt.Errorf("empty bastion chain")
	}

	now := time.Now()
	keepaliveInterval := time.Duration(config.Settings.SSHPoolKeepaliveIntervalSeconds) * time.Second
	keepaliveTimeout := time.Duration(config.Settings.SSHPoolKeepaliveTimeoutMS) * time.Millisecond

	for {
		p.mu.Lock()
		entry := p.pool[key]
		var active int
		var lastKeepalive time.Time
		if entry != nil {
			entry.lastUsedAt = now
			active = entry.activeConnCount
			lastKeepalive = entry.lastKeepaliveAt
		}
		p.mu.Unlock()

		if entry != nil {
			if keepaliveInterval > 0 && now.Sub(lastKeepalive) >= keepaliveInterval && active == 0 {
				if err := sendKeepalive(entry.client, keepaliveTimeout); err != nil {
					atomic.AddUint64(&p.keepaliveFailuresTotal, 1)
					if config.Settings.LogLevel == "DEBUG" {
						log.Printf("SSH keepalive failed for chain %s: %v", key, err)
					}
					p.RemoveConnectionByKey(key)
					continue
				}
				p.updateKeepalive(key, entry, now)
			}

			return entry, nil
		}

		created, err := p.createAndStore(key, bastions, now)
		if err != nil {
			return nil, err
		}
		return created, nil
	}
}

func (p *SSHConnectionPool) createAndStore(key string, bastions []models.Bastion, now time.Time) (*pooledSSHClient, error) {
	maxConns := config.Settings.SSHPoolMaxConns
	var evicted []sshClient

	p.mu.Lock()
	if existing := p.pool[key]; existing != nil {
		p.mu.Unlock()
		return existing, nil
	}

	if maxConns > 0 && len(p.pool) >= maxConns {
		evicted = p.evictIdleLocked(now, len(p.pool)-maxConns+1)
		if len(p.pool) >= maxConns {
			p.mu.Unlock()
			for _, c := range evicted {
				_ = c.Close()
			}
			return nil, fmt.Errorf("ssh pool capacity reached (SSH_POOL_MAX_CONNS=%d)", maxConns)
		}
	}
	p.mu.Unlock()

	for _, c := range evicted {
		_ = c.Close()
	}

	log.Printf("Creating new SSH tunnel chain for: %s", key)
	client, err := p.createChain(bastions)
	if err != nil {
		return nil, fmt.Errorf("failed to establish SSH chain: %w", err)
	}

	entry := &pooledSSHClient{
		client:          client,
		createdAt:       now,
		lastUsedAt:      now,
		lastKeepaliveAt: now,
	}

	p.mu.Lock()
	if existing := p.pool[key]; existing != nil {
		p.mu.Unlock()
		_ = client.Close()
		return existing, nil
	}
	p.pool[key] = entry
	p.mu.Unlock()

	return entry, nil
}

func (p *SSHConnectionPool) evictIdleLocked(now time.Time, need int) []sshClient {
	if need <= 0 {
		return nil
	}

	type candidate struct {
		key     string
		entry   *pooledSSHClient
		lastUse time.Time
	}

	candidates := make([]candidate, 0, len(p.pool))
	for k, e := range p.pool {
		if e == nil || e.activeConnCount != 0 {
			continue
		}
		candidates = append(candidates, candidate{key: k, entry: e, lastUse: e.lastUsedAt})
	}

	// Sort by last used (oldest first).
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].lastUse.Before(candidates[i].lastUse) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	evicted := make([]sshClient, 0, need)
	for _, cand := range candidates {
		if len(evicted) >= need {
			break
		}
		delete(p.pool, cand.key)
		evicted = append(evicted, cand.entry.client)
		log.Printf("Evicting idle SSH connection due to pool capacity: %s", cand.key)
		atomic.AddUint64(&p.idleClosedTotal, 1)
	}

	_ = now // timestamp available for future debugging
	return evicted
}

func (p *SSHConnectionPool) updateKeepalive(key string, entry *pooledSSHClient, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pool[key] != entry {
		return
	}
	entry.lastKeepaliveAt = now
}

func (p *SSHConnectionPool) incActive(key string, entry *pooledSSHClient, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pool[key] != entry {
		return
	}
	entry.activeConnCount++
	entry.lastUsedAt = now
}

func (p *SSHConnectionPool) decActive(key string, entry *pooledSSHClient, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pool[key] != entry {
		return
	}
	if entry.activeConnCount > 0 {
		entry.activeConnCount--
	}
	entry.lastUsedAt = now
}

func (p *SSHConnectionPool) housekeep(now time.Time) {
	idleTimeout := time.Duration(config.Settings.SSHPoolIdleTimeoutSeconds) * time.Second
	keepaliveInterval := time.Duration(config.Settings.SSHPoolKeepaliveIntervalSeconds) * time.Second
	keepaliveTimeout := time.Duration(config.Settings.SSHPoolKeepaliveTimeoutMS) * time.Millisecond

	var idleToClose []sshClient
	var keepaliveCandidates []struct {
		key    string
		entry  *pooledSSHClient
		client sshClient
	}

	p.mu.Lock()
	for key, entry := range p.pool {
		if entry == nil {
			continue
		}

		if idleTimeout > 0 && entry.activeConnCount == 0 && now.Sub(entry.lastUsedAt) >= idleTimeout {
			if config.Settings.LogLevel == "DEBUG" {
				log.Printf("Closing idle SSH connection: %s (idle=%s)", key, now.Sub(entry.lastUsedAt).Truncate(time.Second))
			}
			delete(p.pool, key)
			idleToClose = append(idleToClose, entry.client)
			atomic.AddUint64(&p.idleClosedTotal, 1)
			continue
		}

		if keepaliveInterval > 0 && now.Sub(entry.lastKeepaliveAt) >= keepaliveInterval {
			keepaliveCandidates = append(keepaliveCandidates, struct {
				key    string
				entry  *pooledSSHClient
				client sshClient
			}{key: key, entry: entry, client: entry.client})
		}
	}
	p.mu.Unlock()

	for _, c := range idleToClose {
		_ = c.Close()
	}

	for _, cand := range keepaliveCandidates {
		if err := sendKeepalive(cand.client, keepaliveTimeout); err != nil {
			atomic.AddUint64(&p.keepaliveFailuresTotal, 1)
			if config.Settings.LogLevel == "DEBUG" {
				log.Printf("SSH keepalive failed for chain %s: %v", cand.key, err)
			}

			// Only remove/close when no active conns to avoid killing in-flight channels.
			var toClose sshClient
			p.mu.Lock()
			current := p.pool[cand.key]
			if current != nil && current == cand.entry && current.activeConnCount == 0 {
				delete(p.pool, cand.key)
				toClose = current.client
			} else if current != nil && current == cand.entry {
				current.lastKeepaliveAt = now // throttle repeated probes on broken conns
			}
			p.mu.Unlock()

			if toClose != nil {
				_ = toClose.Close()
			}
			continue
		}

		p.updateKeepalive(cand.key, cand.entry, now)
	}
}

func sendKeepalive(client sshClient, timeout time.Duration) error {
	if client == nil {
		return errors.New("nil ssh client")
	}

	done := make(chan error, 1)
	go func() {
		_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
		done <- err
	}()

	if timeout <= 0 {
		return <-done
	}

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return errors.New("keepalive timeout")
	}
}

type pooledConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *pooledConn) Close() error {
	c.once.Do(func() {
		if c.release != nil {
			c.release()
		}
	})
	return c.Conn.Close()
}

// RemoveConnection removes a specific connection by chain.
func (p *SSHConnectionPool) RemoveConnection(bastions []models.Bastion) {
	if len(bastions) == 0 {
		return
	}
	p.RemoveConnectionByKey(p.getChainKey(bastions))
}

func (p *SSHConnectionPool) RemoveConnectionByKey(key string) {
	var toClose sshClient

	p.mu.Lock()
	entry, exists := p.pool[key]
	if exists {
		delete(p.pool, key)
		toClose = entry.client
	}
	p.mu.Unlock()

	if exists {
		log.Printf("Removing connection: %s", key)
		_ = toClose.Close()
	}
}

// CloseAll closes all pooled connections and stops housekeeping.
func (p *SSHConnectionPool) CloseAll() {
	p.stopHousekeeping()

	p.mu.Lock()
	connsToClose := make(map[string]sshClient, len(p.pool))
	for key, conn := range p.pool {
		if conn != nil {
			connsToClose[key] = conn.client
		}
	}
	p.pool = make(map[string]*pooledSSHClient)
	p.mu.Unlock()

	for key, conn := range connsToClose {
		log.Printf("Closing connection: %s", key)
		if err := conn.Close(); err != nil {
			log.Printf("Error closing connection %s: %v", key, err)
		}
	}
}

// SSHPoolConnections returns the current number of pooled SSH clients.
func (p *SSHConnectionPool) SSHPoolConnections() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.pool)
}

// SSHPoolActiveConns returns the count of in-flight net.Conns opened via the pool.
func (p *SSHConnectionPool) SSHPoolActiveConns() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := 0
	for _, entry := range p.pool {
		if entry != nil {
			total += entry.activeConnCount
		}
	}
	return total
}

// SSHKeepaliveFailuresTotal returns the total number of keepalive failures observed.
func (p *SSHConnectionPool) SSHKeepaliveFailuresTotal() uint64 {
	return atomic.LoadUint64(&p.keepaliveFailuresTotal)
}

// SSHIdleClosedTotal returns the total number of pooled SSH connections closed due to idleness or eviction.
func (p *SSHConnectionPool) SSHIdleClosedTotal() uint64 {
	return atomic.LoadUint64(&p.idleClosedTotal)
}

// createSSHChain builds the SSH chain with retry logic.
func (p *SSHConnectionPool) createSSHChain(bastions []models.Bastion) (sshClient, error) {
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
				_ = conn.Close()
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
					_ = netConn.Close()
					lastErr = err
					continue
				}

				conn = ssh.NewClient(ncc, chans, reqs)
				break
			}
		}

		if lastErr != nil {
			if conn != nil {
				_ = conn.Close()
			}
			return nil, fmt.Errorf("failed to connect to %s after %d attempts: %w", b.Name, maxRetries, lastErr)
		}

		log.Printf("Connected to bastion: %s", b.Name)
	}

	return conn, nil
}

// loadPrivateKey loads a private key (optionally encrypted).
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
