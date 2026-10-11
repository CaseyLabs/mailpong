package app

import (
	"testing"

	"github.com/caseylabs/mailpong/internal/message"
)

func TestAttachChatText(t *testing.T) {
	tests := []struct {
		name     string
		msg      *message.Message
		wantChat bool
	}{
		{"nil message", nil, false},
		{"body not fetched", &message.Message{BodyText: "hi"}, false},
		{"plain fetched", &message.Message{BodyFetched: true, BodyText: "hi"}, true},
		{"smime skipped", &message.Message{BodyFetched: true, BodyText: "MIME-envelope", HasSMIME: true}, false},
		{"pgp skipped", &message.Message{BodyFetched: true, BodyText: "-----BEGIN PGP MESSAGE-----", HasPGP: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attachChatText(tt.msg)
			if tt.msg == nil {
				return
			}
			if got := tt.msg.Chat != nil; got != tt.wantChat {
				t.Fatalf("Chat set = %v, want %v", got, tt.wantChat)
			}
			if tt.wantChat && tt.msg.Chat.Text != "hi" {
				t.Errorf("Chat.Text = %q, want %q", tt.msg.Chat.Text, "hi")
			}
		})
	}
}
