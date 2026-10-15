package oauth2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

// MaxClientJSONSize bounds the client_secret JSON file Mailven will read.
// Google's downloaded files are well under 1 KiB.
const MaxClientJSONSize = 64 * 1024

// ErrClientJSON marks a client file Mailven cannot use. Its message is safe
// to show: it never contains file content.
var ErrClientJSON = errors.New("client file")

func clientJSONError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrClientJSON, fmt.Sprintf(format, args...))
}

// GoogleClient is a Desktop-app OAuth client read from Google's downloaded
// client_secret JSON. Its String and GoString methods redact the secret so
// it cannot reach logs through formatting.
type GoogleClient struct {
	ClientID     string
	ClientSecret string
}

func (GoogleClient) String() string   { return "GoogleClient{redacted}" }
func (GoogleClient) GoString() string { return "GoogleClient{redacted}" }

// ParseGoogleClientJSON reads the file Google's console offers as "Download
// JSON" for an OAuth client. Only the Desktop app shape (top-level
// "installed") is accepted. Errors never include file content.
func ParseGoogleClientJSON(data []byte) (GoogleClient, error) {
	if len(data) > MaxClientJSONSize {
		return GoogleClient{}, clientJSONError("the file is larger than %d KB, so it is not a Google client file", MaxClientJSONSize/1024)
	}
	if !utf8.Valid(data) {
		return GoogleClient{}, clientJSONError("the file is not valid UTF-8 text")
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return GoogleClient{}, clientJSONError("the file is not valid JSON")
	}
	if _, ok := top["installed"]; !ok {
		switch {
		case top["web"] != nil:
			return GoogleClient{}, clientJSONError("this is a Web application client; create a Desktop app client and download its JSON")
		default:
			return GoogleClient{}, clientJSONError("no \"installed\" section; download the JSON of a Desktop app client")
		}
	}

	var inst struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(top["installed"], &inst); err != nil {
		return GoogleClient{}, clientJSONError("the \"installed\" section is malformed")
	}
	if inst.ClientID == "" {
		return GoogleClient{}, clientJSONError("the client file has no client_id")
	}
	if inst.ClientSecret == "" {
		return GoogleClient{}, clientJSONError("the client file has no client_secret")
	}
	return GoogleClient{ClientID: inst.ClientID, ClientSecret: inst.ClientSecret}, nil
}
