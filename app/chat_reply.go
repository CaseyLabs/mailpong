package app

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/draft"
	"github.com/caseylabs/mailven/internal/email"
	"github.com/caseylabs/mailven/internal/logging"
	"github.com/caseylabs/mailven/internal/settings"
	"github.com/caseylabs/mailven/internal/smtp"
)

// ChatReply is a reply typed in the chat composer.
type ChatReply struct {
	AccountID   string            `json:"accountId"`
	ThreadKey   string            `json:"threadKey"`
	MessageID   string            `json:"messageId"` // message replied to
	Text        string            `json:"text"`
	ReplyAll    bool              `json:"replyAll"`
	Attachments []smtp.Attachment `json:"attachments"` // files the user attached
	DraftID     string            `json:"draftId"`     // chat draft being edited, if any
}

// ChatDraft is a thread's saved chat composer state.
type ChatDraft struct {
	DraftID     string            `json:"draftId"`
	MessageID   string            `json:"messageId"`
	Text        string            `json:"text"`
	ReplyAll    bool              `json:"replyAll"`
	Attachments []smtp.Attachment `json:"attachments"`
}

// ============================================================================
// Chat reply API - Exposed to frontend via Wails bindings
// ============================================================================

// QueueChatReply queues a chat reply to send after the undo delay and returns
// the send ID for UndoSend. The send:finished event reports the outcome; a
// failed reply without a draft is saved as one, whose ID the event carries.
func (a *App) QueueChatReply(r ChatReply) (string, error) {
	if chatReplyEmpty(r) {
		return "", fmt.Errorf("chat reply is empty")
	}
	return a.sendQueue.add(func() (string, error) {
		err := a.sendChatReply(r)
		if err == nil || r.DraftID != "" {
			return r.DraftID, err
		}
		// Keep the unsent reply so it survives a restart. A draft typed while
		// it was sending keeps the thread's chat link.
		draftID, saveErr := a.saveChatDraft(r, false)
		if saveErr != nil {
			sendLog().Warn().Err(saveErr).Msg("Failed to save unsent chat reply")
		}
		return draftID, err
	}), nil
}

// sendChatReply sends a chat reply, deletes its draft, and syncs Sent so the
// reply joins the thread.
func (a *App) sendChatReply(r ChatReply) error {
	if chatReplyEmpty(r) {
		return fmt.Errorf("chat reply is empty")
	}
	msg, err := a.buildChatReply(r)
	if err != nil {
		return err
	}
	if r.DraftID != "" {
		// Keep a background draft upload from racing the send.
		a.cancelDraftSync(r.DraftID)
	}
	if _, err := a.composeOps.sendMessage(a.ctx, r.AccountID, *msg, nil); err != nil {
		return err
	}

	log := logging.WithComponent("app.chat")
	// Deleting the draft drops its chat link too (ON DELETE CASCADE); a draft
	// typed while this one was sending keeps its own link.
	if r.DraftID != "" {
		if err := a.DeleteDraft(r.DraftID); err != nil {
			log.Warn().Err(err).Str("draftID", r.DraftID).Msg("Failed to delete chat draft after send")
		}
	}
	go func() {
		if err := a.syncSentFolder(r.AccountID); err != nil {
			log.Debug().Err(err).Msg("Sent folder sync failed")
		}
	}()
	return nil
}

// SaveChatDraft saves the chat composer's text as a reply draft and returns
// its ID. Empty text without attachments deletes the draft and returns "".
func (a *App) SaveChatDraft(r ChatReply) (string, error) {
	return a.saveChatDraft(r, true)
}

// saveChatDraft saves a chat reply draft. replaceLink false keeps a chat link
// the thread already has, so the new draft is linked only when there is none.
func (a *App) saveChatDraft(r ChatReply, replaceLink bool) (string, error) {
	r.Text = normalizeChatText(r.Text)
	if chatReplyEmpty(r) {
		if err := a.draftStore.DeleteChatLink(r.AccountID, r.ThreadKey); err != nil {
			return "", err
		}
		if r.DraftID != "" {
			return "", a.DeleteDraft(r.DraftID)
		}
		return "", nil
	}

	msg, err := a.buildChatReply(r)
	if err != nil {
		return "", err
	}
	res, err := a.SaveDraft(r.AccountID, *msg, r.DraftID)
	if err != nil {
		return "", err
	}
	if !replaceLink {
		existing, err := a.draftStore.GetChatLink(r.AccountID, r.ThreadKey)
		if err != nil || existing != nil {
			return res.Draft.ID, err
		}
	}
	link := draft.ChatLink{DraftID: res.Draft.ID, MessageID: r.MessageID, ReplyAll: r.ReplyAll, TextLen: len(r.Text)}
	if err := a.draftStore.SetChatLink(r.AccountID, r.ThreadKey, link); err != nil {
		return "", err
	}
	return res.Draft.ID, nil
}

