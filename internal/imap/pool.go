package imap

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hkdb/aerion/internal/logging"
	"github.com/rs/zerolog"
)

// ErrPoolClosed is returned by GetConnection after Close.
var ErrPoolClosed = errors.New("imap pool is closed")

// IsConnectionError checks if an error indicates a dead/broken connection.
// These errors warrant discarding the connection and getting a new one from the pool.
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	connectionErrors := []string{
		"use of closed network connection",
		"connection reset",
		"broken pipe",
		"EOF",
		"i/o timeout",
		"connection refused",
		"no such host",
		"network is unreachable",
	}
	for _, connErr := range connectionErrors {
		if strings.Contains(errStr, connErr) {
			return true
		}
	}
	return false
}

// PoolConfig configures the connection pool
type PoolConfig struct {
	// MaxConnections is the maximum number of connections per account
	MaxConnections int

	// IdleTimeout is how long a connection can be idle before being closed
	IdleTimeout time.Duration

	// ConnectTimeout is how long to wait for a connection to be established
	ConnectTimeout time.Duration

	// WaiterTimeout is max time to wait for a connection when pool is exhausted
	WaiterTimeout time.Duration
}

// DefaultPoolConfig returns sensible defaults for the pool
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConnections: 3,
		IdleTimeout:    5 * time.Minute,
		ConnectTimeout: 30 * time.Second,
		WaiterTimeout:  2 * time.Minute, // Don't wait forever for a connection
	}
}

// PooledConnection wraps a Client with pool metadata
type PooledConnection struct {
	client    *Client
	accountID string
	createdAt time.Time
	lastUsed  time.Time
	inUse     bool
	mu        sync.Mutex
}

// IsHealthy checks if the connection is still usable (acquires lock)
func (pc *PooledConnection) IsHealthy() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.isHealthyLocked()
}

// isHealthyLocked checks health without acquiring lock (caller must hold lock).
// Sends a NOOP to verify the connection is alive — catches closed sockets
// that still have non-nil client references (e.g., after server-side disconnect).
func (pc *PooledConnection) isHealthyLocked() bool {
	if pc.client == nil || pc.client.client == nil {
		return false
	}

	// Send NOOP to verify the connection is actually alive.
	// This catches closed sockets (e.g., Proton Bridge dropping idle connections).
	if err := pc.client.client.Noop().Wait(); err != nil {
		return false
	}
	return true
}

// Pool manages IMAP connections for multiple accounts
type Pool struct {
	config      PoolConfig
	connections map[string][]*PooledConnection // accountID -> connections
	dialing     map[string]int                 // accountID -> dials in flight, counted against MaxConnections
	waiters     map[string][]chan *PooledConnection
	mu          sync.Mutex
	closed      bool // set by Close; GetConnection then returns ErrPoolClosed
	log         zerolog.Logger

	// Credentials provider function
	getCredentials func(accountID string) (*ClientConfig, error)
}

// NewPool creates a new connection pool
func NewPool(config PoolConfig, getCredentials func(accountID string) (*ClientConfig, error)) *Pool {
	return &Pool{
		config:         config,
		connections:    make(map[string][]*PooledConnection),
		dialing:        make(map[string]int),
		waiters:        make(map[string][]chan *PooledConnection),
		log:            logging.WithComponent("imap-pool"),
		getCredentials: getCredentials,
	}
}

