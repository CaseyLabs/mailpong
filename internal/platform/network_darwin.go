//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=10.14
#cgo LDFLAGS: -framework Network

#include <Network/Network.h>
#include <dispatch/dispatch.h>

// Forward declaration of the Go callback
extern void goNetworkStatusCallback(int connected);

static nw_path_monitor_t monitor;
static dispatch_queue_t monitorQueue;

// startNetworkMonitor creates an NWPathMonitor, sets a handler, and starts it.
// The handler fires immediately with the current path state.
static void startNetworkMonitor(void) {
    monitor = nw_path_monitor_create();
    monitorQueue = dispatch_queue_create("com.mailpong.networkmonitor", DISPATCH_QUEUE_SERIAL);

    nw_path_monitor_set_update_handler(monitor, ^(nw_path_t path) {
        nw_path_status_t status = nw_path_get_status(path);
        int connected = (status == nw_path_status_satisfied || status == nw_path_status_satisfiable) ? 1 : 0;
        goNetworkStatusCallback(connected);
    });

    nw_path_monitor_set_queue(monitor, monitorQueue);
    nw_path_monitor_start(monitor);
}

// stopNetworkMonitor cancels and releases the monitor.
static void stopNetworkMonitor(void) {
    if (monitor != NULL) {
        nw_path_monitor_cancel(monitor);
        nw_release(monitor);
        monitor = NULL;
    }
    // dispatch queue is released by ARC / when no longer referenced
    monitorQueue = NULL;
}
*/
import "C"

import (
	"context"
	"sync"
	"time"

	"github.com/caseylabs/mailpong/internal/logging"
)

// DarwinNetworkMonitor monitors network connectivity on macOS using NWPathMonitor
type DarwinNetworkMonitor struct {
	events    chan NetworkEvent
	stopChan  chan struct{}
	notifyCh  chan struct{} // signaled on connectivity change for WaitForConnection
	running   bool
	connected bool
	mu        sync.RWMutex
}

// package-level singleton so the C callback can reach the Go instance
var darwinNetMon *DarwinNetworkMonitor

//export goNetworkStatusCallback
func goNetworkStatusCallback(connected C.int) {
	mon := darwinNetMon
	if mon == nil {
		return
	}
	mon.updateState(connected != 0)
}

// NewNetworkMonitor creates a new network connectivity monitor for macOS
func NewNetworkMonitor() NetworkMonitor {
	return &DarwinNetworkMonitor{
		events:    make(chan NetworkEvent, 10),
		stopChan:  make(chan struct{}),
		notifyCh:  make(chan struct{}, 1),
		connected: true, // assume connected until first callback
	}
}

// Start begins monitoring for network connectivity changes using NWPathMonitor
func (m *DarwinNetworkMonitor) Start(ctx context.Context) error {
	log := logging.WithComponent("network-monitor")

	if m.running {
		return nil
	}

	darwinNetMon = m
	m.running = true

	// NWPathMonitor fires the update handler immediately with current state
	C.startNetworkMonitor()

	log.Info().Msg("Network monitor started (NWPathMonitor)")
	return nil
}

// updateState updates the connectivity state and emits an event if it changed
func (m *DarwinNetworkMonitor) updateState(connected bool) {
	log := logging.WithComponent("network-monitor")

	m.mu.Lock()
	changed := m.connected != connected
	m.connected = connected
	m.mu.Unlock()

	if !changed {
		return
	}

	event := NetworkEvent{
		Connected: connected,
		Timestamp: time.Now(),
	}

	if connected {
		log.Info().Msg("Network connectivity restored")
	} else {
		log.Info().Msg("Network connectivity lost")
	}

	// Non-blocking send to events channel
	select {
	case m.events <- event:
	default:
		log.Warn().Msg("Network event channel full, dropping event")
	}

	// Signal WaitForConnection waiters
	select {
	case m.notifyCh <- struct{}{}:
	default:
	}
}

// Events returns the channel for receiving network connectivity events
func (m *DarwinNetworkMonitor) Events() <-chan NetworkEvent {
	return m.events
}

// IsConnected returns the current connectivity state
func (m *DarwinNetworkMonitor) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// WaitForConnection blocks until network is available or context is cancelled.
// Returns true if connected, false if context was cancelled.
func (m *DarwinNetworkMonitor) WaitForConnection(ctx context.Context) bool {
	log := logging.WithComponent("network-monitor")

	if m.IsConnected() {
		return true
	}

	log.Info().Msg("Network not available, waiting for connectivity signal...")

	for {
		select {
		case <-ctx.Done():
			return false
		case <-m.notifyCh:
			if m.IsConnected() {
				return true
			}
		}
	}
}

// Invalidate resets the cached connectivity state to disconnected
func (m *DarwinNetworkMonitor) Invalidate() {
	m.mu.Lock()
	m.connected = false
	m.mu.Unlock()
}

// Stop stops the monitor and cleans up resources
func (m *DarwinNetworkMonitor) Stop() error {
	log := logging.WithComponent("network-monitor")

	if !m.running {
		return nil
	}

	m.running = false
	C.stopNetworkMonitor()
	darwinNetMon = nil

	log.Info().Msg("Network monitor stopped (macOS)")
	return nil
}
