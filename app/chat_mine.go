package app

import (
	"strings"

	"github.com/hkdb/aerion/internal/folder"
	"github.com/hkdb/aerion/internal/logging"
	"github.com/hkdb/aerion/internal/message"
)

// chatOwner recognizes my own messages for chat bubbles: any account or
// identity address as sender, or any message in a Sent folder (covers aliases
// sent from that account).
type chatOwner struct {
	emails        map[string]struct{}
	sentFolderIDs map[string]struct{}
}

func (o chatOwner) mine(m *message.Message) bool {
	if _, ok := o.sentFolderIDs[m.FolderID]; ok {
		return true
	}
	_, ok := o.emails[strings.ToLower(strings.TrimSpace(m.FromEmail))]
	return ok
}

func (o chatOwner) add(email string) {
	if email = strings.ToLower(strings.TrimSpace(email)); email != "" {
		o.emails[email] = struct{}{}
	}
}

// loadChatOwner gathers every account's address and Sent folders. Errors
// are logged and leave the sets partial, so messages default to not mine.
func (a *App) loadChatOwner() chatOwner {
	o := a.loadChatOwnerEmails()
	ids, err := a.folderStore.ListIDsByType(folder.TypeSent)
	if err != nil {
		log := logging.WithComponent("app")
		log.Warn().Err(err).Msg("Chat ownership: failed to list Sent folders")
	}
	for _, id := range ids {
		o.sentFolderIDs[id] = struct{}{}
	}
	return o
}

// loadChatOwnerEmails returns an owner holding every account and identity address.
func (a *App) loadChatOwnerEmails() chatOwner {
	o := chatOwner{emails: map[string]struct{}{}, sentFolderIDs: map[string]struct{}{}}
	emails, err := a.messageStore.MyAddresses()
	if err != nil {
		log := logging.WithComponent("app")
		log.Warn().Err(err).Msg("Chat ownership: failed to list own addresses")
	}
	for _, e := range emails {
		o.add(e)
	}
	return o
}

// attachChatFields sets the computed chat fields (Chat, Mine) on m.
func attachChatFields(m *message.Message, o chatOwner) {
	if m == nil {
		return
	}
	attachChatText(m)
	m.Mine = o.mine(m)
}
