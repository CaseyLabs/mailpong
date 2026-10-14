package gmailapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/caseylabs/mailven/internal/oauth2"
)

type fakeTokens struct{ refreshed atomic.Int32 }

func (f *fakeTokens) Token(context.Context) (string, error) { return "old", nil }
func (f *fakeTokens) Refresh(context.Context, string) (string, error) {
	f.refreshed.Add(1)
	return "new", nil
}

func TestGmailReaderRawIsByteIdentical(t *testing.T) {
	raw := []byte("From: a@example.com\r\nSubject: \xe9t\xe9\r\nContent-Type: text/plain; charset=iso-8859-1\r\n\r\ncaf\xe9\r\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages/m1" || r.URL.Query().Get("format") != "raw" {
			t.Errorf("unexpected request %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"id":"m1","threadId":"t1","labelIds":["INBOX"],"historyId":"9","raw":"` +
			base64.RawURLEncoding.EncodeToString(raw) + `"}`))
	}))
	defer srv.Close()
	m, err := (&Reader{Tokens: &fakeTokens{}, Base: srv.URL}).GetRaw(context.Background(), "m1")
	if err != nil || !bytes.Equal(m.Raw, raw) || m.ThreadID != "t1" || m.HistoryID != "9" {
		t.Fatalf("GetRaw = %+v, %v", m, err)
	}
}

func TestGmailReaderRefreshesOnce(t *testing.T) {
	for _, tc := range []struct {
		name     string
		second   int
		wantErr  bool
		wantAuth []string
	}{
		{"second request succeeds", 200, false, []string{"Bearer old", "Bearer new"}},
		{"still unauthorized stops after one refresh", 401, true, []string{"Bearer old", "Bearer new"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var seen []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = append(seen, r.Header.Get("Authorization"))
				if len(seen) == 1 {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.WriteHeader(tc.second)
				_, _ = w.Write([]byte(`{"historyId":"5"}`))
			}))
			defer srv.Close()
			tok := &fakeTokens{}
			_, err := (&Reader{Tokens: tok, Base: srv.URL}).Profile(context.Background())
			if (err != nil) != tc.wantErr || len(seen) != 2 || seen[1] != tc.wantAuth[1] || tok.refreshed.Load() != 1 {
				t.Fatalf("err=%v seen=%v refreshes=%d", err, seen, tok.refreshed.Load())
			}
		})
	}
}

func TestGmailReaderErrorClasses(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		path        func(*Reader) error
		rateLimited bool
		failure     oauth2.MailAPIFailure
		is          error
	}{
		{"history 404 is expired", 404, `{}`, func(r *Reader) error { _, e := r.History(context.Background(), "1", ""); return e }, false, "", ErrHistoryExpired},
		{"message 404 is gone", 404, `{}`, func(r *Reader) error { _, e := r.GetMinimal(context.Background(), "x"); return e }, false, "", ErrMessageGone},
		{"429 is a rate limit", 429, `{}`, func(r *Reader) error { _, e := r.Labels(context.Background()); return e }, true, "", nil},
		{"403 rate limit reason", 403, `{"error":{"errors":[{"reason":"userRateLimitExceeded"}]}}`, func(r *Reader) error { _, e := r.Labels(context.Background()); return e }, true, "", nil},
		{"403 policy", 403, `{"error":{"errors":[{"reason":"forbidden"}]}}`, func(r *Reader) error { _, e := r.Labels(context.Background()); return e }, false, oauth2.MailAPIFailurePolicy, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "7")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			err := tc.path(&Reader{Tokens: &fakeTokens{}, Base: srv.URL})
			if tc.is != nil {
				if !errors.Is(err, tc.is) {
					t.Fatalf("err = %v, want %v", err, tc.is)
				}
				return
			}
			var re *ReadError
			if !errors.As(err, &re) || re.RateLimited != tc.rateLimited || re.Failure != tc.failure {
				t.Fatalf("err = %#v", err)
			}
			if tc.rateLimited && re.RetryAfter.Seconds() != 7 {
				t.Fatalf("RetryAfter = %v", re.RetryAfter)
			}
		})
	}
}

func TestGmailReaderDoesNotFollowRedirects(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer srv.Close()
	if _, err := (&Reader{Tokens: &fakeTokens{}, Base: srv.URL}).Profile(context.Background()); err == nil || hits.Load() != 0 {
		t.Fatalf("redirect followed: err=%v hits=%d", err, hits.Load())
	}
}

func TestGmailReaderHistoryRequestsAllTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q["historyTypes"]; len(got) != 4 || q.Get("startHistoryId") != "10" {
			t.Errorf("query = %v", q)
		}
		_, _ = w.Write([]byte(`{"history":[{"id":"11","messagesAdded":[{"message":{"id":"a"}}],"labelsRemoved":[{"message":{"id":"b"}}]}],"historyId":"12"}`))
	}))
	defer srv.Close()
	p, err := (&Reader{Tokens: &fakeTokens{}, Base: srv.URL}).History(context.Background(), "10", "")
	if err != nil || p.HistoryID != "12" || len(p.Records) != 1 {
		t.Fatalf("History = %+v, %v", p, err)
	}
	added, _, relabelled := p.Records[0].IDs()
	if len(added) != 1 || added[0] != "a" || len(relabelled) != 1 || relabelled[0] != "b" {
		t.Fatalf("IDs = %v %v", added, relabelled)
	}
}
