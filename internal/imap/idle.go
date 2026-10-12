package imap

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/caseylabs/mailven/internal/logging"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"
	"github.com/rs/zerolog"
)

// IdleConfig configures IDLE connections
type IdleConfig struct {
	// IdleTimeout is how long to stay in IDLE before restarting (RFC 2177 recommends < 29 min)
	IdleTimeout time.Duration

	// ReconnectBackoff is the initial backoff duration for reconnection attempts
	ReconnectBackoff time.Duration

	// MaxReconnectBackoff is the maximum backoff duration
	MaxReconnectBackoff time.Duration

	// MaxReconnectAttempts is the maximum number of reconnection attempts before giving up
	MaxReconnectAttempts int

	// EventSendTimeout is how long to wait when sending events before dropping them
	EventSendTimeout time.Duration

	// HealthCheckEnabled enables NOOP health checks before entering IDLE
	HealthCheckEnabled bool

	// ShutdownTimeout is how long to wait for graceful shutdown
	ShutdownTimeout time.Duration
}

// DefaultIdleConfig returns sensible defaults for IDLE
func DefaultIdleConfig() IdleConfig {
	return IdleConfig{
		IdleTimeout:          10 * time.Minute, // Shorter cycle for better connection health
		ReconnectBackoff:     1 * time.Second,
		MaxReconnectBackoff:  5 * time.Minute,
		MaxReconnectAttempts: 10,
		EventSendTimeout:     2 * time.Second, // Don't block forever on event send
		HealthCheckEnabled:   true,            // Verify connection before IDLE
		ShutdownTimeout:      5 * time.Second, // Graceful shutdown timeout
	}
}

// IdleConnection manages an IDLE connection for a single account folder
type IdleConnection struct {
	accountID      string
	accountName    string
	config         IdleConfig
	getCredentials func(accountID string) (*ClientConfig, error)
	isConnected    func() bool // optional: wait for connectivity before reconnecting
	log            zerolog.Logger

	// State
	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{} // Closed when goroutine exits
	folder  string        // Watched folder (INBOX or the Sent folder)
	client  *imapclient.Client
	events  chan<- MailEvent
}

// newIdleConnection creates a new IDLE connection for one folder of an account
func newIdleConnection(accountID, accountName, folder string, config IdleConfig, getCredentials func(accountID string) (*ClientConfig, error)) *IdleConnection {
	return &IdleConnection{
		accountID:      accountID,
		accountName:    accountName,
		config:         config,
		getCredentials: getCredentials,
		log:            logging.WithComponent("imap-idle").With().Str("account", accountName).Str("folder", folder).Logger(),
		folder:         folder,
	}
}

// sendEvent sends an event with timeout to prevent blocking
func (ic *IdleConnection) sendEvent(event MailEvent) {
	select {
	case ic.events <- event:
		// Event sent successfully
	case <-time.After(ic.config.EventSendTimeout):
		ic.log.Warn().
			Str("type", event.Type.String()).
			Msg("Event channel full, dropping event (receiver may be stuck)")
	case <-ic.stopCh:
		// Connection stopping, discard event
	}
}

// Start starts the IDLE loop for this connection
func (ic *IdleConnection) Start(ctx context.Context, events chan<- MailEvent) {
	ic.mu.Lock()
	if ic.running {
		ic.mu.Unlock()
		return
	}
	ic.running = true
	ic.stopCh = make(chan struct{})
	ic.doneCh = make(chan struct{})
	ic.events = events
	ic.mu.Unlock()

	go ic.run(ctx)
}

// Stop stops the IDLE connection with graceful shutdown
func (ic *IdleConnection) Stop() {
	ic.mu.Lock()
	if !ic.running {
		ic.mu.Unlock()
		return
	}

	ic.running = false
	close(ic.stopCh)
	doneCh := ic.doneCh
	timeout := ic.config.ShutdownTimeout
	ic.mu.Unlock()

	// Wait for graceful shutdown with timeout
	if doneCh != nil {
		select {
		case <-doneCh:
			ic.log.Debug().Msg("IDLE connection stopped gracefully")
		case <-time.After(timeout):
			ic.log.Warn().Msg("IDLE connection shutdown timed out, forcing close")
			ic.mu.Lock()
			if ic.client != nil {
				ic.client.Close()
				ic.client = nil
			}
			ic.mu.Unlock()
		}
	}
}

