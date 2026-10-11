package credentials

import "testing"

func TestServiceName(t *testing.T) {
	if serviceName != "mailpong" {
		t.Errorf("serviceName = %q, want %q", serviceName, "mailpong")
	}
}