// GetConnection gets or creates a connection for an account
func (p *Pool) GetConnection(ctx context.Context, accountID string) (*PooledConnection, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}

	// Try to find an available connection, dropping idle dead ones so they
	// stop counting toward the limit.
	var dead []*PooledConnection
	for _, conn := range p.connections[accountID] {
		conn.mu.Lock()
		if conn.inUse {
			conn.mu.Unlock()
			continue
		}
		if !conn.isHealthyLocked() {
			conn.mu.Unlock()
			dead = append(dead, conn)
			continue
		}
		conn.inUse = true
		conn.lastUsed = time.Now()
		conn.mu.Unlock()
		for _, d := range dead {
			p.discardLocked(d)
		}
		p.mu.Unlock()

		p.log.Debug().
			Str("account", accountID).
			Msg("Reusing existing connection")
		return conn, nil
	}
	for _, d := range dead {
		p.discardLocked(d)
	}

	// Count current connections for this account, including dials in
	// flight so concurrent callers can't all claim the last slot.
	currentCount := len(p.connections[accountID]) + p.dialing[accountID]

	// Can we create a new one?
	if currentCount < p.config.MaxConnections {
		p.dialing[accountID]++
		p.mu.Unlock()
		return p.createConnection(ctx, accountID)
	}

	// At limit - must wait
	p.log.Debug().
		Str("account", accountID).
		Int("current", currentCount).
		Int("max", p.config.MaxConnections).
		Msg("Connection pool exhausted, waiting")

	waiter := make(chan *PooledConnection, 1)
	p.waiters[accountID] = append(p.waiters[accountID], waiter)
	p.mu.Unlock()

	// Wait for a connection, context cancellation, or timeout
	select {
	case conn, ok := <-waiter:
		if !ok {
			// Channel was closed by CloseAccount/CloseAll — pool is being cleared
			return nil, fmt.Errorf("connection pool closed")
		}
		if conn == nil {
			// A dead connection was removed, freeing a slot — retry so a
			// replacement can be created.
			return p.GetConnection(ctx, accountID)
		}
		return conn, nil
	case <-ctx.Done():
		p.abandonWait(accountID, waiter)
		return nil, ctx.Err()
	case <-time.After(p.config.WaiterTimeout):
		// Timeout waiting for connection - pool may be deadlocked
		p.abandonWait(accountID, waiter)
		p.log.Warn().
			Str("account", accountID).
			Dur("timeout", p.config.WaiterTimeout).
			Msg("Timed out waiting for connection from pool")
		return nil, fmt.Errorf("timed out waiting for connection from pool")
	}
}

// abandonWait removes a waiter that stopped waiting. If Release or a discard
// already dequeued it, the value sent to it (under p.mu, so already buffered)
// is passed on: a connection is released instead of staying in use forever,
// and a freed slot wakes the next waiter.
func (p *Pool) abandonWait(accountID string, waiter chan *PooledConnection) {
	p.mu.Lock()
	waiters := p.waiters[accountID]
	for i, w := range waiters {
		if w == waiter {
			p.waiters[accountID] = append(waiters[:i], waiters[i+1:]...)
			p.mu.Unlock()
			return
		}
	}
	p.mu.Unlock()

	select {
	case conn, ok := <-waiter:
		if !ok {
			return
		}
		if conn != nil {
			p.Release(conn)
			return
		}
		p.mu.Lock()
		p.wakeWaiterLocked(accountID)
		p.mu.Unlock()
	default:
	}
}

// wakeWaiterLocked tells the first waiter for accountID that a slot is free
// so it retries. Caller must hold p.mu.
func (p *Pool) wakeWaiterLocked(accountID string) {
	if waiters := p.waiters[accountID]; len(waiters) > 0 {
		p.waiters[accountID] = waiters[1:]
		waiters[0] <- nil
	}
}

// createConnection dials a new connection in a slot the caller reserved by
// incrementing p.dialing, and adds it to the pool. On failure the slot is
// freed and the next waiter is woken to use it.
func (p *Pool) createConnection(ctx context.Context, accountID string) (*PooledConnection, error) {
	client, err := p.dialWithRetry(ctx, accountID, 0)

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dialing[accountID]--; p.dialing[accountID] <= 0 {
		delete(p.dialing, accountID)
	}
	if err != nil {
		p.wakeWaiterLocked(accountID)
		return nil, err
	}
	// Close ran during the dial: don't leave a live session no one will close.
	if p.closed {
		client.ForceClose()
		p.wakeWaiterLocked(accountID)
		return nil, ErrPoolClosed
	}

	conn := &PooledConnection{
		client:    client,
		accountID: accountID,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		inUse:     true,
	}
	p.connections[accountID] = append(p.connections[accountID], conn)

	p.log.Info().
		Str("account", accountID).
		Msg("New connection created")

	return conn, nil
}

