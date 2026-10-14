package submission

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestSentPolicyRouteAndValidation(t *testing.T) {
	s := testStore(t, filepath.Join(t.TempDir(), "mail.db"))
	for _, mode := range []string{"provider", "append", "unresolved"} {
		p := SentPolicy{Route: "custom SMTP / parent auth / shared mailbox", Destinations: []Destination{{AccountID: "shared", FolderPath: "Custom/Sent", Mode: mode}}}
		if err := s.SetSentPolicy("shared", p); err != nil {
			t.Fatal(err)
		}
		got, err := s.SentPolicy("shared", p.Route)
		if err != nil || len(got.Destinations) != 1 || got.Destinations[0] != p.Destinations[0] {
			t.Fatalf("policy lost: %+v %v", got, err)
		}
		got, err = s.SentPolicy("shared", "different SMTP or auth")
		if err != nil || len(got.Destinations) != 0 {
			t.Fatalf("old route policy applied: %+v %v", got, err)
		}
	}
	for _, d := range [][]Destination{{{AccountID: "a", FolderPath: "Sent", Mode: "guess"}}, {{Mode: "append"}}, {{AccountID: "a", FolderPath: "Sent", Mode: "provider"}, {AccountID: "a", FolderPath: "Sent", Mode: "append"}}} {
		if err := s.SetSentPolicy("a", SentPolicy{Destinations: d}); err == nil {
			t.Fatalf("invalid policy allowed: %+v", d)
		}
	}
}
func TestCopyRestartAndDelayedDiscovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mail.db")
	s := testStore(t, path)
	// Real FK-valid disposable mailbox/folder rows for exact destination evidence.
	for _, a := range []string{"acc", "shared"} {
		_, err := s.db.Exec(`INSERT INTO accounts(id,name,email,imap_host,smtp_host,username) VALUES(?,?,?,?,?,?)`, a, a, a+"@example.test", "imap.example.test", "smtp.example.test", a)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range []string{"Sent", "Other"} {
			_, err = s.db.Exec(`INSERT INTO folders(id,account_id,name,path) VALUES(?,?,?,?)`, a+f, a, f, f)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	a, err := s.Begin("acc", "draft", testMessage())
	if err != nil {
		t.Fatal(err)
	}
	p := SentPolicy{Route: "reviewed", Destinations: []Destination{{AccountID: "acc", FolderPath: "Sent", Mode: "append"}, {AccountID: "shared", FolderPath: "Sent", Mode: "provider"}}}
	if err := s.PlanCopies(a, p); err != nil {
		t.Fatal(err)
	}
	c := Copy{AttemptID: a.ID, AccountID: "acc", FolderPath: "Sent"}
	if claimed, err := s.ClaimAppend(c); err != nil || claimed {
		t.Fatalf("append before acceptance: %v %v", claimed, err)
	}
	a.Outcome = "accepted"
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	claims := make(chan bool, 2)
	for range 2 {
		wg.Go(func() {
			claimed, err := s.ClaimAppend(c)
			if err != nil {
				t.Error(err)
			}
			claims <- claimed
		})
	}
	wg.Wait()
	close(claims)
	count := 0
	for claimed := range claims {
		if claimed {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("%d APPEND claims", count)
	}
	s = testStore(t, path)
	if claimed, err := s.ClaimAppend(c); err != nil || claimed {
		t.Fatalf("append replay after restart: %v %v", claimed, err)
	}
	got, err := s.ReconcileCopies("acc")
	if err != nil || len(got) != 2 {
		t.Fatalf("copies lost: %+v %v", got, err)
	}
	_, err = s.db.Exec(`INSERT INTO messages(id,account_id,folder_id,uid,message_id) VALUES('wrong','acc','accOther',1,?)`, a.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.ReconcileCopies("acc")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range got {
		if c.State == "observed" {
			t.Fatal("wrong folder counted")
		}
	}
	for _, dest := range []string{"acc", "shared"} {
		_, err = s.db.Exec(`INSERT INTO messages(id,account_id,folder_id,uid,message_id) VALUES(?,?,?,?,?)`, dest+"copy", dest, dest+"Sent", 2, a.MessageID)
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err = s.ReconcileCopies("acc")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range got {
		if c.State != "observed" {
			t.Fatalf("delayed copy not reconciled: %+v", c)
		}
	}
	if _, err := s.Begin("acc", "draft", testMessage()); err == nil {
		t.Fatal("Sent observation invited resubmission")
	}
}
func TestUnreviewedPolicyCannotAppend(t *testing.T) {
	s := testStore(t, filepath.Join(t.TempDir(), "mail.db"))
	a, err := s.Begin("acc", "draft", testMessage())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PlanCopies(a, SentPolicy{Route: "unknown"}); err != nil {
		t.Fatal(err)
	}
	a.Outcome = "accepted"
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
	copies, err := s.ReconcileCopies("acc")
	if err != nil || len(copies) != 1 || copies[0].Mode != "unresolved" {
		t.Fatalf("unresolved evidence missing: %+v %v", copies, err)
	}
	if claimed, err := s.ClaimAppend(copies[0]); err != nil || claimed {
		t.Fatalf("unknown provider inferred APPEND: %v %v", claimed, err)
	}
}

func TestAbruptExitSentAppendClaim(t *testing.T) {
	const env = "MAILVEN_TEST_SENT_CRASH_DB"
	if path := os.Getenv(env); path != "" {
		s := testStore(t, path)
		a, err := s.Begin("acc", "draft", testMessage())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.PlanCopies(a, SentPolicy{Destinations: []Destination{{AccountID: "acc", FolderPath: "Sent", Mode: "append"}}}); err != nil {
			t.Fatal(err)
		}
		a.Outcome = "accepted"
		if err := s.Update(a); err != nil {
			t.Fatal(err)
		}
		if claimed, err := s.ClaimAppend(Copy{AttemptID: a.ID, AccountID: "acc", FolderPath: "Sent"}); err != nil || !claimed {
			t.Fatalf("claim %v %v", claimed, err)
		}
		os.Exit(23)
	}
	path := filepath.Join(t.TempDir(), "mail.db")
	cmd := exec.Command(os.Args[0], "-test.run=^TestAbruptExitSentAppendClaim$")
	cmd.Env = append(os.Environ(), env+"="+path)
	output, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
		t.Fatalf("child did not exit at claim: %v %s", err, output)
	}
	s := testStore(t, path)
	copies, err := s.ReconcileCopies("acc")
	if err != nil || len(copies) != 1 || copies[0].State != "unknown" {
		t.Fatalf("crash claim lost: %+v %v", copies, err)
	}
	if claimed, err := s.ClaimAppend(copies[0]); err != nil || claimed {
		t.Fatalf("crashed APPEND replayed: %v %v", claimed, err)
	}
	if _, err := s.Begin("acc", "draft", testMessage()); err == nil {
		t.Fatal("crash invited resubmission")
	}
}
