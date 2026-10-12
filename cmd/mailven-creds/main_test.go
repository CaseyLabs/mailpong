package main

import "testing"

func TestCredentialVarsDeclared(t *testing.T) {
	// Verify the credential variables are declared and are string type.
	// They will be empty strings by default since no ldflags are set during testing.
	var _ string = GoogleClientID
	var _ string = GoogleClientSecret
	var _ string = MicrosoftClientID

	// Verify they are empty when built without ldflags
	if GoogleClientID != "" {
		t.Errorf("GoogleClientID = %q, want empty (no ldflags)", GoogleClientID)
	}
	if GoogleClientSecret != "" {
		t.Errorf("GoogleClientSecret = %q, want empty (no ldflags)", GoogleClientSecret)
	}
	if MicrosoftClientID != "" {
		t.Errorf("MicrosoftClientID = %q, want empty (no ldflags)", MicrosoftClientID)
	}
}