// dialWithRetry connects and logs in, retrying once for transient errors
// like "max connections exceeded" (server still has ghost connections after network change).
func (p *Pool) dialWithRetry(ctx context.Context, accountID string, attempt int) (*Client, error) {
	p.log.Debug().
		Str("account", accountID).
		Msg("Creating new connection")

	// Get credentials for this account
	config, err := p.getCredentials(accountID)
	if err != nil {
		p.log.Error().Err(err).Str("account", accountID).Msg("Failed to get credentials")
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	p.log.Debug().
		Str("account", accountID).
		Str("host", config.Host).
		Int("port", config.Port).
		Str("authType", string(config.AuthType)).
		Msg("Got credentials, connecting to IMAP server")

	// Create and connect the client
	client := NewClient(*config)

	// Use a goroutine with context for connection timeout
	done := make(chan error, 1)
	go func() {
		if err := client.Connect(); err != nil {
			p.log.Error().Err(err).Str("account", accountID).Msg("IMAP Connect failed")
			done <- err
			return
		}
		p.log.Debug().Str("account", accountID).Msg("IMAP connected, logging in")
		if err := client.Login(); err != nil {
			p.log.Error().Err(err).Str("account", accountID).Msg("IMAP Login failed")
			client.ForceClose()
			done <- err
			return
		}
		p.log.Debug().Str("account", accountID).Msg("IMAP login successful")
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			// Retry once for "max connections" errors — the server may still
			// have ghost connections from force-closed sessions (network change).
			// Wait 15s to give the server time to drop stale connections.
			if attempt == 0 && strings.Contains(err.Error(), "Maximum number of connections") {
				p.log.Warn().Str("account", accountID).Msg("Max connections exceeded, retrying after 15s")
				select {
				case <-time.After(15 * time.Second):
					return p.dialWithRetry(ctx, accountID, attempt+1)
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			p.log.Error().Err(err).Str("account", accountID).Msg("Connection failed")
			return nil, fmt.Errorf("failed to connect: %w", err)
		}
	case <-ctx.Done():
		// The dial keeps running; close its session once it finishes so a
		// login that completes after the caller left isn't orphaned.
		p.log.Warn().Str("account", accountID).Msg("Connection timed out (context cancelled)")
		go func() {
			if err := <-done; err == nil {
				client.ForceClose()
			}
		}()
		return nil, ctx.Err()
	}

	return client, nil
}

// Release returns a connection to the pool
func (p *Pool) Release(conn *PooledConnection) {
	if conn == nil {
		return
	}

	conn.mu.Lock()
	conn.inUse = false
	conn.lastUsed = time.Now()
	healthy := conn.isHealthyLocked()
	conn.mu.Unlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Verify the connection is still tracked by the pool and healthy before
	// handing it to a waiter. After CloseAll (sleep), dead connections from
	// deferred Release calls must not be given to waiters.
	if !healthy {
		p.log.Debug().
			Str("account", conn.accountID).
			Msg("Released connection is unhealthy, discarding")
		p.discardLocked(conn)
		return
	}

	inPool := false
	if conns, ok := p.connections[conn.accountID]; ok {
		for _, c := range conns {
			if c == conn {
				inPool = true
				break
			}
		}
	}
	if !inPool {
		p.log.Debug().
			Str("account", conn.accountID).
			Msg("Released connection no longer in pool, discarding")
		return
	}

	// Check if anyone is waiting for a connection for this account
	if waiters, ok := p.waiters[conn.accountID]; ok && len(waiters) > 0 {
		waiter := waiters[0]
		p.waiters[conn.accountID] = waiters[1:]

		conn.mu.Lock()
		conn.inUse = true
		conn.mu.Unlock()

		waiter <- conn
		return
	}

	p.log.Debug().
		Str("account", conn.accountID).
		Msg("Connection released to pool")
}

// Discard removes a connection from the pool without returning it for reuse.
// Use this when a connection is known to be dead/unhealthy (e.g., after connection errors).
// Uses ForceClose to avoid blocking on dead TCP sockets.
func (p *Pool) Discard(conn *PooledConnection) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.discardLocked(conn)

	p.log.Debug().
		Str("account", conn.accountID).
		Msg("Discarded dead connection from pool")
}

// discardLocked force-closes conn, removes it from the pool, and wakes the
// first waiter for its account so it can create a replacement in the freed
// slot. Caller must hold p.mu and must not hold conn.mu.
func (p *Pool) discardLocked(conn *PooledConnection) {
	// Force-close the connection (known dead, skip graceful logout)
	conn.mu.Lock()
	if conn.client != nil {
		conn.client.ForceClose()
		conn.client = nil
	}
	conn.mu.Unlock()

	conns := p.connections[conn.accountID]
	removed := false
	for i, c := range conns {
		if c == conn {
			p.connections[conn.accountID] = append(conns[:i], conns[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		return
	}
	// Clean up empty account entry
	if len(p.connections[conn.accountID]) == 0 {
		delete(p.connections, conn.accountID)
	}

	p.wakeWaiterLocked(conn.accountID)
}

// CloseAccount closes all connections for a specific account.
// Uses ForceClose to avoid blocking on dead TCP sockets (e.g., after network changes).
func (p *Pool) CloseAccount(accountID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	conns, ok := p.connections[accountID]
	if !ok {
		return
	}

	for _, conn := range conns {
		conn.mu.Lock()
		if conn.client != nil {
			conn.client.ForceClose()
			conn.client = nil
		}
		conn.mu.Unlock()
	}

	delete(p.connections, accountID)

	// Notify any waiters that we're closing
	if waiters, ok := p.waiters[accountID]; ok {
		for _, w := range waiters {
			close(w)
		}
		delete(p.waiters, accountID)
	}

	p.log.Info().
		Str("account", accountID).
		Int("closed", len(conns)).
		Msg("Closed all connections for account")
}

// Close marks the pool closed, so later GetConnection calls fail with
// ErrPoolClosed instead of dialing, then closes every connection. Use it at
// shutdown; CloseAll leaves the pool usable.
func (p *Pool) Close() {
	p.mu.Lock()
	p.closed = true
	// Wake every queued caller so it fails fast instead of waiting out
	// WaiterTimeout; accounts with only dials in flight have no connections
	// for CloseAll to visit.
	for accountID := range p.waiters {
		for len(p.waiters[accountID]) > 0 {
			p.wakeWaiterLocked(accountID)
		}
	}
	p.mu.Unlock()
	p.CloseAll()
}

// CloseAll closes all connections in the pool
func (p *Pool) CloseAll() {
	p.mu.Lock()
	accountIDs := make([]string, 0, len(p.connections))
	for accountID := range p.connections {
		accountIDs = append(accountIDs, accountID)
	}
	p.mu.Unlock()

	for _, accountID := range accountIDs {
		p.CloseAccount(accountID)
	}

	p.log.Info().Msg("Closed all connections")
}

// CleanupIdle closes connections that have been idle too long
func (p *Pool) CleanupIdle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for accountID, conns := range p.connections {
		var remaining []*PooledConnection

		for _, conn := range conns {
			conn.mu.Lock()
			idle := !conn.inUse && now.Sub(conn.lastUsed) > p.config.IdleTimeout
			conn.mu.Unlock()

			if idle {
				conn.mu.Lock()
				if conn.client != nil {
					conn.client.ForceClose()
				}
				conn.mu.Unlock()
				cleaned++
			} else {
				remaining = append(remaining, conn)
			}
		}

		if len(remaining) == 0 {
			delete(p.connections, accountID)
		} else {
			p.connections[accountID] = remaining
		}
	}

	if cleaned > 0 {
		p.log.Debug().
			Int("cleaned", cleaned).
			Msg("Cleaned up idle connections")
	}
}

// StartCleanupRoutine starts a background goroutine that periodically cleans up idle connections
func (p *Pool) StartCleanupRoutine(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				p.CleanupIdle()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stats returns pool statistics
type PoolStats struct {
	TotalConnections  int
	ActiveConnections int
	IdleConnections   int
	AccountCount      int
}

// GetStats returns current pool statistics
func (p *Pool) GetStats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := PoolStats{
		AccountCount: len(p.connections),
	}

	for _, conns := range p.connections {
		for _, conn := range conns {
			stats.TotalConnections++
			conn.mu.Lock()
			if conn.inUse {
				stats.ActiveConnections++
			} else {
				stats.IdleConnections++
			}
			conn.mu.Unlock()
		}
	}

	return stats
}

// Client returns the underlying IMAP client from a pooled connection
func (pc *PooledConnection) Client() *Client {
	return pc.client
}
