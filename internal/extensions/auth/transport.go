package auth

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	coreapi "github.com/caseylabs/mailpong/internal/core/api/v1"
	"github.com/caseylabs/mailpong/internal/credentials"
	"github.com/caseylabs/mailpong/internal/oauth2"
)

// maxBufferedBody caps how much of a request body without GetBody the
// transport buffers so it can resend it after a refresh. A var so tests can
// lower it.
var maxBufferedBody int64 = 10 << 20

// bearerRefreshTransport is an http.RoundTripper that injects the current
// access token on each request and transparently refreshes it on 401
// responses. Refreshes take the credential store's per-(accountID,
// clientConfigID) lock, so N concurrent requests with the same expired token —
// or a request racing the mail IMAP/SMTP refresh — cause exactly one refresh.
type bearerRefreshTransport struct {
	base           http.RoundTripper
	credStore      *credentials.Store
	oauthManager   *oauth2.Manager
	accountID      string
	clientConfigID string
}

// RoundTrip implements http.RoundTripper.
func (t *bearerRefreshTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req, err := bufferBody(req)
	if err != nil {
		return nil, err
	}

	tokens, err := t.credStore.GetOAuthTokensForClientConfig(t.accountID, t.clientConfigID)
	if err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, fmt.Errorf("auth broker: read tokens: %w", err)
	}
	sent := tokens.AccessToken

	resp, err := t.do(req, sent)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	// 401: buffer + close body; it's handed back if the request can't be resent.
	// ContentLength must match what was kept, which may be truncated.
	body401, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body401))
	resp.ContentLength = int64(len(body401))
	resp.Header.Del("Content-Length")

	accessToken, err := t.freshToken(sent)
	if err != nil {
		return nil, err
	}

	// Rewind only now: a fresh body taken earlier would leak on the error paths above.
	retry, canRetry, err := rewind(req)
	if err != nil {
		return nil, err
	}
	if !canRetry {
		// Only bodies over maxBufferedBody get here. The token is fresh now,
		// so the caller's own retry will succeed.
		return resp, nil
	}
	return t.do(retry, accessToken)
}

// freshToken returns an access token to retry with after sent got a 401. It
// refreshes under the slot lock unless another request already replaced sent.
func (t *bearerRefreshTransport) freshToken(sent string) (string, error) {
	defer t.credStore.LockOAuthRefresh(t.accountID, t.clientConfigID)()

	tokens, err := t.credStore.GetOAuthTokensForClientConfig(t.accountID, t.clientConfigID)
	if err != nil {
		return "", fmt.Errorf("auth broker: re-read tokens before refresh: %w", err)
	}
	if tokens.AccessToken != sent && tokens.AccessToken != "" {
		return tokens.AccessToken, nil
	}

	provider, err := t.resolveProvider()
	if err != nil {
		return "", err
	}

	refreshed, err := t.oauthManager.RefreshTokenWithProvider(provider, tokens.RefreshToken)
	if errors.Is(err, oauth2.ErrInvalidGrant) && canRegrant(t.clientConfigID) {
		// The refresh token is dead (expired/revoked). Surface it as a consent
		// requirement so extensions offer their grant flow instead of a raw
		// error the user has no way to act on. Mail slots keep the refresh
		// error: only mail re-auth can repair them.
		return "", &coreapi.ErrAdditionalConsentRequired{
			AccountID:      t.accountID,
			ClientConfigID: coreapi.ClientConfigID(t.clientConfigID),
			Reason:         "refresh token expired or revoked",
		}
	}
	if err != nil {
		return "", fmt.Errorf("auth broker: refresh: %w", err)
	}

	expiresAt := refreshed.ExpiresAt()
	// Persist the rotated refresh token too — providers like Microsoft and custom
	// OIDC return a NEW refresh_token on refresh; dropping it breaks the next one.
	if err := t.credStore.UpdateOAuthTokensForClientConfig(t.accountID, t.clientConfigID, refreshed.AccessToken, refreshed.RefreshToken, expiresAt); err != nil {
		return "", fmt.Errorf("auth broker: persist refreshed tokens: %w", err)
	}
	return refreshed.AccessToken, nil
}

