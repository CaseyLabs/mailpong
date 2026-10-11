//go:build darwin

package platform

import (
	"github.com/caseylabs/mailpong/internal/logging"
)

// DarwinPowerMonitor monitors power state on macOS
type DarwinPowerMonitor struct {
	callback func(PowerInfo)
	stopChan chan struct{}
}

// NewPowerMonitor creates a new power monitor for the current platform
func NewPowerMonitor() PowerMonitor {
	return &DarwinPowerMonitor{
		stopChan: make(chan struct{}),
	}
}

// GetPowerInfo returns the current power state
func (m *DarwinPowerMonitor) GetPowerInfo() (*PowerInfo, error) {
	// TODO: Implement using IOKit
	// For now, return AC as default
	return &PowerInfo{
		State:             PowerStateAC,
		BatteryPercentage: -1,
		IsCharging:        false,
	}, nil
}

// Subscribe registers a callback for power state changes
func (m *DarwinPowerMonitor) Subscribe(callback func(PowerInfo)) error {
	log := logging.WithComponent("power")
	m.callback = callback
	log.Debug().Msg("Power monitor subscribed (polling mode)")
	return nil
}

// Unsubscribe removes the callback
func (m *DarwinPowerMonitor) Unsubscribe() error {
	m.callback = nil
	return nil
}

// Close cleans up resources
func (m *DarwinPowerMonitor) Close() error {
	close(m.stopChan)
	return nil
}
