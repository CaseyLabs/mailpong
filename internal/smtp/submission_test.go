package smtp

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

type submissionServerConfig struct {
	extensions     []string
	mailReply      string
	rcptRejectAddr string
	rcptReject     string
	dataReply      string
	finalReply     string
	dropFinal      bool
	interruptBody  bool
	authReject     bool
}

type submissionServer struct {
	listener net.Listener
	commands chan string
	bodySeen chan bool // true means the final DATA terminator arrived
}

func startSubmissionServer(t *testing.T, cfg submissionServerConfig) *submissionServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &submissionServer{listener: ln, commands: make(chan string, 16), bodySeen: make(chan bool, 1)}
	go func() {
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		fmt.Fprint(conn, "220 test ready\r\n")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.TrimSpace(line)
			select {
			case s.commands <- cmd:
			default:
			}
			upper := strings.ToUpper(cmd)
			switch {
			case strings.HasPrefix(upper, "EHLO"):
				fmt.Fprint(conn, "250-test\r\n250-AUTH PLAIN\r\n")
				for _, extension := range cfg.extensions {
					fmt.Fprint(conn, "250-", extension, "\r\n")
				}
				fmt.Fprint(conn, "250 8BITMIME\r\n")
			case strings.HasPrefix(upper, "AUTH PLAIN"):
				if cfg.authReject {
					fmt.Fprint(conn, "535 5.7.8 authentication rejected\r\n")
				} else {
					fmt.Fprint(conn, "235 2.7.0 authentication accepted\r\n")
				}
			case strings.HasPrefix(upper, "MAIL FROM:"):
				fmt.Fprint(conn, cfg.mailReply, "\r\n")
			case strings.HasPrefix(upper, "RCPT TO:"):
				if cfg.rcptRejectAddr != "" && strings.Contains(upper, strings.ToUpper(cfg.rcptRejectAddr)) {
					fmt.Fprint(conn, cfg.rcptReject, "\r\n")
				} else {
					fmt.Fprint(conn, "250 2.1.5 recipient ok\r\n")
				}
			case upper == "DATA":
				fmt.Fprint(conn, cfg.dataReply, "\r\n")
				if !strings.HasPrefix(cfg.dataReply, "354") {
					continue
				}
				if cfg.interruptBody {
					_, _ = r.ReadByte()
					s.bodySeen <- false
					if tcp, ok := conn.(*net.TCPConn); ok {
						_ = tcp.SetLinger(0) // RST while the client is still writing DATA.
					}
					return
				}
				for {
					bodyLine, readErr := r.ReadString('\n')
					if readErr != nil {
						s.bodySeen <- false
						return
					}
					if strings.TrimSpace(bodyLine) == "." {
						s.bodySeen <- true
						if cfg.dropFinal {
							return
						}
						fmt.Fprint(conn, cfg.finalReply, "\r\n")
						return
					}
				}
			case upper == "QUIT":
				fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				fmt.Fprint(conn, "250 ok\r\n")
			}
		}
	}()
	return s
}

