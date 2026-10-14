//go:build !unix && !windows

package credentials

import "fmt"

func acquireOAuthFileLock(string) (func() error, error) {
	return nil, fmt.Errorf("OAuth refresh file locks are unsupported on this platform")
}
