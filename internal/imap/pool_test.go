package imap

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var errNoCredentials = errors.New("no credentials")

func newTestPool(max int) *Pool {
	return NewPool(PoolConfig{
		MaxConnections: max,
		WaiterTimeout:  5 * time.Second,
	}, func(string) (*ClientConfig, error) {
		return nil, errNoCredentials
	})
}

// A connection with a nil client is unhealthy without needing a server.
func addDeadConn(p *Pool, accountID string, inUse bool) *PooledConnection {
	conn := &PooledConnection{accountID: accountID, inUse: inUse}
	p.connections[accountID] = append(p.connections[accountID], conn)
	return conn
}

func TestReleaseUnhealthyRemovesConnectionAndWakesWaiter(t *testing.T) {
	p := newTestPool(1)
	conn := addDeadConn(p, "acct", true)

	result := make(chan error, 1)
	go func() {
		_, err := p.GetConnection(context.Background(), "acct")
		result <- err
	}()

	// Wait until the request is queued behind the full pool.
	deadline := time.Now().Add(2 * time.Second)
	for {
		p.mu.Lock()
		n := len(p.waiters["acct"])
		p.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("request never waited for a connection")
		}
		time.Sleep(5 * time.Millisecond)
	}

	p.Release(conn)

	select {
	case err := <-result:
		// The woken waiter retried and attempted a replacement connection.
		if err == nil || !strings.Contains(err.Error(), errNoCredentials.Error()) {
			t.Fatalf("GetConnection error = %v, want credentials error from replacement attempt", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter was not woken after unhealthy release")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if n := len(p.connections["acct"]); n != 0 {
		t.Fatalf("pool still tracks %d connections, want 0", n)
	}
}

func TestGetConnectionDropsIdleDeadConnections(t *testing.T) {
	p := newTestPool(1)
	addDeadConn(p, "acct", false)

	_, err := p.GetConnection(context.Background(), "acct")
	if err == nil || !strings.Contains(err.Error(), errNoCredentials.Error()) {
		t.Fatalf("GetConnection error = %v, want credentials error from new connection attempt", err)
	}
	if n := len(p.connections["acct"]); n != 0 {
		t.Fatalf("pool still tracks %d connections, want 0", n)
	}
}

func TestAbandonWaitPassesOnWhatItWasHanded(t *testing.T) {
	tests := []struct {
		name     string
		queued   bool // still in the waiter list when it gives up
		handed   func(p *Pool) *PooledConnection
		wantNext bool // next waiter is woken
		wantConn int  // connections still tracked afterwards
	}{
		{name: "still queued", queued: true, wantConn: 1},
		{name: "handed a freed slot", handed: func(*Pool) *PooledConnection { return nil }, wantNext: true, wantConn: 1},
		{name: "handed a connection", handed: func(p *Pool) *PooledConnection { return p.connections["acct"][0] }, wantNext: true, wantConn: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestPool(1)
			addDeadConn(p, "acct", true)
			waiter := make(chan *PooledConnection, 1)
			next := make(chan *PooledConnection, 1)
			p.waiters["acct"] = []chan *PooledConnection{waiter, next}
			if !tt.queued {
				p.waiters["acct"] = p.waiters["acct"][1:]
				waiter <- tt.handed(p)
			}

			p.abandonWait("acct", waiter)

			woken := len(next) == 1
			if woken != tt.wantNext {
				t.Fatalf("next waiter woken = %v, want %v", woken, tt.wantNext)
			}
			if n := len(p.connections["acct"]); n != tt.wantConn {
				t.Fatalf("pool tracks %d connections, want %d", n, tt.wantConn)
			}
			for _, w := range p.waiters["acct"] {
				if w == waiter {
					t.Fatal("abandoned waiter is still queued")
				}
			}
		})
	}
}

func TestDialsInFlightCountTowardLimit(t *testing.T) {
	p := newTestPool(1)
	p.dialing["acct"] = 1

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := p.GetConnection(ctx, "acct"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("GetConnection error = %v, want to wait for the in-flight dial", err)
	}
}

func TestFailedDialFreesSlot(t *testing.T) {
	p := newTestPool(1)
	if _, err := p.GetConnection(context.Background(), "acct"); err == nil {
		t.Fatal("GetConnection succeeded without credentials")
	}
	if n := p.dialing["acct"]; n != 0 {
		t.Fatalf("dialing = %d after failed dial, want 0", n)
	}
}

func TestCloseRefusesConnectionsWithoutCredentialLookup(t *testing.T) {
	tests := []struct {
		name  string
		close func(*Pool)
	}{
		{"Close", func(p *Pool) { p.Close() }},
		{"Close after CloseAll", func(p *Pool) { p.CloseAll(); p.Close() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookups := 0
			p := NewPool(PoolConfig{MaxConnections: 1, WaiterTimeout: time.Second},
				func(string) (*ClientConfig, error) {
					lookups++
					return nil, errNoCredentials
				})
			tt.close(p)
			if _, err := p.GetConnection(context.Background(), "acct"); !errors.Is(err, ErrPoolClosed) {
				t.Fatalf("GetConnection error = %v, want ErrPoolClosed", err)
			}
			if lookups != 0 {
				t.Fatalf("getCredentials called %d times after Close", lookups)
			}
		})
	}
}

func TestCloseAllLeavesPoolUsable(t *testing.T) {
	p := newTestPool(1)
	p.CloseAll()
	if _, err := p.GetConnection(context.Background(), "acct"); !errors.Is(err, errNoCredentials) {
		t.Fatalf("GetConnection error = %v, want credentials error (pool still open)", err)
	}
}