// bufferBody reads a body that has no GetBody into memory and sets GetBody,
// so the request can be resent after a refresh. Bodies over maxBufferedBody
// are passed through unbuffered and stay unreplayable.
func bufferBody(req *http.Request) (*http.Request, error) {
	if req.Body == nil || req.Body == http.NoBody || req.GetBody != nil || req.ContentLength > maxBufferedBody {
		return req, nil
	}
	buf, err := io.ReadAll(io.LimitReader(req.Body, maxBufferedBody+1))
	if err != nil {
		_ = req.Body.Close()
		return nil, fmt.Errorf("auth broker: buffer request body: %w", err)
	}
	r := *req // shallow copy is enough: do() clones before mutating
	if int64(len(buf)) > maxBufferedBody {
		r.Body = readCloser{io.MultiReader(bytes.NewReader(buf), req.Body), req.Body}
		return &r, nil
	}
	_ = req.Body.Close()
	r.ContentLength = int64(len(buf)) // known now; avoids a chunked upload
	r.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(buf)), nil }
	r.Body, _ = r.GetBody()
	return &r, nil
}

type readCloser struct {
	io.Reader
	io.Closer
}

// rewind returns req with a fresh body for resending after a refresh. The
// first attempt consumed the body, so ok is false when it can't be replayed
// (no GetBody) — resending would send it empty.
func rewind(req *http.Request) (retry *http.Request, ok bool, err error) {
	if req.Body == nil || req.Body == http.NoBody {
		return req, true, nil
	}
	if req.GetBody == nil {
		return nil, false, nil
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, false, fmt.Errorf("auth broker: rewind request body: %w", err)
	}
	r := *req // shallow copy is enough: do() clones before mutating
	r.Body = body
	return &r, true, nil
}

// canRegrant reports whether StartIncrementalConsent can write fresh tokens
// into the given slot.
func canRegrant(clientConfigID string) bool {
	_, err := oauth2.IncrementalConsentProvider(clientConfigID)
	return err == nil
}

// resolveProvider returns the OAuth2 provider config for refreshing this account's
// token. Custom ("bring your own app") accounts store their endpoints/creds per account
// (no static client-config registration), so they resolve from GetCustomOAuthProvider;
// shipped providers use the static client-config lookup.
func (t *bearerRefreshTransport) resolveProvider() (oauth2.ProviderConfig, error) {
	if t.clientConfigID == "custom-mail" {
		cfg, ok, err := t.credStore.GetCustomOAuthProvider(t.accountID)
		if err != nil {
			return oauth2.ProviderConfig{}, fmt.Errorf("auth broker: get custom provider for %s: %w", t.accountID, err)
		}
		if !ok {
			return oauth2.ProviderConfig{}, fmt.Errorf("auth broker: no custom provider configured for account %s", t.accountID)
		}
		return oauth2.CustomProviderConfig(cfg.AuthURL, cfg.TokenURL, cfg.UserinfoEndpoint, cfg.Scopes, cfg.ClientID, cfg.ClientSecret), nil
	}

	provider, err := oauth2.GetProviderForClientConfig(t.clientConfigID)
	if err != nil {
		return oauth2.ProviderConfig{}, fmt.Errorf("auth broker: resolve provider for %s: %w", t.clientConfigID, err)
	}
	return provider, nil
}

// do clones the request, sets the bearer header, and dispatches via the base
// transport. Cloning is necessary because RoundTrip implementations must not
// mutate the input request.
func (t *bearerRefreshTransport) do(req *http.Request, accessToken string) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	if cloned.Header == nil {
		cloned.Header = make(http.Header)
	}
	// Don't double-set if the caller already added one (rare; just in case).
	if strings.TrimSpace(cloned.Header.Get("Authorization")) == "" {
		cloned.Header.Set("Authorization", "Bearer "+accessToken)
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(cloned)
}
