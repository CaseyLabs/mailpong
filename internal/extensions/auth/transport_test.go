package auth

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	coreapi "github.com/caseylabs/mailven/internal/core/api/v1"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/oauth2"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(r *http.Request, status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(http.StatusText(status))), Request: r}
}

// newCustomAccountClient returns an extension client for a custom ("bring
// your own app") account holding a stale access token. api stands in for the
// network (it does no body rewinding of its own, unlike http.Transport) and
// token serves the account's token endpoint.
func newCustomAccountClient(t *testing.T, api roundTripFunc, token http.HandlerFunc) *http.Client {
	t.Helper()
	broker, credStore, db := newTestBroker(t)
	broker.baseTransport = api
	insertTestAccount(t, db, "acct")

	tokenSrv := httptest.NewServer(token)
	t.Cleanup(tokenSrv.Close)

	if err := credStore.SetOAuthTokens("acct", &credentials.OAuthTokens{
		Provider:     "custom",
		AccessToken:  "stale-token",
		RefreshToken: "refresh-token",
	}); err != nil {
		t.Fatalf("set tokens: %v", err)
	}
	if err := credStore.SetCustomOAuthProvider("acct", credentials.CustomOAuthProvider{
		AuthURL:  tokenSrv.URL + "/auth",
		TokenURL: tokenSrv.URL,
		ClientID: "client-id",
	}); err != nil {
		t.Fatalf("set custom provider: %v", err)
	}

	scopes := []coreapi.AuthScope{{Resource: "https://example.com/calendar"}}
	client, err := broker.HTTPClientForExtension("calendar", coreapi.Manifest{}, "acct", scopes)
	if err != nil {
		t.Fatalf("HTTPClientForExtension: %v", err)
	}
	return client
}

func tokenResponse(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// newRefreshingClient's API rejects every token but "fresh-token", which the
// token endpoint issues. It records each request body it receives.
func newRefreshingClient(t *testing.T) (*http.Client, *[]string) {
	var bodies []string
	api := func(r *http.Request) (*http.Response, error) {
		var b []byte
		if r.Body != nil {
			b, _ = io.ReadAll(r.Body)
		}
		bodies = append(bodies, string(b))
		if r.Header.Get("Authorization") != "Bearer fresh-token" {
			return respond(r, http.StatusUnauthorized), nil
		}
		return respond(r, http.StatusOK), nil
	}
	token := tokenResponse(http.StatusOK, `{"access_token":"fresh-token","expires_in":3600}`)
	return newCustomAccountClient(t, api, token), &bodies
}

// A refresh token the provider rejects (invalid_grant: expired, revoked) on a
// custom ("bring your own app") account lives in the custom-mail slot, which no
// extension grant flow can re-grant. The transport must keep the refresh error
// (so mail re-auth handles it) instead of offering a "Grant access" button that
// would loop.
func TestTransport_InvalidGrantOnMailSlotKeepsRefreshError(t *testing.T) {
	api := func(r *http.Request) (*http.Response, error) { return respond(r, http.StatusUnauthorized), nil }
	token := tokenResponse(http.StatusBadRequest, `{"error":"invalid_grant","error_description":"Bad Request"}`)
	client := newCustomAccountClient(t, api, token)

	if _, err := client.Get("https://api.test/"); !errors.Is(err, oauth2.ErrInvalidGrant) {
		t.Fatalf("expected ErrInvalidGrant, got %T: %v", err, err)
	}
}

// The retry after a refresh resends the body, whether or not the caller's
// request has GetBody (bodies without one are buffered).
func TestTransport_RetryAfterRefreshResendsBody(t *testing.T) {
	for name, body := range map[string]io.Reader{
		"with GetBody":    strings.NewReader("BEGIN:VCALENDAR"),
		"without GetBody": io.NopCloser(strings.NewReader("BEGIN:VCALENDAR")),
	} {
		t.Run(name, func(t *testing.T) {
			client, bodies := newRefreshingClient(t)

			req, err := http.NewRequest(http.MethodPut, "https://api.test/", body)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("PUT: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			if got := *bodies; len(got) != 2 || got[1] != "BEGIN:VCALENDAR" {
				t.Fatalf("API bodies = %q, want the retry to resend the body", got)
			}
		})
	}
}

// lowerBodyBuffer makes bodies over n bytes unreplayable for one test.
func lowerBodyBuffer(t *testing.T, n int64) {
	old := maxBufferedBody
	maxBufferedBody = n
	t.Cleanup(func() { maxBufferedBody = old })
}

// unreplayablePut returns a PUT whose body has no GetBody.
func unreplayablePut() *http.Request {
	req, _ := http.NewRequest(http.MethodPut, "https://api.test/", io.NopCloser(strings.NewReader("BEGIN:VCALENDAR")))
	return req
}

// A body too large to buffer can't be resent, so the caller gets the 401
// (body intact) instead of an empty-body retry — but the token is still
// refreshed, so the caller's own retry succeeds.
func TestTransport_OversizedBodyReturns401AfterRefresh(t *testing.T) {
	lowerBodyBuffer(t, 4)
	client, bodies := newRefreshingClient(t)

	resp, err := client.Do(unreplayablePut())
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || string(body) != "Unauthorized" {
		t.Fatalf("got %d %q, want the 401 with its body", resp.StatusCode, body)
	}
	if got := *bodies; len(got) != 1 || got[0] != "BEGIN:VCALENDAR" {
		t.Fatalf("API bodies = %q, want one request with the whole body", got)
	}

	resp, err = client.Get("https://api.test/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(*bodies) != 2 {
		t.Fatalf("follow-up: status %d after %d API requests, want 200 on the first try", resp.StatusCode, len(*bodies))
	}
}

// Concurrent requests that all got a 401 for the same stale token refresh
// once; the rest retry with the token that refresh stored.
func TestTransport_ConcurrentUnauthorizedRefreshOnce(t *testing.T) {
	const n = 5
	var stale sync.WaitGroup
	stale.Add(n)
	api := func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") == "Bearer fresh-token" {
			return respond(r, http.StatusOK), nil
		}
		// Hold every 401 until all requests have sent the stale token.
		stale.Done()
		stale.Wait()
		return respond(r, http.StatusUnauthorized), nil
	}
	var refreshes atomic.Int32
	token := func(w http.ResponseWriter, r *http.Request) {
		refreshes.Add(1)
		tokenResponse(http.StatusOK, `{"access_token":"fresh-token","expires_in":3600}`)(w, r)
	}
	client := newCustomAccountClient(t, api, token)

	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			resp, err := client.Get("https://api.test/")
			if err != nil {
				t.Error(err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status %d, want 200", resp.StatusCode)
			}
		})
	}
	wg.Wait()
	if got := refreshes.Load(); got != 1 {
		t.Fatalf("token endpoint hit %d times, want 1", got)
	}
}

