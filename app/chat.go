package app

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/caseylabs/mailven/internal/logging"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/notification"
	"github.com/caseylabs/mailven/internal/undo"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// snoozeMaxWait caps one snooze timer wait so wall-clock changes are picked
// up; system wake re-arms the timer since the monotonic clock pauses in sleep.
const snoozeMaxWait = 5 * time.Minute

// snoozeRetryWait delays the next wake-up after a failed one.
const snoozeRetryWait = time.Minute

// ============================================================================
// Chat API - Exposed to frontend via Wails bindings
// ============================================================================

// GetChats returns a page of chats. scope is a folder ID, or "" for the
// unified inbox; section is all, unread, priority, low, or snoozed.
func (a *App) GetChats(scope, section string, offset, limit int) ([]*message.Chat, error) {
	return a.messageStore.ListChats(scope, section, time.Now(), offset, limit)
}

// GetChatCount returns the number of chats in a scope and section.
func (a *App) GetChatCount(scope, section string) (int, error) {
	return a.messageStore.CountChats(scope, section, time.Now())
}

// SearchChats searches a folder like SearchConversations, adding Sent
// recipients so chat rows name the other side.
func (a *App) SearchChats(folderID, query string, offset, limit int, filter string) ([]*message.ChatSearchResult, error) {
	return a.messageStore.SearchChats(folderID, query, offset, limit, filter)
}

// SearchChatsUnifiedInbox searches all inboxes like SearchUnifiedInbox,
// merging each account's sender chat threads into one result.
func (a *App) SearchChatsUnifiedInbox(query string, offset, limit int, filter string) ([]*message.ChatSearchResult, error) {
	return a.messageStore.SearchChatsUnifiedInbox(query, offset, limit, filter)
}

// GetSenderChat returns a combined sender chat as one conversation: the
// sender's threads in folderID ("" for the account's inbox) with their Sent
// and Drafts messages. It returns nil when the chat has no messages there.
func (a *App) GetSenderChat(accountID, email, folderID string) (*message.Conversation, error) {
	conv, err := a.messageStore.GetSenderConversation(accountID, email, folderID)
	if err != nil || conv == nil {
		return conv, err
	}
	owner := a.loadChatOwner()
	for _, m := range conv.Messages {
		attachChatFields(m, owner)
	}
	return conv, nil
}

// PinChat pins or unpins a thread. Undoable.
func (a *App) PinChat(accountID, threadKey string, pinned bool) error {
	desc := "Unpin chat"
	if pinned {
		desc = "Pin chat"
	}
	return a.changeChatState(accountID, threadKey, desc, func(st *message.ChatState) {
		st.PinnedAt = nil
		if pinned {
			now := time.Now()
			st.PinnedAt = &now
		}
	})
}

// SnoozeChat hides a thread until untilUnixMs (Unix milliseconds). Undoable.
func (a *App) SnoozeChat(accountID, threadKey string, untilUnixMs int64) error {
	until := time.UnixMilli(untilUnixMs)
	if !until.After(time.Now()) {
		return fmt.Errorf("snooze time must be in the future")
	}
	return a.changeChatState(accountID, threadKey, "Snooze chat", func(st *message.ChatState) {
		now := time.Now()
		st.SnoozedUntil, st.SnoozedAt = &until, &now
	})
}

// UnsnoozeChat returns a snoozed thread to the chat list now. Undoable.
func (a *App) UnsnoozeChat(accountID, threadKey string) error {
	return a.changeChatState(accountID, threadKey, "Unsnooze chat", func(st *message.ChatState) {
		st.SnoozedUntil, st.SnoozedAt = nil, nil
	})
}

// SetSenderCategory marks a sender's mail as priority or low; "" restores
// header-based classification. Applies to existing and future mail.
func (a *App) SetSenderCategory(accountID, email, category string) error {
	if err := a.messageStore.SetSenderCategory(accountID, email, category); err != nil {
		return err
	}
	a.emitChatsChanged(accountID)
	return nil
}

