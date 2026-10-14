package graphapi

import (
	"context"
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

// Documented Microsoft Graph behaviour, recorded as constants and never
// probed (learn.microsoft.com/en-us/graph/api/message-delta and
// /graph/throttling-limits#outlook-service-limits).
const (
	// DeltaPageSize is the odata.maxpagesize requested for message delta. It is
	// a conservative local choice, not the service maximum.
	DeltaPageSize = 50
	// FolderPageSize is the $top requested for folder listings (conservative).
	FolderPageSize = 100
	// MaxConcurrency is Outlook's documented limit of 4 concurrent requests
	// per app and mailbox.
	MaxConcurrency = 4
	// maxJSONBytes and maxRawBytes bound one response.
	maxJSONBytes = 8 << 20
	maxRawBytes  = 64 << 20
)

// WellKnownFolders are the well-known names resolved to folder IDs.
var WellKnownFolders = []string{"inbox", "sentitems", "drafts", "deleteditems", "junkemail", "archive"}

var (
	// ErrDeltaExpired means the delta token is no longer valid (410 Gone,
	// syncStateNotFound or resyncRequired): that folder needs a reconcile.
	ErrDeltaExpired = errors.New("graph: delta state expired")
	// ErrFolderGone means the folder no longer exists (404 on its delta).
	ErrFolderGone = errors.New("graph: folder not found")
	// ErrMessageGone means the message no longer exists (404 on a message).
	ErrMessageGone = errors.New("graph: message not found")
	// ErrMessageTooLarge means the raw message exceeds the download cap; the
	// message exists, so callers keep it as a header-only row instead of failing.
	ErrMessageTooLarge = errors.New("graph: message exceeds the download size limit")
)

// errTooLarge marks a response past its byte cap.
var errTooLarge = errors.New("response too large")

// ReadError is a failed read request, classified for the sync driver.
type ReadError = apiread.Error

// Reader is the read-only Graph client for one mailbox. Redirects are never
// followed, and a server-supplied paging link must stay on the API origin so
// the bearer token is never sent elsewhere.
type Reader struct {
	Tokens submission.TokenSource
	HTTP   *http.Client // nil uses submission.NewHTTPClient
	Base   string       // "" uses the Graph API
	// Shared names a shared mailbox (read through users/{Shared}); "" is the
	// signed-in user's own mailbox. The caller checks the Shared grant first.
	Shared string
	// MaxRawBytes caps one raw message download; 0 uses the default (64 MiB).
	MaxRawBytes int64
}

// Folder is one mail folder.
type Folder struct {
	ID, Name, ParentID string
	// WellKnown is the well-known name whose resolved ID equals ID.
	WellKnown string
	Children  int
}

// Meta is the per-message data requested from delta and message reads.
type Meta struct {
	ID             string
	ParentFolderID string
	ChangeKey      string
	ConversationID string
	IsRead         bool
	IsDraft        bool
	Flagged        bool
	// Subject, FromName, FromEmail and ReceivedAt describe the message without
	// its MIME content: ReceivedAt stands in for a missing Date header and the
	// rest build a header-only row for a message too large to download.
	Subject, FromName, FromEmail string
	ReceivedAt                   time.Time
	// Removed marks an @removed delta entry (the other fields are empty).
	Removed bool
}

// DeltaPage is one page of a folder's message delta. NextLink continues the
// enumeration; DeltaLink (set on the last page) is the resume cursor.
type DeltaPage struct {
	Items     []Meta
	NextLink  string
	DeltaLink string
}

const messageSelect = "id,parentFolderId,changeKey,conversationId,isRead,isDraft,flag,subject,from,receivedDateTime"

type wireMeta struct {
	ID             string `json:"id"`
	ParentFolderID string `json:"parentFolderId"`
	ChangeKey      string `json:"changeKey"`
	ConversationID string `json:"conversationId"`
	IsRead         bool   `json:"isRead"`
	IsDraft        bool   `json:"isDraft"`
	Flag           struct {
		Status string `json:"flagStatus"`
	} `json:"flag"`
	Subject          string    `json:"subject"`
	ReceivedDateTime time.Time `json:"receivedDateTime"`
	From             struct {
		EmailAddress struct {
			Name    string `json:"name"`
			Address string `json:"address"`
		} `json:"emailAddress"`
	} `json:"from"`
	Removed *struct {
		Reason string `json:"reason"`
	} `json:"@removed"`
}

func (w wireMeta) meta() Meta {
	return Meta{ID: w.ID, ParentFolderID: w.ParentFolderID, ChangeKey: w.ChangeKey, ConversationID: w.ConversationID,
		IsRead: w.IsRead, IsDraft: w.IsDraft, Flagged: strings.EqualFold(w.Flag.Status, "flagged"), Removed: w.Removed != nil,
		Subject: w.Subject, FromName: w.From.EmailAddress.Name, FromEmail: w.From.EmailAddress.Address, ReceivedAt: w.ReceivedDateTime.UTC()}
}

func (r *Reader) base() string {
	if r.Base != "" {
		return r.Base
	}
	return graphBase
}

func (r *Reader) mailbox() string {
	if r.Shared != "" {
		return "/users/" + url.PathEscape(r.Shared)
	}
	return "/me"
}

// sameOrigin reports whether link is on the API origin.
func (r *Reader) sameOrigin(link string) bool {
	l, err1 := url.Parse(link)
	b, err2 := url.Parse(r.base())
	return err1 == nil && err2 == nil && l.Scheme == b.Scheme && l.Host == b.Host
}

// fetch performs one GET with the 401-once refresh policy and classifies failures.
// IdType="ImmutableId" is sent on every request so IDs survive moves and renames.
func (r *Reader) fetch(ctx context.Context, target string, maxBytes int64, extraPrefer string) ([]byte, error) {
	prefer := `IdType="ImmutableId"`
	if extraPrefer != "" {
		prefer += ", " + extraPrefer
	}
	c := apiread.Client{Tokens: r.Tokens, HTTP: r.HTTP, Label: "graph read",
		Header: http.Header{"Prefer": {prefer}}, TooLarge: errTooLarge}
	return c.Get(ctx, target, maxBytes)
}

func status(err error) int {
	var re *ReadError
	if errors.As(err, &re) {
		return re.Status
	}
	return 0
}

// getJSON reads one JSON document into out.
func (r *Reader) getJSON(ctx context.Context, target string, out any) error {
	body, err := r.fetch(ctx, target, maxJSONBytes, "")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("graph read: decode: %w", err)
	}
	return nil
}

