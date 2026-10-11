package gmailapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/hkdb/aerion/internal/account"
	"github.com/hkdb/aerion/internal/database"
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
		{"sendAsEmail":"new@x.com","verificationStatus":"pending"}]}`)
	got, err := ListSendAs(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d entries", len(got))
	}
	if got[1].Email != "sean@caseylabs.com" || got[1].ReplyTo != "r@x.com" || got[1].Signature != "<b>s</b>" || !got[1].Verified() {
		t.Errorf("entry 1 = %+v", got[1])
	}
	if !got[0].Verified() || got[2].Verified() {
		t.Error("Verified() wrong for primary or pending")
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
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	c, err = s.Get(acc.ID)
	if err != nil || len(c.Entries) != 3 || !c.FetchedAt.Equal(now) {
		t.Fatalf("cache: %+v %v", c, err)
	}
	if c.Entries[1].Signature != "<i>s</i>" {
		t.Errorf("signature not cached: %+v", c.Entries)
	}
	for email, want := range map[string]string{
		"me@gmail.com":       StatusVerified,
		"sean@caseylabs.com": StatusVerified,
		"WAIT@x.com":         StatusPending,
		"other@x.com":        StatusNotConfigured,
	} {
		if got := c.StatusFor(email); got != want {
			t.Errorf("StatusFor(%s) = %s, want %s", email, got, want)
		}
	}

	if err := s.Replace(acc.ID, []SendAs{{Email: "me@gmail.com", IsPrimary: true}}, now); err != nil {
		t.Fatal(err)
	}
	if c, _ = s.Get(acc.ID); len(c.Entries) != 1 {
		t.Errorf("Replace did not clear old rows: %+v", c.Entries)
	}
}