func submissionClient(t *testing.T, s *submissionServer) *Client {
	t.Helper()
	addr := s.listener.Addr().(*net.TCPAddr)
	cfg := DefaultConfig()
	cfg.Host, cfg.Port, cfg.Security = "127.0.0.1", addr.Port, SecurityNone
	cfg.ConnectTimeout, cfg.ReadTimeout, cfg.WriteTimeout = 3*time.Second, 5*time.Second, 5*time.Second
	c := NewClient(cfg)
	if err := c.Connect(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestSendMailTrackedClassifiesSMTPOutcomes(t *testing.T) {
	tests := []struct {
		name           string
		cfg            submissionServerConfig
		to             []string
		want           Outcome
		stage          string
		wantTerminator bool
	}{
		{name: "mail rejected", cfg: submissionServerConfig{mailReply: "550 5.1.0 sender rejected"}, to: []string{"a@example.test"}, want: OutcomeRejectedPermanent, stage: "mail_from"},
		{name: "temporary recipient rejection after an accepted recipient", cfg: submissionServerConfig{mailReply: "250 sender ok", rcptRejectAddr: "b@example.test", rcptReject: "451 4.1.1 recipient temporarily rejected"}, to: []string{"a@example.test", "b@example.test", "c@example.test"}, want: OutcomeRejectedTemporary, stage: "rcpt_to"},
		{name: "permanent recipient rejection", cfg: submissionServerConfig{mailReply: "250 sender ok", rcptRejectAddr: "a@example.test", rcptReject: "550 5.1.1 recipient rejected"}, to: []string{"a@example.test"}, want: OutcomeRejectedPermanent, stage: "rcpt_to"},
		{name: "permanent data rejection", cfg: submissionServerConfig{mailReply: "250 sender ok", dataReply: "554 5.5.0 no data"}, to: []string{"a@example.test"}, want: OutcomeRejectedPermanent, stage: "data_command"},
		{name: "temporary final DATA rejection", cfg: submissionServerConfig{mailReply: "250 sender ok", dataReply: "354 go", finalReply: "451 4.3.0 temporary failure"}, to: []string{"a@example.test"}, want: OutcomeRejectedTemporary, stage: "data_final_reply", wantTerminator: true},
		{name: "permanent final DATA rejection", cfg: submissionServerConfig{mailReply: "250 sender ok", dataReply: "354 go", finalReply: "554 5.5.0 rejected"}, to: []string{"a@example.test"}, want: OutcomeRejectedPermanent, stage: "data_final_reply", wantTerminator: true},
		{name: "accepted", cfg: submissionServerConfig{mailReply: "250 sender ok", dataReply: "354 go", finalReply: "250 2.0.0 queued"}, to: []string{"a@example.test"}, want: OutcomeAccepted, stage: "accepted", wantTerminator: true},
		{name: "final reply lost", cfg: submissionServerConfig{mailReply: "250 sender ok", dataReply: "354 go", dropFinal: true}, to: []string{"a@example.test"}, want: OutcomeUnknown, stage: "data_final_reply", wantTerminator: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := startSubmissionServer(t, tt.cfg)
			c := submissionClient(t, s)
			result, err := c.SendMailTracked("from@example.test", tt.to, testWireMessage("body\r\n"), nil)
			if (err == nil) != (tt.want == OutcomeAccepted) {
				t.Fatalf("err = %v, outcome = %s", err, result.Outcome)
			}
			if result.Outcome != tt.want || result.Stage != tt.stage {
				t.Errorf("result = %+v, want outcome %s stage %s", result, tt.want, tt.stage)
			}
			if tt.name == "temporary recipient rejection after an accepted recipient" {
				if len(result.Recipients) != 3 || !result.Recipients[0].Accepted || result.Recipients[1].Accepted || result.Recipients[1].Code != 451 || result.Recipients[2].Address != "c@example.test" || result.Recipients[2].Code != 0 {
					t.Errorf("recipient results = %+v", result.Recipients)
				}
			}
			if strings.Contains(tt.name, "DATA") || tt.wantTerminator {
				select {
				case got := <-s.bodySeen:
					if got != tt.wantTerminator {
						t.Errorf("terminator seen = %v, want %v", got, tt.wantTerminator)
					}
				case <-time.After(time.Second):
					t.Fatal("server did not finish observing DATA")
				}
			}
		})
	}
}

func TestSendMailTrackedInterruptedBodyIsUnknown(t *testing.T) {
	s := startSubmissionServer(t, submissionServerConfig{mailReply: "250 sender ok", dataReply: "354 go", interruptBody: true})
	c := submissionClient(t, s)
	msg := testWireMessage(strings.Repeat(strings.Repeat("x", 76)+"\r\n", 900000))
	result, err := c.SendMailTracked("from@example.test", []string{"a@example.test"}, msg, nil)
	if err == nil || result.Outcome != OutcomeUnknown || result.Stage != "data_body" {
		t.Fatalf("result = %+v, err = %v; want unknown after an interrupted body write", result, err)
	}
	select {
	case terminatorSeen := <-s.bodySeen:
		if terminatorSeen {
			t.Fatal("server received the DATA terminator after interrupting the body")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not interrupt DATA")
	}
}

func TestSMTPConnectAndAuthRejectionDoNotSubmit(t *testing.T) {
	t.Run("connection refused", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := ln.Addr().(*net.TCPAddr)
		_ = ln.Close()
		cfg := DefaultConfig()
		cfg.Host, cfg.Port, cfg.Security = "127.0.0.1", addr.Port, SecurityNone
		cfg.ConnectTimeout = time.Second
		if err := NewClient(cfg).Connect(); err == nil {
			t.Fatal("Connect unexpectedly succeeded")
		}
	})
	t.Run("authentication rejected before MAIL", func(t *testing.T) {
		s := startSubmissionServer(t, submissionServerConfig{mailReply: "250 sender ok", authReject: true})
		c := submissionClient(t, s)
		if err := c.Login(); err == nil {
			t.Fatal("Login unexpectedly succeeded")
		}
		for {
			select {
			case cmd := <-s.commands:
				if strings.HasPrefix(strings.ToUpper(cmd), "MAIL FROM:") || strings.EqualFold(cmd, "DATA") {
					t.Fatalf("submission command sent after auth rejection: %q", cmd)
				}
			default:
				return
			}
		}
	})
}

func TestSendMailTrackedCallbackAbortsBeforeBody(t *testing.T) {
	s := startSubmissionServer(t, submissionServerConfig{mailReply: "250 sender ok", dataReply: "354 go", finalReply: "250 accepted"})
	c := submissionClient(t, s)
	called := false
	result, err := c.SendMailTracked("from@example.test", []string{"a@example.test"}, testWireMessage("body\r\n"), func(pending SubmissionResult) error {
		called = true
		if pending.Outcome != OutcomeUnknown || len(pending.Recipients) != 1 || !pending.Recipients[0].Accepted {
			t.Errorf("callback result = %+v", pending)
		}
		return fmt.Errorf("persistence unavailable")
	})
	if !called || err == nil || result.Outcome != OutcomeNotSubmitted || result.Stage != "before_data_body" {
		t.Fatalf("called=%v result=%+v err=%v", called, result, err)
	}
	select {
	case terminatorSeen := <-s.bodySeen:
		if terminatorSeen {
			t.Fatal("server received the DATA terminator")
		}
	case <-time.After(time.Second):
		t.Fatal("server did not observe connection close")
	}
}
