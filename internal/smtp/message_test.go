package smtp

import (
	"mime"
	"strings"
	"testing"
)

func TestAddressString_WithName(t *testing.T) {
	addr := Address{Name: "John Doe", Address: "john@example.com"}
	result := addr.String()

	if !strings.Contains(result, "John Doe") {
		t.Errorf("expected result to contain 'John Doe', got %q", result)
	}
	if !strings.Contains(result, "john@example.com") {
		t.Errorf("expected result to contain 'john@example.com', got %q", result)
	}
}

func TestAddressString_WithoutName(t *testing.T) {
	addr := Address{Address: "john@example.com"}
	result := addr.String()

	if result != "john@example.com" {
		t.Errorf("expected 'john@example.com', got %q", result)
	}
}

func TestAddressString_CommaName(t *testing.T) {
	// "Last, First" display names must be quoted — unquoted, the comma
	// splits the header into two addresses and sending fails (#398)
	addr := Address{Name: "Thomas, Annette", Address: "anna@example.com"}
	result := addr.String()

	expected := `"Thomas, Annette" <anna@example.com>`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestAddressString_QuoteInName(t *testing.T) {
	addr := Address{Name: `An "odd" name`, Address: "odd@example.com"}
	result := addr.String()

	expected := `"An \"odd\" name" <odd@example.com>`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestAddressString_HeaderInjection(t *testing.T) {
	// A display name carrying CR/LF must never reach the header block raw —
	// that would let a crafted sender name inject arbitrary headers into a
	// reply. net/mail forces non-printable characters into an encoded-word.
	addr := Address{Name: "x\r\nBcc: evil@example.com", Address: "victim@example.com"}
	result := addr.String()

	if strings.ContainsAny(result, "\r\n") {
		t.Errorf("serialized address contains raw CR/LF (header injection): %q", result)
	}
	if !strings.Contains(result, "victim@example.com") {
		t.Errorf("expected result to contain the address, got %q", result)
	}
}

func TestAddressString_Unicode(t *testing.T) {
	addr := Address{Name: "\u65e5\u672c\u8a9e", Address: "test@example.com"}
	result := addr.String()

	if !strings.Contains(result, "test@example.com") {
		t.Errorf("expected result to contain 'test@example.com', got %q", result)
	}
	// The name should be Q-encoded for non-ASCII characters
	if !strings.Contains(result, "=?utf-8?") {
		t.Errorf("expected result to contain encoded name, got %q", result)
	}
}

func TestAllRecipients(t *testing.T) {
	msg := &ComposeMessage{
		To:  []Address{{Address: "to1@example.com"}, {Address: "to2@example.com"}},
		Cc:  []Address{{Address: "cc1@example.com"}},
		Bcc: []Address{{Address: "bcc1@example.com"}},
	}

	recipients := msg.AllRecipients()
	if len(recipients) != 4 {
		t.Fatalf("AllRecipients() returned %d recipients, want 4", len(recipients))
	}

	expected := []string{"to1@example.com", "to2@example.com", "cc1@example.com", "bcc1@example.com"}
	for i, want := range expected {
		if recipients[i] != want {
			t.Errorf("AllRecipients()[%d] = %q, want %q", i, recipients[i], want)
		}
	}
}

func TestAllRecipients_Empty(t *testing.T) {
	msg := &ComposeMessage{}
	recipients := msg.AllRecipients()

	if recipients != nil {
		t.Errorf("AllRecipients() = %v, want nil", recipients)
	}
}

func TestToRFC822_Basic(t *testing.T) {
	msg := &ComposeMessage{
		From:     Address{Name: "Sender", Address: "sender@example.com"},
		To:       []Address{{Name: "Recipient", Address: "recipient@example.com"}},
		Subject:  "Test Subject",
		TextBody: "Hello, this is a test.",
	}

	data, err := msg.ToRFC822()
	if err != nil {
		t.Fatalf("ToRFC822() returned error: %v", err)
	}

	output := string(data)

	checks := []struct {
		label    string
		contains string
	}{
		{"From header", "From:"},
		{"To header", "To:"},
		{"Subject header", "Subject:"},
		{"MIME-Version", "MIME-Version: 1.0"},
		{"text body", "Hello, this is a test"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.contains) {
			t.Errorf("expected RFC822 output to contain %s (%q), but it was missing", check.label, check.contains)
		}
	}
}

// TestToRFC822_AttachmentsWithoutBody verifies that attachments are serialized
// even when the message has no body, including inline attachments that have
// no HTML part to reference them.
func TestToRFC822_AttachmentsWithoutBody(t *testing.T) {
	msg := &ComposeMessage{
		From:    Address{Address: "sender@example.com"},
		To:      []Address{{Address: "recipient@example.com"}},
		Subject: "Files",
		Attachments: []Attachment{
			{Filename: "report.pdf", ContentType: "application/pdf", Content: []byte("pdf-data")},
			{Filename: "image.png", ContentType: "image/png", Content: []byte("png-data"), Inline: true, ContentID: "img1"},
		},
	}

	data, err := msg.ToRFC822()
	if err != nil {
		t.Fatalf("ToRFC822() returned error: %v", err)
	}
	output := string(data)

	for _, want := range []string{
		"multipart/mixed",
		`filename=report.pdf`,
		`filename=image.png`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestToRFC822_MessageIDDomain(t *testing.T) {
	tests := []struct {
		from, want string
	}{
		{"sender@example.com", "@example.com>"},
	}
	for _, tt := range tests {
		msg := &ComposeMessage{From: Address{Address: tt.from}, To: []Address{{Address: "to@example.test"}}, TextBody: "x"}
		data, err := msg.ToRFC822()
		if err != nil {
			t.Fatalf("ToRFC822(%q) returned error: %v", tt.from, err)
		}
		var id string
		for _, line := range strings.Split(string(data), "\r\n") {
			if v, ok := strings.CutPrefix(line, "Message-ID: "); ok {
				id = v
				break
			}
		}
		if !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, tt.want) {
			t.Errorf("from %q: Message-ID = %q, want suffix %q", tt.from, id, tt.want)
		}
	}
}

func TestFoldHeader(t *testing.T) {
	refs := strings.Repeat("<0123456789abcdef0123456789abcdef@mail.example.com> ", 30)
	tests := []struct {
		name        string
		field       string
		value       string
		unbreakable bool
	}{
		{"short", "Subject", "Hello", false},
		{"empty", "To", "", false},
		{"long references", "References", strings.TrimSpace(refs), false},
		{"double spaces", "Subject", strings.Repeat("word  ", 30), false},
		{"unbreakable", "X-Long", strings.Repeat("x", 120), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := foldHeader(tt.field, tt.value)
			lines := strings.Split(got, "\r\n")
			for i, line := range lines {
				if len(line) > maxHeaderLineLen && !tt.unbreakable {
					t.Errorf("line %d is %d chars and could have been folded: %q", i, len(line), line)
				}
				if i > 0 && strings.TrimSpace(line) == "" {
					t.Errorf("line %d is whitespace-only", i)
				}
			}
			if unfolded := strings.ReplaceAll(got, "\r\n", ""); unfolded != tt.field+": "+tt.value {
				t.Errorf("unfolding changed the value:\n got %q\nwant %q", unfolded, tt.field+": "+tt.value)
			}
		})
	}
}

func TestToRFC822_BccOnlyHasUndisclosedTo(t *testing.T) {
	msg := &ComposeMessage{
		From:     Address{Address: "me@example.com"},
		Bcc:      []Address{{Address: "hidden@example.com"}},
		Subject:  "Hi",
		TextBody: "Hello",
	}
	raw, err := msg.ToRFC822()
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "To: undisclosed-recipients:;\r\n") {
		t.Errorf("expected undisclosed-recipients To header, got:\n%s", s)
	}
	if strings.Contains(s, "hidden@example.com") {
		t.Error("Bcc address leaked into headers")
	}
}

func TestToRFC822_HeaderLinesWithinLimit(t *testing.T) {
	var refs []string
	for i := 0; i < 40; i++ {
		refs = append(refs, "<"+strings.Repeat("a", 40)+"@mail.example.com>")
	}
	var to []Address
	for i := 0; i < 30; i++ {
		to = append(to, Address{Name: "Recipient Name", Address: "recipient@example.com"})
	}
	msg := &ComposeMessage{
		From:       Address{Address: "me@example.com"},
		To:         to,
		Subject:    strings.Repeat("A long subject line ", 10) + "with ünïcödé",
		TextBody:   "Hello",
		References: refs,
	}
	raw, err := msg.ToRFC822()
	if err != nil {
		t.Fatal(err)
	}
	// Encoded-words may reach 75 chars on their own, so check that every line
	// is far below the 998-octet hard limit rather than the 78-char target.
	header, _, _ := strings.Cut(string(raw), "\r\n\r\n")
	for _, line := range strings.Split(header, "\r\n") {
		if len(line) > 100 {
			t.Errorf("header line is %d chars: %q", len(line), line)
		}
	}
}

func TestToRFC822_AttachmentFilenameEncoding(t *testing.T) {
	tests := []struct {
		name            string
		filename        string
		wantDisposition string
		wantType        string
	}{
		{"ascii", "report.pdf", `attachment; filename=report.pdf`, `application/pdf; name=report.pdf`},
		{"spaces", "Q3 report.pdf", `attachment; filename="Q3 report.pdf"`, `application/pdf; name="Q3 report.pdf"`},
		{"non-ascii", "résumé.pdf", `attachment; filename*=utf-8''r%C3%A9sum%C3%A9.pdf`, `application/pdf; name="=?utf-8?b?csOpc3Vtw6kucGRm?="`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &ComposeMessage{
				From:        Address{Address: "me@example.com"},
				To:          []Address{{Address: "you@example.com"}},
				TextBody:    "See attached",
				Attachments: []Attachment{{Filename: tt.filename, ContentType: "application/pdf", Content: []byte("x")}},
			}
			raw, err := msg.ToRFC822()
			if err != nil {
				t.Fatal(err)
			}
			s := string(raw)
			if !strings.Contains(s, "Content-Disposition: "+tt.wantDisposition+"\r\n") {
				t.Errorf("missing disposition %q in:\n%s", tt.wantDisposition, s)
			}
			if !strings.Contains(s, "Content-Type: "+tt.wantType+"\r\n") {
				t.Errorf("missing content type %q in:\n%s", tt.wantType, s)
			}
			for i := 0; i < len(s); i++ {
				if s[i] > 127 {
					t.Fatalf("raw 8-bit byte in message at offset %d", i)
				}
			}
		})
	}
}

