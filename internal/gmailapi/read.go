package gmailapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/apiread"
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
	// ErrMessageTooLarge means the raw message exceeds the download cap; the
	// message exists, so callers keep it as a header-only row instead of failing.
	ErrMessageTooLarge = errors.New("gmail: message exceeds the download size limit")
)

// errTooLarge marks a response past its byte cap.
var errTooLarge = errors.New("response too large")

// ReadError is a failed read request, classified for the sync driver.
type ReadError = apiread.Error

// Reader is the read-only Gmail client. Redirects are never followed.
type Reader struct {
	Tokens submission.TokenSource
	HTTP   *http.Client // nil uses submission.NewHTTPClient
	Base   string       // "" uses the Gmail API
	// MaxRawBytes caps one format=raw response; 0 uses the JSON cap (40 MiB).
	MaxRawBytes int64
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
	// InternalDate is Gmail's receive time (zero when absent), used when the
	// message has no usable Date header.
	InternalDate time.Time
	// Headers holds the format=metadata headers, keyed canonically
	// ("Subject", "From", "Message-Id"); nil for other formats.
	Headers map[string]string
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
	return r.getCapped(ctx, path, q, maxJSONBytes)
}

func (r *Reader) getCapped(ctx context.Context, path string, q url.Values, maxBytes int64) ([]byte, error) {
	target := r.base() + path
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	c := apiread.Client{Tokens: r.Tokens, HTTP: r.HTTP, Label: "gmail read", Detail: path,
		Header:               http.Header{"Accept": {"application/json"}},
		ForbiddenRateLimited: isRateLimit, TooLarge: errTooLarge}
	return c.Get(ctx, target, maxBytes)
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
// folder mirrors the mailbox. A non-zero after limits the list to messages
// received after it (Gmail search, second precision).
func (r *Reader) ListIDs(ctx context.Context, pageToken string, after time.Time) (ids []string, next string, err error) {
	q := url.Values{"maxResults": {strconv.Itoa(ListPageSize)}, "includeSpamTrash": {"true"}}
	if !after.IsZero() {
		q.Set("q", "after:"+strconv.FormatInt(after.Unix(), 10))
	}
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

// GetMetadata fetches labels, the history version and the Subject, From and
// Message-ID headers without the body, for a header-only row.
func (r *Reader) GetMetadata(ctx context.Context, id string) (*Message, error) {
	return r.getMessage(ctx, id, "metadata")
}

func (r *Reader) getMessage(ctx context.Context, id, format string) (*Message, error) {
	q := url.Values{"format": {format}}
	limit := int64(maxJSONBytes)
	switch format {
	case "raw":
		if r.MaxRawBytes > 0 {
			limit = r.MaxRawBytes
		}
	case "metadata":
		q["metadataHeaders"] = []string{"Subject", "From", "Message-ID"}
	}
	body, err := r.getCapped(ctx, "/messages/"+url.PathEscape(id), q, limit)
	if err != nil {
		var re *ReadError
		switch {
		case errors.As(err, &re) && re.Status == http.StatusNotFound:
			return nil, ErrMessageGone
		case errors.Is(err, errTooLarge):
			return nil, ErrMessageTooLarge
		}
		return nil, err
	}
	var out struct {
		ID        string   `json:"id"`
		ThreadID  string   `json:"threadId"`
		LabelIDs  []string `json:"labelIds"`
		HistoryID string   `json:"historyId"`
		Raw       string   `json:"raw"`
		Internal  string   `json:"internalDate"` // milliseconds since the epoch
		Payload   struct {
			Headers []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode message %s: %w", id, err)
	}
	m := &Message{ID: out.ID, ThreadID: out.ThreadID, LabelIDs: out.LabelIDs, HistoryID: out.HistoryID}
	for _, h := range out.Payload.Headers {
		if m.Headers == nil {
			m.Headers = map[string]string{}
		}
		m.Headers[http.CanonicalHeaderKey(h.Name)] = h.Value
	}
	if ms, perr := strconv.ParseInt(out.Internal, 10, 64); perr == nil && ms > 0 {
		m.InternalDate = time.UnixMilli(ms).UTC()
	}
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