// Folders returns the mailbox root's folder ID and every folder below it
// (children included), with well-known names resolved to their folder IDs.
func (r *Reader) Folders(ctx context.Context) (rootID string, folders []Folder, err error) {
	mb := r.base() + r.mailbox()
	var root struct {
		ID string `json:"id"`
	}
	if err = r.getJSON(ctx, mb+"/mailFolders/msgfolderroot?$select=id", &root); err != nil {
		return "", nil, err
	}
	if root.ID == "" {
		return "", nil, fmt.Errorf("graph read: mailbox root has no id")
	}
	wellKnown := map[string]string{} // folder ID -> name
	for _, name := range WellKnownFolders {
		var f struct {
			ID string `json:"id"`
		}
		err := r.getJSON(ctx, mb+"/mailFolders/"+name+"?$select=id", &f)
		if status(err) == http.StatusNotFound {
			continue // e.g. no archive folder yet
		}
		if err != nil {
			return "", nil, err
		}
		wellKnown[f.ID] = name
	}
	list := func(target string) error {
		for target != "" {
			var page struct {
				Value []struct {
					ID       string `json:"id"`
					Name     string `json:"displayName"`
					ParentID string `json:"parentFolderId"`
					Children int    `json:"childFolderCount"`
				} `json:"value"`
				Next string `json:"@odata.nextLink"`
			}
			if err := r.getJSON(ctx, target, &page); err != nil {
				return err
			}
			for _, f := range page.Value {
				folders = append(folders, Folder{ID: f.ID, Name: f.Name, ParentID: f.ParentID,
					WellKnown: wellKnown[f.ID], Children: f.Children})
			}
			if page.Next != "" && !r.sameOrigin(page.Next) {
				return fmt.Errorf("graph read: paging link leaves the API origin")
			}
			target = page.Next
		}
		return nil
	}
	sel := "?$select=id,displayName,parentFolderId,childFolderCount&$top=" + strconv.Itoa(FolderPageSize)
	if err = list(mb + "/mailFolders" + sel); err != nil {
		return "", nil, err
	}
	for i := 0; i < len(folders); i++ { // breadth-first; folders grows as children are found
		if folders[i].Children > 0 {
			if err = list(mb + "/mailFolders/" + url.PathEscape(folders[i].ID) + "/childFolders" + sel); err != nil {
				return "", nil, err
			}
		}
	}
	return root.ID, folders, nil
}

