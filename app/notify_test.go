package app

import (
	"testing"

	"github.com/caseylabs/mailven/internal/message"
)

func TestPickNotifyMail(t *testing.T) {
	news := message.NewMail{Subject: "sale", IsLow: true}
	alice := message.NewMail{Subject: "hi", FromEmail: "alice@example.com"}
	bob := message.NewMail{Subject: "lunch", FromEmail: "bob@example.com"}

	tests := []struct {
		name         string
		newest       []message.NewMail
		count        int
		onlyPriority bool
		wantCount    int
		wantSubject  string
	}{
		{"all low, filtered", []message.NewMail{news, news}, 2, true, 0, ""},
		{"all low, unfiltered", []message.NewMail{news, news}, 2, false, 2, "sale"},
		{"mixed, newest low", []message.NewMail{news, alice, bob}, 3, true, 2, "hi"},
		{"single priority", []message.NewMail{bob}, 1, true, 1, "lunch"},
		{"rows missing, unfiltered", nil, 4, false, 4, ""},
		{"rows missing, filtered", nil, 4, true, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, latest := pickNotifyMail(tt.newest, tt.count, tt.onlyPriority)
			subject := ""
			if latest != nil {
				subject = latest.Subject
			}
			if n != tt.wantCount || subject != tt.wantSubject {
				t.Errorf("pickNotifyMail = (%d, %q), want (%d, %q)", n, subject, tt.wantCount, tt.wantSubject)
			}
		})
	}
}
