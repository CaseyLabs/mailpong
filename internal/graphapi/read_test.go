package graphapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caseylabs/mailven/internal/oauth2"
)

func readerFor(t *testing.T, h http.HandlerFunc) *Reader {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Reader{Tokens: staticToken{}, Base: srv.URL}
}

func TestReaderSendsImmutableIDAndMailboxPath(t *testing.T) {
	var paths, prefer []string
	r := readerFor(t, func(w http.ResponseWriter, req *http.Request) {
		paths = append(paths, req.URL.EscapedPath())
		prefer = append(prefer, req.Header.Get("Prefer"))
		_, _ = w.Write([]byte(`{"id":"m1","parentFolderId":"p"}`))
	})
	if _, err := r.GetMeta(context.Background(), "a/b+c="); err != nil {
		t.Fatal(err)
	}
	r.Shared = "team@contoso.com"
	if _, err := r.GetMeta(context.Background(), "m1"); err != nil {
		t.Fatal(err)
	}
	if paths[0] != "/me/messages/a%2Fb+c=" || paths[1] != "/users/team@contoso.com/messages/m1" {
		t.Fatalf("paths = %v", paths)
	}
	for _, p := range prefer {
		if !strings.Contains(p, `IdType="ImmutableId"`) {
			t.Fatalf("Prefer = %q", p)
		}
	}
}

func TestReaderDeltaClassification(t *testing.T) {
	status := 200
	body := ""
	r := readerFor(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status); _, _ = w.Write([]byte(body)) })
	cases := []struct {
		name, link, body string
		status           int
		want             error
	}{
		{"gone", "", "{}", 410, ErrDeltaExpired},
		{"notfound", "", "{}", 404, ErrFolderGone},
		{"syncstate", r.Base + "/x", `{"error":{"message":"The syncState is invalid"}}`, 400, ErrDeltaExpired},
	}
	for _, c := range cases {
		status, body = c.status, c.body
		if _, err := r.Delta(context.Background(), "f", c.link); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v", c.name, err)
		}
	}
	status, body = 200, `{"value":[{"id":"a","isRead":true,"flag":{"flagStatus":"flagged"}},{"id":"b","@removed":{"reason":"changed"}}],"@odata.deltaLink":"`+r.Base+`/d"}`
	p, err := r.Delta(context.Background(), "f", "")
	if err != nil || len(p.Items) != 2 || !p.Items[0].Flagged || !p.Items[0].IsRead || !p.Items[1].Removed || p.DeltaLink == "" {
		t.Fatalf("page = %+v, %v", p, err)
	}
}

func TestReaderRejectsOffOriginLinks(t *testing.T) {
	r := readerFor(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"value":[],"@odata.nextLink":"https://evil.example/n"}`))
	})
	if _, err := r.Delta(context.Background(), "f", ""); err == nil || !strings.Contains(err.Error(), "origin") {
		t.Fatalf("response link: %v", err)
	}
	if _, err := r.Delta(context.Background(), "f", "https://evil.example/n"); err == nil {
		t.Fatal("caller link accepted")
	}
}

func TestReaderErrorClassification(t *testing.T) {
	status := 429
	r := readerFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":{"code":"x","message":"m"}}`))
	})
	var re *ReadError
	_, err := r.GetRaw(context.Background(), "m")
	if !errors.As(err, &re) || !re.RateLimited || re.RetryAfter.Seconds() != 7 || re.Failure != oauth2.MailAPIFailureNone {
		t.Fatalf("429 = %+v", re)
	}
	status = 503
	if _, err = r.GetRaw(context.Background(), "m"); !errors.As(err, &re) || !re.RateLimited {
		t.Fatalf("503 = %v", err)
	}
	status = 403
	if _, err = r.GetRaw(context.Background(), "m"); !errors.As(err, &re) || re.RateLimited || re.Failure == oauth2.MailAPIFailureNone {
		t.Fatalf("403 = %+v", re)
	}
	status = 404
	if _, err = r.GetRaw(context.Background(), "m"); !errors.Is(err, ErrMessageGone) {
		t.Fatalf("404 = %v", err)
	}
}

func TestReaderFoldersWellKnownAndChildren(t *testing.T) {
	r := readerFor(t, func(w http.ResponseWriter, req *http.Request) {
		switch p := req.URL.Path; p {
		case "/me/mailFolders/msgfolderroot":
			_, _ = w.Write([]byte(`{"id":"root"}`))
		case "/me/mailFolders/inbox":
			_, _ = w.Write([]byte(`{"id":"in"}`))
		case "/me/mailFolders":
			_, _ = w.Write([]byte(`{"value":[{"id":"in","displayName":"Inbox","parentFolderId":"root","childFolderCount":1}]}`))
		case "/me/mailFolders/in/childFolders":
			_, _ = w.Write([]byte(`{"value":[{"id":"kid","displayName":"Kid","parentFolderId":"in"}]}`))
		default:
			w.WriteHeader(404)
		}
	})
	root, fs, err := r.Folders(context.Background())
	if err != nil || root != "root" || len(fs) != 2 || fs[0].WellKnown != "inbox" || fs[1].ID != "kid" || fs[1].WellKnown != "" {
		t.Fatalf("Folders = %q %+v %v", root, fs, err)
	}
}