// Delta returns one page of a folder's message delta. link is "" for a fresh
// enumeration, or a nextLink/deltaLink from an earlier page. A 410 (or an
// unusable delta token) is ErrDeltaExpired; a 404 on a fresh start is ErrFolderGone.
func (r *Reader) Delta(ctx context.Context, folderID, link string) (*DeltaPage, error) {
	target := link
	if link == "" {
		target = r.base() + r.mailbox() + "/mailFolders/" + url.PathEscape(folderID) + "/messages/delta?$select=" + messageSelect
	} else if !r.sameOrigin(link) {
		return nil, fmt.Errorf("graph read: delta link leaves the API origin")
	}
	body, err := r.fetch(ctx, target, maxJSONBytes, "odata.maxpagesize="+strconv.Itoa(DeltaPageSize))
	if err != nil {
		var re *ReadError
		switch {
		case errors.As(err, &re) && (re.Status == http.StatusGone || (re.Status == http.StatusBadRequest && link != "" &&
			strings.Contains(re.Err.Error(), "syncState"))):
			return nil, ErrDeltaExpired
		case status(err) == http.StatusNotFound:
			return nil, ErrFolderGone
		}
		return nil, err
	}
	var out struct {
		Value     []wireMeta `json:"value"`
		NextLink  string     `json:"@odata.nextLink"`
		DeltaLink string     `json:"@odata.deltaLink"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("graph read: decode delta: %w", err)
	}
	for _, l := range []string{out.NextLink, out.DeltaLink} {
		if l != "" && !r.sameOrigin(l) {
			return nil, fmt.Errorf("graph read: delta link leaves the API origin")
		}
	}
	page := &DeltaPage{NextLink: out.NextLink, DeltaLink: out.DeltaLink}
	for _, w := range out.Value {
		page.Items = append(page.Items, w.meta())
	}
	return page, nil
}

// GetMeta reads one message's metadata; a 404 is ErrMessageGone.
func (r *Reader) GetMeta(ctx context.Context, id string) (*Meta, error) {
	var w wireMeta
	if err := r.getJSON(ctx, r.base()+r.mailbox()+"/messages/"+url.PathEscape(id)+"?$select="+messageSelect, &w); err != nil {
		if status(err) == http.StatusNotFound {
			return nil, ErrMessageGone
		}
		return nil, err
	}
	m := w.meta()
	if m.ID == "" {
		return nil, fmt.Errorf("graph message %s: response has no id", id)
	}
	return &m, nil
}

// GetRaw reads the message's MIME content (messages/{id}/$value), byte-identical
// to what the service stores. A 404 is ErrMessageGone.
func (r *Reader) GetRaw(ctx context.Context, id string) ([]byte, error) {
	limit := r.MaxRawBytes
	if limit <= 0 {
		limit = maxRawBytes
	}
	body, err := r.fetch(ctx, r.base()+r.mailbox()+"/messages/"+url.PathEscape(id)+"/$value", limit, "")
	switch {
	case status(err) == http.StatusNotFound:
		return nil, ErrMessageGone
	case errors.Is(err, errTooLarge):
		return nil, ErrMessageTooLarge
	}
	return body, err
}
