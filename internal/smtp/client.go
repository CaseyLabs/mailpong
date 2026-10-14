// Package smtp provides SMTP client functionality for Mailven
package smtp

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/logging"
	"github.com/rs/zerolog"
)

// deadlineConn wraps a net.Conn and sets per-read/write deadlines before each
// I/O operation, preventing indefinite blocking on unresponsive servers.
type deadlineConn struct {
	net.Conn
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func (c *deadlineConn) Read(b []byte) (int, error) {
	if c.readTimeout > 0 {
		if err := c.Conn.SetReadDeadline(time.Now().Add(c.readTimeout)); err != nil {
			return 0, err
		}
	}
	return c.Conn.Read(b)
}

func (c *deadlineConn) Write(b []byte) (int, error) {
	if c.writeTimeout > 0 {
		if err := c.Conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
			return 0, err
		}
	}
	return c.Conn.Write(b)
}

// SecurityType represents the connection security method
type SecurityType string

const (
	SecurityNone     SecurityType = "none"
	SecurityTLS      SecurityType = "tls"
	SecurityStartTLS SecurityType = "starttls"
)

// ClientConfig holds the configuration for connecting to an SMTP server
type ClientConfig struct {
	Host     string
	Port     int
	Security SecurityType
	Username string
	Password string

	// OAuth2 authentication
	AuthType    AuthType // "password" or "oauth2" (defaults to "password")
	AccessToken string   // OAuth2 access token (when AuthType is "oauth2")

	// AuthMechanism selects the password SASL mechanism: "plain", "login", or
	// ""/"auto" (pick from the server's EHLO AUTH advertisement). Ignored for
	// OAuth2.
	AuthMechanism string

	// Timeouts
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration

	// TLS config (optional)
	TLSConfig *tls.Config
}

// DefaultConfig returns a ClientConfig with sensible defaults
func DefaultConfig() ClientConfig {
	return ClientConfig{
		Port:           587,
		Security:       SecurityStartTLS,
		ConnectTimeout: 30 * time.Second,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
	}
}

// Client wraps the net/smtp client with additional functionality
type Client struct {
	config ClientConfig
	client *smtp.Client
	log    zerolog.Logger
}

// NewClient creates a new SMTP client but does not connect
func NewClient(config ClientConfig) *Client {
	return &Client{
		config: config,
		log:    logging.WithComponent("smtp"),
	}
}

// Connect establishes a connection to the SMTP server
func (c *Client) Connect() error {
	addr := net.JoinHostPort(c.config.Host, strconv.Itoa(c.config.Port))

	c.log.Debug().
		Str("host", c.config.Host).
		Int("port", c.config.Port).
		Str("security", string(c.config.Security)).
		Msg("Connecting to SMTP server")

	var conn net.Conn
	var err error

	// Create TLS config if not provided
	tlsConfig := c.config.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			ServerName: c.config.Host,
		}
	}

	switch c.config.Security {
	case SecurityTLS:
		// Connect with TLS directly (port 465).
		// Wrap with deadlineConn BEFORE TLS so tls.Conn is the outer layer.
		// net/smtp checks for *tls.Conn to set serverInfo.TLS — if deadlineConn
		// wraps the outside, PlainAuth sees "unencrypted connection" and refuses.
		dialer := &net.Dialer{Timeout: c.config.ConnectTimeout}
		rawConn, dialErr := dialer.Dial("tcp", addr)
		if dialErr != nil {
			return fmt.Errorf("failed to connect: %w", dialErr)
		}
		wrappedConn := &deadlineConn{
			Conn:         rawConn,
			readTimeout:  c.config.ReadTimeout,
			writeTimeout: c.config.WriteTimeout,
		}
		tlsConn := tls.Client(wrappedConn, tlsConfig)
		if hsErr := tlsConn.Handshake(); hsErr != nil {
			tlsConn.Close()
			return fmt.Errorf("TLS handshake failed: %w", hsErr)
		}
		conn = tlsConn

	case SecurityStartTLS, SecurityNone:
		// Connect plain first, wrap with deadline
		dialer := &net.Dialer{Timeout: c.config.ConnectTimeout}
		rawConn, dialErr := dialer.Dial("tcp", addr)
		if dialErr != nil {
			return fmt.Errorf("failed to connect: %w", dialErr)
		}
		conn = &deadlineConn{
			Conn:         rawConn,
			readTimeout:  c.config.ReadTimeout,
			writeTimeout: c.config.WriteTimeout,
		}
	}

	// Create SMTP client
	c.client, err = smtp.NewClient(conn, c.config.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}

	// net/smtp would greet with "EHLO localhost", a mild spam signal. Greet
	// with the local address literal instead, as Thunderbird does, which
	// also avoids leaking the machine's hostname.
	if err := c.client.Hello(ehloName(conn.LocalAddr())); err != nil {
		c.client.Close()
		c.client = nil
		return fmt.Errorf("EHLO failed: %w", err)
	}

	// Upgrade to TLS if using STARTTLS
	if c.config.Security == SecurityStartTLS {
		// Never fall back to plaintext: credentials would be sent unencrypted
		// if STARTTLS is unsupported or stripped by an attacker.
		if ok, _ := c.client.Extension("STARTTLS"); !ok {
			c.client.Close()
			c.client = nil
			return fmt.Errorf("server does not support STARTTLS; refusing to continue without TLS")
		}
		if err := c.client.StartTLS(tlsConfig); err != nil {
			c.client.Close()
			c.client = nil
			return fmt.Errorf("failed to upgrade to TLS: %w", err)
		}
		c.log.Debug().Msg("Upgraded connection to TLS via STARTTLS")
	}

	c.log.Info().
		Str("host", c.config.Host).
		Msg("Connected to SMTP server")

	return nil
}

