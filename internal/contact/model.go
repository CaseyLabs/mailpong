// Package contact provides contact management for email autocomplete
package contact

import (
	coreapi "github.com/caseylabs/mailven/internal/core/api/v1"
	"time"
)

// Contact represents a contact for email autocomplete. One *Contact corresponds
// to a (record, email) pair in the unified schema — autocomplete is per-email.
// Multi-field data (phones/addresses/etc.) is exposed via Record.
type Contact struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	// Legacy identifier from Aerion; persisted, do not rename.
	Source    string    `json:"source"`         // legacy: "aerion", "carddav", "vcard"; new unified mapping: "local"→"aerion", "carddav"→"carddav"
	Kind      string    `json:"kind,omitempty"` // "manual" | "collected" — only set on local contacts
	AvatarURL string    `json:"avatar_url,omitempty"`
	SendCount int       `json:"send_count"`
	LastUsed  time.Time `json:"last_used"`
	CreatedAt time.Time `json:"created_at"`
}

// LocalContact is a thin shape historically used in a couple of places. Kept
// for back-compat. Use Record for new multi-field needs.
type LocalContact struct {
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	SendCount   int       `json:"send_count"`
	LastUsed    time.Time `json:"last_used"`
	CreatedAt   time.Time `json:"created_at"`
}

// ContactSource describes a contact source (mostly used by older code paths).
type ContactSource struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // "aerion", "google", "vcard"
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Path    string `json:"path,omitempty"`
}

// SearchResult represents a contact search result with ranking info.
type SearchResult struct {
	Contact
	Score float64 `json:"score"`
}

// Record is the rich, multi-field contact_records row + its sub-table data.
// Returned by GetRecord/ListRecords, consumed by the Contacts extension.
type Record = coreapi.ContactRecord

// ContactPhoto is a minimal inline-photo payload for a single email, used to
// render message-list avatars. Returned as a slice (not a map) because Wails
// codegen reliably emits slice-element structs but not map-value structs. Only
// contacts with an inline photo (PhotoData + PhotoMediaType) appear; URL-ref-only
// contacts are omitted.
type ContactPhoto struct {
	Email     string `json:"email"`
	Data      string `json:"data"`
	MediaType string `json:"mediaType"`
}

// RecordEmail is a single email belonging to a Record. Carries the per-email
// autocomplete metadata that lived on the legacy `contacts` table.
type RecordEmail = coreapi.ContactRecordEmail

// RecordPhone is a phone number belonging to a Record.
type RecordPhone = coreapi.ContactRecordPhone

// RecordAddress is a structured address belonging to a Record.
type RecordAddress = coreapi.ContactRecordAddress

// RecordURL is a URL belonging to a Record.
type RecordURL = coreapi.ContactRecordURL

// RecordIMPP is an instant-messaging handle belonging to a Record (vCard IMPP).
type RecordIMPP = coreapi.ContactRecordIMPP

// RecordFilter parameterizes ListRecords queries.
type RecordFilter = coreapi.ContactRecordFilter
