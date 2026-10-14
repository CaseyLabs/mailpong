package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	gosync "sync"
	"time"

	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/message"
)

// Rate-limit policy shared by the API receive drivers.
const (
	apiRetryLimit   = 3                // rate-limit retries per request before pausing sync
	apiMaxBackoff   = 60 * time.Second // ceiling for one Retry-After wait
	apiParseTimeout = 30 * time.Second
)

// apiFolders resolves mapped API containers to local folders, creating each
// once. The path index is loaded lazily and kept for the life of a driver.
type apiFolders struct {
	engine    *Engine
	accountID string

	mu      gosync.Mutex
	folders map[string]string // path -> folder ID
}

func (c *apiFolders) ensure(f message.APIFolder) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.folders == nil {
		existing, err := c.engine.folderStore.List(c.accountID)
		if err != nil {
			return "", err
		}
		c.folders = make(map[string]string, len(existing))
		for _, ex := range existing {
			c.folders[ex.Path] = ex.ID
		}
	}
	if id, ok := c.folders[f.Path]; ok {
		return id, nil
	}
	nf := &folder.Folder{AccountID: c.accountID, Name: f.Name, Path: f.Path, Type: f.Type, Subscribed: true}
	if err := c.engine.folderStore.Create(nf); err != nil {
		return "", fmt.Errorf("failed to create folder %q: %w", f.Path, err)
	}
	c.folders[f.Path] = nf.ID
	return nf.ID, nil
}

// forget drops a path from the index after its folder was deleted.
func (c *apiFolders) forget(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.folders, path)
}

// apiBackoff runs one request, retrying rate limits a bounded number of
// times with the server's Retry-After (capped). rate reports whether err is a
// rate limit and the wait the server asked for. After the limit the error is
// returned as a hint and nothing is replayed.
func apiBackoff[T any](ctx context.Context, sleep func(context.Context, time.Duration) error,
	rate func(error) (bool, time.Duration), do func() (T, error)) (T, error) {
	for attempt := 0; ; attempt++ {
		v, err := do()
		limited, wait := false, time.Duration(0)
		if err != nil {
			limited, wait = rate(err)
		}
		if err == nil || !limited || attempt >= apiRetryLimit {
			return v, err
		}
		if wait <= 0 {
			wait = time.Second << attempt
		}
		wait = min(wait, apiMaxBackoff)
		if sleep == nil {
			sleep = func(ctx context.Context, d time.Duration) error {
				t := time.NewTimer(d)
				defer t.Stop()
				select {
				case <-t.C:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		if err := sleep(ctx, wait); err != nil {
			var zero T
			return zero, err
		}
	}
}

// parseAPIRaw parses a raw RFC 5322 message with the IMAP pipeline's parser,
// so signed, encrypted, attachment, charset and threading handling is
// identical to IMAP rows. Flags and folder are left for the caller. A message
// without a Message-ID gets a deterministic one from the remote ID.
func (e *Engine) parseAPIRaw(accountID, remoteID, fallbackDomain string, raw []byte) (message.Message, *message.APIRowExtras) {
	header, _, _ := bytes.Cut(raw, []byte("\r\n\r\n"))
	if len(header) == len(raw) {
		header, _, _ = bytes.Cut(raw, []byte("\n\n"))
	}
	var m message.Message
	if err := parseHeadersIntoMessage(&m, header); err != nil {
		e.log.Debug().Err(err).Str("remoteID", remoteID).Msg("API message headers did not parse; keeping the body")
	}
	if refs := e.extractReferences(raw); len(refs) > 0 {
		if b, err := json.Marshal(refs); err == nil {
			m.References = string(b)
		}
	}
	m.ReadReceiptTo = e.extractDispositionNotificationTo(raw)
	applyHeaderFlags(&m, header)
	if m.Date.IsZero() {
		m.Date = time.Now().UTC()
	}
	if m.MessageID == "" {
		m.MessageID = remoteID + "@" + fallbackDomain // header-less message; keeps threading deterministic
	}
	m.ThreadID = e.computeThreadID(accountID, &m)
	parsed := e.parseMessageBodyFull(raw, remoteID, apiParseTimeout)
	m.BodyText, m.BodyHTML, m.HasAttachments, m.BodyFetched = parsed.BodyText, parsed.BodyHTML, parsed.HasAttachments, true
	if parsed.BodyText != "" {
		m.Snippet = generateSnippet(parsed.BodyText, 200)
	} else if parsed.BodyHTML != "" {
		m.Snippet = generateSnippet(stripHTMLTags(parsed.BodyHTML), 200)
	}
	m.Size = len(raw)
	return m, &message.APIRowExtras{SMIMERawBody: parsed.SMIMERawBody, SMIMEEncrypted: parsed.SMIMEEncrypted,
		PGPRawBody: parsed.PGPRawBody, PGPEncrypted: parsed.PGPEncrypted, Attachments: parsed.Attachments}
}

// apiFanOut runs fn over in with at most workers in flight and returns the
// results in input order. The first error cancels the rest and is returned.
func apiFanOut[T, R any](ctx context.Context, workers int, in []T, fn func(context.Context, T) (R, error)) ([]R, error) {
	out := make([]R, len(in))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg gosync.WaitGroup
	var errOnce gosync.Once
	var firstErr error
	sem := make(chan struct{}, max(workers, 1))
	for i, v := range in {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			r, err := fn(ctx, v)
			if err != nil {
				errOnce.Do(func() { firstErr = err; cancel() })
				return
			}
			out[i] = r
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, ctx.Err()
}

// ParseAPIBody parses a raw message fetched on demand for an API row, with the
// same parser the drivers use at ingestion.
func (e *Engine) ParseAPIBody(accountID, provider, remoteID string, raw []byte) (message.Message, *message.APIRowExtras) {
	domain := "graph.invalid"
	if provider == message.APIProviderGmail {
		domain = "gmail.invalid"
	}
	return e.parseAPIRaw(accountID, remoteID, domain, raw)
}
