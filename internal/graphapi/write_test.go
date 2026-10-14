package graphapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/caseylabs/mailven/internal/apiwrite"
	"github.com/caseylabs/mailven/internal/credentials"
)

func graphGrant(scopes ...string) *credentials.OAuthTokens {
	var q []string
	for _, s := range scopes {
		q = append(q, "https://graph.microsoft.com/"+s)
	}
	return &credentials.OAuthTokens{Provider: "microsoft", ScopeResource: "https://graph.microsoft.com", ScopesKnown: true, Scopes: q}
}

func graphWriter(t *testing.T, grant *credentials.OAuthTokens, h http.HandlerFunc) *Writer {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Writer{Tokens: staticToken{}, Grant: grant, Base: srv.URL}
}

func TestGraphWriteUpdate(t *testing.T) {
	var method, path, prefer string
	var body map[string]any
	w := graphWriter(t, graphGrant("Mail.ReadWrite"), func(rw http.ResponseWriter, r *http.Request) {
		method, path, prefer = r.Method, r.URL.EscapedPath(), r.Header.Get("Prefer")
		b, _ := io.ReadAll(r.Body)
		body = nil
		_ = json.Unmarshal(b, &body)
		_, _ = rw.Write([]byte(`{}`))
	})
	yes := true
	if err := w.Update(context.Background(), "a/b=", Patch{IsRead: &yes, Flagged: &yes}); err != nil {
		t.Fatal(err)
	}
	flag, _ := body["flag"].(map[string]any)
	if method != "PATCH" || path != "/me/messages/a%2Fb=" || body["isRead"] != true || flag["flagStatus"] != "flagged" ||
		!strings.Contains(prefer, `IdType="ImmutableId"`) {
		t.Fatalf("method=%s path=%s body=%v prefer=%s", method, path, body, prefer)
	}
	no := false
	if err := w.Update(context.Background(), "m1", Patch{Flagged: &no}); err != nil {
		t.Fatal(err)
	}
	if _, has := body["isRead"]; has || body["flag"].(map[string]any)["flagStatus"] != "notFlagged" {
		t.Fatalf("body = %v", body)
	}
}

func TestGraphWriteEmptyPatchSendsNothing(t *testing.T) {
	var hits atomic.Int32
	w := graphWriter(t, graphGrant("Mail.ReadWrite"), func(http.ResponseWriter, *http.Request) { hits.Add(1) })
	if err := w.Update(context.Background(), "m1", Patch{}); err != nil || hits.Load() != 0 {
		t.Fatalf("err=%v hits=%d", err, hits.Load())
	}
}

func TestGraphWriteMoveAndTrash(t *testing.T) {
	var dests []string
	w := graphWriter(t, graphGrant("Mail.ReadWrite"), func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me/messages/m1/move" || r.Method != "POST" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		var b map[string]string
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &b)
		dests = append(dests, b["destinationId"])
		_, _ = rw.Write([]byte(`{"id":"m1-new","parentFolderId":"f2"}`))
	})
	mv, err := w.Move(context.Background(), "m1", "f2")
	if err != nil || mv.ID != "m1-new" || mv.ParentFolderID != "f2" {
		t.Fatalf("Move = %+v, %v", mv, err)
	}
	if _, err := w.Trash(context.Background(), "m1"); err != nil {
		t.Fatal(err)
	}
	if dests[0] != "f2" || dests[1] != "deleteditems" {
		t.Fatalf("destinations = %v", dests)
	}
}

func TestGraphWriteMoveUnknownIsNotReplayed(t *testing.T) {
	var hits atomic.Int32
	w := graphWriter(t, graphGrant("Mail.ReadWrite"), func(rw http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		rw.WriteHeader(http.StatusBadGateway)
	})
	_, err := w.Move(context.Background(), "m1", "f2")
	if apiwrite.OutcomeOf(err) != apiwrite.Unknown || hits.Load() != 1 {
		t.Fatalf("outcome=%s hits=%d", apiwrite.OutcomeOf(err), hits.Load())
	}
	// A success without a readable identity is also unknown, not applied.
	w = graphWriter(t, graphGrant("Mail.ReadWrite"), func(rw http.ResponseWriter, _ *http.Request) { _, _ = rw.Write([]byte(`{}`)) })
	if _, err := w.Move(context.Background(), "m1", "f2"); apiwrite.OutcomeOf(err) != apiwrite.Unknown {
		t.Fatalf("outcome = %s", apiwrite.OutcomeOf(err))
	}
}

func TestGraphWriteGoneAndSharedPath(t *testing.T) {
	var path string
	w := graphWriter(t, graphGrant("Mail.ReadWrite.Shared"), func(rw http.ResponseWriter, r *http.Request) {
		path = r.URL.EscapedPath()
		rw.WriteHeader(404)
	})
	w.Shared = "team@contoso.com"
	err := w.Update(context.Background(), "m1", Patch{IsRead: new(bool)})
	if !errors.Is(err, ErrWriteGone) || apiwrite.OutcomeOf(err) != apiwrite.Rejected || path != "/users/team@contoso.com/messages/m1" {
		t.Fatalf("err=%v outcome=%s path=%s", err, apiwrite.OutcomeOf(err), path)
	}
}

func TestGraphWriteCapabilityGateSendsNothing(t *testing.T) {
	var hits atomic.Int32
	h := func(http.ResponseWriter, *http.Request) { hits.Add(1) }
	yes := true
	cases := map[string]struct {
		grant  *credentials.OAuthTokens
		shared string
	}{
		"nil":                      {nil, ""},
		"read only":                {graphGrant("Mail.Read"), ""},
		"send only":                {graphGrant("Mail.Send"), ""},
		"own grant for shared box": {graphGrant("Mail.ReadWrite"), "team@contoso.com"},
		"unknown":                  {&credentials.OAuthTokens{Provider: "microsoft"}, ""},
	}
	for name, c := range cases {
		w := graphWriter(t, c.grant, h)
		w.Shared = c.shared
		errs := []error{w.Update(context.Background(), "m1", Patch{IsRead: &yes})}
		_, e2 := w.Move(context.Background(), "m1", "f")
		_, e3 := w.Trash(context.Background(), "m1")
		for _, e := range append(errs, e2, e3) {
			if apiwrite.OutcomeOf(e) != apiwrite.NotSent {
				t.Errorf("%s: outcome = %s (%v)", name, apiwrite.OutcomeOf(e), e)
			}
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("%d requests sent despite a refused grant", hits.Load())
	}
}
