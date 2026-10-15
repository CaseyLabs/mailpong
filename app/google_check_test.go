package app

import (
	"context"
	"testing"

	"github.com/caseylabs/mailven/internal/account"
)

func TestCheckGoogleSetupRejectsNonGoogleAccounts(t *testing.T) {
	for _, tt := range []struct {
		name string
		host string
		auth account.AuthType
	}{
		{"password gmail", "imap.gmail.com", account.AuthPassword},
		{"oauth other host", "outlook.office365.com", account.AuthOAuth2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ops, acc := newSendAsOps(t, tt.host, tt.auth)
			a := &App{ctx: context.Background(), accountStore: ops.accountStore}
			if got, err := a.CheckGoogleSetup(acc.ID); err == nil || got != nil {
				t.Fatalf("got %+v, %v; want an error and no probes", got, err)
			}
		})
	}
}
