package app

import "testing"

func TestContactDataRestrictedToContacts(t *testing.T) {
	for _, id := range []string{"", "calendar", "third-party"} {
		t.Run(id, func(t *testing.T) {
			core := &coreImpl{extensionID: id, app: &App{}}
			if got := core.Contacts().Data(); got != nil {
				t.Fatal("non-Contacts extension received privileged data service")
			}
		})
	}
}
