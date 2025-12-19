package core

import (
	"errors"
	"net"
	"testing"
	"time"

	"bastion/config"
	"bastion/models"
)

type fakeSSHClient struct {
	sendErr error
	dialErr error
	closed  bool
}

func (f *fakeSSHClient) Dial(network, addr string) (net.Conn, error) {
	if f.dialErr != nil {
		return nil, f.dialErr
	}
	c1, c2 := net.Pipe()
	_ = c2.Close()
	return c1, nil
}

func (f *fakeSSHClient) SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error) {
	return false, nil, f.sendErr
}

func (f *fakeSSHClient) Close() error {
	f.closed = true
	return nil
}

func TestSSHConnectionPool_Housekeep_IdleClose(t *testing.T) {
	oldIdle := config.Settings.SSHPoolIdleTimeoutSeconds
	oldKeepalive := config.Settings.SSHPoolKeepaliveIntervalSeconds
	t.Cleanup(func() {
		config.Settings.SSHPoolIdleTimeoutSeconds = oldIdle
		config.Settings.SSHPoolKeepaliveIntervalSeconds = oldKeepalive
	})
	config.Settings.SSHPoolIdleTimeoutSeconds = 10
	config.Settings.SSHPoolKeepaliveIntervalSeconds = 0

	pool := NewSSHConnectionPool()
	pool.createChain = func(_ []models.Bastion) (sshClient, error) {
		return &fakeSSHClient{}, nil
	}

	now := time.Now()
	_, err := pool.GetConnection([]models.Bastion{{Name: "b1"}})
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}

	pool.mu.Lock()
	entry := pool.pool["b1"]
	entry.lastUsedAt = now.Add(-time.Hour)
	pool.mu.Unlock()

	pool.housekeep(now)

	if got := pool.SSHPoolConnections(); got != 0 {
		t.Fatalf("expected pool empty, got %d", got)
	}
	if got := pool.SSHIdleClosedTotal(); got != 1 {
		t.Fatalf("expected idle_closed_total=1, got %d", got)
	}
}

func TestSSHConnectionPool_Housekeep_DoesNotCloseActive(t *testing.T) {
	oldIdle := config.Settings.SSHPoolIdleTimeoutSeconds
	oldKeepalive := config.Settings.SSHPoolKeepaliveIntervalSeconds
	t.Cleanup(func() {
		config.Settings.SSHPoolIdleTimeoutSeconds = oldIdle
		config.Settings.SSHPoolKeepaliveIntervalSeconds = oldKeepalive
	})
	config.Settings.SSHPoolIdleTimeoutSeconds = 10
	config.Settings.SSHPoolKeepaliveIntervalSeconds = 0

	pool := NewSSHConnectionPool()
	pool.createChain = func(_ []models.Bastion) (sshClient, error) {
		return &fakeSSHClient{}, nil
	}

	now := time.Now()
	_, err := pool.GetConnection([]models.Bastion{{Name: "b1"}})
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}

	pool.mu.Lock()
	entry := pool.pool["b1"]
	entry.lastUsedAt = now.Add(-time.Hour)
	entry.activeConnCount = 1
	pool.mu.Unlock()

	pool.housekeep(now)

	if got := pool.SSHPoolConnections(); got != 1 {
		t.Fatalf("expected pool size 1, got %d", got)
	}
}

func TestSSHConnectionPool_Housekeep_KeepaliveFailureRemovesIdle(t *testing.T) {
	oldIdle := config.Settings.SSHPoolIdleTimeoutSeconds
	oldKeepalive := config.Settings.SSHPoolKeepaliveIntervalSeconds
	oldTimeout := config.Settings.SSHPoolKeepaliveTimeoutMS
	t.Cleanup(func() {
		config.Settings.SSHPoolIdleTimeoutSeconds = oldIdle
		config.Settings.SSHPoolKeepaliveIntervalSeconds = oldKeepalive
		config.Settings.SSHPoolKeepaliveTimeoutMS = oldTimeout
	})
	config.Settings.SSHPoolIdleTimeoutSeconds = 0
	config.Settings.SSHPoolKeepaliveIntervalSeconds = 1
	config.Settings.SSHPoolKeepaliveTimeoutMS = 0

	pool := NewSSHConnectionPool()
	pool.createChain = func(_ []models.Bastion) (sshClient, error) {
		return &fakeSSHClient{sendErr: errors.New("keepalive failed")}, nil
	}

	now := time.Now()
	_, err := pool.GetConnection([]models.Bastion{{Name: "b1"}})
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}

	pool.mu.Lock()
	entry := pool.pool["b1"]
	entry.lastKeepaliveAt = now.Add(-time.Minute)
	pool.mu.Unlock()

	pool.housekeep(now)

	if got := pool.SSHPoolConnections(); got != 0 {
		t.Fatalf("expected pool empty, got %d", got)
	}
	if got := pool.SSHKeepaliveFailuresTotal(); got != 1 {
		t.Fatalf("expected keepalive_failures_total=1, got %d", got)
	}
}

func TestSSHConnectionPool_Capacity_EvictsIdleOrErrorsWhenAllActive(t *testing.T) {
	oldMax := config.Settings.SSHPoolMaxConns
	oldKeepalive := config.Settings.SSHPoolKeepaliveIntervalSeconds
	t.Cleanup(func() {
		config.Settings.SSHPoolMaxConns = oldMax
		config.Settings.SSHPoolKeepaliveIntervalSeconds = oldKeepalive
	})
	config.Settings.SSHPoolMaxConns = 1
	config.Settings.SSHPoolKeepaliveIntervalSeconds = 0

	pool := NewSSHConnectionPool()
	pool.createChain = func(_ []models.Bastion) (sshClient, error) {
		return &fakeSSHClient{}, nil
	}

	_, err := pool.GetConnection([]models.Bastion{{Name: "a"}})
	if err != nil {
		t.Fatalf("GetConnection a: %v", err)
	}

	pool.mu.Lock()
	pool.pool["a"].activeConnCount = 1
	pool.mu.Unlock()

	_, err = pool.GetConnection([]models.Bastion{{Name: "b"}})
	if err == nil {
		t.Fatalf("expected capacity error when all conns are active")
	}

	pool.mu.Lock()
	pool.pool["a"].activeConnCount = 0
	pool.pool["a"].lastUsedAt = time.Now().Add(-time.Hour)
	pool.mu.Unlock()

	_, err = pool.GetConnection([]models.Bastion{{Name: "b"}})
	if err != nil {
		t.Fatalf("expected eviction to allow new conn: %v", err)
	}
	if got := pool.SSHPoolConnections(); got != 1 {
		t.Fatalf("expected pool size 1, got %d", got)
	}
}
