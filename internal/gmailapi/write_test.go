package gmailapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/caseylabs/mailven/internal/apiwrite"
	"github.com/caseylabs/mailven/internal/credentials"
)

func gmailGrant(scope string) *credentials.OAuthTokens {
	return &credentials.OAuthTokens{Provider: "google", ScopeResource: "https://www.googleapis.com", ScopesKnown: true,
		Scopes: []string{"https://www.googleapis.com/auth/" + scope}}
}

func writerFor(t *testing.T, grant *credentials.OAuthTokens, h http.HandlerFunc) *Writer {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Writer{Tokens: &fakeTokens{}, Grant: grant, Base: srv.URL}
}

func TestGmailWriteModify(t *testing.T) {
	var gotPath string
	var gotBody map[string][]string
	w := writerFor(t, gmailGrant("gmail.modify"), func(rw http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer old" {
			t.Errorf("method=%s auth=%s", r.Method, r.Header.Get("Authorization"))
		}
		_, _ = rw.Write([]byte(`{"id":"m/1","threadId":"t","labelIds":["STARRED"]}`))
	})
	m, err := w.Modify(context.Background(), "m/1", []string{LabelStarred}, []string{LabelUnread})
	if err != nil || m.ID != "m/1" || len(m.LabelIDs) != 1 {
		t.Fatalf("Modify = %+v, %v", m, err)
	}
	if gotPath != "/messages/m%2F1/modify" || gotBody["addLabelIds"][0] != "STARRED" || gotBody["removeLabelIds"][0] != "UNREAD" {
		t.Fatalf("path=%s body=%v", gotPath, gotBody)
	}
}

func TestGmailWriteTrashUntrash(t *testing.T) {
	var paths []string
	w := writerFor(t, gmailGrant("gmail.modify"), func(rw http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = rw.Write([]byte(`{"id":"m1","labelIds":["TRASH"]}`))
	})
	if _, err := w.Trash(context.Background(), "m1"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Untrash(context.Background(), "m1"); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "/messages/m1/trash" || paths[1] != "/messages/m1/untrash" {
		t.Fatalf("paths = %v", paths)
	}
}

func TestGmailWriteBatchModifyChunks(t *testing.T) {
	var sizes []int
	var fail atomic.Bool
	w := writerFor(t, gmailGrant("gmail.modify"), func(rw http.ResponseWriter, r *http.Request) {
		var body struct {
			IDs []string `json:"ids"`
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		sizes = append(sizes, len(body.IDs))
		if len(sizes) == 2 && fail.Load() {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		rw.WriteHeader(http.StatusNoContent)
	})
	ids := make([]string, 2500)
	for i := range ids {
		ids[i] = "m" + strconv.Itoa(i)
	}
	n, err := w.BatchModify(context.Background(), ids, []string{LabelStarred}, nil)
	if err != nil || n != 2500 || len(sizes) != 3 || sizes[0] != 1000 || sizes[1] != 1000 || sizes[2] != 500 {
		t.Fatalf("n=%d sizes=%v err=%v", n, sizes, err)
	}
	sizes = nil
	fail.Store(true)
	n, err = w.BatchModify(context.Background(), ids, []string{LabelStarred}, nil)
	if n != 1000 || apiwrite.OutcomeOf(err) != apiwrite.Unknown || len(sizes) != 2 {
		t.Fatalf("n=%d sizes=%v outcome=%s", n, sizes, apiwrite.OutcomeOf(err))
	}
}

func TestGmailWriteCapabilityGateSendsNothing(t *testing.T) {
	var hits atomic.Int32
	h := func(rw http.ResponseWriter, _ *http.Request) { hits.Add(1) }
	for name, grant := range map[string]*credentials.OAuthTokens{
		"nil grant":    nil,
		"read only":    gmailGrant("gmail.readonly"),
		"send only":    gmailGrant("gmail.send"),
		"unknown":      {Provider: "google"},
		"wrong source": {Provider: "microsoft", ScopesKnown: true},
	} {
		w := writerFor(t, grant, h)
		_, err := w.Modify(context.Background(), "m1", nil, nil)
		_, err2 := w.Trash(context.Background(), "m1")
		_, err3 := w.BatchModify(context.Background(), []string{"m1"}, nil, nil)
		for _, e := range []error{err, err2, err3} {
			if apiwrite.OutcomeOf(e) != apiwrite.NotSent {
				t.Errorf("%s: outcome = %s (%v)", name, apiwrite.OutcomeOf(e), e)
			}
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("%d requests sent despite a refused grant", hits.Load())
	}
}

func TestGmailWriteGoneIsRejected(t *testing.T) {
	w := writerFor(t, gmailGrant("gmail.modify"), func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(404) })
	_, err := w.Modify(context.Background(), "m1", nil, []string{LabelInbox})
	if !errors.Is(err, ErrWriteGone) || apiwrite.OutcomeOf(err) != apiwrite.Rejected {
		t.Fatalf("err=%v outcome=%s", err, apiwrite.OutcomeOf(err))
	}
}

func TestGmailWriteRefreshesOnce(t *testing.T) {
	var seen []string
	tk := &fakeTokens{}
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		if len(seen) == 1 {
			rw.WriteHeader(401)
			return
		}
		_, _ = rw.Write([]byte(`{"id":"m1"}`))
	}))
	defer srv.Close()
	w := &Writer{Tokens: tk, Grant: gmailGrant("gmail.modify"), Base: srv.URL}
	if _, err := w.Modify(context.Background(), "m1", nil, nil); err != nil || len(seen) != 2 || seen[1] != "Bearer new" {
		t.Fatalf("err=%v seen=%v", err, seen)
	}
}

func TestGmailWriteNeverOffersPermanentDelete(t *testing.T) {
	var paths []string
	w := writerFor(t, gmailGrant("gmail.modify"), func(rw http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		_, _ = rw.Write([]byte(`{"id":"m1"}`))
	})
	_, _ = w.Modify(context.Background(), "m1", nil, nil)
	_, _ = w.Trash(context.Background(), "m1")
	for _, p := range paths {
		if p[:6] == "DELETE" {
			t.Fatalf("unexpected %s", p)
		}
	}
}
