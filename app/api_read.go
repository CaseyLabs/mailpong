package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/graphapi"
	"github.com/caseylabs/mailven/internal/message"
)

// apiReadTimeout bounds one on-demand raw fetch.
const apiReadTimeout = 60 * time.Second

// fetchRawMessage returns a message's RFC 5322 source. API-routed accounts
// read it through the provider API; every other account reads it over IMAP.
// A routed account never falls back to IMAP: its local UIDs are surrogates.
func (a *App) fetchRawMessage(ctx context.Context, msg *message.Message) ([]byte, error) {
	route := a.apiRouted(msg.AccountID)
	if route == nil {
		return a.syncEngine.FetchRawMessage(ctx, msg.AccountID, msg.FolderID, msg.UID)
	}
	return a.fetchAPIRaw(ctx, route, msg)
}

func (a *App) fetchAPIRaw(ctx context.Context, route *account.MailAPIRoute, msg *message.Message) ([]byte, error) {
	if route.Shared {
		return nil, errors.New("shared mailbox messages cannot be read through the API yet")
	}
	if a.apiPaused(msg.AccountID) {
		return nil, errors.New("the mail API needs reauthorization before messages can be read")
	}
	provider := apiProvider(route)
	remoteID, _, err := a.messageStore.APIMessageRemote(msg.AccountID, provider, msg.ID)
	if err != nil {
		return nil, fmt.Errorf("message has no API link: %w", err)
	}
	ops := a.mailAPIOps()
	grant, err := ops.validToken(msg.AccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API authorization: %w", err)
	}
	if err := grant.RequireMailAPICapability("read", route.Shared); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, apiReadTimeout)
	defer cancel()
	tokens := &apiTokenSource{ops: ops, accountID: msg.AccountID}
	var raw []byte
	switch provider {
	case message.APIProviderGmail:
		var m *gmailapi.Message
		if m, err = (&gmailapi.Reader{Tokens: tokens, HTTP: a.apiRoutes.http, Base: a.apiRoutes.gmailBase}).GetRaw(ctx, remoteID); err == nil {
			raw = m.Raw
		}
	default:
		raw, err = (&graphapi.Reader{Tokens: tokens, HTTP: a.apiRoutes.http, Base: a.apiRoutes.graphBase}).GetRaw(ctx, remoteID)
	}
	switch {
	case errors.Is(err, gmailapi.ErrMessageGone), errors.Is(err, graphapi.ErrMessageGone):
		return nil, errors.New("this message no longer exists on the server")
	case errors.Is(err, graphapi.ErrMessageTooLarge):
		return nil, errors.New("this message is too large to download")
	case err != nil:
		return nil, fmt.Errorf("failed to read message from the mail API: %w", err)
	}
	return raw, nil
}

// fetchBody returns the message with its body, loading it on demand: over IMAP
// for ordinary accounts, from the API's raw message for routed ones.
func (a *App) fetchBody(ctx context.Context, msg *message.Message) (*message.Message, error) {
	route := a.apiRouted(msg.AccountID)
	if route == nil {
		return a.syncEngine.FetchMessageBody(ctx, msg.AccountID, msg.ID)
	}
	raw, err := a.fetchAPIRaw(ctx, route, msg)
	if err != nil {
		return nil, err
	}
	remoteID, _, err := a.messageStore.APIMessageRemote(msg.AccountID, apiProvider(route), msg.ID)
	if err != nil {
		return nil, err
	}
	parsed, extras := a.syncEngine.ParseAPIBody(msg.AccountID, apiProvider(route), remoteID, raw)
	if err := a.messageStore.StoreAPIBody(msg.ID, &parsed, extras); err != nil {
		return nil, err
	}
	return a.messageStore.Get(msg.ID)
}
