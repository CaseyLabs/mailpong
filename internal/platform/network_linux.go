//go:build linux

package platform

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caseylabs/mailpong/internal/logging"
	"github.com/godbus/dbus/v5"
)

// LinuxNetworkMonitor monitors network connectivity on Linux using
// XDG Desktop Portal NetworkMonitor (primary) or NetworkManager D-Bus (fallback).
// Both are purely event-driven with zero polling.
type LinuxNetworkMonitor struct {
	conn      *dbus.Conn
	events    chan NetworkEvent
	stopChan  chan struct{}
	notifyCh  chan struct{} // signaled on connectivity change for WaitForConnection
	running   bool
	connected bool
	method    string // "portal", "networkmanager", or "none"
	mu        sync.RWMutex
}

// NewNetworkMonitor creates a new network connectivity monitor for Linux
func NewNetworkMonitor() NetworkMonitor {
	return &LinuxNetworkMonitor{
		events:    make(chan NetworkEvent, 10),
		stopChan:  make(chan struct{}),
		notifyCh:  make(chan struct{}, 1),
		connected: true, // assume connected until proven otherwise
	}
}

// Start begins monitoring for network connectivity changes via D-Bus
func (m *LinuxNetworkMonitor) Start(ctx context.Context) error {
	log := logging.WithComponent("network-monitor")

	if m.running {
		return nil
	}

	// Try XDG Desktop Portal first (works in Flatpak without extra permissions)
	if err := m.startPortal(ctx); err == nil {
		return nil
	}

	// Fall back to NetworkManager on system bus
	if err := m.startNetworkManager(ctx); err == nil {
		return nil
	}

	// Neither available — run without connectivity monitoring
	m.method = "none"
	m.running = true
	log.Warn().Msg("No network monitor available (portal and NetworkManager both unavailable) — assuming online")
	return nil
}

// startPortal tries to set up the XDG Desktop Portal NetworkMonitor
func (m *LinuxNetworkMonitor) startPortal(ctx context.Context) error {
	log := logging.WithComponent("network-monitor")

	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}

	// Check if the portal service is actually running before calling it.
	// NameHasOwner is a fast call to the D-Bus daemon that does NOT trigger
	// service activation. Without this guard, GetAvailable hangs forever
	// in environments where the portal is activatable but not running
	// (e.g. containers, minimal desktops without xdg-desktop-portal).
	var hasOwner bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.freedesktop.portal.Desktop").Store(&hasOwner); err != nil || !hasOwner {
		return fmt.Errorf("portal service not running")
	}

	// Check if the portal NetworkMonitor is available by calling GetAvailable
	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	var available bool
	err = obj.Call("org.freedesktop.portal.NetworkMonitor.GetAvailable", 0).Store(&available)
	if err != nil {
		return err
	}

	m.conn = conn
	m.method = "portal"
	m.connected = available
	m.running = true

	// Subscribe to the changed signal
	matchRule := "type='signal',interface='org.freedesktop.portal.NetworkMonitor',member='changed',path='/org/freedesktop/portal/desktop'"
	call := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchRule)
	if call.Err != nil {
		// Don't close conn — it's the shared session bus used by GTK/WebKit
		m.conn = nil
		m.running = false
		return call.Err
	}

	go m.listenPortal(ctx)

	log.Info().Str("method", "portal").Bool("connected", available).Msg("Network monitor started")
	return nil
}

// startNetworkManager tries to set up NetworkManager D-Bus monitoring
func (m *LinuxNetworkMonitor) startNetworkManager(ctx context.Context) error {
	log := logging.WithComponent("network-monitor")

	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	// Get current state: org.freedesktop.NetworkManager.State() -> uint32
	obj := conn.Object("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")
	var state uint32
	err = obj.Call("org.freedesktop.NetworkManager.state", 0).Store(&state)
	if err != nil {
		// Don't close conn — it's the shared system bus
		return err
	}

	m.conn = conn
	m.method = "networkmanager"
	m.connected = nmStateConnected(state)
	m.running = true

	// Subscribe to StateChanged signal
	matchRule := "type='signal',interface='org.freedesktop.NetworkManager',member='StateChanged'"
	call := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchRule)
	if call.Err != nil {
		// Don't close conn — it's the shared system bus
		m.conn = nil
		m.running = false
		return call.Err
	}

	go m.listenNetworkManager(ctx)

	log.Info().Str("method", "networkmanager").Bool("connected", m.connected).Msg("Network monitor started")
	return nil
}

