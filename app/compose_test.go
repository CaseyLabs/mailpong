package app

import (
	"strings"
	"testing"

	"github.com/caseylabs/mailpong/internal/cid"
)

func TestSanitizeAttachmentFilename(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"clean name passes", "image001.png", "image001.png"},
		{"path separators stripped", "../..\\evil.png", "....evil.png"},
		{"control chars stripped", "inv\r\nite.ics", "invite.ics"},
		{"empty becomes generic", "", "attachment.bin"},
		{"whitespace-only becomes generic", "  \t ", "attachment.bin"},
	}
	for _, c := range cases {
		if got := sanitizeAttachmentFilename(c.in); got != c.want {
			t.Errorf("%s: sanitizeAttachmentFilename(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}

	long := strings.Repeat("a", 300) + ".pdf"
	if got := sanitizeAttachmentFilename(long); len([]rune(got)) != 180 {
		t.Errorf("length cap: got %d runes, want 180", len([]rune(got)))
	}
}

// TestReplyReferencesCID covers the #381 guard: replies re-attach only the
// inline parts the quoted HTML embeds.
func TestReplyReferencesCID(t *testing.T) {
	tests := []struct {
		name string
		html string
		cid  string
		want bool
	}{
		{
			name: "double-quoted src reference",
			html: `<p>hi</p><img src="cid:abc123@mailer">`,
			cid:  "abc123@mailer",
			want: true,
		},
		{
			name: "single-quoted src reference",
			html: `<img src='cid:img1@x'>`,
			cid:  "img1@x",
			want: true,
		},
		{
			name: "cid absent (misclassified document, #381)",
			html: `<p>just text, no embeds</p>`,
			cid:  "aljdusoh@mailer",
			want: false,
		},
		{
			name: "empty html",
			html: "",
			cid:  "abc@x",
			want: false,
		},
		{
			name: "prefix of another cid is not referenced",
			html: `<img src="cid:part10">`,
			cid:  "part1",
			want: false,
		},
		{
			name: "whole cid next to a longer one",
			html: `<img src="cid:part10"><img src="cid:part1">`,
			cid:  "part1",
			want: true,
		},
		{
			name: "unquoted src reference",
			html: `<img src=cid:logo@x width=10>`,
			cid:  "logo@x",
			want: true,
		},
		{
			name: "unquoted self-closing src drops the slash",
			html: `<img src=cid:logo@x/>`,
			cid:  "logo@x",
			want: true,
		},
		{
			name: "quoted cid containing a slash",
			html: `<img src="cid:a/b@x">`,
			cid:  "a/b@x",
			want: true,
		},
		{
			name: "cid outside src= is not referenced",
			html: `<p>see cid:abc@x</p>`,
			cid:  "abc@x",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, got := cid.Embedded(tt.html)[tt.cid]; got != tt.want {
				t.Fatalf("cid.Embedded(%q) has %q = %v, want %v", tt.html, tt.cid, got, tt.want)
			}
		})
	}
}

func TestParseAddressList(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"message JSON", `[{"name":"Al","email":"al@example.com"}]`, []string{"al@example.com"}},
		{"smtp JSON", `[{"name":"Al","address":"al@example.com"}]`, []string{"al@example.com"}},
		{"group markers only", `[{"name":"","email":""},{"name":"","email":""}]`, nil},
		{"group markers around address", `[{"name":"","email":""},{"name":"Al","email":"al@example.com"}]`, []string{"al@example.com"}},
		{"empty JSON list", `[]`, nil},
		{"legacy comma list", "Al <al@example.com>, bo@example.com", []string{"al@example.com", "bo@example.com"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, a := range parseAddressList(c.in) {
				got = append(got, a.Address)
			}
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Errorf("parseAddressList(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
