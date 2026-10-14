package smtp

import (
	"bytes"
	"fmt"
	"mime"
	"net/mail"
	"strings"
	"testing"
)

func testWireMessage(body string) []byte {
	return []byte("From: from@example.test\r\nTo: a@example.test\r\nDate: Tue, 13 Oct 2026 10:00:00 +0000\r\nMessage-ID: <test@example.test>\r\nSubject: test\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + body)
}

func TestMessageHeaderPreflight(t *testing.T) {
	base := string(testWireMessage("body\r\n"))
	tests := []struct {
		name, message string
		gmail, fail   bool
	}{
		{"valid", base, true, false},
		{"physical boundary", strings.Replace(base, "Subject: test", "Subject: "+strings.Repeat("x", 989), 1), false, false},
		{"physical over", strings.Replace(base, "Subject: test", "Subject: "+strings.Repeat("x", 990), 1), false, true},
		{"unbroken 1100", strings.Replace(base, "Subject: test", "Subject: "+strings.Repeat("x", 1100), 1), true, true},
		{"folded subject boundary", strings.Replace(base, "Subject: test", "Subject:"+strings.Repeat(" x\r\n", 249)+" xx", 1), true, false},
		{"folded subject over", strings.Replace(base, "Subject: test", "Subject:"+strings.Repeat(" xx\r\n", 334), 1), true, true},
		{"missing date", strings.Replace(base, "Date: Tue, 13 Oct 2026 10:00:00 +0000\r\n", "", 1), false, true},
		{"bad date", strings.Replace(base, "Tue, 13 Oct 2026 10:00:00 +0000", "never", 1), false, true},
		{"duplicate from", "From: other@example.test\r\n" + base, false, true},
		{"from list", strings.Replace(base, "From: from@example.test", "From: a@example.test, b@example.test", 1), false, true},
		{"bad message id", strings.Replace(base, "<test@example.test>", "not-an-id", 1), false, true},
		{"bad threading", "References: invalid\r\n" + base, false, true},
		{"valid threading", "In-Reply-To: <old@example.test>\r\nReferences: <a@example.test> <b@example.test>\r\n" + base, false, false},
		{"bcc leak", "Bcc: private@example.test\r\n" + base, false, true},
		{"fake results", "Authentication-Results: invented\r\n" + base, false, true},
		{"broken multipart", strings.Replace(base, "text/plain; charset=utf-8", "multipart/mixed; boundary=missing", 1), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateMessage([]byte(tt.message), tt.gmail); (err != nil) != tt.fail {
				t.Fatalf("err=%v want failure %v", err, tt.fail)
			}
		})
	}
}

func TestGmailHeaderValueAndAggregateLimits(t *testing.T) {
	// Unfolding removes CRLF, retaining continuation whitespace and the
	// leading space after the colon. Test bytes independently of line length.
	field := func(name string, size int) string {
		return name + ":" + strings.Repeat("\r\n "+strings.Repeat("x", 99), size/100) + strings.Repeat("x", size%100) + "\r\n"
	}
	for _, name := range []string{"X-Test", "To", "Cc", "References"} {
		for _, size := range []int{32768, 32769} {
			t.Run(fmt.Sprintf("%s-%d", name, size), func(t *testing.T) {
				_, err := validateHeaderBlock([]byte(strings.TrimSuffix(field(name, size), "\r\n")), true)
				want := name == "X-Test" && size > 32768
				if (err != nil) != want {
					t.Fatalf("err=%v want fail=%v", err, want)
				}
			})
		}
	}
	for _, n := range []int{5000, 5001} {
		_, err := validateHeaderBlock([]byte(strings.TrimSuffix(strings.Repeat("X: a\r\n", n), "\r\n")), true)
		if (err != nil) != (n > 5000) {
			t.Fatalf("fields=%d err=%v", n, err)
		}
	}
	for _, n := range []int{512000, 512001} {
		block := strings.Repeat("X:"+strings.Repeat("a", 995)+"\r\n", 510)
		remainder := n - len(block) - 2
		block += "X:" + strings.Repeat("a", remainder-2)
		_, err := validateHeaderBlock([]byte(block), true)
		if (err != nil) != (n > 512000) {
			t.Fatalf("aggregate=%d err=%v", n, err)
		}
	}
	for _, n := range []int{500, 501} {
		_, err := validateHeaderBlock([]byte(field("Message-ID", n)[:len(field("Message-ID", n))-2]), true)
		if (err != nil) != (n > 500) {
			t.Fatalf("ID=%d err=%v", n, err)
		}
	}
}

