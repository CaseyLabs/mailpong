package imap

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/rs/zerolog"
)

// SY9: ForceClose must not wait behind a command stuck on the server. An
// APPEND to a server without LITERAL+ holds the connection's command lock
// while it waits for a continuation that never comes, which is how removing
// an account hung.
func TestForceCloseDoesNotWaitForStuckCommand(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	appendSeen := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("* OK [CAPABILITY IMAP4rev1] ready\r\n"))
		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if strings.Contains(line, "APPEND") {
				close(appendSeen)
			}
			// Never answer: no continuation, no BYE.
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{client: imapclient.New(conn, nil), log: zerolog.Nop()}
	if err := c.client.WaitGreeting(); err != nil {
		t.Fatal(err)
	}
	appendDone := make(chan struct{})
	go func() {
		defer close(appendDone)
		_ = c.client.Append("INBOX", 10, nil)
	}()
	select {
	case <-appendSeen:
	case <-time.After(5 * time.Second):
		t.Fatal("server never saw the APPEND")
	}

	done := make(chan struct{})
	go func() {
		_ = c.ForceClose()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ForceClose waited for the stuck command")
	}
	// Closing fails the stuck command, so it releases the lock the
	// background LOGOUT waits on and nothing is left blocked.
	select {
	case <-appendDone:
	case <-time.After(5 * time.Second):
		t.Fatal("the stuck command never returned after ForceClose")
	}
}
