package credentials

import (
	"testing"

	gokeyring "github.com/zalando/go-keyring"
)

// Test binaries never touch the developer's OS keyring: every package whose
// tests reach this one gets go-keyring's in-memory mock, so test runs leave no
// entries behind and take the same credential path on every machine (TF3).
func init() {
	if testing.Testing() {
		gokeyring.MockInit()
	}
}
