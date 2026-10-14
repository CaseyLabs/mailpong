package app

import (
	"errors"
	"net/textproto"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/caseylabs/mailven/internal/submission"
)

func TestFinishAttemptKeepsAcceptedCleanupSeparate(t *testing.T) {
	for _, outcome := range []string{"accepted", "unknown", "not_submitted"} {
		t.Run(outcome, func(t *testing.T) {
			db, err := database.Open(filepath.Join(t.TempDir(), "mail.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.Migrate(); err != nil {
				t.Fatal(err)
			}
			ops := composeOps{submissions: submission.NewStore(db)}
			a, err := ops.prepareSubmission("acc", smtp.ComposeMessage{}, "draft")
			if err != nil {
				t.Fatal(err)
			}
			a.Outcome = outcome
			if err := ops.submissions.Update(a); err != nil {
				t.Fatal(err)
			}
			var sendErr error
			func() { defer ops.finishAttempt(a, &sendErr); panic("cleanup failure") }()
			got, err := ops.submissions.Get(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Outcome != outcome {
				t.Fatalf("outcome changed: %+v", got)
			}
			if outcome == "accepted" {
				if sendErr != nil || !strings.Contains(got.Warnings, "cleanup failure") {
					t.Fatalf("accepted mail invited retry: %v %+v", sendErr, got)
				}
			} else if sendErr == nil {
				t.Fatal("interrupted send not reported")
			}
			if outcome == "unknown" && !strings.Contains(sendErr.Error(), "duplicates") {
				t.Fatal("unknown omitted duplicate risk")
			}
		})
	}
}

func TestFinishAttemptClassifiesConnectionAndAuth(t *testing.T) {
	for _, tt := range []struct {
		stage string
		err   error
		want  string
	}{
		{"connect", errors.New("connection refused"), "not_submitted"},
		{"auth", &textproto.Error{Code: 454, Msg: "try later"}, "rejected_temporary"},
		{"auth", &textproto.Error{Code: 535, Msg: "bad credentials"}, "rejected_permanent"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			db, err := database.Open(filepath.Join(t.TempDir(), "mail.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.Migrate(); err != nil {
				t.Fatal(err)
			}
			ops := composeOps{submissions: submission.NewStore(db)}
			a, err := ops.prepareSubmission("acc", smtp.ComposeMessage{}, "draft")
			if err != nil {
				t.Fatal(err)
			}
			a.Stage = tt.stage
			sendErr := tt.err
			ops.finishAttempt(a, &sendErr)
			got, _ := ops.submissions.Get(a.ID)
			if got.Outcome != tt.want || got.Stage != tt.stage {
				t.Fatalf("got %+v", got)
			}
		})
	}
}
