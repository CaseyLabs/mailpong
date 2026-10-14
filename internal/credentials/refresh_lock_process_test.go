package credentials

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/database"
)

const refreshLockHelperEnv = "MAILVEN_TEST_OAUTH_REFRESH_LOCK_HELPER"

func TestOAuthRefreshLockCrossProcess(t *testing.T) {
	if os.Getenv(refreshLockHelperEnv) == "1" {
		dbPath := os.Getenv("MAILVEN_TEST_OAUTH_REFRESH_DB")
		dataDir := os.Getenv("MAILVEN_TEST_OAUTH_REFRESH_DATA")
		childID := os.Getenv("MAILVEN_TEST_OAUTH_REFRESH_ACCOUNT")
		db, err := database.Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		store, err := NewStore(db.DB, dataDir)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = os.Stdout.WriteString("attempting\n")
		unlock, err := store.LockOAuthRefresh(childID, "microsoft-mail")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = os.Stdout.WriteString("acquired\n")
		unlock()
		return
	}

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "refresh-lock.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	accounts := account.NewStore(db)
	parent, err := accounts.Create(&account.AccountConfig{
		Name: "Parent", DisplayName: "Parent", Email: "parent@example.test", Username: "parent@example.test",
		IMAPHost: "imap.example.test", SMTPHost: "smtp.example.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	child, err := accounts.Create(&account.AccountConfig{
		Name: "Shared", DisplayName: "Shared", Email: "shared@example.test", Username: "shared@example.test",
		IMAPHost: "imap.example.test", SMTPHost: "smtp.example.test", SharedMailboxParentID: parent.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(db.DB, tmp)
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := store.LockOAuthRefresh(parent.ID, "microsoft-mail")
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			unlock()
		}
	}()

	cmd := exec.Command(os.Args[0], "-test.run=^TestOAuthRefreshLockCrossProcess$")
	cmd.Env = append(os.Environ(),
		refreshLockHelperEnv+"=1",
		"MAILVEN_TEST_OAUTH_REFRESH_DB="+dbPath,
		"MAILVEN_TEST_OAUTH_REFRESH_DATA="+tmp,
		"MAILVEN_TEST_OAUTH_REFRESH_ACCOUNT="+child.ID,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(stdout)
	firstLine, err := reader.ReadString('\n')
	if err != nil || firstLine != "attempting\n" {
		_ = cmd.Process.Kill()
		t.Fatalf("subprocess did not start lock attempt: %q, %v", firstLine, err)
	}
	acquired := make(chan string, 1)
	readErr := make(chan error, 1)
	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			readErr <- err
			return
		}
		acquired <- line
	}()
	select {
	case line := <-acquired:
		unlock()
		released = true
		_ = cmd.Wait()
		t.Fatalf("child acquired parent OAuth refresh lock before it was released: %q", line)
	case err := <-readErr:
		unlock()
		released = true
		_ = cmd.Wait()
		t.Fatalf("read child lock result: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	unlock()
	released = true
	select {
	case line := <-acquired:
		if line != "acquired\n" {
			t.Fatalf("subprocess lock result = %q, want acquired", line)
		}
	case err := <-readErr:
		t.Fatalf("read child lock result: %v", err)
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("child did not acquire OAuth refresh lock after parent released it")
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("lock subprocess: %v", err)
	}
}
