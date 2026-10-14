// Package setupdiagnostic contains user-invoked, read-only account probes.
// It deliberately has no message, submission, mailbox-write or consent API.
package setupdiagnostic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	mailimap "github.com/caseylabs/mailven/internal/imap"
	mailsmtp "github.com/caseylabs/mailven/internal/smtp"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"
)

// Check contains only structured, redacted evidence. Never store raw server replies.
type Check struct {
	Name               string `json:"name"`
	Status             string `json:"status"`
	At                 string `json:"at"`
	EvidenceAgeSeconds int64  `json:"evidenceAgeSeconds"`
	Detail             string `json:"detail"`
	Code               string `json:"code"`
}

func NewCheck(name, status, detail string) Check {
	return Check{Name: name, Status: status, At: time.Now().UTC().Format(time.RFC3339Nano), Detail: detail}
}

// Config is internal input only; it must never be bound or serialized to the UI.
type Config struct {
	Protocol, Host, Security, Username, Password, Token, Mechanism string
	Port                                                           int
	OAuth                                                          bool
	AuthSkip                                                       string
	TLSConfig                                                      *tls.Config // tests may supply roots; production uses system verification
}

// Probe has a hard deadline and closes the socket on cancellation, including while
// waiting for a greeting or SASL response. Only CAPABILITY/EHLO and login are used.
func Probe(parent context.Context, c Config) []Check {
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()
	transport := NewCheck(c.Protocol+" transport", "unknown", "")
	auth := NewCheck(c.Protocol+" authentication", "skipped", "Transport did not complete; no authentication attempted.")
	if c.AuthSkip != "" {
		auth.Detail = c.AuthSkip
	}
	fail := func(err error) []Check {
		transport.Status, transport.Detail, transport.Code = Failure(ctx, err)
		return []Check{transport, auth}
	}
	if c.Protocol != "imap" && c.Protocol != "smtp" {
		return fail(errors.New("unsupported protocol"))
	}
	if c.Security != "tls" && c.Security != "starttls" {
		transport.Status, transport.Detail = "skipped", "This diagnostic requires TLS or STARTTLS. Review the saved security setting; no plaintext login attempted."
		return []Check{transport, auth}
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(c.Host, strconv.Itoa(c.Port)))
	if err != nil {
		return fail(err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err = conn.SetDeadline(deadline); err != nil {
		return fail(err)
	}
	tc := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}
	if c.TLSConfig != nil {
		tc = c.TLSConfig.Clone()
		tc.ServerName = c.Host
	}
	if tc.InsecureSkipVerify {
		return fail(errors.New("insecure TLS configuration refused"))
	}
	var wire net.Conn = conn
	if c.Security == "tls" {
		secured := tls.Client(conn, tc)
		if err = secured.HandshakeContext(ctx); err != nil {
			return fail(err)
		}
		wire = secured
	}
	if c.Protocol == "smtp" {
		client, err := smtp.NewClient(wire, c.Host)
		if err != nil {
			return fail(err)
		}
		defer client.Close()
		if err = client.Hello("[127.0.0.1]"); err != nil {
			return fail(err)
		}
		if c.Security == "starttls" {
			if ok, _ := client.Extension("STARTTLS"); !ok {
				transport.Status, transport.Detail = "failed", "STARTTLS is not advertised. Confirm the port/security setting and protocol restrictions with the administrator. No authentication attempted."
				return []Check{transport, auth}
			}
			if err = client.StartTLS(tc); err != nil {
				return fail(err)
			}
		}
		var caps []string
		for _, cap := range []string{"AUTH", "SIZE", "SMTPUTF8", "8BITMIME", "DSN"} {
			if ok, _ := client.Extension(cap); ok {
				caps = append(caps, cap)
			}
		}
		transport.Status, transport.Detail = "passed", "TLS verified; EHLO completed. Advertised: "+strings.Join(caps, ", ")+". No mail transaction was started."
		if c.AuthSkip != "" {
			auth.Detail = c.AuthSkip
			return []Check{transport, auth}
		}
		var method smtp.Auth
		_, mechanisms := client.Extension("AUTH")
		has := func(m string) bool {
			for _, v := range strings.Fields(mechanisms) {
				if strings.EqualFold(v, m) {
					return true
				}
			}
			return false
		}
		switch {
		case c.OAuth && has("XOAUTH2"):
			method = mailsmtp.XOAuth2Auth(c.Username, c.Token)
		case !c.OAuth && c.Mechanism == "login" && has("LOGIN"):
			method = mailsmtp.LoginAuth(c.Username, c.Password)
		case !c.OAuth && c.Mechanism != "login" && has("PLAIN"):
			method = smtp.PlainAuth("", c.Username, c.Password, c.Host)
		case !c.OAuth && (c.Mechanism == "" || c.Mechanism == "auto") && has("LOGIN"):
			method = mailsmtp.LoginAuth(c.Username, c.Password)
		default:
			auth.Status, auth.Detail = "unknown", "Configured authentication is not advertised; check protocol/auth restrictions. No fallback or consent attempted."
			return []Check{transport, auth}
		}
		err = client.Auth(method)
		auth = authResult(ctx, c.Protocol, err)
		return []Check{transport, auth}
	}
	var client *imapclient.Client
	if c.Security == "starttls" {
		client, err = imapclient.NewStartTLS(wire, &imapclient.Options{TLSConfig: tc})
	} else {
		client = imapclient.New(wire, nil)
		err = client.WaitGreeting()
	}
	if err != nil {
		return fail(err)
	}
	defer client.Close()
	caps, err := client.Capability().Wait()
	if err != nil {
		return fail(err)
	}
	transport.Status, transport.Detail = "passed", "TLS verified; IMAP CAPABILITY completed."
	for _, cap := range []imap.Cap{imap.CapIMAP4rev1, imap.CapIMAP4rev2, imap.CapLoginDisabled, imap.CapIdle} {
		if caps.Has(cap) {
			transport.Detail += " " + string(cap)
		}
	}
	if c.AuthSkip != "" {
		auth.Detail = c.AuthSkip
		return []Check{transport, auth}
	}
	switch {
	case c.OAuth:
		err = client.Authenticate(mailimap.NewXOAuth2Client(c.Username, c.Token))
	case c.Mechanism == "plain" || ((c.Mechanism == "" || c.Mechanism == "auto") && caps.Has(imap.CapLoginDisabled)):
		err = client.Authenticate(sasl.NewPlainClient("", c.Username, c.Password))
	default:
		err = client.Login(c.Username, c.Password).Wait()
	}
	return []Check{transport, authResult(ctx, c.Protocol, err)}
}
func authResult(ctx context.Context, protocol string, err error) Check {
	c := NewCheck(protocol+" authentication", "passed", "Login accepted for this mailbox. This proves neither Send As nor sending policy, delivery or inbox placement.")
	if err != nil {
		c.Status, c.Detail, c.Code = Failure(ctx, err)
		if c.Code == "" && c.Status == "failed" {
			c.Detail = "Login rejected or protocol restricted. Verify credentials, granted protocol scopes and administrator restrictions; rejection alone cannot distinguish them. Reauthorize only through the account's explicit action."
		}
	}
	return c
}

// Failure deliberately discards free-form server errors, request IDs and URLs:
// these may echo bearer tokens, SASL payloads, passwords or mailbox addresses.
func Failure(ctx context.Context, err error) (status, detail, code string) {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return "cancelled", "Cancelled by user; no automatic retry.", ""
	}
	var ne net.Error
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.As(err, &ne) && ne.Timeout() {
		return "unknown", "Probe timed out. Retry manually after 60 seconds; check connectivity or provider status.", "timeout"
	}
	var unknown x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &unknown) || errors.As(err, &hostname) || errors.As(err, &invalid) {
		return "failed", "TLS certificate trust, hostname or validity failed. Check the device clock, saved hostname and certificate chain; review certificate trust in account settings. Verification was not bypassed.", "tls-verification"
	}
	var reply *textproto.Error
	if errors.As(err, &reply) {
		return "failed", "Server rejected the probe. Check credentials, protocol permissions and provider/tenant restrictions. Retry transient failures manually after 60 seconds; use explicit account reauthorization for missing consent.", fmt.Sprintf("smtp-%d", reply.Code)
	}
	var ie *imap.Error
	if errors.As(err, &ie) {
		switch ie.Code {
		case imap.ResponseCodeAuthenticationFailed, imap.ResponseCodeAuthorizationFailed:
			code = "imap-auth-rejected"
		default:
			code = "imap-rejected"
		}
		return "failed", "IMAP rejected the probe. Check credentials, protocol permissions and provider/tenant restrictions; login rejection does not identify which policy caused it.", code
	}
	return "failed", "Connection or protocol probe failed. Confirm endpoint, port, TLS mode and administrator restrictions; retry manually after 60 seconds. Raw error/request ID omitted for privacy.", "probe-failed"
}