// GetChatDraft returns a thread's saved chat draft, or nil when there is none.
func (a *App) GetChatDraft(accountID, threadKey string) (*ChatDraft, error) {
	link, err := a.draftStore.GetChatLink(accountID, threadKey)
	if err != nil || link == nil {
		return nil, err
	}
	d, err := a.draftStore.Get(link.DraftID)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, a.draftStore.DeleteChatLink(accountID, threadKey)
	}
	msg := a.draftOps.toComposeMessage(d)
	out := &ChatDraft{DraftID: d.ID, MessageID: link.MessageID, ReplyAll: link.ReplyAll, Text: chatDraftText(msg.TextBody, link.TextLen)}
	for _, att := range msg.Attachments {
		if !att.Inline {
			out.Attachments = append(out.Attachments, att)
		}
	}
	return out, nil
}

// ReleaseChatDraft detaches a thread from its chat draft, keeping the draft,
// so it can be edited in the full composer.
func (a *App) ReleaseChatDraft(accountID, threadKey string) error {
	return a.draftStore.DeleteChatLink(accountID, threadKey)
}

// ChatReplyFrom returns the address a reply to the message goes out from when
// that is not the default identity (an alias the mail was addressed or
// forwarded to), else "". It picks the identity as PrepareReply does, so the
// chat reply box can show the forwarding and Bcc aliases it cannot see.
func (a *App) ChatReplyFrom(messageID string) (string, error) {
	msg, err := a.messageStore.Get(messageID)
	if err != nil || msg == nil {
		return "", err
	}
	identities, err := a.accountStore.GetIdentities(msg.AccountID)
	if err != nil {
		return "", fmt.Errorf("failed to get identities: %w", err)
	}
	id := selectReplyFromIdentity(identities, msg, func() []string {
		return resolveForwarding(a.ctx, identities, msg, a.accountStore, a.messageStore, a.folderStore, a.imapPool)
	})
	if id == nil || id.IsDefault {
		return "", nil
	}
	return strings.TrimSpace(id.Email), nil
}

// chatFlushWait bounds how long shutdown waits for the chat composer to save.
const chatFlushWait = 2 * time.Second

// ChatDraftsFlushed tells a shutting-down app that the chat composer has
// saved its unsaved text.
func (a *App) ChatDraftsFlushed() {
	select {
	case a.chatFlushed <- struct{}{}:
	default:
	}
}

// waitChatDraftsFlushed waits (bounded) for the frontend's ChatDraftsFlushed
// after app:shutting-down, so text typed just before quitting is saved.
func (a *App) waitChatDraftsFlushed() {
	select {
	case <-a.chatFlushed:
	case <-time.After(chatFlushWait):
	}
}

// ============================================================================
// Reply assembly
// ============================================================================

// buildChatReply turns a chat reply into a full reply message: PrepareReply's
// recipients and threading headers, the typed text, the reply signature, and
// the quoted original when the chat_include_quote setting is on.
func (a *App) buildChatReply(r ChatReply) (*smtp.ComposeMessage, error) {
	if r.MessageID == "" {
		return nil, fmt.Errorf("no message to reply to")
	}
	mode := "reply"
	if r.ReplyAll {
		mode = "reply-all"
	}
	msg, err := a.PrepareReply(r.MessageID, mode)
	if err != nil {
		return nil, err
	}
	identities, err := a.accountStore.GetIdentities(r.AccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get identities: %w", err)
	}
	includeQuote, _ := a.settingsStore.GetChatIncludeQuote()
	from := identityByEmail(identities, msg.From.Address)
	assembleChatReply(msg, normalizeChatText(r.Text), chatSignatureFor(from), includeQuote)
	accountReplyTo, _ := a.settingsStore.GetDefaultAddress(settings.DefaultAddressReplyTo, r.AccountID)
	applyIdentityReplyTo(msg, from, accountReplyTo)
	msg.Attachments = append(msg.Attachments, r.Attachments...)
	return msg, nil
}

