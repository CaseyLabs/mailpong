package oauth2

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestTokenResponseScopeAndContext(t *testing.T) {
	for _, scope := range []string{`"https://mail.google.com/"`, `""`, `null`} {
		t.Run(scope, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token":"fixture","expires_in":3600,"scope":` + scope + `}`))
			}))
			defer server.Close()
			config := GoogleProvider()
			config.ClientID = "fixture-client"
			config.TokenURL = server.URL
			for _, refresh := range []bool{false, true} {
				manager := NewManager()
				var tokens *TokenResponse
				var err error
				if refresh {
					tokens, err = manager.RefreshTokenWithProvider(config, "prior-refresh")
				} else {
					tokens, err = manager.exchangeCode(config, "code", "verifier", 1234)
				}
				if err != nil {
					t.Fatal(err)
				}
				if tokens.ScopePresent != (scope != "null") || tokens.Provider != "google" || tokens.ScopeResource != "https://www.googleapis.com" || !reflect.DeepEqual(tokens.RequestedScopes, config.Scopes) {
					t.Fatalf("response metadata lost: %+v", tokens)
				}
				if refresh && tokens.RefreshToken != "prior-refresh" {
					t.Fatal("refresh retention lost")
				}
			}
		})
	}
}
