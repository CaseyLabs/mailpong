package backend

import (
	"errors"
	"testing"

	coreapi "github.com/caseylabs/mailpong/internal/core/api/v1"
)

func TestEmitConflict(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantEvent bool
	}{
		{"nil", nil, false},
		{"other error", errors.New("boom"), false},
		{"conflict", &coreapi.ErrConflict{ContactID: "r1", Message: "changed"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var events []string
			b := &ContactsBridge{deps: ContactsBridgeDeps{
				Emitter: func(name string, _ any) { events = append(events, name) },
			}}
			if got := b.emitConflict(tt.err); got != tt.err {
				t.Fatalf("emitConflict returned %v, want the original error %v", got, tt.err)
			}
			if gotEvent := len(events) == 1 && events[0] == "contacts:conflict"; gotEvent != tt.wantEvent {
				t.Fatalf("events = %v, want conflict event: %v", events, tt.wantEvent)
			}
		})
	}
}