// SetSenderChat combines (or splits again) the threads a sender starts into
// one sender chat, and reports whether that changed anything: a sender that
// already combines (or is already split) is left alone, with no undo entry.
// Splitting hands the sender chat's pin/snooze state to the newest inbox
// thread the sender started and its chat draft link to the thread of the
// message being replied to (see splitSenderChat); with no such thread they
// are dropped (the draft is kept). Undoable; undo returns every key the split
// touched to its state and link from before.
func (a *App) SetSenderChat(accountID, email string, combined bool) (bool, error) {
	email = message.NormalizeSenderEmail(email)
	if email == "" {
		return false, fmt.Errorf("sender email is required")
	}
	acc, err := a.accountStore.Get(accountID)
	if err != nil {
		return false, err
	}
	if acc == nil {
		return false, fmt.Errorf("account %s not found", accountID)
	}
	a.chatStateMu.Lock()
	defer a.chatStateMu.Unlock()
	previous, err := a.messageStore.IsSenderChat(accountID, email)
	if err != nil {
		return false, err
	}
	if previous == combined {
		return false, nil
	}
	choice, err := a.messageStore.SenderChatChoice(accountID, email)
	if err != nil {
		return false, err
	}
	snapshot, err := a.applySenderChat(accountID, email, &combined, choice, nil)
	if err != nil {
		return false, err
	}
	description := "Combine sender chat"
	if !combined {
		description = "Split sender chat"
	}
	a.undoStack.Push(undo.NewSenderChatCommand(senderChatRestorer{a}, accountID, email, choice, snapshot, description))
	return true, nil
}

// writeSenderChoice records a sender's combine choice; nil clears it.
func (a *App) writeSenderChoice(accountID, email string, choice *bool) error {
	if choice == nil {
		return a.messageStore.ClearSenderChat(accountID, email)
	}
	return a.messageStore.SetSenderChat(accountID, email, *choice)
}

// applySenderChat records choice (nil follows the default). If the sender
// then combines, restore (a split's snapshot, if any) is written back;
// otherwise the sender chat's state and draft link are split off (see
// splitSenderChat). It returns the snapshot of the keys it changed, taken
// before it changed them. If a write fails, that snapshot is restored and
// rollback is recorded, so the choice, state and links keep their previous
// form.
func (a *App) applySenderChat(accountID, email string, choice, rollback *bool, restore undo.SenderChatSnapshot) (undo.SenderChatSnapshot, error) {
	if err := a.writeSenderChoice(accountID, email, choice); err != nil {
		return nil, err
	}
	combined, err := a.messageStore.IsSenderChat(accountID, email)
	if err != nil {
		return nil, errors.Join(err, a.writeSenderChoice(accountID, email, rollback))
	}
	var snapshot undo.SenderChatSnapshot
	if combined {
		snapshot, err = a.restoreSenderSnapshot(accountID, restore)
	} else {
		snapshot, err = a.splitSenderChat(accountID, email)
	}
	if err != nil {
		err = errors.Join(err, a.restoreChatKeys(accountID, snapshot), a.writeSenderChoice(accountID, email, rollback))
		snapshot = nil
	}
	a.armSnoozeTimer(time.Second)
	a.emitChatsChanged(accountID)
	return snapshot, err
}

