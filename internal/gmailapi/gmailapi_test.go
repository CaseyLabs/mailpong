package gmailapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/kit/googleerr"
)

func serve(t *testing.T, status int, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	old := sendAsURL
	sendAsURL = srv.URL
	t.Cleanup(func() { sendAsURL = old; srv.Close() })
}

func TestListSendAs(t *testing.T) {
	serve(t, 200, `{"sendAs":[
		{"sendAsEmail":"me@gmail.com","displayName":"Me","isPrimary":true,"isDefault":true},
		{"sendAsEmail":"sean@caseylabs.com","displayName":"Sean","replyToAddress":"r@x.com","verificationStatus":"accepted","signature":"<b>s</b>"},
		{"sendAsEmail":"new@x.com","verificationStatus":"pending"},
		{"sendAsEmail":"legacy@x.com"},
		{"sendAsEmail":"future@x.com","verificationStatus":"future-status"}]}`)
	got, err := ListSendAs(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d entries", len(got))
	}
	if got[1].Email != "sean@caseylabs.com" || got[1].ReplyTo != "r@x.com" || got[1].Signature != "<b>s</b>" || !got[1].Verified() {
		t.Errorf("entry 1 = %+v", got[1])
	}
	if !got[0].Verified() || !got[1].Verified() || got[2].Verified() || got[3].Verified() || got[4].Verified() {
		t.Error("Verified() must accept primary and explicit accepted only")
	}
}

func TestListSendAsErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		scope  bool
	}{
		{"forbidden", 403, `{"error":"insufficient"}`, true},
		{"forbidden scope", 403, `{"error":{"code":403,"errors":[{"reason":"insufficientPermissions"}]}}`, true},
		{"rate limited", 403, `{"error":{"code":403,"errors":[{"reason":"userRateLimitExceeded"}]}}`, false},
		{"server error", 500, `oops`, false},
		{"bad json", 200, `{`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serve(t, tt.status, tt.body)
			_, err := ListSendAs(context.Background(), "tok")
			if err == nil {
				t.Fatal("want error")
			}
			if errors.Is(err, ErrInsufficientScope) != tt.scope {
				t.Errorf("scope error = %v, want %v (%v)", !tt.scope, tt.scope, err)
			}
		})
	}
}

func TestListSendAsServiceDisabled(t *testing.T) {
	serve(t, 403, `{"error":{"code":403,"message":"Gmail API has not been used in project 77 before or it is disabled. Visit https://evil.example","status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"SERVICE_DISABLED","metadata":{"service":"gmail.googleapis.com","consumer":"projects/77","activationUrl":"https://evil.example"}}]}}`)
	_, err := ListSendAs(context.Background(), "tok")
	var disabled *googleerr.DisabledError
	if !errors.As(err, &disabled) || errors.Is(err, ErrInsufficientScope) {
		t.Fatalf("err = %v, want DisabledError and not ErrInsufficientScope", err)
	}
	if disabled.Service != "gmail.googleapis.com" || disabled.Project != "77" ||
		disabled.EnableURL() != "https://console.developers.google.com/apis/api/gmail.googleapis.com/overview?project=77" ||
		strings.Contains(err.Error(), "evil.example") {
		t.Errorf("err = %v", err)
	}
}

func TestStoreAndStatus(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	acc, err := account.NewStore(db).Create(&account.AccountConfig{
		Name: "G", DisplayName: "G", Email: "me@gmail.com", IMAPHost: "imap.gmail.com", IMAPPort: 993,
		IMAPSecurity: account.SecurityTLS, SMTPHost: "smtp.gmail.com", SMTPPort: 587,
		SMTPSecurity: account.SecurityStartTLS, AuthType: account.AuthOAuth2, Username: "me@gmail.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(db.DB)

	c, err := s.Get(acc.ID)
	if err != nil || !c.FetchedAt.IsZero() || c.StatusFor("me@gmail.com") != StatusUnknown {
		t.Fatalf("empty cache: %+v %v", c, err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	err = s.Replace(acc.ID, []SendAs{
		{Email: "me@gmail.com", IsPrimary: true},
		{Email: "Sean@CaseyLabs.com", VerificationStatus: "accepted", Signature: "<i>s</i>"},
		{Email: "wait@x.com", VerificationStatus: "pending"},
		{Email: "legacy@x.com"},
		{Email: "future@x.com", VerificationStatus: "future-status"},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	c, err = s.Get(acc.ID)
	if err != nil || len(c.Entries) != 5 || !c.FetchedAt.Equal(now) {
		t.Fatalf("cache: %+v %v", c, err)
	}
	if got := c.StatusFor("sean@caseylabs.com"); got != StatusVerified {
		t.Errorf("accepted entry status = %s, want %s", got, StatusVerified)
	}
	var accepted *SendAs
	for i := range c.Entries {
		if c.Entries[i].Email == "sean@caseylabs.com" {
			accepted = &c.Entries[i]
			break
		}
	}
	if accepted == nil || accepted.Signature != "<i>s</i>" {
		t.Errorf("signature not cached: %+v", c.Entries)
	}
	for email, want := range map[string]string{
		"me@gmail.com":       StatusVerified,
		"sean@caseylabs.com": StatusVerified,
		"WAIT@x.com":         StatusPending,
		"legacy@x.com":       StatusUnknown,
		"future@x.com":       StatusUnknown,
		"other@x.com":        StatusNotConfigured,
	} {
		if got := c.StatusFor(email); got != want {
			t.Errorf("StatusFor(%s) = %s, want %s", email, got, want)
		}
	}

	attemptedAt := now.Add(time.Minute)
	if err := s.MarkLookupFailed(acc.ID, attemptedAt); err != nil {
		t.Fatal(err)
	}
	c, err = s.Get(acc.ID)
	if err != nil || len(c.Entries) != 5 || !c.FetchedAt.Equal(now) || !c.LookupFailed {
		t.Fatalf("failed refresh must preserve successful cache and report failure: %+v %v", c, err)
	}

	if err := s.Replace(acc.ID, []SendAs{{Email: "me@gmail.com", IsPrimary: true}}, now); err != nil {
		t.Fatal(err)
	}
	if c, err = s.Get(acc.ID); err != nil || len(c.Entries) != 1 || c.LookupFailed {
		t.Errorf("Replace did not clear old rows: %+v", c.Entries)
	}

	emptyFetchedAt := now.Add(time.Minute)
	if err := s.Replace(acc.ID, nil, emptyFetchedAt); err != nil {
		t.Fatal(err)
	}
	c, err = s.Get(acc.ID)
	if err != nil || len(c.Entries) != 0 || !c.FetchedAt.Equal(emptyFetchedAt) {
		t.Fatalf("empty cache must retain successful fetch time: %+v %v", c, err)
	}
	if got := c.StatusFor("me@gmail.com"); got != StatusNotConfigured {
		t.Errorf("StatusFor after successful empty lookup = %s, want %s", got, StatusNotConfigured)
	}
	if err := s.MarkLookupFailed(acc.ID, emptyFetchedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	c, err = s.Get(acc.ID)
	if err != nil || len(c.Entries) != 0 || !c.FetchedAt.Equal(emptyFetchedAt) || !c.LookupFailed {
		t.Fatalf("failed refresh must preserve successful empty observation: %+v %v", c, err)
	}
}
