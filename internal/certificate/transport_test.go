package certificate

import (
	"errors"
	"fmt"
	"github.com/caseylabs/mailven/internal/kit/errorutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestTransport checks that trust is scoped to the request host, including an
// IP-literal host (no SNI, so the TLS connection state carries no server name).
func TestTransport(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	fingerprint := Fingerprint(srv.Certificate().Raw)

	store := openTestStore(t)
	client := &http.Client{Transport: NewTransport(store)}
	get := func(rawURL string) error {
		resp, err := client.Get(rawURL)
		if err == nil {
			resp.Body.Close()
		}
		return err
	}

	var certErr *Error
	if err := get(srv.URL); !errors.As(err, &certErr) {
		t.Fatalf("untrusted: want *Error, got %v", err)
	}

	if err := store.AcceptSession("127.0.0.1", fingerprint); err != nil {
		t.Fatal(err)
	}
	if err := get(srv.URL); err != nil {
		t.Fatalf("trusted for 127.0.0.1: %v", err)
	}

	// Same listener, different host name: the pin does not carry over.
	if err := get("https://localhost:" + u.Port() + "/"); !errors.As(err, &certErr) {
		t.Fatalf("localhost: want *Error, got %v", err)
	}
}

func TestTransportFailureDetails(t *testing.T) {
	store := openTestStore(t)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/discovery-target" {
			http.Redirect(w, r, srv.URL, http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()
	client := &http.Client{Transport: NewTransport(store)}
	resp, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	_, err = client.Get(origin.URL + "/discovery-target")
	var failure *Error
	if !errors.As(err, &failure) {
		t.Fatalf("want certificate failure, got %v", err)
	}
	if failure.Host != "127.0.0.1" {
		t.Fatalf("failing host=%q", failure.Host)
	}
	resolved := store.FailureDetails(fmt.Sprintf("DAV discovery failed: %v", err))
	if resolved == nil || resolved.Host != failure.Host || resolved.Info.Fingerprint != Fingerprint(srv.Certificate().Raw) {
		t.Fatalf("failure details=%+v", resolved)
	}
	if store.FailureDetails("untrusted certificate: fabricated remote response") != nil {
		t.Fatal("fabricated failure accepted")
	}
}

func TestFailureRegistryBounded(t *testing.T) {
	store := NewStore(nil)
	first := store.recordFailure(&Error{Info: &CertificateInfo{Fingerprint: "test"}}, "host")
	for i := 0; i < 64; i++ {
		_ = store.recordFailure(&Error{Info: &CertificateInfo{Fingerprint: "test"}}, "host")
	}
	if len(store.failures) != 64 || store.FailureDetails(first.Error()) != nil {
		t.Fatal("registry did not evict oldest failure")
	}
}

func TestListTrustedDAVHosts(t *testing.T) {
	store := openTestStore(t)
	for _, host := range []string{"imap.example.com", "dav.example.com"} {
		if err := store.AcceptPermanently(host, &CertificateInfo{Fingerprint: "same-cert", Subject: host}); err != nil {
			t.Fatal(err)
		}
	}
	pins, err := store.GetAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 2 {
		t.Fatalf("pins=%+v", pins)
	}
	for _, pin := range pins {
		if pin.Host == "" {
			t.Fatal("trusted pin omits host")
		}
	}
}

func TestRemoveForHostPreservesOtherPin(t *testing.T) {
	store := openTestStore(t)
	for _, host := range []string{"mail.example.org", "dav.example.org"} {
		if err := store.AcceptPermanently(host, &CertificateInfo{Fingerprint: "shared"}); err != nil {
			t.Fatal(err)
		}
		if err := store.AcceptSession(host, "shared"); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RemoveForHost("dav.example.org", "shared"); err != nil {
		t.Fatal(err)
	}
	if store.IsTrusted("dav.example.org", "shared") {
		t.Fatal("removed host still trusted")
	}
	if !store.IsTrusted("mail.example.org", "shared") {
		t.Fatal("other host lost trust")
	}
	pins, err := store.GetAll()
	if err != nil || len(pins) != 1 || pins[0].Host != "mail.example.org" {
		t.Fatalf("remaining pins=%+v,err=%v", pins, err)
	}
}

func TestFailurePublicMessage(t *testing.T) {
	store := NewStore(nil)
	failure := store.recordFailure(&Error{Info: &CertificateInfo{Fingerprint: "test"}, Reason: "unknown CA"}, "dav.example.org")
	err := fmt.Errorf("discovery failed: %w", failure)
	public := errorutil.Message(err)
	if strings.Contains(public, "certificate-check:") || !strings.HasPrefix(public, "discovery failed: ") {
		t.Fatalf("public message=%q", public)
	}
	if store.FailureDetails(err.Error()) == nil {
		t.Fatal("raw error no longer resolves")
	}
}