// A mail-slot refresh by IMAP/SMTP auth holds the credential store's slot
// lock. A 401 arriving meanwhile must wait for it and use the rotated token
// rather than spend the refresh token a second time.
func TestTransport_WaitsForMailPathRefresh(t *testing.T) {
	broker, credStore, db := newTestBroker(t)
	insertTestAccount(t, db, "acct")
	sawStale := make(chan struct{}, 1)
	broker.baseTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") == "Bearer rotated-token" {
			return respond(r, http.StatusOK), nil
		}
		sawStale <- struct{}{}
		return respond(r, http.StatusUnauthorized), nil
	})
	var refreshes atomic.Int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshes.Add(1)
		tokenResponse(http.StatusOK, `{"access_token":"fresh-token","expires_in":3600}`)(w, r)
	}))
	t.Cleanup(tokenSrv.Close)
	if err := credStore.SetOAuthTokens("acct", &credentials.OAuthTokens{
		Provider: "custom", AccessToken: "stale-token", RefreshToken: "refresh-token",
	}); err != nil {
		t.Fatalf("set tokens: %v", err)
	}
	if err := credStore.SetCustomOAuthProvider("acct", credentials.CustomOAuthProvider{
		AuthURL: tokenSrv.URL + "/auth", TokenURL: tokenSrv.URL, ClientID: "client-id",
	}); err != nil {
		t.Fatalf("set custom provider: %v", err)
	}
	client, err := broker.HTTPClientForExtension("calendar", coreapi.Manifest{}, "acct",
		[]coreapi.AuthScope{{Resource: "https://example.com/calendar"}})
	if err != nil {
		t.Fatalf("HTTPClientForExtension: %v", err)
	}

	unlock := credStore.LockOAuthRefresh("acct", "custom-mail")
	result := make(chan int, 1)
	go func() {
		resp, err := client.Get("https://api.test/")
		if err != nil {
			t.Error(err)
			result <- 0
			return
		}
		resp.Body.Close()
		result <- resp.StatusCode
	}()
	<-sawStale
	time.Sleep(50 * time.Millisecond) // let the transport reach the lock
	if err := credStore.UpdateOAuthTokensForClientConfig("acct", "custom-mail", "rotated-token", "rotated-refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("rotate tokens: %v", err)
	}
	unlock()

	if got := <-result; got != http.StatusOK {
		t.Fatalf("status %d, want 200", got)
	}
	if got := refreshes.Load(); got != 0 {
		t.Fatalf("token endpoint hit %d times, want 0", got)
	}
}

// A truncated 401 body must report its real length, not the server's.
func TestTransport_HandedBack401LengthMatchesBody(t *testing.T) {
	big := strings.Repeat("x", 100_000)
	api := func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, ContentLength: int64(len(big)),
			Header: http.Header{"Content-Length": {"100000"}}, Body: io.NopCloser(strings.NewReader(big)), Request: r}, nil
	}
	lowerBodyBuffer(t, 4)
	client := newCustomAccountClient(t, api, tokenResponse(http.StatusOK, `{"access_token":"fresh-token","expires_in":3600}`))

	resp, err := client.Do(unreplayablePut())
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.ContentLength != int64(len(body)) || resp.Header.Get("Content-Length") != "" {
		t.Fatalf("ContentLength=%d header=%q, read %d bytes (err %v)", resp.ContentLength, resp.Header.Get("Content-Length"), len(body), err)
	}
}

// When the refresh fails there's no retry, so no fresh body may be opened.
func TestTransport_FailedRefreshDoesNotOpenRetryBody(t *testing.T) {
	api := func(r *http.Request) (*http.Response, error) { return respond(r, http.StatusUnauthorized), nil }
	token := tokenResponse(http.StatusBadRequest, `{"error":"invalid_grant","error_description":"Bad Request"}`)
	client := newCustomAccountClient(t, api, token)

	req, _ := http.NewRequest(http.MethodPost, "https://api.test/", strings.NewReader("BEGIN:VCALENDAR"))
	opened := 0
	getBody := req.GetBody
	req.GetBody = func() (io.ReadCloser, error) { opened++; return getBody() }

	if _, err := client.Do(req); !errors.Is(err, oauth2.ErrInvalidGrant) {
		t.Fatalf("expected ErrInvalidGrant, got %v", err)
	}
	if opened != 0 {
		t.Fatalf("GetBody called %d times on a failed refresh, want 0", opened)
	}
}