// ehloName formats a local address as an RFC 5321 address literal:
// "[192.0.2.1]" or "[IPv6:2001:db8::1]". It falls back to "localhost" when
// the address is not an IP.
func ehloName(addr net.Addr) string {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok || tcp.IP == nil {
		return "localhost"
	}
	if ip4 := tcp.IP.To4(); ip4 != nil {
		return "[" + ip4.String() + "]"
	}
	return "[IPv6:" + tcp.IP.String() + "]"
}

// Login authenticates with the SMTP server
func (c *Client) Login() error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	// Determine auth type (default to password)
	authType := c.config.AuthType
	if authType == "" {
		authType = AuthTypePassword
	}

	c.log.Debug().
		Str("username", c.config.Username).
		Str("authType", string(authType)).
		Msg("Authenticating")

	// Check available auth mechanisms
	if ok, mechanisms := c.client.Extension("AUTH"); !ok {
		return fmt.Errorf("server does not support authentication")
	} else {
		c.log.Debug().Str("mechanisms", mechanisms).Msg("Available auth mechanisms")
	}

	var err error
	switch authType {
	case AuthTypeOAuth2:
		err = c.loginOAuth2()
	default:
		err = c.loginPassword()
	}

	if err != nil {
		return err
	}

	c.log.Info().
		Str("username", c.config.Username).
		Msg("Authenticated successfully")

	return nil
}

// loginPassword authenticates using password (PLAIN or LOGIN mechanism). A
// configured AuthMechanism forces that mechanism; otherwise the choice honors
// the server's EHLO AUTH advertisement. Some servers (e.g. advertising only
// "GSSAPI NTLM LOGIN") drop the connection on a rejected AUTH PLAIN, which
// would make a post-failure LOGIN fallback hit a closed socket (#355).
func (c *Client) loginPassword() error {
	switch strings.ToLower(c.config.AuthMechanism) {
	case "plain":
		return c.authPlain()
	case "login":
		return c.authLogin()
	}

	_, mechanisms := c.client.Extension("AUTH")
	if !mechanismAdvertised(mechanisms, "PLAIN") && mechanismAdvertised(mechanisms, "LOGIN") {
		c.log.Debug().Msg("PLAIN not advertised, using LOGIN")
		return c.authLogin()
	}

	// PLAIN advertised — or neither PLAIN nor LOGIN advertised (best-effort for
	// servers with broken advertisements): try PLAIN first, fall back to LOGIN.
	if err := c.authPlain(); err != nil {
		c.log.Debug().Msg("PLAIN auth failed, trying LOGIN")
		return c.authLogin()
	}
	return nil
}

// authPlain authenticates with the PLAIN mechanism.
func (c *Client) authPlain() error {
	auth := smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.Host)
	if err := c.client.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	return nil
}

// authLogin authenticates with the LOGIN mechanism.
func (c *Client) authLogin() error {
	auth := LoginAuth(c.config.Username, c.config.Password)
	if err := c.client.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	return nil
}

// mechanismAdvertised reports whether name appears in the space-separated EHLO
// AUTH parameter list (case-insensitive).
func mechanismAdvertised(mechanisms, name string) bool {
	for _, m := range strings.Fields(mechanisms) {
		if strings.EqualFold(m, name) {
			return true
		}
	}
	return false
}

// loginOAuth2 authenticates using OAuth2 XOAUTH2 mechanism
func (c *Client) loginOAuth2() error {
	if c.config.AccessToken == "" {
		return fmt.Errorf("OAuth2 authentication requires an access token")
	}

	c.log.Debug().Msg("Authenticating with XOAUTH2")

	auth := XOAuth2Auth(c.config.Username, c.config.AccessToken)
	if err := c.client.Auth(auth); err != nil {
		return fmt.Errorf("XOAUTH2 authentication failed: %w", err)
	}

	return nil
}