// splitSenderChat moves a split sender chat's pin/snooze state to the newest
// inbox thread the sender started, where state already there wins, and its
// chat draft link to the sender's thread that contains the message being
// replied to (the newest thread when that message is gone or in no thread the
// sender started), where a link already there wins. With no such thread they
// are dropped. It returns the state and links of the sender chat and both
// target threads from before the move, sender chat first, also when the
// move fails part-way.
func (a *App) splitSenderChat(accountID, email string) (undo.SenderChatSnapshot, error) {
	key := message.SenderChatKey(email)
	newest, err := a.messageStore.NewestInboxThreadKey(accountID, email)
	if err != nil {
		return nil, err
	}
	state, err := a.messageStore.GetChatState(accountID, key)
	if err != nil {
		return nil, err
	}
	link, err := a.draftStore.GetChatLink(accountID, key)
	if err != nil {
		return nil, err
	}
	if state == (message.ChatState{}) && link == nil {
		return undo.SenderChatSnapshot{{Key: key}}, nil // nothing to move
	}
	linkTo := newest
	if link != nil && link.MessageID != "" {
		replyKey, err := a.messageStore.StartedInboxThreadKey(accountID, email, link.MessageID)
		if err != nil {
			return nil, err
		}
		if replyKey != "" {
			linkTo = replyKey
		}
	}
	snapshot, err := a.snapshotChatKeys(accountID, key, newest, linkTo)
	if err != nil {
		return nil, err
	}
	move := message.ChatKeyMove{AccountID: accountID, From: key, To: newest}
	if err := a.messageStore.MoveChatState(move); err != nil {
		return snapshot, err
	}
	return snapshot, a.draftStore.MoveChatLink(accountID, key, linkTo)
}

// snapshotChatKeys records the pin/snooze state and chat draft link of each
// distinct non-empty key.
func (a *App) snapshotChatKeys(accountID string, keys ...string) (undo.SenderChatSnapshot, error) {
	var snapshot undo.SenderChatSnapshot
	for _, key := range keys {
		if key == "" || slices.ContainsFunc(snapshot, func(k undo.ChatKeySnapshot) bool { return k.Key == key }) {
			continue
		}
		state, err := a.messageStore.GetChatState(accountID, key)
		if err != nil {
			return nil, err
		}
		link, err := a.draftStore.GetChatLink(accountID, key)
		if err != nil {
			return nil, err
		}
		snapshot = append(snapshot, undo.ChatKeySnapshot{Key: key, State: state, Link: link})
	}
	return snapshot, nil
}

// restoreChatKeys writes each snapshot entry's state back and returns each
// snapshotted link to its key (see draft.Store.RestoreChatLink), trying every
// entry even after a failure. Links of drafts not in the snapshot stay put.
func (a *App) restoreChatKeys(accountID string, snapshot undo.SenderChatSnapshot) error {
	var errs []error
	for _, k := range snapshot {
		errs = append(errs, a.messageStore.SetChatState(accountID, k.Key, k.State))
		if k.Link != nil {
			errs = append(errs, a.draftStore.RestoreChatLink(accountID, k.Key, *k.Link))
		}
	}
	return errors.Join(errs...)
}

// restoreSenderSnapshot undoes a split from its snapshot (sender chat first):
// the sender chat gets its state back, and each target thread gives back only
// the pin and snooze the split moved onto it, keeping later changes. It
// returns the snapshot of the same keys from before, also when the restore
// fails part-way.
func (a *App) restoreSenderSnapshot(accountID string, restore undo.SenderChatSnapshot) (undo.SenderChatSnapshot, error) {
	keys := make([]string, len(restore))
	for i, k := range restore {
		keys[i] = k.Key
	}
	before, err := a.snapshotChatKeys(accountID, keys...)
	if err != nil || len(restore) == 0 {
		return before, err
	}
	moved := restore[0].State
	target := slices.Clone(restore)
	for i := 1; i < len(target); i++ {
		cur, st := before[i].State, &target[i].State
		if !sameTime(cur.PinnedAt, moved.PinnedAt) {
			st.PinnedAt = cur.PinnedAt
		}
		if !sameTime(cur.SnoozedUntil, moved.SnoozedUntil) {
			st.SnoozedUntil, st.SnoozedAt = cur.SnoozedUntil, cur.SnoozedAt
		}
	}
	return before, a.restoreChatKeys(accountID, target)
}

// sameTime reports whether a and b are both set to the same instant.
func sameTime(a, b *time.Time) bool {
	return a != nil && b != nil && a.Equal(*b)
}

// senderChatRestorer adapts App to undo.SenderChatRestorer without adding a
// frontend binding.
type senderChatRestorer struct{ a *App }