// listenPortal listens for XDG Portal NetworkMonitor changed signals
func (m *LinuxNetworkMonitor) listenPortal(ctx context.Context) {
	log := logging.WithComponent("network-monitor")

	signals := make(chan *dbus.Signal, 10)
	m.conn.Signal(signals)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case signal := <-signals:
			if signal == nil {
				continue
			}
			if signal.Name != "org.freedesktop.portal.NetworkMonitor.changed" {
				continue
			}

			// Portal changed signal has no parameters — query current state
			obj := m.conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
			var available bool
			if err := obj.Call("org.freedesktop.portal.NetworkMonitor.GetAvailable", 0).Store(&available); err != nil {
				log.Warn().Err(err).Msg("Failed to get network availability after portal changed signal")
				continue
			}

			m.updateState(available)
		}
	}
}

// listenNetworkManager listens for NetworkManager StateChanged signals
func (m *LinuxNetworkMonitor) listenNetworkManager(ctx context.Context) {
	log := logging.WithComponent("network-monitor")

	signals := make(chan *dbus.Signal, 10)
	m.conn.Signal(signals)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case signal := <-signals:
			if signal == nil {
				continue
			}
			if signal.Name != "org.freedesktop.NetworkManager.StateChanged" {
				continue
			}
			if len(signal.Body) == 0 {
				continue
			}

			state, ok := signal.Body[0].(uint32)
			if !ok {
				log.Warn().Msg("Unexpected type in NetworkManager StateChanged signal")
				continue
			}

			m.updateState(nmStateConnected(state))
		}
	}
}

// updateState updates the connectivity state and emits an event if it changed
func (m *LinuxNetworkMonitor) updateState(connected bool) {
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
func (m *LinuxNetworkMonitor) Events() <-chan NetworkEvent {
	return m.events
}

// IsConnected returns the current connectivity state
func (m *LinuxNetworkMonitor) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// Invalidate resets the cached connectivity state to disconnected.
// Call this when connections are known to be dead (e.g. system sleep)
// so that WaitForConnection will wait for a fresh signal.
func (m *LinuxNetworkMonitor) Invalidate() {
	m.mu.Lock()
	m.connected = false
	m.mu.Unlock()
}

// refreshState re-queries the portal or NetworkManager for the current connectivity
// state. This is needed because the OS may not emit a signal if the network state
// didn't actually change (e.g. WiFi stays associated through suspend-to-RAM).
func (m *LinuxNetworkMonitor) refreshState() {
	switch m.method {
	case "portal":
		obj := m.conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
		var available bool
		if err := obj.Call("org.freedesktop.portal.NetworkMonitor.GetAvailable", 0).Store(&available); err == nil {
			m.mu.Lock()
			m.connected = available
			m.mu.Unlock()
		}
	case "networkmanager":
		obj := m.conn.Object("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")
		var state uint32
		if err := obj.Call("org.freedesktop.NetworkManager.state", 0).Store(&state); err == nil {
			m.mu.Lock()
			m.connected = nmStateConnected(state)
			m.mu.Unlock()
		}
	}
}

// WaitForConnection blocks until network is available or context is cancelled.
// Returns true if connected, false if context was cancelled.
func (m *LinuxNetworkMonitor) WaitForConnection(ctx context.Context) bool {
	log := logging.WithComponent("network-monitor")

	// Quick check: re-query current state in case we're already connected
	m.refreshState()
	if m.IsConnected() {
		return true
	}

	log.Info().Msg("Network not available, waiting for connectivity signal...")

	// Wait for a D-Bus signal indicating connectivity change
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

// Stop stops the monitor and cleans up resources
func (m *LinuxNetworkMonitor) Stop() error {
	log := logging.WithComponent("network-monitor")

	if !m.running {
		return nil
	}

	m.running = false
	close(m.stopChan)

	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}

	log.Info().Str("method", m.method).Msg("Network monitor stopped")
	return nil
}

// nmStateConnected returns true if the NetworkManager state indicates connectivity.
// NM states: 70 = connected_global, 60 = connected_site, 50 = connected_local
func nmStateConnected(state uint32) bool {
	return state >= 60 // connected_site or connected_global
}
