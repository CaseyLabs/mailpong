package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/database"
	gokeyring "github.com/zalando/go-keyring"
)

func TestImportGoogleClientFile(t *testing.T) {
	const secret = "GOCSPX-supersecretvalue"
	// Never touch the real OS keyring: it can hold the developer's own client.
	gokeyring.MockInit()
	tmp := t.TempDir()
	db, err := database.Open(filepath.Join(tmp, "import.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = db.Migrate(); err != nil {
		t.Fatal(err)
	}
	cred, err := credentials.NewStore(db.DB, tmp)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{db: db, credStore: cred}
	write := func(name, body string) string {
		p := filepath.Join(tmp, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	stored := func(id string) bool {
		_, _, ok, err := cred.GetUserClientCreds(id)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}

	bad := map[string]string{
		"web.json":       `{"web":{"client_id":"a","client_secret":"` + secret + `"}}`,
		"malformed.json": `{"installed":`,
		"binary.json":    "{\"installed\":{\"client_id\":\"\xff\",\"client_secret\":\"" + secret + "\"}}",
		"big.json":       `{"installed":{"client_id":"a","client_secret":"` + secret + `","x":"` + strings.Repeat("a", 70000) + `"}}`,
	}
	for name, body := range bad {
		_, err := a.ImportGoogleClientFile(write(name, body), googleImportSlots)
		if err == nil {
			t.Fatalf("%s: expected an error", name)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("%s: error leaks the secret: %v", name, err)
		}
	}
	if _, err := a.ImportGoogleClientFile(filepath.Join(tmp, "missing.json"), googleImportSlots); err == nil {
		t.Fatal("missing file: expected an error")
	}
	valid := write("ok.json", `{"installed":{"client_id":"123-abcd.apps.googleusercontent.com","client_secret":"`+secret+`"}}`)
	if _, err := a.ImportGoogleClientFile(valid, []string{"microsoft-mail"}); err == nil {
		t.Fatal("non-Google slot: expected an error")
	}
	if _, err := a.ImportGoogleClientFile(valid, nil); err == nil {
		t.Fatal("no slots: expected an error")
	}
	for _, id := range googleImportSlots {
		if stored(id) {
			t.Fatalf("%s changed by a rejected import", id)
		}
	}

	res, err := a.ImportGoogleClientFile(valid, []string{"google-mail", "google-calendar"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(res.Slots, ",")+res.ClientIDFingerprint, secret) || res.ClientIDFingerprint != "….com" {
		t.Fatalf("unexpected result %+v", res)
	}
	if !stored("google-mail") || !stored("google-calendar") || stored("google-contacts") {
		t.Fatal("wrong slots saved")
	}
	id, sec, _, _ := cred.GetUserClientCreds("google-calendar")
	if id != "123-abcd.apps.googleusercontent.com" || sec != secret {
		t.Fatal("stored client differs from the file")
	}
	if c, _ := cred.GetOAuthActiveChoice("google-mail"); c != "custom" {
		t.Fatalf("active choice = %q", c)
	}
}
