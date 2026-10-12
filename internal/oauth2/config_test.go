package oauth2

import "testing"

// A Microsoft-only generated file must still mark the credentials compiled,
// so init() doesn't let the mailven-creds shim replace them.
func TestSetCredentialsMarksCompiled(t *testing.T) {
	vars := []*string{&GoogleClientID, &GoogleClientSecret, &MicrosoftClientID, &GoogleTestingClientID, &GoogleTestingClientSecret}
	saved := make([]string, len(vars))
	for i, v := range vars {
		saved[i] = *v
	}
	savedCompiled := compiled
	t.Cleanup(func() {
		for i, v := range vars {
			*v = saved[i]
		}
		compiled = savedCompiled
	})
	compiled = false

	setCredentials(map[string]string{"microsoft_client_id": "generated-ms"})

	if !compiled {
		t.Error("compiled = false, want true so init() skips the shim")
	}
	if MicrosoftClientID != "generated-ms" || GoogleClientID != "" {
		t.Errorf("MicrosoftClientID=%q GoogleClientID=%q, want generated-ms and empty", MicrosoftClientID, GoogleClientID)
	}
}
