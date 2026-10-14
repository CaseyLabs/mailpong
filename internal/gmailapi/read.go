package gmailapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/oauth2"
	"github.com/caseylabs/mailven/internal/submission"
)

// Documented Gmail API limits, recorded as constants and never probed
// (users.messages.list and users.history.list references, developers.google.com/gmail/api).
const (
	// ListPageSize is the documented maximum maxResults of messages.list and history.list.
	ListPageSize = 500
	// HistoryRetention is the documented minimum history availability
	// ("typically at least one week and often much longer"). It is a hint for
	// messages only: an expired history ID is detected by the 404 response.
	HistoryRetention = 7 * 24 * time.Hour
	// maxJSONBytes bounds one response: a 25 MiB message grows by a third as
	// base64url inside the JSON body.
	maxJSONBytes = 40 << 20
)

// readBase is the default endpoint; tests set Reader.Base.
const readBase = "https://gmail.googleapis.com/gmail/v1/users/me"

var (
	// ErrHistoryExpired means the start historyId is too old or invalid
	// (history.list answers 404). Incremental sync must fall back to a reconcile.
	ErrHistoryExpired = errors.New("gmail: history ID expired")
	// ErrMessageGone means the message no longer exists (404 on messages.get).
	ErrMessageGone = errors.New("gmail: message not found")
)

// ReadError is a failed read request. It carries the classification the sync
// driver needs: rate limits pause sync, policy and reauthorization failures
// are reported, and none of them clears credentials.
type ReadError struct {
	Status      int
	RetryAfter  time.Duration
	RateLimited bool
	Failure     oauth2.MailAPIFailure
	Err         error
}

func (e *ReadError) Error() string { return e.Err.Error() }
func (e *ReadError) Unwrap() error { return e.Err }

// Reader is the read-only Gmail client. Redirects are never followed.
type Reader struct {
	Tokens submission.TokenSource
	HTTP   *http.Client // nil uses submission.NewHTTPClient
	Base   string       // "" uses the Gmail API
}

// Label is one entry of labels.list.
type Label struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Message is a fetched message. Raw is set only for format=raw.
type Message struct {
	ID        string
	ThreadID  string
	LabelIDs  []string
	HistoryID string
	Raw       []byte
}

// HistoryRecord is one change; only the fields the sync uses are decoded.
type HistoryRecord struct {
	ID              string    `json:"id"`
	MessagesAdded   []histMsg `json:"messagesAdded"`
	MessagesDeleted []histMsg `json:"messagesDeleted"`
	LabelsAdded     []histMsg `json:"labelsAdded"`
	LabelsRemoved   []histMsg `json:"labelsRemoved"`
}

type histMsg struct {
	Message struct {
		ID string `json:"id"`
	} `json:"message"`
}

// IDs returns the message IDs of each change kind.
func (r HistoryRecord) IDs() (added, deleted, relabelled []string) {
	for _, m := range r.MessagesAdded {
		added = append(added, m.Message.ID)
	}
	for _, m := range r.MessagesDeleted {
		deleted = append(deleted, m.Message.ID)
	}
	for _, m := range r.LabelsAdded {
		relabelled = append(relabelled, m.Message.ID)
	}
	for _, m := range r.LabelsRemoved {
		relabelled = append(relabelled, m.Message.ID)
	}
	return added, deleted, relabelled
}

// HistoryPage is one page of history.list. HistoryID is the mailbox's current
// history ID when the page was served.
type HistoryPage struct {
	Records       []HistoryRecord `json:"history"`
	NextPageToken string          `json:"nextPageToken"`
	HistoryID     string          `json:"historyId"`
}

func (r *Reader) base() string {
	if r.Base != "" {
		return r.Base
	}
	return readBase
}

// get performs one GET with the 401-once refresh policy and classifies failures.
func (r *Reader) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	hc := r.HTTP
	if hc == nil {
		hc = submission.NewHTTPClient()
	}
	token, err := r.Tokens.Token(ctx)
	if err != nil {
		return nil, &ReadError{Failure: oauth2.ClassifyMailAPIError(err), Err: err}
	}
	target := r.base() + path
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		resp, err := hc.Do(req)
		if err != nil {
			return nil, fmt.Errorf("gmail read %s: %w", path, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxJSONBytes+1))
		_ = resp.Body.Close()
		status := resp.StatusCode
		if status >= 200 && status < 300 {
			if readErr != nil {
				return nil, fmt.Errorf("gmail read %s: %w", path, readErr)
			}
			if len(body) > maxJSONBytes {
				return nil, fmt.Errorf("gmail read %s: response exceeds %d bytes", path, maxJSONBytes)
			}
			return body, nil
		}
		if status == http.StatusUnauthorized && attempt == 0 {
			if token, err = r.Tokens.Refresh(ctx); err != nil {
				return nil, &ReadError{Status: status, Failure: oauth2.ClassifyMailAPIError(err), Err: err}
			}
			continue
		}
		return nil, classifyRead(status, resp.Header, body)
	}
}