// run is the main IDLE loop
func (ic *IdleConnection) run(ctx context.Context) {
	defer func() {
		ic.mu.Lock()
		ic.running = false
		if ic.client != nil {
			ic.client.Close()
			ic.client = nil
		}
		if ic.doneCh != nil {
			close(ic.doneCh)
		}
		ic.mu.Unlock()
	}()

	backoff := ic.config.ReconnectBackoff
	attempts := 0

	for {
		select {
		case <-ctx.Done():
			ic.log.Debug().Msg("Context cancelled, stopping IDLE")
			return
		case <-ic.stopCh:
			ic.log.Debug().Msg("Stop requested, stopping IDLE")
			return
		default:
		}

		// Stop if offline — processNetworkEvents will restart IDLE
		// when connectivity is restored, avoiding wasteful retries
		if ic.isConnected != nil && !ic.isConnected() {
			ic.log.Debug().Msg("Offline, stopping IDLE (will restart when online)")
			return
		}

		// Connect if needed
		if err := ic.ensureConnected(ctx); err != nil {
			attempts++
			if attempts >= ic.config.MaxReconnectAttempts {
				ic.log.Error().
					Err(err).
					Int("attempts", attempts).
					Msg("Max reconnection attempts reached, giving up")
				return
			}

			ic.log.Warn().
				Err(err).
				Dur("backoff", backoff).
				Int("attempt", attempts).
				Msg("Failed to connect for IDLE, retrying")

			select {
			case <-time.After(backoff):
				backoff = min(backoff*2, ic.config.MaxReconnectBackoff)
				continue
			case <-ctx.Done():
				return
			case <-ic.stopCh:
				return
			}
		}

		// Reset backoff on successful connection
		backoff = ic.config.ReconnectBackoff
		attempts = 0

		// Run IDLE cycle
		if err := ic.idleCycle(ctx); err != nil {
			ic.log.Warn().Err(err).Msg("IDLE cycle failed")
			// Close the connection so we reconnect on next iteration
			ic.mu.Lock()
			if ic.client != nil {
				ic.client.Close()
				ic.client = nil
			}
			ic.mu.Unlock()
		}
	}
}

