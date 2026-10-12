package v1

import "time"

// ContactRecord and its field types describe host-owned contact mirror data.
type ContactRecord struct {
	ID        string `json:"id"`
	Source    string `json:"source"`               // 'local' | 'carddav'
	Kind      string `json:"kind,omitempty"`       // local: 'manual' | 'collected'; empty for carddav
	SourceRef string `json:"source_ref,omitempty"` // carddav: addressbook_id
	Fn        string `json:"fn"`                   // display name
	NGiven    string `json:"n_given,omitempty"`
	NFamily   string `json:"n_family,omitempty"`
	Org       string `json:"org,omitempty"`
	Title     string `json:"title,omitempty"`
	Note      string `json:"note,omitempty"`
	Bday      string `json:"bday,omitempty"`
	Nickname  string `json:"nickname,omitempty"`
	// Photo fields (Phase 2b.2.b.2). Flat-scalar pattern matching Org/Title/Note.
	// At most one of {PhotoData + PhotoMediaType} OR PhotoURL is populated:
	//   - PhotoData (base64) + PhotoMediaType (e.g. "image/jpeg") = inline embed
	//     (vCard PHOTO;ENCODING=b;TYPE=...). Common CardDAV shape.
	//   - PhotoURL = vCard PHOTO URL-ref (PHOTO;VALUE=URI:...). Parsed but not
	//     fetched in this phase; Avatar falls back to initials.
	// All-empty = no photo.
	PhotoData      string    `json:"photo_data,omitempty"`
	PhotoMediaType string    `json:"photo_media_type,omitempty"`
	PhotoURL       string    `json:"photo_url,omitempty"`
	VCardRaw       string    `json:"-"` // preserved for round-trip; not exposed to JSON consumers
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// Sub-table data — populated by GetRecord and (optionally) ListRecords.
	Emails     []ContactRecordEmail   `json:"emails,omitempty"`
	Phones     []ContactRecordPhone   `json:"phones,omitempty"`
	Addresses  []ContactRecordAddress `json:"addresses,omitempty"`
	URLs       []ContactRecordURL     `json:"urls,omitempty"`
	IMPPs      []ContactRecordIMPP    `json:"impps,omitempty"`
	Categories []string               `json:"categories,omitempty"`
}

type ContactRecordEmail struct {
	Email          string    `json:"email"`
	EmailType      string    `json:"email_type,omitempty"`
	IsPrimary      bool      `json:"is_primary,omitempty"`
	SendCount      int       `json:"send_count"`
	LastUsed       time.Time `json:"last_used"`
	NameOverridden bool      `json:"name_overridden,omitempty"`
}

type ContactRecordPhone struct {
	Number    string `json:"number"`
	PhoneType string `json:"phone_type,omitempty"`
	IsPrimary bool   `json:"is_primary,omitempty"`
}

type ContactRecordAddress struct {
	AddrType string `json:"addr_type,omitempty"`
	Street   string `json:"street,omitempty"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Postcode string `json:"postcode,omitempty"`
	Country  string `json:"country,omitempty"`
}

type ContactRecordURL struct {
	URL     string `json:"url"`
	URLType string `json:"url_type,omitempty"`
}

type ContactRecordIMPP struct {
	Handle   string `json:"handle"`
	IMPPType string `json:"impp_type,omitempty"`
}

type ContactRecordFilter struct {
	Source    string // 'local' | 'carddav' | '' for both
	Kind      string // for local: 'manual' | 'collected' | ''
	SourceRef string // optional: addressbook_id for carddav scope
	Query     string // optional case-insensitive fn/email substring
	Limit     int
	Offset    int
}

type ContactSourceType string

const (
	ContactSourceCardDAV   ContactSourceType = "carddav"
	ContactSourceGoogle    ContactSourceType = "google"
	ContactSourceMicrosoft ContactSourceType = "microsoft"
)

type ContactDAVSource struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         ContactSourceType `json:"type"`
	URL          string            `json:"url"`                  // CardDAV server URL (empty for OAuth sources)
	Username     string            `json:"username"`             // CardDAV username (empty for OAuth sources)
	AccountID    *string           `json:"account_id,omitempty"` // Linked email account ID (for OAuth sources using account's token)
	Enabled      bool              `json:"enabled"`
	Writable     bool              `json:"writable"`      // Phase 2b: write capability flag (opt-in per source)
	SyncInterval int               `json:"sync_interval"` // Minutes (0 = manual only)
	LastSyncedAt *time.Time        `json:"last_synced_at,omitempty"`
	LastError    string            `json:"last_error,omitempty"`
	LastErrorAt  *time.Time        `json:"last_error_at,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`

	// Addressbooks associated with this source (populated by GetSourceWithAddressbooks)
	Addressbooks []*ContactDAVAddressbook `json:"addressbooks,omitempty"`
}

type ContactDAVAddressbook struct {
	ID           string     `json:"id"`
	SourceID     string     `json:"source_id"`
	Path         string     `json:"path"`
	Name         string     `json:"name"`
	Enabled      bool       `json:"enabled"`
	SyncToken    string     `json:"sync_token,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

// ContactMatch is the host autocomplete row used by Contacts search.
type ContactMatch struct {
	Email, DisplayName, Source string
	LastUsed, CreatedAt        time.Time
}
