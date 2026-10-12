package pgp

import (
	"github.com/caseylabs/mailven/internal/logging"
)

// LookupKeyResult contains the result of a unified key lookup.
type LookupKeyResult struct {
	Armored string
	Source  string // "wkd" or "hkp"
}

// LookupKey performs a cascading key lookup: WKD first, then HKP key servers.
// Returns nil if neither method found a key.
func LookupKey(email string, hkpServers []string) (*LookupKeyResult, error) {
	log := logging.WithComponent("pgp.lookup")

	// Try WKD first (more authoritative — hosted by recipient's domain)
	armored, err := LookupWKD(email)
	if err != nil {
		log.Debug().Err(err).Str("email", email).Msg("WKD lookup failed, trying HKP")
	}
	if armored != "" {
		return &LookupKeyResult{Armored: armored, Source: "wkd"}, nil
	}

	// Fall back to HKP key servers
	armored, err = LookupHKP(email, hkpServers)
	if err != nil {
		log.Debug().Err(err).Str("email", email).Msg("HKP lookup failed")
		return nil, err
	}
	if armored != "" {
		return &LookupKeyResult{Armored: armored, Source: "hkp"}, nil
	}

	return nil, nil
}
