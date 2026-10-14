package gmailapi

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

// MaxBatchIDs is the documented ceiling of users.messages.batchModify
// (developers.google.com/gmail/api/reference/rest/v1/users.messages/batchModify).
const MaxBatchIDs = 1000

// Well-known system label IDs used by actions.
const (
	LabelInbox   = "INBOX"
	LabelUnread  = "UNREAD"
	LabelStarred = "STARRED"
	LabelTrash   = "TRASH"
)

// ErrWriteGone means the message no longer exists (404 on a write).
var ErrWriteGone = errors.New("gmail: message not found")

// Writer applies user actions with the ordinary profile: label changes and
// trash/untrash. Permanent deletion is not offered. A grant that fails the
// recorded write capability sends no request. Redirects are never followed.
type Writer struct {
	Tokens submission.TokenSource
	// Grant is the account's recorded API grant; nil refuses every call.
	Grant *credentials.OAuthTokens
	HTTP  *http.Client // nil uses submission.NewHTTPClient
	Base  string       // "" uses the Gmail API
}

func (w *Writer) base() string {
	if w.Base != "" {
		return w.Base
	}
	return readBase
}

func (w *Writer) post(ctx context.Context, path string, payload any) ([]byte, error) {
	if err := w.Grant.RequireMailAPICapability("write", false); err != nil {
		return nil, &apiwrite.Error{Outcome: apiwrite.NotSent, Err: err}
	}
	var body []byte
	if payload != nil {
		var err error
		if body, err = json.Marshal(payload); err != nil {
			return nil, &apiwrite.Error{Outcome: apiwrite.NotSent, Err: err}
		}
	}
	out, err := apiwrite.Do(ctx, w.HTTP, w.Tokens, apiwrite.Request{Method: http.MethodPost, URL: w.base() + path,
		Header: http.Header{"Content-Type": {"application/json"}, "Accept": {"application/json"}}, Body: body})
	if apiwrite.StatusOf(err) == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %w", ErrWriteGone, err)
	}
	return out, err
}

func decodeMessage(body []byte) (*Message, error) {
	var out struct {
		ID       string   `json:"id"`
		ThreadID string   `json:"threadId"`
		LabelIDs []string `json:"labelIds"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.ID == "" {
		return nil, fmt.Errorf("decode message result: no id in response")
	}
	return &Message{ID: out.ID, ThreadID: out.ThreadID, LabelIDs: out.LabelIDs}, nil
}

// Modify adds and removes labels on one message (read/unread, star, move
// between labels, archive). It is a set operation, so a verified retry is safe.
// The result carries the message's labels after the change.
func (w *Writer) Modify(ctx context.Context, id string, add, remove []string) (*Message, error) {
	body, err := w.post(ctx, "/messages/"+url.PathEscape(id)+"/modify",
		map[string][]string{"addLabelIds": nonNil(add), "removeLabelIds": nonNil(remove)})
	if err != nil {
		return nil, err
	}
	m, err := decodeMessage(body)
	if err != nil {
		// The change was applied; only the echo is unreadable.
		return nil, &apiwrite.Error{Outcome: apiwrite.Unknown, Err: err}
	}
	return m, nil
}

// Trash moves one message to the trash; Untrash restores it. Both are idempotent.
func (w *Writer) Trash(ctx context.Context, id string) (*Message, error) {
	return w.trash(ctx, id, "trash")
}

func (w *Writer) Untrash(ctx context.Context, id string) (*Message, error) {
	return w.trash(ctx, id, "untrash")
}

func (w *Writer) trash(ctx context.Context, id, verb string) (*Message, error) {
	body, err := w.post(ctx, "/messages/"+url.PathEscape(id)+"/"+verb, nil)
	if err != nil {
		return nil, err
	}
	m, err := decodeMessage(body)
	if err != nil {
		return nil, &apiwrite.Error{Outcome: apiwrite.Unknown, Err: err}
	}
	return m, nil
}

// BatchModify applies one label change to many messages in chunks of
// MaxBatchIDs. It returns how many leading IDs were applied; on failure the
// chunk that failed has unknown or rejected state per the error, and later
// chunks were not sent.
func (w *Writer) BatchModify(ctx context.Context, ids, add, remove []string) (applied int, err error) {
	for start := 0; start < len(ids); start += MaxBatchIDs {
		chunk := ids[start:min(start+MaxBatchIDs, len(ids))]
		if _, err = w.post(ctx, "/messages/batchModify", map[string][]string{
			"ids": chunk, "addLabelIds": nonNil(add), "removeLabelIds": nonNil(remove)}); err != nil {
			return applied, err
		}
		applied += len(chunk)
	}
	return applied, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
