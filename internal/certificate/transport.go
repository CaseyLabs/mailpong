package certificate

import (
	"errors"
	"net/http"
	"sync"
)

// Transport is an http.RoundTripper for clients that talk to many HTTPS hosts,
// such as the one transport shared by every CardDAV/CalDAV source (the auth
// broker also hands out account-level clients before the DAV host is known).
// It keeps one *http.Transport per request host, each built with
// BuildTLSConfig for that host, so trust is checked against the exact host the
// request was addressed to. Taking the host from the request rather than from
// the TLS connection state matters for IP-literal hosts: they send no SNI, so
// tls.ConnectionState.ServerName is empty and a pinned certificate could never
// match.
type Transport struct {
	store  *Store
	mu     sync.Mutex
	byHost map[string]*http.Transport
}

// NewTransport returns a host-scoped, trust-store-aware transport. Connections
// otherwise behave like http.DefaultTransport (proxy, timeouts, HTTP/2).
func NewTransport(store *Store) *Transport {
	return &Transport{store: store, byHost: make(map[string]*http.Transport)}
}

// RoundTrip sends req through the transport for req's host.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := normalizeHost(req.URL.Hostname())
	resp, err := t.forHost(host).RoundTrip(req)
	var certErr *Error
	if errors.As(err, &certErr) && t.store != nil {
		return resp, t.store.recordFailure(certErr, host)
	}
	return resp, err
}

func (t *Transport) forHost(host string) *http.Transport {
	t.mu.Lock()
	defer t.mu.Unlock()
	if tr, ok := t.byHost[host]; ok {
		return tr
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = BuildTLSConfig(host, t.store)
	t.byHost[host] = tr
	return tr
}

// CloseIdleConnections closes idle connections on every per-host transport.
func (t *Transport) CloseIdleConnections() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, tr := range t.byHost {
		tr.CloseIdleConnections()
	}
}