// ensureConnected ensures we have a valid connection with unilateral data handler
func (ic *IdleConnection) ensureConnected(ctx context.Context) error {
	ic.mu.Lock()
	if ic.client != nil {
		ic.mu.Unlock()
		return nil
	}
	ic.mu.Unlock()

	// Get credentials
	creds, err := ic.getCredentials(ic.accountID)
	if err != nil {
		return err
	}

	// Create client with unilateral data handler for IDLE notifications
	options := &imapclient.Options{
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				if data.NumMessages != nil {
					ic.log.Info().Uint32("count", *data.NumMessages).Msg("New messages notification (EXISTS)")
					ic.sendEvent(MailEvent{
						Type:      EventNewMail,
						AccountID: ic.accountID,
						Folder:    ic.folder,
						Count:     *data.NumMessages,
					})
				}
			},
			// EXPUNGE = a message was removed on the server. The app debounces this
			// into a lightweight deletion reconcile (see handleIdleExpunge in
			// app/background.go) — required for RFC-strict servers like Dovecot that
			// signal deletes with EXPUNGE only and no follow-up EXISTS.
			Expunge: func(seqNum uint32) {
				ic.log.Debug().Uint32("seqNum", seqNum).Msg("Message expunged")
				ic.sendEvent(MailEvent{
					Type:      EventExpunge,
					AccountID: ic.accountID,
					Folder:    ic.folder,
					SeqNum:    seqNum,
				})
			},
			// Unilateral FETCH = a flag changed on the server (another client
			// marked read/unread/starred). Emit EventFlagsChanged so the app can
			// re-sync flags (debounced). We must consume the streamed data.
			Fetch: func(msg *imapclient.FetchMessageData) {
				seqNum := msg.SeqNum
				_, _ = msg.Collect()
				ic.log.Debug().Uint32("seqNum", seqNum).Msg("Flags changed notification (FETCH)")
				ic.sendEvent(MailEvent{
					Type:      EventFlagsChanged,
					AccountID: ic.accountID,
					Folder:    ic.folder,
					SeqNum:    seqNum,
				})
			},
		},
	}

	addr := net.JoinHostPort(creds.Host, strconv.Itoa(creds.Port))
	var client *imapclient.Client

	// IDLE connections use a longer read timeout (slightly beyond the IDLE cycle)
	// so deadlines don't fire during normal IDLE waiting, but still catch dead connections.
	// Write timeout stays short for login/select/IDLE start commands.
	idleReadTimeout := ic.config.IdleTimeout + 1*time.Minute
	idleWriteTimeout := 30 * time.Second
	connectDialer := &net.Dialer{Timeout: 30 * time.Second}

	switch SecurityType(creds.Security) {
	case SecurityTLS:
		tlsConfig := creds.TLSConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{ServerName: creds.Host}
		}
		rawConn, dialErr := tls.DialWithDialer(connectDialer, "tcp", addr, tlsConfig)
		if dialErr != nil {
			return fmt.Errorf("failed to connect with TLS: %w", dialErr)
		}
		wrappedConn := &deadlineConn{
			Conn:         rawConn,
			readTimeout:  idleReadTimeout,
			writeTimeout: idleWriteTimeout,
		}
		client = imapclient.New(wrappedConn, options)

	case SecurityStartTLS:
		rawConn, dialErr := connectDialer.Dial("tcp", addr)
		if dialErr != nil {
			return fmt.Errorf("failed to connect: %w", dialErr)
		}
		wrappedConn := &deadlineConn{
			Conn:         rawConn,
			readTimeout:  idleReadTimeout,
			writeTimeout: idleWriteTimeout,
		}
		options.TLSConfig = creds.TLSConfig
		if options.TLSConfig == nil {
			options.TLSConfig = &tls.Config{ServerName: creds.Host}
		}
		client, err = imapclient.NewStartTLS(wrappedConn, options)

	case SecurityNone:
		rawConn, dialErr := connectDialer.Dial("tcp", addr)
		if dialErr != nil {
			return fmt.Errorf("failed to connect: %w", dialErr)
		}
		wrappedConn := &deadlineConn{
			Conn:         rawConn,
			readTimeout:  idleReadTimeout,
			writeTimeout: idleWriteTimeout,
		}
		client = imapclient.New(wrappedConn, options)

	default:
		tlsConfig := creds.TLSConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{ServerName: creds.Host}
		}
		rawConn, dialErr := tls.DialWithDialer(connectDialer, "tcp", addr, tlsConfig)
		if dialErr != nil {
			return fmt.Errorf("failed to connect with TLS: %w", dialErr)
		}
		wrappedConn := &deadlineConn{
			Conn:         rawConn,
			readTimeout:  idleReadTimeout,
			writeTimeout: idleWriteTimeout,
		}
		client = imapclient.New(wrappedConn, options)
	}

	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Wait for greeting
	if err := client.WaitGreeting(); err != nil {
		client.Close()
		return fmt.Errorf("failed to receive greeting: %w", err)
	}

	// Login
	authType := creds.AuthType
	if authType == "" {
		authType = AuthTypePassword
	}

	switch authType {
	case AuthTypeOAuth2:
		saslClient := NewXOAuth2Client(creds.Username, creds.AccessToken)
		if err := client.Authenticate(saslClient); err != nil {
			client.Close()
			return fmt.Errorf("OAuth2 authentication failed: %w", err)
		}
	default:
		saslClient := sasl.NewPlainClient("", creds.Username, creds.Password)
		if err := client.Authenticate(saslClient); err != nil {
			// Fall back to LOGIN command
			if err := client.Login(creds.Username, creds.Password).Wait(); err != nil {
				client.Close()
				return fmt.Errorf("authentication failed: %w", err)
			}
		}
	}

	// Check if server supports IDLE
	if !client.Caps().Has("IDLE") {
		client.Close()
		ic.log.Info().Msg("Server does not support IDLE, falling back to polling only")
		// Return a special error that indicates IDLE is not supported
		return fmt.Errorf("server does not support IDLE")
	}

	// Select the watched folder
	selectCmd := client.Select(ic.folder, nil)
	if _, err := selectCmd.Wait(); err != nil {
		client.Close()
		return fmt.Errorf("failed to select %s: %w", ic.folder, err)
	}

	ic.mu.Lock()
	ic.client = client
	ic.mu.Unlock()

	ic.log.Info().Msg("IDLE connection established")
	return nil
}