func (r senderChatRestorer) RestoreSenderChat(accountID, email string, choice *bool, snapshot undo.SenderChatSnapshot) error {
	r.a.chatStateMu.Lock()
	defer r.a.chatStateMu.Unlock()
	current, err := r.a.messageStore.SenderChatChoice(accountID, email)
	if err != nil {
		return err
	}
	_, err = r.a.applySenderChat(accountID, email, choice, current, snapshot)
	return err
}

// chatStateRestorer adapts App to undo.ChatStateRestorer without adding a
// frontend binding.
type chatStateRestorer struct{ a *App }

func (r chatStateRestorer) RestoreChatState(accountID, threadKey string, state message.ChatState) error {
	r.a.chatStateMu.Lock()
	defer r.a.chatStateMu.Unlock()
	return r.a.restoreChatState(accountID, threadKey, state)
}

// restoreChatState writes a thread's state, re-arms the snooze timer, and
// notifies the chat list.
func (a *App) restoreChatState(accountID, threadKey string, state message.ChatState) error {
	if err := a.messageStore.SetChatState(accountID, threadKey, state); err != nil {
		return err
	}
	a.armSnoozeTimer(time.Second)
	a.emitChatsChanged(accountID)
	return nil
}

// changeChatState applies change to a thread's state and pushes an undo
// command that restores the previous state.
func (a *App) changeChatState(accountID, threadKey, description string, change func(*message.ChatState)) error {
	a.chatStateMu.Lock()
	defer a.chatStateMu.Unlock()
	previous, err := a.messageStore.GetChatState(accountID, threadKey)
	if err != nil {
		return err
	}
	next := previous
	change(&next)
	if err := a.restoreChatState(accountID, threadKey, next); err != nil {
		return err
	}
	a.undoStack.Push(undo.NewChatStateCommand(chatStateRestorer{a}, accountID, threadKey, previous, description))
	return nil
}

// emitChatsChanged tells the chat list that pin, snooze, or sender
// categories changed for an account.
func (a *App) emitChatsChanged(accountID string) {
	if a.ctx == nil { // before startup, and in tests
		return
	}
	wailsRuntime.EventsEmit(a.ctx, "chats:changed", map[string]interface{}{
		"accountId": accountID,
	})
}

// ============================================================================
// Snooze wake-up
// ============================================================================

// initChatState (run off the startup path) drops state rows for threads with no messages left and arms
// the snooze timer, waking anything that came due while the app was closed.
func (a *App) initChatState() {
	defer recoverPanic("app.chat", "init chat state")
	log := logging.WithComponent("app.chat")
	a.chatStateMu.Lock()
	n, err := a.messageStore.CleanupChatState()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to clean up chat state")
	} else if n > 0 {
		log.Info().Int64("rows", n).Msg("Removed chat state for deleted threads")
	}
	changed, err := a.moveStaleChatState()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to move chat state to sender chats")
	}
	a.chatStateMu.Unlock()
	for accountID := range changed {
		a.emitChatsChanged(accountID)
	}
	a.armSnoozeTimer(time.Second)
}

// moveStaleChatState moves pins, snoozes and chat draft links whose thread now
// belongs to a sender chat into that chat, and moves them from sender chats
// whose sender no longer combines to the thread of the sender's newest inbox
// message (dropping them when there is none), so state follows the chat list
// after an upgrade or a change to the combine default. It returns
// the accounts it changed. Callers hold chatStateMu.
func (a *App) moveStaleChatState() (map[string]bool, error) {
	moves, err := a.messageStore.StaleChatKeys()
	if err != nil {
		return nil, err
	}
	changed := map[string]bool{}
	var errs []error
	for _, m := range moves {
		if err := a.messageStore.MoveChatState(m); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := a.draftStore.MoveChatLink(m.AccountID, m.From, m.To); err != nil {
			errs = append(errs, err)
		}
		changed[m.AccountID] = true
	}
	if len(moves) > 0 {
		log := logging.WithComponent("app.chat")
		log.Info().Int("keys", len(moves)).Msg("Moved chat state to match sender chats")
	}
	return changed, errors.Join(errs...)
}

