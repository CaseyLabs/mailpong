package credentials

import "testing"

func TestServiceName(t *testing.T) {
	if serviceName != "mailven" {
		t.Errorf("serviceName = %q, want %q", serviceName, "mailven")
	}
}
