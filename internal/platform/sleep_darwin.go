//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation

#include <IOKit/pwr_mgt/IOPMLib.h>
#include <IOKit/IOMessage.h>
#include <CoreFoundation/CoreFoundation.h>

// Forward declaration of the Go callback
extern void goSleepWakeCallback(int isSleeping);

static io_connect_t rootPort;
static IONotificationPortRef notifyPortRef;
static io_object_t notifierObject;
static CFRunLoopRef sleepRunLoop;

static void sleepCallback(void *refCon, io_service_t service,
                          natural_t messageType, void *messageArgument) {
    switch (messageType) {
    case kIOMessageSystemWillSleep:
        goSleepWakeCallback(1);
        IOAllowPowerChange(rootPort, (long)messageArgument);
        break;
    case kIOMessageSystemHasPoweredOn:
        goSleepWakeCallback(0);
        break;
    case kIOMessageCanSystemSleep:
        IOAllowPowerChange(rootPort, (long)messageArgument);
        break;
    }
}

// startSleepWakeMonitor registers for IOKit power notifications and runs CFRunLoop.
// This function blocks until stopSleepWakeMonitor is called.
static int startSleepWakeMonitor(void) {
    rootPort = IORegisterForSystemPower(NULL, &notifyPortRef, sleepCallback, &notifierObject);
    if (rootPort == 0) {
        return -1;
    }

    sleepRunLoop = CFRunLoopGetCurrent();
    CFRunLoopAddSource(sleepRunLoop,
                       IONotificationPortGetRunLoopSource(notifyPortRef),
                       kCFRunLoopDefaultMode);

    CFRunLoopRun();
    return 0;
}

// stopSleepWakeMonitor stops the run loop and cleans up.
static void stopSleepWakeMonitor(void) {
    if (sleepRunLoop != NULL) {
        CFRunLoopStop(sleepRunLoop);
        sleepRunLoop = NULL;
    }

    if (notifierObject != 0) {
        IODeregisterForSystemPower(&notifierObject);
        notifierObject = 0;
    }

    if (notifyPortRef != NULL) {
        IONotificationPortDestroy(notifyPortRef);
        notifyPortRef = NULL;
    }

    rootPort = 0;
}
*/
import "C"

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/caseylabs/mailven/internal/logging"
)

// DarwinSleepWakeMonitor monitors sleep/wake events on macOS using IOKit
type DarwinSleepWakeMonitor struct {
	events   chan SleepWakeEvent
	stopChan chan struct{}
	wg       sync.WaitGroup
	running  bool
}

// package-level singleton so the C callback can reach the Go instance
var darwinSleepMon *DarwinSleepWakeMonitor

//export goSleepWakeCallback
func goSleepWakeCallback(isSleeping C.int) {
	mon := darwinSleepMon
	if mon == nil {
		return
	}

	event := SleepWakeEvent{
		IsSleeping: isSleeping != 0,
		Timestamp:  time.Now(),
	}

	// Non-blocking send to events channel
	select {
	case mon.events <- event:
	default:
	}
}

// NewSleepWakeMonitor creates a new sleep/wake monitor for macOS
func NewSleepWakeMonitor() SleepWakeMonitor {
	return &DarwinSleepWakeMonitor{
		events:   make(chan SleepWakeEvent, 10),
		stopChan: make(chan struct{}),
	}
}

// Start begins monitoring for sleep/wake events using IOKit
func (m *DarwinSleepWakeMonitor) Start(ctx context.Context) error {
	log := logging.WithComponent("sleep-wake")

	if m.running {
		return nil
	}

	darwinSleepMon = m
	m.running = true

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if rc := C.startSleepWakeMonitor(); rc != 0 {
			log.Warn().Msg("Failed to register for IOKit system power notifications")
			return
		}
	}()

	log.Info().Msg("Sleep/wake monitor started (IOKit)")
	return nil
}

// Events returns the channel for receiving sleep/wake events
func (m *DarwinSleepWakeMonitor) Events() <-chan SleepWakeEvent {
	return m.events
}

// Stop stops the monitor and cleans up resources
func (m *DarwinSleepWakeMonitor) Stop() error {
	log := logging.WithComponent("sleep-wake")

	if !m.running {
		return nil
	}

	m.running = false
	C.stopSleepWakeMonitor()
	m.wg.Wait()

	darwinSleepMon = nil

	log.Info().Msg("Sleep/wake monitor stopped (macOS)")
	return nil
}
