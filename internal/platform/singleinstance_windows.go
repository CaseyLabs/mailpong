package platform

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/caseylabs/mailpong/internal/logging"
)

// windowsSingleInstanceLock uses a TCP socket on localhost for single-instance detection.
// Windows doesn't support Unix sockets reliably, so we use a fixed TCP port on loopback.
// The port number is stored in a lock file at %APPDATA%\MailPong\instance.port.
type windowsSingleInstanceLock struct {
	listener net.Listener
	lockFile string
	onShow   func(data string)
	mu       sync.Mutex
	done     chan struct{}
}

// NewSingleInstanceLock creates a new single-instance lock.
func NewSingleInstanceLock() SingleInstanceLock {
	return &windowsSingleInstanceLock{
		done: make(chan struct{}),
	}
}

// TryLock attempts to acquire the single-instance lock.
// activateMsg is the command to send to an existing instance (e.g. "show" or "mailto:...").
func (l *windowsSingleInstanceLock) TryLock(activateMsg string) (bool, error) {
	log := logging.WithComponent("singleinstance")

	lockFile, err := l.buildLockFilePath()
	if err != nil {
		return true, fmt.Errorf("failed to build lock file path: %w", err)
	}
	l.lockFile = lockFile

	// Check if an existing instance is running by reading the port from the lock file
	if portData, err := os.ReadFile(lockFile); err == nil {
		addr := string(portData)
		conn, dialErr := net.DialTimeout("tcp", addr, 2*time.Second)
		if dialErr == nil {
			// Existing instance is alive — send activation command
			_, _ = conn.Write([]byte(activateMsg + "\n"))
			conn.Close()
			log.Info().Str("command", activateMsg).Msg("Activated existing instance")
			return false, nil
		}
		// Lock file exists but instance is dead — stale, remove it
		log.Warn().Msg("Stale instance lock file found, removing")
		os.Remove(lockFile)
	}

	// Listen on a random port on localhost
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return true, fmt.Errorf("failed to listen for single-instance detection: %w", err)
	}

	// Write the listener address to the lock file so other instances can find us
	addr := listener.Addr().String()
	if err := os.WriteFile(lockFile, []byte(addr), 0600); err != nil {
		listener.Close()
		return true, fmt.Errorf("failed to write lock file: %w", err)
	}

	l.listener = listener
	go l.acceptLoop()
	log.Info().Str("addr", addr).Msg("Single-instance lock acquired")
	return true, nil
}

// SetOnShow sets the callback invoked when a second instance sends a command.
func (l *windowsSingleInstanceLock) SetOnShow(fn func(data string)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onShow = fn
}

// Unlock releases the lock and cleans up resources.
func (l *windowsSingleInstanceLock) Unlock() {
	close(l.done)
	if l.listener != nil {
		l.listener.Close()
	}
	if l.lockFile != "" {
		os.Remove(l.lockFile)
	}
}

// acceptLoop handles incoming connections from second instances.
func (l *windowsSingleInstanceLock) acceptLoop() {
	log := logging.WithComponent("singleinstance")

	for {
		conn, err := l.listener.Accept()
		if err != nil {
			select {
			case <-l.done:
				return
			default:
				log.Debug().Err(err).Msg("Accept error")
				return
			}
		}

		// Only accept connections from localhost
		remoteAddr := conn.RemoteAddr().String()
		host, _, _ := net.SplitHostPort(remoteAddr)
		if host != "127.0.0.1" && host != "::1" {
			conn.Close()
			continue
		}

		go l.handleConnection(conn)
	}
}

// handleConnection reads the command from a second instance.
func (l *windowsSingleInstanceLock) handleConnection(conn net.Conn) {
	defer conn.Close()
	log := logging.WithComponent("singleinstance")

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	scanner := bufio.NewScanner(conn)
	// Limit scanner buffer to 2KB — no legitimate command exceeds this
	scanner.Buffer(make([]byte, 2048), 2048)
	if !scanner.Scan() {
		return
	}

	cmd := scanner.Text()
	// Command allowlist: only accept "show" or "mailto:..." — reject everything else
	if cmd != "show" && !strings.HasPrefix(cmd, "mailto:") {
		log.Warn().Str("cmd", cmd).Msg("Rejected unknown command from second instance")
		return
	}

	l.mu.Lock()
	fn := l.onShow
	l.mu.Unlock()

	if fn == nil {
		return
	}

	log.Info().Str("command", cmd).Msg("Command received from second instance")
	fn(cmd)
}

// buildLockFilePath returns the path for the instance lock file.
func (l *windowsSingleInstanceLock) buildLockFilePath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}

	lockDir := filepath.Join(appData, "MailPong")
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create lock directory: %w", err)
	}

	return filepath.Join(lockDir, "instance.lock"), nil
}