// armSnoozeTimer schedules wakeSnoozes for the earliest snooze end, waiting
// at least minWait.
func (a *App) armSnoozeTimer(minWait time.Duration) {
	next, err := a.messageStore.NextSnoozeWake()
	if err != nil {
		log := logging.WithComponent("app.chat")
		log.Warn().Err(err).Msg("Failed to read next snooze")
		return
	}

	a.snoozeMu.Lock()
	defer a.snoozeMu.Unlock()
	if a.snoozeTimer != nil {
		a.snoozeTimer.Stop()
		a.snoozeTimer = nil
	}
	if next == nil || a.snoozeStopped {
		return
	}
	wait := min(max(time.Until(*next), minWait), snoozeMaxWait)
	a.snoozeTimer = time.AfterFunc(wait, a.wakeSnoozes)
}

func (a *App) stopSnoozeTimer() {
	a.snoozeMu.Lock()
	defer a.snoozeMu.Unlock()
	a.snoozeStopped = true
	if a.snoozeTimer != nil {
		a.snoozeTimer.Stop()
		a.snoozeTimer = nil
	}
}

// wakeSnoozes ends due snoozes: it clears the snooze (keeping any pin),
// marks the thread's latest inbox message unread, and notifies. Threads
// already woken by new mail were announced by the new-mail notification;
// threads now inside a sender chat have no row to wake, so their snooze is
// only cleared.
func (a *App) wakeSnoozes() {
	defer recoverPanic("app.chat", "wake snoozed chats")
	minWait := time.Second
	defer func() { a.armSnoozeTimer(minWait) }()
	log := logging.WithComponent("app.chat")

	due, err := a.messageStore.ListDueSnoozes(time.Now())
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list due snoozes")
		minWait = snoozeRetryWait
		return
	}
	for _, d := range due {
		cleared, err := a.clearDueSnooze(d)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to clear snooze")
			minWait = snoozeRetryWait
			continue
		}
		if !cleared {
			continue
		}
		if d.InSenderChat {
			continue
		}
		a.emitChatsChanged(d.AccountID)

		if d.LatestMessageID == "" || d.WokeByMail {
			continue
		}
		if err := a.setReadStatus([]string{d.LatestMessageID}, false); err != nil {
			log.Warn().Err(err).Msg("Failed to mark woken chat unread")
		}
		a.notifySnoozeWake(d)
	}
}

// clearDueSnooze clears a due snooze, keeping any pin, and reports whether it
// did. It rechecks the row under the state lock so a snooze changed since
// listing is left alone.
func (a *App) clearDueSnooze(d message.DueSnooze) (bool, error) {
	a.chatStateMu.Lock()
	defer a.chatStateMu.Unlock()
	st, err := a.messageStore.GetChatState(d.AccountID, d.ThreadKey)
	if err != nil {
		return false, err
	}
	if st.SnoozedUntil == nil || st.SnoozedUntil.After(time.Now()) {
		return false, nil
	}
	st.SnoozedUntil, st.SnoozedAt = nil, nil
	if err := a.messageStore.SetChatState(d.AccountID, d.ThreadKey, st); err != nil {
		return false, err
	}
	return true, nil
}

func (a *App) notifySnoozeWake(d message.DueSnooze) {
	if a.notifier == nil {
		return
	}
	sender := d.FromName
	if sender == "" {
		sender = d.FromEmail
	}
	_, err := a.notifier.Show(notification.Notification{
		Title: "Snoozed chat with " + sender,
		Body:  d.Subject,
		Icon:  "mail-unread",
		Data: notification.NotificationData{
			AccountID: d.AccountID,
			FolderID:  d.FolderID,
			ThreadID:  d.ThreadID,
		},
	})
	if err != nil {
		log := logging.WithComponent("app.chat")
		log.Debug().Err(err).Msg("Failed to show snooze notification")
	}
}