func classifyRead(status int, h http.Header, body []byte) error {
	detail := strings.TrimSpace(string(body[:min(len(body), 200)]))
	e := &ReadError{Status: status, RetryAfter: parseRetryAfter(h),
		Err: fmt.Errorf("gmail read: HTTP %d: %s", status, detail)}
	switch {
	case status == http.StatusTooManyRequests, status == http.StatusForbidden && isRateLimit(body):
		e.RateLimited = true
	case status == http.StatusForbidden:
		e.Failure = oauth2.ClassifyMailAPIError(oauth2.NewProviderError("access_denied", detail))
	case status == http.StatusUnauthorized:
		e.Failure = oauth2.ClassifyMailAPIError(oauth2.NewProviderError("invalid_grant", detail))
	case status >= 500:
		e.RateLimited = true // transient: pause like a throttle, never replay in a loop
	}
	return e
}

func parseRetryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(time.Until(t), 0)
	}
	return 0
}

// Profile returns users.getProfile's current historyId, the starting point to
// capture before listing.
func (r *Reader) Profile(ctx context.Context) (historyID string, err error) {
	body, err := r.get(ctx, "/profile", nil)
	if err != nil {
		return "", err
	}
	var out struct {
		HistoryID string `json:"historyId"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.HistoryID == "" {
		return "", fmt.Errorf("gmail profile: no historyId in response")
	}
	return out.HistoryID, nil
}

// Labels returns labels.list.
func (r *Reader) Labels(ctx context.Context) ([]Label, error) {
	body, err := r.get(ctx, "/labels", nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Labels []Label `json:"labels"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode labels: %w", err)
	}
	return out.Labels, nil
}

// ListIDs returns one page of message IDs, spam and trash included so every
// folder mirrors the mailbox.
func (r *Reader) ListIDs(ctx context.Context, pageToken string) (ids []string, next string, err error) {
	q := url.Values{"maxResults": {strconv.Itoa(ListPageSize)}, "includeSpamTrash": {"true"}}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	body, err := r.get(ctx, "/messages", q)
	if err != nil {
		return nil, "", err
	}
	var out struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, "", fmt.Errorf("decode message list: %w", err)
	}
	for _, m := range out.Messages {
		ids = append(ids, m.ID)
	}
	return ids, out.NextPageToken, nil
}

// GetRaw fetches the message with format=raw: the base64url body decodes to
// the byte-identical RFC 5322 message.
func (r *Reader) GetRaw(ctx context.Context, id string) (*Message, error) {
	return r.getMessage(ctx, id, "raw")
}

// GetMinimal fetches labels and the history version without the body.
func (r *Reader) GetMinimal(ctx context.Context, id string) (*Message, error) {
	return r.getMessage(ctx, id, "minimal")
}

func (r *Reader) getMessage(ctx context.Context, id, format string) (*Message, error) {
	body, err := r.get(ctx, "/messages/"+url.PathEscape(id), url.Values{"format": {format}})
	if err != nil {
		var re *ReadError
		if errors.As(err, &re) && re.Status == http.StatusNotFound {
			return nil, ErrMessageGone
		}
		return nil, err
	}
	var out struct {
		ID        string   `json:"id"`
		ThreadID  string   `json:"threadId"`
		LabelIDs  []string `json:"labelIds"`
		HistoryID string   `json:"historyId"`
		Raw       string   `json:"raw"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode message %s: %w", id, err)
	}
	m := &Message{ID: out.ID, ThreadID: out.ThreadID, LabelIDs: out.LabelIDs, HistoryID: out.HistoryID}
	if m.ID == "" {
		return nil, fmt.Errorf("gmail message %s: response has no id", id)
	}
	if format == "raw" {
		// Gmail pads inconsistently; accept both forms.
		if m.Raw, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(out.Raw, "=")); err != nil {
			return nil, fmt.Errorf("decode raw message %s: %w", id, err)
		}
	}
	return m, nil
}

// History returns one page of history.list from startHistoryID for the change
// kinds the sync applies. A 404 means the ID expired: ErrHistoryExpired.
func (r *Reader) History(ctx context.Context, startHistoryID, pageToken string) (*HistoryPage, error) {
	q := url.Values{"startHistoryId": {startHistoryID}, "maxResults": {strconv.Itoa(ListPageSize)},
		"historyTypes": {"messageAdded", "messageDeleted", "labelAdded", "labelRemoved"}}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	body, err := r.get(ctx, "/history", q)
	if err != nil {
		var re *ReadError
		if errors.As(err, &re) && re.Status == http.StatusNotFound {
			return nil, ErrHistoryExpired
		}
		return nil, err
	}
	var out HistoryPage
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode history: %w", err)
	}
	return &out, nil
}
