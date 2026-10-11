package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/caseylabs/mailpong/internal/oauth2"
)

func TestOAuthRefreshFailedPromptsOnlyForInvalidGrant(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantReauth bool
	}{
		{"invalid_grant", fmt.Errorf("%w - Bad Request", oauth2.ErrInvalidGrant), true},
		{"not configured", fmt.Errorf("%w: google", oauth2.ErrNotConfigured), false},
		{"server error", errors.New("token refresh failed: server_error - "), false},
		{"network error", errors.New("request failed: dial tcp: connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var emitted []string
			orig := eventsEmit
			eventsEmit = func(_ context.Context, name string, _ ...interface{}) { emitted = append(emitted, name) }
			t.Cleanup(func() { eventsEmit = orig })

			err := (&composeOps{}).oauthRefreshFailed(context.Background(), "acc", "google", tt.err)
			if !errors.Is(err, tt.err) {
				t.Fatalf("error %v does not wrap %v", err, tt.err)
			}
			if got := len(emitted) == 1 && emitted[0] == "oauth:reauth-required"; got != tt.wantReauth {
				t.Errorf("emitted %v, want reauth %v", emitted, tt.wantReauth)
			}
			if got := strings.Contains(err.Error(), "re-authorization required"); got != tt.wantReauth {
				t.Errorf("message %q: re-authorization wording %v, want %v", err, got, tt.wantReauth)
			}
		})
	}
}