// Close closes the connection to the SMTP server
func (c *Client) Close() error {
	if c.client == nil {
		return nil
	}

	c.log.Debug().Msg("Closing SMTP connection")

	// Send QUIT command
	if err := c.client.Quit(); err != nil {
		c.log.Warn().Err(err).Msg("QUIT failed, closing anyway")
		return c.client.Close()
	}

	return nil
}

// SendMail sends an email message
func (c *Client) SendMail(from string, to []string, msg []byte) error {
	_, err := c.SendMailTracked(from, to, msg, nil)
	return err
}

// SendMailTracked sends a message and reports the SMTP submission outcome.
// beforeData runs after the server accepts DATA and before any message bytes
// are written. If the callback fails, the SMTP connection is closed to abort
// DATA without sending the message body or its terminator.
func (c *Client) SendMailTracked(from string, to []string, msg []byte, beforeData func(SubmissionResult) error) (SubmissionResult, error) {
	result := SubmissionResult{Outcome: OutcomeNotSubmitted, Stage: "preflight"}
	if c.client == nil {
		return result, submissionError(result, fmt.Errorf("not connected"))
	}

	var err error
	from, to, err = c.preflight(from, to, msg)
	if err != nil {
		return result, submissionError(result, err)
	}

	c.log.Debug().
		Str("from", from).
		Strs("to", to).
		Int("size", len(msg)).
		Msg("Sending message")

	// Set the sender
	if err := c.client.Mail(from); err != nil {
		result.Stage = "mail_from"
		if outcome, _, _, rejected := rejectedOutcome(err); rejected {
			result.Outcome = outcome
		}
		return result, submissionError(result, fmt.Errorf("failed to set sender: %w", err))
	}

	// Set recipients
	for _, recipient := range to {
		if err := c.client.Rcpt(recipient); err != nil {
			entry := RecipientResult{Address: recipient}
			if outcome, code, response, rejected := rejectedOutcome(err); rejected {
				entry.Code, entry.Response = code, response
				result.Outcome = outcome
			} else {
				result.Outcome = OutcomeNotSubmitted
			}
			result.Recipients = append(result.Recipients, entry)
			for _, unattempted := range to[len(result.Recipients):] {
				result.Recipients = append(result.Recipients, RecipientResult{Address: unattempted})
			}
			result.Stage = "rcpt_to"
			return result, submissionError(result, fmt.Errorf("failed to add recipient %s: %w", recipient, err))
		}
		result.Recipients = append(result.Recipients, RecipientResult{Address: recipient, Accepted: true})
	}

	// Send the message body
	w, err := c.client.Data()
	if err != nil {
		result.Stage = "data_command"
		if outcome, _, _, rejected := rejectedOutcome(err); rejected {
			result.Outcome = outcome
		}
		return result, submissionError(result, fmt.Errorf("failed to start data transfer: %w", err))
	}
	result.Outcome = OutcomeUnknown
	result.Stage = "data_body"
	if beforeData != nil {
		if err := beforeData(result); err != nil {
			result.Outcome = OutcomeNotSubmitted
			result.Stage = "before_data_body"
			_ = c.client.Close()
			c.client = nil
			return result, submissionError(result, fmt.Errorf("before-data callback failed: %w", err))
		}
	}

	if _, err := io.Copy(w, bytes.NewReader(msg)); err != nil {
		return result, submissionError(result, fmt.Errorf("failed to write message: %w", err))
	}

	if err := w.Close(); err != nil {
		result.Stage = "data_final_reply"
		if outcome, _, _, rejected := rejectedOutcome(err); rejected {
			result.Outcome = outcome
		}
		return result, submissionError(result, fmt.Errorf("failed to complete message: %w", err))
	}
	result.Outcome = OutcomeAccepted
	result.Stage = "accepted"

	c.log.Info().
		Str("from", from).
		Int("recipients", len(to)).
		Msg("Message sent successfully")

	return result, nil
}

// Reset resets the SMTP session, allowing a new message to be sent
func (c *Client) Reset() error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	return c.client.Reset()
}

// loginAuth implements smtp.Auth for the LOGIN mechanism
type loginAuth struct {
	username string
	password string
}

// LoginAuth returns an Auth that implements the LOGIN authentication mechanism
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}

	prompt := strings.ToLower(string(fromServer))
	switch {
	case strings.Contains(prompt, "username"):
		return []byte(a.username), nil
	case strings.Contains(prompt, "password"):
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unknown prompt: %s", fromServer)
	}
}
