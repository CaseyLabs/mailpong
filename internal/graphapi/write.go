package graphapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/caseylabs/mailven/internal/apiwrite"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/submission"
)

// ErrWriteGone means the message no longer exists (404 on a write).
var ErrWriteGone = errors.New("graph: message not found")

// Writer applies user actions: read and flag updates, moves, and trash (a move
// to Deleted Items). Permanent deletion is not offered. A grant that fails the
// recorded write capability (the Shared permission for a shared mailbox) sends
// no request. Redirects are never followed.
type Writer struct {
	Tokens submission.TokenSource
	// Grant is the account's recorded API grant; nil refuses every call.
	Grant *credentials.OAuthTokens
	HTTP  *http.Client // nil uses submission.NewHTTPClient
	Base  string       // "" uses the Graph API
	// Shared names a shared mailbox (users/{Shared}); "" is the signed-in user's.
	Shared string
}

// Patch is a set of message property changes; nil fields are left alone.
type Patch struct {
	IsRead  *bool
	Flagged *bool
}

func (w *Writer) message(id string) string {
	base := w.Base
	if base == "" {
		base = graphBase
	}
	mb := "/me"
	if w.Shared != "" {
		mb = "/users/" + url.PathEscape(w.Shared)
	}
	return base + mb + "/messages/" + url.PathEscape(id)
}

func (w *Writer) do(ctx context.Context, method, target string, payload any) ([]byte, error) {
	if err := w.Grant.RequireMailAPICapability("write", w.Shared != ""); err != nil {
		return nil, &apiwrite.Error{Outcome: apiwrite.NotSent, Err: err}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &apiwrite.Error{Outcome: apiwrite.NotSent, Err: err}
	}
	out, err := apiwrite.Do(ctx, w.HTTP, w.Tokens, apiwrite.Request{Method: method, URL: target,
		Header: http.Header{"Content-Type": {"application/json"}, "Accept": {"application/json"},
			"Prefer": {`IdType="ImmutableId"`}}, Body: body})
	if apiwrite.StatusOf(err) == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %w", ErrWriteGone, err)
	}
	return out, err
}

// Update changes isRead and/or flag on one message. It sets absolute values,
// so a verified retry is safe.
func (w *Writer) Update(ctx context.Context, id string, p Patch) error {
	payload := map[string]any{}
	if p.IsRead != nil {
		payload["isRead"] = *p.IsRead
	}
	if p.Flagged != nil {
		status := "notFlagged"
		if *p.Flagged {
			status = "flagged"
		}
		payload["flag"] = map[string]string{"flagStatus": status}
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.do(ctx, http.MethodPatch, w.message(id), payload)
	return err
}

// Moved is the result of a move: the message's ID and folder afterwards.
type Moved struct{ ID, ParentFolderID string }

// Move moves a message to a folder ID or well-known folder name and returns
// the (possibly new) ID. A move is not idempotent: after an unknown outcome
// the caller must verify with GetMeta, never re-send.
func (w *Writer) Move(ctx context.Context, id, destination string) (*Moved, error) {
	body, err := w.do(ctx, http.MethodPost, w.message(id)+"/move", map[string]string{"destinationId": destination})
	if err != nil {
		return nil, err
	}
	var out struct {
		ID             string `json:"id"`
		ParentFolderID string `json:"parentFolderId"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.ID == "" {
		// Applied, but the new identity is unreadable: the caller must re-read it.
		return nil, &apiwrite.Error{Outcome: apiwrite.Unknown, Err: fmt.Errorf("graph move: no id in response")}
	}
	return &Moved{ID: out.ID, ParentFolderID: out.ParentFolderID}, nil
}

// Trash moves a message to Deleted Items.
func (w *Writer) Trash(ctx context.Context, id string) (*Moved, error) {
	return w.Move(ctx, id, "deleteditems")
}
