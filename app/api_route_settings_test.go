package app

import (
	"strings"
	"testing"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/folder"
)

// editConfig is the Edit Account form for the rig's account with a starred
// mapping and the given sync period.
func (r *routeRig) editConfig(starred string, days int) account.AccountConfig {
	return account.AccountConfig{
		Name: r.acc.Name, DisplayName: "User", Email: r.acc.Email,
		IMAPHost: "imap.example.test", SMTPHost: "smtp.example.test",
		AuthType: r.acc.AuthType, Username: r.acc.Email,
		SyncPeriodDays: days, StarredFolderPath: starred,
	}
}

// MA8: Edit Account on an active API route checks mappings against the hidden
// IMAP rows, keeps the stored IMAP paths, and offers IMAP folders to map.
func TestUpdateAccountOnActiveAPIRouteKeepsIMAPMappings(t *testing.T) {
	r := newRouteRig(t)
	a := r.app
	starred := &folder.Folder{AccountID: r.acc.ID, Name: "Starred", Path: "[Gmail]/Starred", Type: folder.TypeStarred}
	if err := a.folderStore.Create(starred); err != nil {
		t.Fatal(err)
	}
	if _, err := a.UpdateAccount(r.acc.ID, r.editConfig("[Gmail]/Starred", 30)); err != nil {
		t.Fatalf("mapping on IMAP: %v", err)
	}
	if err := a.ActivateMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}

	acc, err := a.UpdateAccount(r.acc.ID, r.editConfig("[Gmail]/Starred", 7))
	if err != nil {
		t.Fatalf("saving settings on the API route: %v", err)
	}
	if acc.SyncPeriodDays != 7 || acc.StarredFolderPath != "[Gmail]/Starred" {
		t.Fatalf("saved period %d, starred %q", acc.SyncPeriodDays, acc.StarredFolderPath)
	}
	if _, err := a.UpdateAccount(r.acc.ID, r.editConfig("INBOX-api-only", 7)); err == nil || !strings.Contains(err.Error(), "starred folder not found") {
		t.Fatalf("unknown path = %v; want rejected", err)
	}

	mapping, err := a.GetAccountFoldersForMapping(r.acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]folder.Type{}
	for _, f := range mapping {
		paths[f.Path] = f.Type
	}
	if len(paths) != 2 || paths["INBOX"] != folder.TypeInbox || paths["[Gmail]/Starred"] != folder.TypeStarred {
		t.Fatalf("mapping folders = %v; want the IMAP rows with their original paths and types", paths)
	}
	detected, err := a.GetAutoDetectedFolders(r.acc.ID)
	if err != nil || detected["starred"] != "[Gmail]/Starred" {
		t.Fatalf("auto-detected = %v, %v", detected, err)
	}

	// Back on IMAP the same mapping still resolves.
	if err := a.DeactivateMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.UpdateAccount(r.acc.ID, r.editConfig("[Gmail]/Starred", 7)); err != nil {
		t.Fatalf("mapping after deactivation: %v", err)
	}
}