// idleCycle runs a single IDLE cycle with timeout
func (ic *IdleConnection) idleCycle(ctx context.Context) error {
	ic.mu.Lock()
	client := ic.client
	if client == nil {
		ic.mu.Unlock()
		return nil
	}
	ic.mu.Unlock()

	// Health check: verify connection is alive before entering IDLE
	if ic.config.HealthCheckEnabled {
		ic.log.Debug().Msg("Running connection health check (NOOP)")
		if err := client.Noop().Wait(); err != nil {
			return fmt.Errorf("health check failed (connection may be dead): %w", err)
		}
	}

	ic.log.Debug().Msg("Starting IDLE")

	// Start IDLE command
	idleCmd, err := client.Idle()
	if err != nil {
		return fmt.Errorf("failed to start IDLE: %w", err)
	}

	// Set up timeout timer
	timer := time.NewTimer(ic.config.IdleTimeout)
	defer timer.Stop()

	// Wait for timeout or stop signal
	// Note: Unilateral data (EXISTS, EXPUNGE) is handled by the UnilateralDataHandler
	// we set up when creating the client
	select {
	case <-ctx.Done():
		ic.log.Debug().Msg("Context cancelled during IDLE")
		idleCmd.Close()
		return nil

	case <-ic.stopCh:
		ic.log.Debug().Msg("Stop requested during IDLE")
		idleCmd.Close()
		return nil

	case <-timer.C:
		// IDLE timeout - restart to keep connection alive
		ic.log.Debug().Msg("IDLE timeout, restarting")
		if err := idleCmd.Close(); err != nil {
			return err
		}
		return nil
	}
}

// idleKey identifies one IDLE connection: an account's watched folder.
type idleKey struct {
	accountID string
	folder    string
}

// IdleManager manages IDLE connections for multiple accounts
type IdleManager struct {
	config         IdleConfig
	getCredentials func(accountID string) (*ClientConfig, error)
	isConnected    func() bool // optional: propagated to connections
	log            zerolog.Logger

	// Connections per account and folder
	connections map[idleKey]*IdleConnection
	mu          sync.Mutex

	// Event channel
	events chan MailEvent

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	closed bool // set by Close; Start and StartAccount do nothing after it
}

// NewIdleManager creates a new IDLE manager
func NewIdleManager(config IdleConfig, getCredentials func(accountID string) (*ClientConfig, error)) *IdleManager {
	return &IdleManager{
		config:         config,
		getCredentials: getCredentials,
		log:            logging.WithComponent("idle-manager"),
		connections:    make(map[idleKey]*IdleConnection),
		events:         make(chan MailEvent, 100),
	}
}

// SetConnectivityCheck sets a function to check network connectivity.
// When set, IDLE connections will skip reconnect attempts when offline
// to avoid wasted connection attempts and unnecessary error logging.
func (m *IdleManager) SetConnectivityCheck(check func() bool) {
	m.isConnected = check
}

// Start starts the IDLE manager
func (m *IdleManager) Start(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.log.Info().Msg("IDLE manager started")
}

