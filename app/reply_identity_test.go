package app

import (
	"testing"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/message"
)

// TestSelectReplyFromIdentity verifies the reply/forward From selection prefers
// the identity the original message was addressed to (To/Cc/Bcc), then a forwarding-header match, then the
// default identity, then the first — the fix for #325.
func TestSelectReplyFromIdentity(t *testing.T) {
	def := &account.Identity{Email: "me@x.com", IsDefault: true}
	alias := &account.Identity{Email: "alias@x.com"}
	full := []*account.Identity{def, alias}

	cases := []struct {
		name       string
		identities []*account.Identity
		msg        *message.Message
		forwarded  []string
		want       string // expected identity email; "" means nil
	}{
		{"To match wins over default", full, &message.Message{ToList: "alias@x.com"}, nil, "alias@x.com"},
		{"Cc match", full, &message.Message{ToList: "someone@else.com", CcList: "alias@x.com"}, nil, "alias@x.com"},
		{"Bcc match", full, &message.Message{BccList: "alias@x.com"}, nil, "alias@x.com"},
		{"case-insensitive and whitespace", full, &message.Message{ToList: "  ALIAS@X.COM "}, nil, "alias@x.com"},
		{"no match falls back to default", full, &message.Message{ToList: "stranger@x.com"}, nil, "me@x.com"},
		{"no default falls back to first", []*account.Identity{{Email: "a@x.com"}, {Email: "b@x.com"}}, &message.Message{ToList: "none@x.com"}, nil, "a@x.com"},
		{"forwarding header only", full, &message.Message{ToList: "list@x.com"}, []string{"alias@x.com"}, "alias@x.com"},
		{"empty To (Bcc) with forwarding header", full, &message.Message{}, []string{"alias@x.com"}, "alias@x.com"},
		{"To match beats forwarding header", full, &message.Message{ToList: "alias@x.com"}, []string{"me@x.com"}, "alias@x.com"},
		{"alias ahead of Delivered-To gateway address", full, &message.Message{ToList: "list@x.com"}, []string{"alias@x.com", "me@x.com"}, "alias@x.com"},
		{"forwarding case-insensitive", full, &message.Message{}, []string{" ALIAS@X.COM"}, "alias@x.com"},
		{"empty forwarding list falls back to default", full, &message.Message{ToList: "list@x.com"}, []string{}, "me@x.com"},
		{"nil forwarding list falls back to default", full, &message.Message{ToList: "list@x.com"}, nil, "me@x.com"},
		{"empty identities yields nil", nil, &message.Message{ToList: "alias@x.com"}, []string{"alias@x.com"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectReplyFromIdentity(tc.identities, tc.msg, func() []string { return tc.forwarded })
			gotEmail := ""
			if got != nil {
				gotEmail = got.Email
			}
			if gotEmail != tc.want {
				t.Errorf("selectReplyFromIdentity = %q, want %q", gotEmail, tc.want)
			}
		})
	}
}

// TestWithoutAddress verifies the account's own address (what Delivered-To
// carries) is dropped from forwarding matches, so it cannot beat the default.
func TestWithoutAddress(t *testing.T) {
	cases := []struct {
		name  string
		addrs []string
		drop  string
		want  []string
	}{
		{"drops account address case-insensitively", []string{"alias@x.com", "Me@X.com"}, "me@x.com", []string{"alias@x.com"}},
		{"only the account address yields empty", []string{"me@x.com"}, "me@x.com", []string{}},
		{"no match keeps all", []string{"alias@x.com"}, "me@x.com", []string{"alias@x.com"}},
		{"empty drop keeps all", []string{"alias@x.com"}, "", []string{"alias@x.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := withoutAddress(tc.addrs, tc.drop)
			if len(got) != len(tc.want) {
				t.Fatalf("withoutAddress = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("withoutAddress = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
