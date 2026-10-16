package app

import (
	"testing"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/folder"
)

// activating reports whether the account's activation mark is set.
func (r *routeRig) activating(t *testing.T) bool {
	t.Helper()
	ids, err := r.app.accountStore.ListActivatingMailAPIRoutes()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if id == r.acc.ID {
			return true
		}
	}
	return false
}

// MA7: an activation the app quit during returns to IMAP at the next start,
// whichever step it stopped after.
func TestRecoverInterruptedAPIActivationReturnsToIMAP(t *testing.T) {
	setActive := func(t *testing.T, r *routeRig) {
		t.Helper()
		route, err := r.app.accountStore.GetMailAPIRoute(r.acc.ID)
		if err != nil {
			t.Fatal(err)
		}
		route.State = account.MailRouteStateActive
		if err := r.app.accountStore.SetMailAPIRoute(*route); err != nil {
			t.Fatal(err)
		}
	}
	swap := func(t *testing.T, r *routeRig) {
		t.Helper()
		if _, err := r.app.folderStore.SwapHidden(r.acc.ID, folder.RowsIMAP, folder.RowsAPI); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name  string
		steps []func(*testing.T, *routeRig)
	}{
		{"after the mark", nil},
		{"after going active", []func(*testing.T, *routeRig){setActive}},
		{"during the reconcile", []func(*testing.T, *routeRig){setActive, swap}},
		{"after the folders returned", []func(*testing.T, *routeRig){swap, func(t *testing.T, r *routeRig) {
			if _, err := r.app.folderStore.SwapHidden(r.acc.ID, folder.RowsAPI, folder.RowsIMAP); err != nil {
				t.Fatal(err)
			}
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRouteRig(t)
			if err := r.app.accountStore.SetMailAPIActivating(r.acc.ID, true); err != nil {
				t.Fatal(err)
			}
			for _, step := range tc.steps {
				step(t, r)
			}
			r.app.recoverInterruptedAPIActivations()
			if r.state(t) != account.MailRouteStateStaged || r.app.apiRouted(r.acc.ID) != nil {
				t.Fatal("interrupted activation left the route active")
			}
			if got := r.visibleSubjects(t); len(got) != 1 || got[0] != "imap" {
				t.Fatalf("views = %v; want the IMAP rows", got)
			}
			if r.activating(t) {
				t.Fatal("activation mark not cleared")
			}
			// Recovery is safe to repeat.
			r.app.recoverInterruptedAPIActivations()
			if got := r.visibleSubjects(t); len(got) != 1 || got[0] != "imap" {
				t.Fatalf("views after a second recovery = %v", got)
			}
		})
	}
}

// MA7: settled activations leave no mark, so startup recovery keeps a finished
// activation active.
func TestAPIActivationOutcomeClearsMark(t *testing.T) {
	r := newRouteRig(t)
	// A running IMAP sync refuses the activation after the route went active.
	r.app.syncContexts = map[string]*syncSlot{r.acc.ID + ":" + r.imapID: {}}
	if err := r.app.ActivateMailAPIRoute(r.acc.ID); err == nil {
		t.Fatal("activation during an IMAP sync succeeded")
	}
	if r.activating(t) || r.state(t) != account.MailRouteStateStaged {
		t.Fatal("refused activation left the mark or the route active")
	}
	r.app.syncContexts = nil
	if err := r.app.ActivateMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	if r.activating(t) {
		t.Fatal("successful activation left the mark")
	}
	r.app.recoverInterruptedAPIActivations()
	if r.state(t) != account.MailRouteStateActive {
		t.Fatal("recovery reverted a finished activation")
	}
	if got := r.visibleSubjects(t); len(got) != 1 || got[0] != "api" {
		t.Fatalf("views = %v; want the API rows", got)
	}
}
