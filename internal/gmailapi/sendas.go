// Package gmailapi is a minimal client for the Gmail REST API settings
// endpoints Aerion needs (currently only the send-as list).
package gmailapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// sendAsURL is a variable so tests can point it at an httptest server.
var sendAsURL = "https://gmail.googleapis.com/gmail/v1/users/me/settings/sendAs"

// ErrInsufficientScope means Gmail refused the send-as list (HTTP 403 other than a
// rate or quota limit): the token lacks the scope, or the Gmail API is disabled for the OAuth client. Re-authorizing is the user's decision, never automatic.
var ErrInsufficientScope = errors.New("gmail: token lacks permission to read send-as addresses")

// SendAs is one "Send mail as" address of a Gmail account.
type SendAs struct {
	Email              string `json:"sendAsEmail"`
	DisplayName        string `json:"displayName"`
	ReplyTo            string `json:"replyToAddress"`
	IsPrimary          bool   `json:"isPrimary"`
	VerificationStatus string `json:"verificationStatus"`
	Signature          string `json:"signature"`
}

// Verified reports whether Gmail will send mail from this address as-is.
// The primary address has no verification status.
func (s SendAs) Verified() bool {
	return s.IsPrimary || s.VerificationStatus == "" || s.VerificationStatus == "accepted"
}

// ListSendAs returns the account's send-as addresses.
func ListSendAs(ctx context.Context, token string) ([]SendAs, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sendAsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build send-as request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list send-as: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read send-as response: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusForbidden && !isRateLimit(body):
		return nil, ErrInsufficientScope
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("list send-as: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body[:min(len(body), 200)])))
	}

	var out struct {
		SendAs []SendAs `json:"sendAs"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode send-as response: %w", err)
	}
	return out.SendAs, nil
}

// isRateLimit reports whether a Gmail error body is a quota or rate-limit
// error, which Gmail also sends as HTTP 403.
func isRateLimit(body []byte) bool {
	var e struct {
		Error struct {
			Errors []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) != nil {
		return false
	}
	for _, r := range e.Error.Errors {
		switch r.Reason {
		case "rateLimitExceeded", "userRateLimitExceeded", "dailyLimitExceeded", "quotaExceeded":
			return true
		}
	}
	return false
}