// Stop stops all IDLE connections
func (m *IdleManager) Stop() {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	conns := m.connections
	m.connections = make(map[idleKey]*IdleConnection)
	m.mu.Unlock()

	for key, conn := range conns {
		m.log.Debug().Str("account", key.accountID).Str("folder", key.folder).Msg("Stopping IDLE connection")
		conn.Stop()
	}

	m.log.Info().Msg("IDLE manager stopped")
}

// Close stops all IDLE connections for good: unlike Stop (sleep, network
// loss), later Start and StartAccount calls do nothing. Shutdown uses it so a
// sync still finishing (restartIDLE, ensureSentIdle) can't start connections
// that then read the closed database.
func (m *IdleManager) Close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.Stop()
}

// Events returns the channel for receiving mail events
func (m *IdleManager) Events() <-chan MailEvent {
	return m.events
}

// StartAccount starts one IDLE connection per folder for an account, watching
// INBOX when no folders are given. Folders that already have a running
// connection are left alone, so calling it again only adds missing ones.
func (m *IdleManager) StartAccount(accountID, accountName string, folders ...string) {
	if len(folders) == 0 {
		folders = []string{"INBOX"}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}

	for _, folder := range folders {
		key := idleKey{accountID, folder}
		if conn, exists := m.connections[key]; exists {
			conn.mu.Lock()
			running := conn.running
			conn.mu.Unlock()
			if running {
				m.log.Debug().Str("account", accountName).Str("folder", folder).Msg("IDLE already running for folder")
				continue
			}
			// Goroutine exited (e.g., max reconnect attempts reached) — remove stale entry
			m.log.Debug().Str("account", accountName).Str("folder", folder).Msg("Replacing dead IDLE connection")
			delete(m.connections, key)
		}

		conn := newIdleConnection(accountID, accountName, folder, m.config, m.getCredentials)
		conn.isConnected = m.isConnected
		m.connections[key] = conn

		// Start returns at once (the loop runs in its own goroutine) and marks
		// the connection running, so a repeated StartAccount sees it as live.
		conn.Start(m.ctx, m.events)

		m.log.Info().Str("account", accountName).Str("folder", folder).Msg("Started IDLE for folder")
	}
}

// StopAccount stops all IDLE connections of an account
func (m *IdleManager) StopAccount(accountID string) {
	m.stopWhere(func(key idleKey) bool { return key.accountID == accountID })
}

// StopFolder stops the IDLE connection of one account folder, if any.
func (m *IdleManager) StopFolder(accountID, folder string) {
	m.stopWhere(func(key idleKey) bool { return key == idleKey{accountID, folder} })
}

// stopWhere removes the matching connections under the lock, then stops them
// in parallel without it, so a slow teardown doesn't block StartAccount.
func (m *IdleManager) stopWhere(match func(idleKey) bool) {
	m.mu.Lock()
	stopping := make(map[idleKey]*IdleConnection)
	for key, conn := range m.connections {
		if match(key) {
			stopping[key] = conn
			delete(m.connections, key)
		}
	}
	m.mu.Unlock()

	var wg sync.WaitGroup
	for key, conn := range stopping {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn.Stop()
			m.log.Info().Str("accountID", key.accountID).Str("folder", key.folder).Msg("Stopped IDLE for folder")
		}()
	}
	wg.Wait()
}

// Watched returns the folders an account has IDLE connections for, running or not.
func (m *IdleManager) Watched(accountID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var folders []string
	for key := range m.connections {
		if key.accountID == accountID {
			folders = append(folders, key.folder)
		}
	}
	return folders
}

// Running reports whether an account folder's IDLE connection is running.
func (m *IdleManager) Running(accountID, folder string) bool {
	m.mu.Lock()
	conn := m.connections[idleKey{accountID, folder}]
	m.mu.Unlock()
	if conn == nil {
		return false
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	return conn.running
}

// RestartAccount restarts IDLE for an account on the given folders
func (m *IdleManager) RestartAccount(accountID, accountName string, folders ...string) {
	m.StopAccount(accountID)
	m.StartAccount(accountID, accountName, folders...)
}
