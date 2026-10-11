package email

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/caseylabs/mailpong/internal/message"
)

func TestSafeFilename(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"report.pdf", "report.pdf"},
		{"../../.bashrc", ".bashrc"},
		{"/etc/passwd", "passwd"},
		{`..\..\evil.exe`, "evil.exe"},
		{"dir/", "dir"},
		{"", "attachment"},
		{".", "attachment"},
		{"..", "attachment"},
		{"/", "attachment"},
		{"a/..", "attachment"},
	}
	for _, tt := range tests {
		if got := SafeFilename(tt.in); got != tt.want {
			t.Errorf("SafeFilename(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	if got := UniquePath(dir, "a.txt"); got != filepath.Join(dir, "a.txt") {
		t.Fatalf("free name: got %q", got)
	}
	for _, n := range []string{"a.txt", "a_1.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if got := UniquePath(dir, "a.txt"); got != filepath.Join(dir, "a_2.txt") {
		t.Fatalf("taken name: got %q", got)
	}
}

func TestSaveAttachmentDefaultDir(t *testing.T) {
	tests := []struct {
		name, messageID, filename, wantRel string
	}{
		{"short id", "abc", "x.txt", "abc/x.txt"},
		{"empty id", "", "x.txt", "unknown/x.txt"},
		{"long id truncated", "0123456789", "x.txt", "01234567/x.txt"},
		{"traversal name", "0123456789", "../../x.txt", "01234567/x.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			d := NewAttachmentDownloader(dir)
			got, err := d.SaveAttachment(&message.Attachment{MessageID: tt.messageID, Filename: tt.filename}, []byte("hi"), "")
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.Join(dir, tt.wantRel); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}
