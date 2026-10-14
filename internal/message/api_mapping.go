package message

import (
	"sort"
	"strings"

	"github.com/caseylabs/mailven/internal/folder"
)

// API providers whose remote messages map onto local rows.
const (
	APIProviderGmail = "gmail"
	APIProviderGraph = "graph"
)

// gmailAllMailPath is the local folder for a Gmail message that carries no
// folder label (archived mail); it is the only synthetic container.
const gmailAllMailPath = "[Gmail]/All Mail"

// APIFolder is a local folder an API container maps to. RemoteID is the
// provider's container ID ("" for Gmail's synthetic All Mail).
type APIFolder struct {
	RemoteID string
	Path     string
	Name     string
	Type     folder.Type
}

// GmailLabel is one entry of labels.list.
type GmailLabel struct {
	ID   string
	Name string
	// Type is "system" or "user", as reported by the API.
	Type string
}

// APIMapping is the result of mapping one remote message's containers.
type APIMapping struct {
	// Folders holds one entry per local row the message needs.
	Folders   []APIFolder
	IsRead    bool
	IsStarred bool
	IsDraft   bool
	// Metadata keeps IMPORTANT and CATEGORY_* labels; they are not folders.
	Metadata []string
	// Unmapped keeps containers that have no mapping, never dropped or guessed.
	Unmapped []string
}

var gmailSystemFolders = map[string]APIFolder{
	"INBOX": {RemoteID: "INBOX", Path: "INBOX", Name: "INBOX", Type: folder.TypeInbox},
	"SENT":  {RemoteID: "SENT", Path: "SENT", Name: "Sent", Type: folder.TypeSent},
	"DRAFT": {RemoteID: "DRAFT", Path: "DRAFT", Name: "Drafts", Type: folder.TypeDrafts},
	"TRASH": {RemoteID: "TRASH", Path: "TRASH", Name: "Trash", Type: folder.TypeTrash},
	"SPAM":  {RemoteID: "SPAM", Path: "SPAM", Name: "Spam", Type: folder.TypeSpam},
}

// MapGmailLabels maps a Gmail message's label IDs. labels is the account's
// labels.list result: a user label is a folder only when the list names it.
// STARRED and UNREAD become flags, DRAFT also sets the draft flag, IMPORTANT
// and CATEGORY_* are metadata, and any other ID (including CHAT and labels
// absent from the list) is unmapped. A message with no folder label lives in
// All Mail.
func MapGmailLabels(labelIDs []string, labels []GmailLabel) APIMapping {
	known := make(map[string]GmailLabel, len(labels))
	for _, l := range labels {
		known[l.ID] = l
	}
	out := APIMapping{IsRead: true}
	seen := map[string]bool{}
	for _, id := range labelIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		switch {
		case id == "STARRED":
			out.IsStarred = true
		case id == "UNREAD":
			out.IsRead = false
		case id == "IMPORTANT" || strings.HasPrefix(id, "CATEGORY_"):
			out.Metadata = append(out.Metadata, id)
		default:
			if f, ok := gmailSystemFolders[id]; ok {
				out.IsDraft = out.IsDraft || id == "DRAFT"
				out.Folders = append(out.Folders, f)
				continue
			}
			if l, ok := known[id]; ok && l.Type == "user" && l.Name != "" {
				out.Folders = append(out.Folders, APIFolder{RemoteID: id, Path: l.Name, Name: l.Name, Type: folder.TypeFolder})
				continue
			}
			out.Unmapped = append(out.Unmapped, id)
		}
	}
	if len(out.Folders) == 0 {
		out.Folders = []APIFolder{{Path: gmailAllMailPath, Name: "All Mail", Type: folder.TypeAll}}
	}
	sort.Slice(out.Folders, func(i, j int) bool { return out.Folders[i].RemoteID < out.Folders[j].RemoteID })
	sort.Strings(out.Metadata)
	sort.Strings(out.Unmapped)
	return out
}

// GraphFolder is one mail folder of the mailbox. WellKnown is the well-known
// name (inbox, sentitems, ...) whose resolved ID equals ID, set by the client
// that resolved it.
type GraphFolder struct {
	ID        string
	Name      string
	ParentID  string
	WellKnown string
}

var graphWellKnown = map[string]folder.Type{
	"inbox":        folder.TypeInbox,
	"sentitems":    folder.TypeSent,
	"drafts":       folder.TypeDrafts,
	"deleteditems": folder.TypeTrash,
	"junkemail":    folder.TypeSpam,
	"archive":      folder.TypeArchive,
}

// GraphFolderMap maps Graph folder IDs to local folders. Folders whose parent
// chain does not reach the mailbox root (or loops) are listed in Unmapped.
type GraphFolderMap struct {
	Folders  map[string]APIFolder
	Unmapped []string
}

// MapGraphFolders maps folders by well-known name and parentFolderId. Paths
// join ancestor names with "/"; a folder is top level when its parent is
// rootID. A well-known name wins over the display name for the type only;
// the display name is kept so the user's labels stay recognisable.
func MapGraphFolders(rootID string, folders []GraphFolder) GraphFolderMap {
	byID := make(map[string]GraphFolder, len(folders))
	for _, f := range folders {
		byID[f.ID] = f
	}
	out := GraphFolderMap{Folders: map[string]APIFolder{}}
	for _, f := range folders {
		var parts []string
		cur, ok := f, true
		visited := map[string]bool{}
		for ok {
			if visited[cur.ID] || cur.Name == "" {
				ok = false
				break
			}
			visited[cur.ID] = true
			parts = append([]string{cur.Name}, parts...)
			if cur.ParentID == rootID {
				break
			}
			cur, ok = byID[cur.ParentID]
		}
		if !ok {
			out.Unmapped = append(out.Unmapped, f.ID)
			continue
		}
		typ, special := graphWellKnown[strings.ToLower(f.WellKnown)]
		if !special {
			typ = folder.TypeFolder
		}
		out.Folders[f.ID] = APIFolder{RemoteID: f.ID, Path: strings.Join(parts, "/"), Name: f.Name, Type: typ}
	}
	sort.Strings(out.Unmapped)
	return out
}

// MapGraphMessage maps a Graph message by its parentFolderId; flags come from
// the message itself, not the mapping. A message in a
// folder that has no mapping keeps that ID as unmapped and yields no row.
func (m GraphFolderMap) MapGraphMessage(parentFolderID string) APIMapping {
	if f, ok := m.Folders[parentFolderID]; ok {
		return APIMapping{Folders: []APIFolder{f}}
	}
	return APIMapping{Unmapped: []string{parentFolderID}}
}
