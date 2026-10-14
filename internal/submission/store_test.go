package submission

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/smtp"
)

// The child exits without running deferred DB closes or test cleanup. The
// parent proves that committed checkpoints survive an abrupt process stop.
func TestAbruptExitPreservesJournal(t *testing.T) {
	if path := os.Getenv("MAILVEN_SUBMISSION_CRASH_DB"); path != "" {
		s := testStore(t, path)
		a, err := s.Begin("acc", "draft", testMessage())
		if err != nil {
			t.Fatal(err)
		}
		a.Outcome = os.Getenv("MAILVEN_SUBMISSION_CRASH_OUTCOME")
		a.Stage = "data_body"
		if err := s.Update(a); err != nil {
			t.Fatal(err)
		}
		os.Exit(23)
	}
	for _, outcome := range []string{"unknown", "accepted"} {
		t.Run(outcome, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mail.db")
			cmd := exec.Command(os.Args[0], "-test.run=^TestAbruptExitPreservesJournal$")
			cmd.Env = append(os.Environ(), "MAILVEN_SUBMISSION_CRASH_DB="+path, "MAILVEN_SUBMISSION_CRASH_OUTCOME="+outcome)
			output, err := cmd.CombinedOutput()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
				t.Fatalf("child did not exit at checkpoint: %v %s", err, output)
			}
			s := testStore(t, path)
			if err := s.RecoverQueued(); err != nil {
				t.Fatal(err)
			}
			a, err := s.Review("acc", "draft", testMessage())
			if err != nil || a == nil || a.Outcome != outcome || a.MessageID == "" {
				t.Fatalf("checkpoint lost: %+v %v", a, err)
			}
			if _, err := s.Begin("acc", "draft", testMessage()); err == nil {
				t.Fatal("abruptly interrupted attempt replayed")
			}
		})
	}
}

func testStore(t *testing.T, path string) *Store {
	t.Helper()
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStore(db)
}
func testMessage() smtp.ComposeMessage {
	return smtp.ComposeMessage{From: smtp.Address{Address: "me@example.test"}, To: []smtp.Address{{Address: "you@example.test"}}, Subject: "hello", TextBody: "body"}
}

func TestCrashRecoveryDoesNotReplay(t *testing.T) {
	for _, outcome := range []string{"not_submitted", "unknown", "accepted", "rejected_temporary", "rejected_permanent"} {
		t.Run(outcome, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mail.db")
			s := testStore(t, path)
			msg := testMessage()
			a, err := s.Begin("acc", "draft", msg)
			if err != nil {
				t.Fatal(err)
			}
			a.Outcome = outcome
			if outcome != "not_submitted" {
				a.Stage = "data_body"
			}
			a.Recipients = `[{"address":"you@example.test","accepted":true}]`
			if err := s.Update(a); err != nil {
				t.Fatal(err)
			}
			s.db.Close()
			s = testStore(t, path)
			if err := s.RecoverQueued(); err != nil {
				t.Fatal(err)
			}
			got, err := s.Get(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.MessageID != a.MessageID || got.Outcome != outcome || got.Recipients != a.Recipients {
				t.Fatalf("attempt changed on restart: %+v", got)
			}
			rows, err := s.List("acc")
			if err != nil || len(rows) != 1 {
				t.Fatalf("recovery created an attempt: %v %v", rows, err)
			}
			review, err := s.Review("acc", "draft", msg)
			if err != nil {
				t.Fatal(err)
			}
			ambiguous := outcome == "unknown" || outcome == "accepted"
			if (review != nil) != ambiguous {
				t.Fatalf("review=%v outcome=%s", review, outcome)
			}
			if ambiguous {
				msg.Subject = "edited after restart"
				if _, err := s.Begin("acc", "draft", msg); err == nil {
					t.Fatal("edited draft bypassed review")
				}
				msg.ReviewAttemptID = a.ID
				next, err := s.Begin("acc", "draft", msg)
				if err != nil {
					t.Fatal(err)
				}
				if next.ID == a.ID || next.MessageID == a.MessageID {
					t.Fatal("explicit new submission reused identifiers")
				}
				original, _ := s.Get(a.ID)
				if original.Outcome != outcome {
					t.Fatal("previous attempt overwritten")
				}
			}
		})
	}
}