func TestToRFC822_LongNonASCIIFilename(t *testing.T) {
	filename := strings.Repeat("報告書", 60) + ".pdf"
	msg := &ComposeMessage{
		From:        Address{Address: "me@example.com"},
		To:          []Address{{Address: "you@example.com"}},
		TextBody:    "See attached",
		Attachments: []Attachment{{Filename: filename, ContentType: "application/pdf; name=old.pdf", Content: []byte("x")}},
	}
	raw, err := msg.ToRFC822()
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, line := range strings.Split(s, "\r\n") {
		if len(line) > 100 {
			t.Errorf("line is %d chars: %q", len(line), line)
		}
	}
	// Unfold the part headers and check that both parse back to the name.
	unfolded := strings.ReplaceAll(s, "\r\n ", " ")
	// The attachment is the last part, so take each header's last occurrence.
	header := func(name string) string {
		i := strings.LastIndex(unfolded, "\r\n"+name+": ")
		if i < 0 {
			t.Fatalf("missing %s in:\n%s", name, s)
		}
		v, _, _ := strings.Cut(unfolded[i+len(name)+4:], "\r\n")
		return v
	}
	_, params, err := mime.ParseMediaType(header("Content-Disposition"))
	if err != nil || params["filename"] != filename {
		t.Errorf("disposition filename = %q, err %v", params["filename"], err)
	}
	ct := header("Content-Type")
	if strings.Count(ct, "name=") != 1 {
		t.Errorf("want one name parameter, got %q", ct)
	}
	_, params, err = mime.ParseMediaType(ct)
	if err != nil {
		t.Fatalf("content type %q: %v", ct, err)
	}
	if got, _ := new(mime.WordDecoder).DecodeHeader(params["name"]); got != filename {
		t.Errorf("content type name = %q", got)
	}
}