func TestInputRejectsInjectionAndMalformedEnvelopes(t *testing.T) {
	for _, bad := range []string{"", "no-at-sign", "two@at@signs", "trailing@", "a@example.test\r\nBcc: b@example.test", "Name <a@example.test>", "a@-bad.test", "a@bad..test"} {
		if _, err := NormalizeEnvelopeAddress(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	for _, change := range []func(*ComposeMessage){
		func(m *ComposeMessage) { m.Subject = "hello\r\nBcc: private@example.test" },
		func(m *ComposeMessage) { m.From.Name = "hello\nX: bad" },
		func(m *ComposeMessage) { m.References = []string{"invalid"} },
		func(m *ComposeMessage) { m.Attachments = []Attachment{{Filename: "file\r\nX: bad"}} },
		func(m *ComposeMessage) { m.To = nil },
	} {
		m := ComposeMessage{From: Address{Address: "a@example.test"}, To: []Address{{Address: "b@example.test"}}}
		change(&m)
		if _, err := m.ToRFC822(); err == nil {
			t.Fatal("accepted injected/malformed input")
		}
	}
}

func TestSMTPPreflightCapabilitiesAndNoSubmission(t *testing.T) {
	wire := testWireMessage("body\r\n")
	tests := []struct {
		name, from string
		to         []string
		extensions []string
		wire       []byte
		fail       bool
	}{
		{"ASCII unknown size", "from@example.test", []string{"a@example.test"}, nil, wire, false},
		{"IDN without UTF8", "from@bücher.test", []string{"a@bücher.test"}, nil, wire, false},
		{"UTF8 unsupported", "日本@example.test", []string{"a@example.test"}, nil, wire, true},
		{"UTF8 supported", "日本@example.test", []string{"a@example.test"}, []string{"SMTPUTF8"}, wire, false},
		{"recipient UTF8 unsupported", "from@example.test", []string{"日本@example.test"}, nil, wire, true},
		{"empty recipients", "from@example.test", nil, nil, wire, true},
		{"malformed recipient", "from@example.test", []string{"a"}, nil, wire, true},
		{"size boundary", "from@example.test", []string{"a@example.test"}, []string{fmt.Sprintf("SIZE %d", len(wire))}, wire, false},
		{"size over", "from@example.test", []string{"a@example.test"}, []string{fmt.Sprintf("SIZE %d", len(wire)-1)}, wire, true},
		{"size no ceiling", "from@example.test", []string{"a@example.test"}, []string{"SIZE"}, wire, false},
		{"size zero", "from@example.test", []string{"a@example.test"}, []string{"SIZE 0"}, wire, false},
		{"size malformed", "from@example.test", []string{"a@example.test"}, []string{"SIZE invalid"}, wire, true},
		{"long subject", "from@example.test", []string{"a@example.test"}, nil, []byte(strings.Replace(string(wire), "Subject: test", "Subject: "+strings.Repeat("x", 1100), 1)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := startSubmissionServer(t, submissionServerConfig{extensions: tt.extensions, mailReply: "250 ok", dataReply: "354 go", finalReply: "250 ok"})
			c := submissionClient(t, s)
			result, err := c.SendMailTracked(tt.from, tt.to, tt.wire, nil)
			if (err != nil) != tt.fail {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if tt.fail {
				if result.Outcome != OutcomeNotSubmitted || result.Stage != "preflight" {
					t.Fatalf("result=%+v", result)
				}
				_ = c.Close()
				for {
					select {
					case cmd := <-s.commands:
						if strings.HasPrefix(cmd, "MAIL") || strings.HasPrefix(cmd, "RCPT") || cmd == "DATA" {
							t.Fatalf("submitted rejected input: %s", cmd)
						}
					default:
						return
					}
				}
			}
			if strings.Contains(tt.name, "IDN") {
				for {
					select {
					case cmd := <-s.commands:
						if strings.HasPrefix(cmd, "MAIL") || strings.HasPrefix(cmd, "RCPT") {
							if strings.Contains(cmd, "bücher") || !strings.Contains(cmd, "xn--bcher-kva.test") {
								t.Fatalf("unnormalized envelope: %s", cmd)
							}
						}
					default:
						return
					}
				}
			}
		})
	}
}

func TestInternationalContentAndBccOnly(t *testing.T) {
	m := ComposeMessage{From: Address{Name: "日本", Address: "from@bücher.test"}, Bcc: []Address{{Address: "hidden@example.test"}}, Subject: "こんにちは", TextBody: "世界", Attachments: []Attachment{{Filename: "a.txt", Content: []byte("attachment")}, {Filename: "inline.png", Content: []byte("inline"), Inline: true, ContentID: "inline"}}}
	wire, err := m.ToRFC822()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateMessage(wire, true); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), "hidden@example.test") || !strings.Contains(string(wire), "undisclosed-recipients:;") {
		t.Fatal("Bcc leaked or missing undisclosed group")
	}
	if !strings.Contains(string(wire), "xn--bcher-kva.test") || m.From.Address != "from@bücher.test" {
		t.Fatal("IDN normalization changed original content")
	}
	s := startSubmissionServer(t, submissionServerConfig{mailReply: "250 ok", dataReply: "354 go", finalReply: "250 ok"})
	c := submissionClient(t, s)
	if _, err := c.SendMailTracked(m.From.Address, m.AllRecipients(), wire, nil); err != nil {
		t.Fatal(err)
	}
	s2 := startSubmissionServer(t, submissionServerConfig{extensions: []string{fmt.Sprintf("SIZE %d", len(wire)-1)}})
	c2 := submissionClient(t, s2)
	if _, err := c2.SendMailTracked(m.From.Address, m.AllRecipients(), wire, nil); err == nil {
		t.Fatal("combined MIME size not checked")
	}
}

func TestJournalMessageIDNormalizesIDNDomain(t *testing.T) {
	m := ComposeMessage{From: Address{Address: "from@bücher.test"}, To: []Address{{Address: "to@example.test"}}, TextBody: "body"}
	m.MessageID = NewMessageID(m.From.Address)
	wire, err := m.ToRFC822()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(m.MessageID, "@xn--bcher-kva.test>") {
		t.Fatalf("journal Message-ID retains Unicode domain: %q", m.MessageID)
	}
	s := startSubmissionServer(t, submissionServerConfig{mailReply: "250 ok", dataReply: "354 go", finalReply: "250 ok"})
	c := submissionClient(t, s) // No SMTPUTF8 advertisement.
	if _, err := c.SendMailTracked(m.From.Address, m.AllRecipients(), wire, nil); err != nil {
		t.Fatal(err)
	}
	if m.From.Address != "from@bücher.test" {
		t.Fatal("selected From was changed")
	}
}

func TestSubjectFoldingPreservesContentAndValueBoundary(t *testing.T) {
	for _, size := range []int{997, 998} {
		subject := strings.Repeat("word ", size/5) + strings.Repeat("x", size%5)
		m := ComposeMessage{From: Address{Address: "a@example.test"}, To: []Address{{Address: "b@example.test"}}, Subject: subject}
		wire, err := m.ToRFC822()
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateMessage(wire, true); (err != nil) != (size == 998) {
			t.Fatalf("subject bytes=%d (+ leading space), err=%v", size, err)
		}
		if m.Subject != subject {
			t.Fatal("subject changed")
		}
	}
	m := ComposeMessage{From: Address{Name: "日本語", Address: "a@example.test"}, To: []Address{{Address: "b@example.test"}}, Subject: strings.Repeat("こんにちは ", 10), TextBody: "世界"}
	wire, err := m.ToRFC822()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateMessage(wire, false); err != nil {
		t.Fatal(err)
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(wire))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil || decoded != m.Subject {
		t.Fatalf("subject lost: %q %v", decoded, err)
	}
}

func TestWireSizeCanonicalization(t *testing.T) {
	for _, tt := range []struct {
		msg  string
		want int
	}{{"body", 6}, {"body\n", 6}, {"body\r\n", 6}, {"a\nb\n", 6}, {".\r\n", 3}} {
		if got := messageWireSize([]byte(tt.msg)); got != tt.want {
			t.Errorf("wire size %q=%d want%d", tt.msg, got, tt.want)
		}
	}
	if normalized, err := NormalizeEnvelopeAddress(`"a b"@bücher.test`); err != nil || normalized != `"a b"@xn--bcher-kva.test` {
		t.Fatalf("quoted local normalization=%q err=%v", normalized, err)
	}
}

func TestMessageIDIsNotAnEnvelopeMailbox(t *testing.T) {
	wire := testWireMessage("body\r\n")
	wire = bytes.Replace(wire, []byte("<test@example.test>"), []byte("<"+strings.Repeat("x", 480)+"@example.test>"), 1)
	if err := ValidateMessage(wire, true); err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeEnvelopeAddress(strings.Repeat("x", 65) + "@example.test"); err == nil {
		t.Fatal("accepted overlong envelope local part")
	}
}