// applyIdentityReplyTo sets msg's Reply-To from the identity it is sent as,
// else the first address of the account default list, unless one is already set.
func applyIdentityReplyTo(msg *smtp.ComposeMessage, id *account.Identity, accountDefault string) {
	if msg.ReplyTo != nil {
		return
	}
	addr := ""
	if id != nil {
		addr = id.ReplyTo
	}
	if addr == "" {
		fields := strings.FieldsFunc(accountDefault, func(r rune) bool { return r == ',' || r == ';' })
		if len(fields) > 0 {
			addr = strings.TrimSpace(fields[0])
		}
	}
	if addr == "" {
		return
	}
	msg.ReplyTo = &smtp.Address{Address: addr}
}

// chatSignature is an identity's reply signature, ready to append.
type chatSignature struct {
	html, text string
	below      bool // after the quote rather than above it
}

func chatSignatureFor(id *account.Identity) chatSignature {
	if id == nil || !id.SignatureEnabled || !id.SignatureForReply {
		return chatSignature{}
	}
	html, text := id.SignatureHTML, id.SignatureText
	if text == "" && html != "" {
		text = email.ExtractPlainTextFromHTML(html)
	}
	if html == "" && text != "" {
		html = chatTextToHTML(text)
	}
	if html == "" {
		return chatSignature{}
	}
	if id.SignatureSeparator {
		html, text = "<p>-- </p>"+html, "-- \n"+text
	}
	return chatSignature{html: html, text: text, below: id.SignaturePlacement == "below"}
}

// assembleChatReply replaces msg's PrepareReply body with text, sig, and
// (when quote is set) the prepared quote. The text body starts with text
// exactly, which GetChatDraft relies on. Without the quote, the inline parts
// PrepareReply added for it are dropped.
func assembleChatReply(msg *smtp.ComposeMessage, text string, sig chatSignature, quote bool) {
	htmlParts := []string{chatTextToHTML(text)}
	textParts := []string{text}
	add := func(html, txt string) {
		htmlParts = append(htmlParts, html)
		textParts = append(textParts, txt)
	}
	if sig.html != "" && !(quote && sig.below) {
		add(sig.html, sig.text)
	}
	if quote {
		// PrepareReply leaves two empty paragraphs (lines) for the reply text.
		add(strings.TrimPrefix(msg.HTMLBody, "<p></p><p></p>"), strings.TrimLeft(msg.TextBody, "\n"))
		if sig.html != "" && sig.below {
			add(sig.html, sig.text)
		}
	} else {
		kept := msg.Attachments[:0]
		for _, att := range msg.Attachments {
			if !att.Inline {
				kept = append(kept, att)
			}
		}
		msg.Attachments = kept
	}
	msg.HTMLBody = strings.Join(htmlParts, "")
	msg.TextBody = strings.Join(textParts, "\n\n")
}

var blankLines = regexp.MustCompile(`\n[ \t]*\n\s*`)

// chatTextToHTML renders plain text as escaped paragraphs; single newlines
// become line breaks.
func chatTextToHTML(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	var b strings.Builder
	for _, para := range blankLines.Split(text, -1) {
		b.WriteString("<p>")
		b.WriteString(strings.ReplaceAll(escapeHTML(para), "\n", "<br>"))
		b.WriteString("</p>")
	}
	return b.String()
}

// normalizeChatText uses \n line endings and drops trailing whitespace.
func normalizeChatText(text string) string {
	return strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), " \t\r\n")
}

// chatReplyEmpty reports whether a reply has neither text nor attachments.
func chatReplyEmpty(r ChatReply) bool {
	return strings.TrimSpace(r.Text) == "" && len(r.Attachments) == 0
}

// chatDraftText recovers the typed text: the first n bytes of the draft's
// text body. A body that no longer fits (edited elsewhere) is returned whole.
func chatDraftText(body string, n int) string {
	if n < 0 || n > len(body) || !utf8.ValidString(body[:n]) {
		return body
	}
	return body[:n]
}

// identityByEmail returns the identity with the given address, or nil.
func identityByEmail(identities []*account.Identity, addr string) *account.Identity {
	for _, id := range identities {
		if strings.EqualFold(strings.TrimSpace(id.Email), strings.TrimSpace(addr)) {
			return id
		}
	}
	return nil
}