func TestAcceptancePersistenceFailureLeavesUnknown(t *testing.T) {
	s := testStore(t, filepath.Join(t.TempDir(), "mail.db"))
	msg := testMessage()
	a, err := s.Begin("acc", "draft", msg)
	if err != nil {
		t.Fatal(err)
	}
	a.Outcome = "unknown"
	a.Stage = "data_body"
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`CREATE TRIGGER reject_acceptance BEFORE UPDATE ON submission_attempts WHEN NEW.outcome='accepted' BEGIN SELECT RAISE(ABORT,'simulated disk failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	a.Outcome = "accepted"
	if err := s.Update(a); err == nil {
		t.Fatal("fixture did not reject acceptance write")
	}
	got, _ := s.Get(a.ID)
	if got.Outcome != "unknown" {
		t.Fatalf("got %s", got.Outcome)
	}
	if _, err := s.Begin("acc", "draft", msg); err == nil {
		t.Fatal("ambiguous message replay permitted")
	}
}

func TestDraftReviewFollowsAccountSelection(t *testing.T) {
	s := testStore(t, filepath.Join(t.TempDir(), "mail.db"))
	msg := testMessage()
	a, err := s.Begin("original-account", "draft", msg)
	if err != nil {
		t.Fatal(err)
	}
	changed := msg
	changed.From.Address = "another@example.test"
	if _, err := s.Begin("another-account", "draft", changed); err == nil {
		t.Fatal("pending draft replay allowed through another account")
	}
	a.Outcome = "unknown"
	a.Stage = "data_final_reply"
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
	review, err := s.Review("another-account", "draft", changed)
	if err != nil || review == nil || review.ID != a.ID {
		t.Fatalf("cross-account review lost: %+v %v", review, err)
	}
	if _, err := s.Begin("another-account", "draft", changed); err == nil {
		t.Fatal("unknown draft review bypassed by account change")
	}
	changed.ReviewAttemptID = a.ID
	if _, err := s.Begin("another-account", "draft", changed); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentWindowsAndBoundedTemporaryRetry(t *testing.T) {
	s := testStore(t, filepath.Join(t.TempDir(), "mail.db"))
	msg := testMessage()
	var wg sync.WaitGroup
	results := make(chan *Attempt, 2)
	for range 2 {
		wg.Go(func() {
			a, err := s.Begin("acc", "draft", msg)
			if err == nil {
				results <- a
			}
		})
	}
	wg.Wait()
	close(results)
	var a *Attempt
	count := 0
	for result := range results {
		a = result
		count++
	}
	if count != 1 {
		t.Fatalf("%d concurrent attempts created", count)
	}
	a.Outcome = "rejected_temporary"
	a.Stage = "data_command"
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Begin("acc", "draft", msg); err == nil || !strings.Contains(err.Error(), "5 seconds") {
		t.Fatalf("retry not bounded: %v", err)
	}
	_, err := s.db.Exec(`UPDATE submission_attempts SET updated_at=? WHERE id=?`, time.Now().Add(-6*time.Second).UnixMilli(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Begin("acc", "draft", msg); err != nil {
		t.Fatal(err)
	}
}

func TestSenderReviewDoesNotChangeDuplicateProtection(t *testing.T) {
	s := testStore(t, filepath.Join(t.TempDir(), "sender-review.db"))
	msg := testMessage()
	a, err := s.Begin("acc", "", msg)
	if err != nil {
		t.Fatal(err)
	}
	a.Outcome = "unknown"
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
	msg.SenderReviewToken = "new-sender-observation"
	review, err := s.Review("acc", "", msg)
	if err != nil || review == nil || review.ID != a.ID {
		t.Fatalf("sender review changed content identity: %+v, %v", review, err)
	}
	if _, err := s.Begin("acc", "", msg); err == nil {
		t.Fatal("sender review bypassed duplicate-risk review without draft ID")
	}
	msg.ReviewAttemptID = a.ID
	if _, err := s.Begin("acc", "", msg); err != nil {
		t.Fatal(err)
	}
}
