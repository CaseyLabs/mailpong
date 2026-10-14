package setupdiagnostic

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func probeCert(t *testing.T, expired bool) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(time.Hour)
	if expired {
		until = time.Now().Add(-time.Hour)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-2 * time.Hour), NotAfter: until, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots.AddCert(parsed)
	return cert, roots
}
func probeServer(t *testing.T, cert *tls.Certificate, serve func(net.Conn)) (string, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if cert != nil {
		ln = tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{*cert}, MinVersion: tls.VersionTLS12})
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_ = c.SetDeadline(time.Now().Add(4 * time.Second))
		serve(c)
	}()
	t.Cleanup(func() { _ = ln.Close(); <-done })
	return "127.0.0.1", ln.Addr().(*net.TCPAddr).Port
}
func TestSMTPProbeNeverStartsTransaction(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(fmt.Sprint(reject), func(t *testing.T) {
			cert, roots := probeCert(t, false)
			var commands []string
			var mu sync.Mutex
			host, port := probeServer(t, &cert, func(c net.Conn) {
				_, _ = fmt.Fprint(c, "220 fixture\r\n")
				s := bufio.NewScanner(c)
				for s.Scan() {
					line := s.Text()
					verb := strings.Fields(line)[0]
					mu.Lock()
					commands = append(commands, verb)
					mu.Unlock()
					switch verb {
					case "EHLO":
						_, _ = fmt.Fprint(c, "250-fixture\r\n250-AUTH PLAIN\r\n250 SIZE 5000\r\n")
					case "AUTH":
						if reject {
							_, _ = fmt.Fprint(c, "535 5.7.139 disabled secret-token user@example.test\r\n")
						} else {
							_, _ = fmt.Fprint(c, "235 accepted\r\n")
						}
					case "*":
						_, _ = fmt.Fprint(c, "501 authentication cancelled\r\n")
					case "QUIT":
						_, _ = fmt.Fprint(c, "221 bye\r\n")
						return
					default:
						t.Errorf("forbidden command %q", verb)
						return
					}
				}
			})
			checks := Probe(context.Background(), Config{Protocol: "smtp", Host: host, Port: port, Security: "tls", Username: "user@example.test", Password: "secret-token", TLSConfig: &tls.Config{RootCAs: roots}})
			want := "passed"
			if reject {
				want = "failed"
			}
			if checks[0].Status != "passed" || checks[1].Status != want {
				t.Fatalf("%+v", checks)
			}
			for _, c := range checks {
				if strings.Contains(c.Detail, "secret-token") || strings.Contains(c.Detail, "user@example.test") {
					t.Fatal("raw rejection leaked")
				}
			}
			mu.Lock()
			defer mu.Unlock()
			for _, v := range commands {
				if v != "EHLO" && v != "AUTH" && v != "QUIT" && v != "*" {
					t.Fatal(v)
				}
			}
		})
	}
}
func TestIMAPReadOnlyProbe(t *testing.T) {
	cert, roots := probeCert(t, false)
	host, port := probeServer(t, &cert, func(c net.Conn) {
		_, _ = fmt.Fprint(c, "* OK fixture\r\n")
		s := bufio.NewScanner(c)
		for s.Scan() {
			parts := strings.Fields(s.Text())
			if len(parts) < 2 {
				return
			}
			tag, cmd := parts[0], parts[1]
			switch cmd {
			case "CAPABILITY":
				_, _ = fmt.Fprintf(c, "* CAPABILITY IMAP4rev1\r\n%s OK done\r\n", tag)
			case "LOGIN":
				_, _ = fmt.Fprintf(c, "%s OK login accepted\r\n", tag)
			default:
				t.Errorf("forbidden IMAP command %s", cmd)
				return
			}
		}
	})
	checks := Probe(context.Background(), Config{Protocol: "imap", Host: host, Port: port, Security: "tls", Username: "user", Password: "secret", TLSConfig: &tls.Config{RootCAs: roots}})
	if checks[0].Status != "passed" || checks[1].Status != "passed" {
		t.Fatalf("%+v", checks)
	}
}
func TestTLSFailuresDoNotAuthenticate(t *testing.T) {
	for _, kind := range []string{"trust", "hostname", "expiry"} {
		t.Run(kind, func(t *testing.T) {
			cert, roots := probeCert(t, kind == "expiry")
			tc := &tls.Config{RootCAs: roots}
			if kind == "trust" {
				tc = nil
			}
			if kind == "hostname" {
				key, err := rsa.GenerateKey(rand.Reader, 2048)
				if err != nil {
					t.Fatal(err)
				}
				parsed, _ := x509.ParseCertificate(cert.Certificate[0])
				parsed.PublicKey = &key.PublicKey
				parsed.IPAddresses = nil
				parsed.DNSNames = []string{"wrong.example.test"}
				der, err := x509.CreateCertificate(rand.Reader, parsed, parsed, &key.PublicKey, key)
				if err != nil {
					t.Fatal(err)
				}
				cert.Certificate = [][]byte{der}
				cert.PrivateKey = key
				tc.RootCAs = x509.NewCertPool()
				p, _ := x509.ParseCertificate(der)
				tc.RootCAs.AddCert(p)
			}
			host, port := probeServer(t, &cert, func(c net.Conn) { _, _ = fmt.Fprint(c, "220 fixture\r\n"); var b [1]byte; _, _ = c.Read(b[:]) })
			checks := Probe(context.Background(), Config{Protocol: "smtp", Host: host, Port: port, Security: "tls", TLSConfig: tc})
			if checks[0].Status != "failed" || checks[0].Code != "tls-verification" || checks[1].Status != "skipped" {
				t.Fatalf("%+v", checks)
			}
		})
	}
}
func TestMissingSTARTTLSAndCancellation(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		host, port := probeServer(t, nil, func(c net.Conn) {
			_, _ = fmt.Fprint(c, "220 fixture\r\n")
			s := bufio.NewScanner(c)
			if s.Scan() {
				_, _ = fmt.Fprint(c, "250 fixture\r\n")
			}
			var b [1]byte
			_, _ = c.Read(b[:])
		})
		checks := Probe(context.Background(), Config{Protocol: "smtp", Host: host, Port: port, Security: "starttls"})
		if checks[0].Status != "failed" || !strings.Contains(checks[0].Detail, "STARTTLS") || checks[1].Status != "skipped" {
			t.Fatalf("%+v", checks)
		}
	})
	for _, timeout := range []bool{false, true} {
		t.Run(fmt.Sprint(timeout), func(t *testing.T) {
			host, port := probeServer(t, nil, func(c net.Conn) { _, _ = io.Copy(io.Discard, c) })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if timeout {
				ctx, cancel = context.WithTimeout(ctx, 30*time.Millisecond)
				defer cancel()
			} else {
				time.AfterFunc(30*time.Millisecond, cancel)
			}
			started := time.Now()
			checks := Probe(ctx, Config{Protocol: "imap", Host: host, Port: port, Security: "tls"})
			want := "cancelled"
			if timeout {
				want = "unknown"
			}
			if checks[0].Status != want || time.Since(started) > time.Second {
				t.Fatalf("%+v duration %s", checks, time.Since(started))
			}
		})
	}
}
func TestFailureRedactionAndPlaintextSkip(t *testing.T) {
	status, detail, code := Failure(context.Background(), fmt.Errorf("secret token password user@example.test request-id"))
	if status != "failed" || strings.Contains(detail, "secret") || code != "probe-failed" {
		t.Fatal(status, detail, code)
	}
	c := Probe(context.Background(), Config{Protocol: "smtp", Host: "must-not-resolve.invalid", Security: "none"})
	if c[0].Status != "skipped" || c[1].Status != "skipped" {
		t.Fatal(c)
	}
}
