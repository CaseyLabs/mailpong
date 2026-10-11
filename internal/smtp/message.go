// Package smtp provides SMTP client functionality for MailPong
package smtp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Address represents an email address with optional display name
type Address struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// String returns the RFC 5322 formatted address. Delegates to net/mail so
// display names with specials (comma, quotes, …) are quoted/escaped and
// non-printable or non-ASCII names are RFC 2047-encoded — an unquoted
// "Last, First" name parses as TWO addresses and breaks sending (#398),
// and a raw CR/LF would be header injection.
func (a Address) String() string {
	if a.Name == "" {
		return a.Address
	}
	return (&mail.Address{Name: a.Name, Address: a.Address}).String()
}

// Attachment represents a file attachment
type Attachment struct {
	Filename      string `json:"filename"`
	ContentType   string `json:"content_type"`
	Content       []byte `json:"content"`
	ContentBase64 string `json:"content_base64,omitempty"` // Base64 string for efficient Wails RPC transfer
	ContentID     string `json:"content_id"`               // For inline attachments
	Inline        bool   `json:"inline"`
}

// ResolveContent returns the attachment's binary content.
// It prefers the Content field; if empty, decodes ContentBase64.
func (a *Attachment) ResolveContent() ([]byte, error) {
	if len(a.Content) > 0 {
		return a.Content, nil
	}
	if a.ContentBase64 != "" {
		return base64.StdEncoding.DecodeString(a.ContentBase64)
	}
	return nil, nil
}

// ComposeMessage represents an email message to be composed and sent
type ComposeMessage struct {
	// Envelope
	From    Address   `json:"from"`
	To      []Address `json:"to"`
	Cc      []Address `json:"cc"`
	Bcc     []Address `json:"bcc"`
	ReplyTo *Address  `json:"reply_to,omitempty"`
	Subject string    `json:"subject"`

	// Content
	TextBody string `json:"text_body"` // Plain text version
	HTMLBody string `json:"html_body"` // HTML version

	// Attachments
	Attachments []Attachment `json:"attachments"`

	// Headers
	InReplyTo  string   `json:"in_reply_to,omitempty"` // Message-ID of the message being replied to
	References []string `json:"references,omitempty"`  // Thread references

	// Options
	RequestReadReceipt bool `json:"request_read_receipt"`
	SignMessage        bool `json:"sign_message"`        // S/MIME sign this message
	EncryptMessage     bool `json:"encrypt_message"`     // S/MIME encrypt this message
	PGPSignMessage     bool `json:"pgp_sign_message"`    // PGP sign this message
	PGPEncryptMessage  bool `json:"pgp_encrypt_message"` // PGP encrypt this message
}

// AllRecipients returns all recipients (To + Cc + Bcc)
func (m *ComposeMessage) AllRecipients() []string {
	var recipients []string
	for _, addr := range m.To {
		recipients = append(recipients, addr.Address)
	}
	for _, addr := range m.Cc {
		recipients = append(recipients, addr.Address)
	}
	for _, addr := range m.Bcc {
		recipients = append(recipients, addr.Address)
	}
	return recipients
}

// ToRFC822 converts the message to RFC 822 format for sending
func (m *ComposeMessage) ToRFC822() ([]byte, error) {
	var buf bytes.Buffer

	// Generate Message-ID under the sender's domain; spam filters
	// penalize IDs without a real domain. The "mailpong" fallback avoids
	// "localhost", which looks like a misconfigured host to those filters.
	messageID := fmt.Sprintf("<%s@%s>", uuid.New().String(), domainFromEmail(m.From.Address, "mailpong"))

	// Write headers
	writeHeader(&buf, "From", m.From.String())
	// An empty or missing To header is a common spam-filter signal (e.g.
	// SpamAssassin MISSING_HEADERS), so Bcc-only mail names an empty group.
	to := "undisclosed-recipients:;"
	if len(m.To) > 0 {
		to = formatAddresses(m.To)
	}
	writeHeader(&buf, "To", to)
	if len(m.Cc) > 0 {
		writeHeader(&buf, "Cc", formatAddresses(m.Cc))
	}
	// Note: BCC is not written to headers (handled by SMTP)
	if m.ReplyTo != nil {
		writeHeader(&buf, "Reply-To", m.ReplyTo.String())
	}
	writeHeader(&buf, "Subject", encodeSubject(m.Subject))
	writeHeader(&buf, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&buf, "Message-ID", messageID)
	writeHeader(&buf, "MIME-Version", "1.0")
	writeHeader(&buf, "User-Agent", "MailPong")

	// Threading headers
	if m.InReplyTo != "" {
		writeHeader(&buf, "In-Reply-To", m.InReplyTo)
	}
	if len(m.References) > 0 {
		writeHeader(&buf, "References", strings.Join(m.References, " "))
	}

	// Read receipt
	if m.RequestReadReceipt {
		writeHeader(&buf, "Disposition-Notification-To", m.From.String())
	}

	// Determine message structure
	hasHTML := m.HTMLBody != ""
	hasText := m.TextBody != ""
	hasAttachments := len(m.Attachments) > 0

	// Separate inline and regular attachments
	var inlineAttachments, regularAttachments []Attachment
	for _, att := range m.Attachments {
		if att.Inline {
			inlineAttachments = append(inlineAttachments, att)
		} else {
			regularAttachments = append(regularAttachments, att)
		}
	}

	// Choose message structure based on content
	switch {
	case hasAttachments:
		// multipart/mixed with the body (if any) followed by attachments
		if err := writeMultipartMixed(&buf, m, regularAttachments, inlineAttachments); err != nil {
			return nil, err
		}
	case hasHTML && hasText:
		// multipart/alternative (HTML + plain text)
		if err := writeMultipartAlternative(&buf, m.TextBody, m.HTMLBody); err != nil {
			return nil, err
		}
	case hasHTML:
		// HTML only
		writeHeader(&buf, "Content-Type", "text/html; charset=utf-8")
		writeHeader(&buf, "Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		writeQuotedPrintable(&buf, m.HTMLBody)
	case hasText:
		// Plain text only
		writeHeader(&buf, "Content-Type", "text/plain; charset=utf-8")
		writeHeader(&buf, "Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		writeQuotedPrintable(&buf, m.TextBody)
	default:
		// Empty message
		writeHeader(&buf, "Content-Type", "text/plain; charset=utf-8")
		buf.WriteString("\r\n")
	}

	return buf.Bytes(), nil
}

// maxHeaderLineLen is the RFC 5322 recommended header line length.
const maxHeaderLineLen = 78

// writeHeader writes a single header field, folded at whitespace so lines
// stay within RFC 5322 limits (a long References chain or recipient list
// otherwise exceeds the 998-octet hard limit, which servers and spam filters
// reject). CRLF characters are stripped from the value to prevent header
// injection.
func writeHeader(w io.Writer, name, value string) {
	value = strings.NewReplacer("\r\n", "", "\r", "", "\n", "").Replace(value)
	fmt.Fprintf(w, "%s\r\n", foldHeader(name, value))
}

// foldHeader returns "name: value", inserting CRLF before whitespace wherever
// a line would exceed maxHeaderLineLen. Unfolding (removing the CRLFs)
// restores the original value. Single words longer than the limit stay whole.
func foldHeader(name, value string) string {
	return name + ": " + foldValue(len(name)+2, value)
}

// foldValue folds value as foldHeader does, for a value that starts at
// column lineLen.
func foldValue(lineLen int, value string) string {
	var b strings.Builder
	for i, word := range strings.Split(value, " ") {
		if i > 0 {
			if word != "" && lineLen+1+len(word) > maxHeaderLineLen {
				b.WriteString("\r\n")
				lineLen = 0
			}
			b.WriteString(" ")
			lineLen++
		}
		b.WriteString(word)
		lineLen += len(word)
	}
	return b.String()
}

// createPart starts a MIME part with its header values folded like top-level
// headers; multipart.Writer writes part header values verbatim.
func createPart(w *multipart.Writer, header textproto.MIMEHeader) (io.Writer, error) {
	for name, values := range header {
		for i, v := range values {
			values[i] = foldValue(len(name)+2, v)
		}
	}
	return w.CreatePart(header)
}

// formatAddresses formats a list of addresses for headers
func formatAddresses(addrs []Address) string {
	var parts []string
	for _, addr := range addrs {
		parts = append(parts, addr.String())
	}
	return strings.Join(parts, ", ")
}

// encodeSubject encodes the subject line if needed
func encodeSubject(subject string) string {
	// Encode returns subject unchanged when it needs no encoding.
	return mime.QEncoding.Encode("utf-8", subject)
}

// isASCII reports whether s contains only 7-bit characters.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// writeQuotedPrintable writes content using quoted-printable encoding
func writeQuotedPrintable(w io.Writer, content string) {
	qpWriter := quotedprintable.NewWriter(w)
	_, _ = qpWriter.Write([]byte(content))
	_ = qpWriter.Close()
}

// writeMultipartAlternative writes a multipart/alternative message
func writeMultipartAlternative(w *bytes.Buffer, textBody, htmlBody string) error {
	mpWriter := multipart.NewWriter(w)
	boundary := mpWriter.Boundary()

	writeHeader(w, "Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", boundary))
	w.WriteString("\r\n")

	// Write plain text part
	textHeader := textproto.MIMEHeader{}
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textHeader.Set("Content-Transfer-Encoding", "quoted-printable")

	textPart, err := createPart(mpWriter, textHeader)
	if err != nil {
		return err
	}
	writeQuotedPrintable(textPart, textBody)

	// Write HTML part
	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", "text/html; charset=utf-8")
	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")

	htmlPart, err := createPart(mpWriter, htmlHeader)
	if err != nil {
		return err
	}
	writeQuotedPrintable(htmlPart, htmlBody)

	return mpWriter.Close()
}

// writeMultipartMixed writes a multipart/mixed message with attachments
func writeMultipartMixed(w *bytes.Buffer, m *ComposeMessage, attachments, inlineAttachments []Attachment) error {
	mpWriter := multipart.NewWriter(w)
	boundary := mpWriter.Boundary()

	writeHeader(w, "Content-Type", fmt.Sprintf("multipart/mixed; boundary=%q", boundary))
	w.WriteString("\r\n")

	hasHTML := m.HTMLBody != ""
	hasText := m.TextBody != ""

	// Inline attachments need an HTML part to reference them; without one,
	// send them as regular attachments rather than dropping them.
	if !hasHTML {
		attachments = append(attachments, inlineAttachments...)
	}

	if hasHTML && hasText {
		// Create multipart/alternative nested inside the mixed section.
		// The altWriter MUST write to bodyPart (not w) so its boundaries
		// are properly nested inside the mixed boundary.
		altBoundary := uuid.New().String()
		altHeader := textproto.MIMEHeader{}
		altHeader.Set("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", altBoundary))

		bodyPart, err := createPart(mpWriter, altHeader)
		if err != nil {
			return err
		}

		altWriter := multipart.NewWriter(bodyPart)
		if err := altWriter.SetBoundary(altBoundary); err != nil {
			return err
		}

		// Plain text alternative
		textHeader := textproto.MIMEHeader{}
		textHeader.Set("Content-Type", "text/plain; charset=utf-8")
		textHeader.Set("Content-Transfer-Encoding", "quoted-printable")
		textPart, err := createPart(altWriter, textHeader)
		if err != nil {
			return err
		}
		writeQuotedPrintable(textPart, m.TextBody)

		// HTML alternative (with optional inline attachments)
		if len(inlineAttachments) > 0 {
			if err := writeRelatedPart(altWriter, m.HTMLBody, inlineAttachments); err != nil {
				return err
			}
		} else {
			htmlHeader := textproto.MIMEHeader{}
			htmlHeader.Set("Content-Type", "text/html; charset=utf-8")
			htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")
			htmlPart, err := createPart(altWriter, htmlHeader)
			if err != nil {
				return err
			}
			writeQuotedPrintable(htmlPart, m.HTMLBody)
		}

		if err := altWriter.Close(); err != nil {
			return err
		}
	} else if hasHTML {
		if len(inlineAttachments) > 0 {
			if err := writeRelatedPart(mpWriter, m.HTMLBody, inlineAttachments); err != nil {
				return err
			}
		} else {
			htmlHeader := textproto.MIMEHeader{}
			htmlHeader.Set("Content-Type", "text/html; charset=utf-8")
			htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")
			bodyPart, err := createPart(mpWriter, htmlHeader)
			if err != nil {
				return err
			}
			writeQuotedPrintable(bodyPart, m.HTMLBody)
		}
	} else if hasText {
		textHeader := textproto.MIMEHeader{}
		textHeader.Set("Content-Type", "text/plain; charset=utf-8")
		textHeader.Set("Content-Transfer-Encoding", "quoted-printable")
		bodyPart, err := createPart(mpWriter, textHeader)
		if err != nil {
			return err
		}
		writeQuotedPrintable(bodyPart, m.TextBody)
	}

	// Write regular attachments
	for _, att := range attachments {
		if err := writeAttachment(mpWriter, att); err != nil {
			return err
		}
	}

	return mpWriter.Close()
}

// writeRelatedPart creates a multipart/related part inside a parent multipart writer,
// containing HTML and inline attachments with proper MIME headers.
func writeRelatedPart(parentWriter *multipart.Writer, htmlBody string, inlineAttachments []Attachment) error {
	relBoundary := uuid.New().String()
	relHeader := textproto.MIMEHeader{}
	relHeader.Set("Content-Type", fmt.Sprintf("multipart/related; boundary=%q", relBoundary))

	relPart, err := createPart(parentWriter, relHeader)
	if err != nil {
		return err
	}

	relWriter := multipart.NewWriter(relPart)
	if err := relWriter.SetBoundary(relBoundary); err != nil {
		return err
	}

	// HTML sub-part
	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", "text/html; charset=utf-8")
	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	htmlPart, err := createPart(relWriter, htmlHeader)
	if err != nil {
		return err
	}
	writeQuotedPrintable(htmlPart, htmlBody)

	// Inline attachments
	for _, att := range inlineAttachments {
		if err := writeInlineAttachment(relWriter, att); err != nil {
			return err
		}
	}

	return relWriter.Close()
}

// writeAttachment writes a single attachment
func writeAttachment(w *multipart.Writer, att Attachment) error {
	contentType := att.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	header := textproto.MIMEHeader{}
	header.Set("Content-Type", attachmentContentType(contentType, att.Filename))
	header.Set("Content-Transfer-Encoding", "base64")
	header.Set("Content-Disposition", attachmentDisposition("attachment", att.Filename))

	part, err := createPart(w, header)
	if err != nil {
		return err
	}

	content, err := att.ResolveContent()
	if err != nil {
		return fmt.Errorf("failed to resolve attachment content: %w", err)
	}

	// Write base64 encoded content
	encoder := base64.NewEncoder(base64.StdEncoding, &base64LineWrapper{Writer: part})
	_, err = encoder.Write(content)
	if err != nil {
		return err
	}
	return encoder.Close()
}

// writeInlineAttachment writes an inline attachment (for HTML images)
func writeInlineAttachment(w *multipart.Writer, att Attachment) error {
	contentType := att.ContentType
	if contentType == "" {
		// Try to guess from filename
		ext := strings.ToLower(filepath.Ext(att.Filename))
		switch ext {
		case ".png":
			contentType = "image/png"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".gif":
			contentType = "image/gif"
		case ".webp":
			contentType = "image/webp"
		default:
			contentType = "application/octet-stream"
		}
	}

	header := textproto.MIMEHeader{}
	header.Set("Content-Type", attachmentContentType(contentType, att.Filename))
	header.Set("Content-Transfer-Encoding", "base64")
	header.Set("Content-Disposition", attachmentDisposition("inline", att.Filename))
	if att.ContentID != "" {
		header.Set("Content-ID", fmt.Sprintf("<%s>", att.ContentID))
	}

	part, err := createPart(w, header)
	if err != nil {
		return err
	}

	content, err := att.ResolveContent()
	if err != nil {
		return fmt.Errorf("failed to resolve inline attachment content: %w", err)
	}

	// Write base64 encoded content
	encoder := base64.NewEncoder(base64.StdEncoding, &base64LineWrapper{Writer: part})
	_, err = encoder.Write(content)
	if err != nil {
		return err
	}
	return encoder.Close()
}

// attachmentDisposition builds a Content-Disposition value. Non-ASCII
// filenames use RFC 2231 percent-encoding (filename* with a utf-8 prefix)
// instead of raw 8-bit bytes, which are invalid in headers without SMTPUTF8,
// split into RFC 2231 continuations so the header can fold within line limits.
func attachmentDisposition(disposition, filename string) string {
	if filename != "" && !isASCII(filename) {
		return disposition + "; " + rfc2231Param("filename", filename)
	}
	if v := mime.FormatMediaType(disposition, map[string]string{"filename": filename}); v != "" {
		return v
	}
	return disposition
}

// attachmentContentType sets the legacy name parameter that Outlook and
// other clients read when Content-Disposition lacks a usable filename,
// replacing any name already present. Non-ASCII names use RFC 2047
// encoded-words inside the quoted value; RFC 2047 does not sanction that,
// but it is what Gmail, Outlook and Thunderbird send and read.
func attachmentContentType(contentType, filename string) string {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || filename == "" {
		return contentType
	}
	ascii := isASCII(filename)
	delete(params, "name")
	if ascii {
		params["name"] = filename
	}
	v := mime.FormatMediaType(mediaType, params)
	if v == "" {
		return contentType
	}
	if !ascii {
		v += "; name=\"" + mime.BEncoding.Encode("utf-8", filename) + "\""
	}
	return v
}

// rfc2231ChunkLen bounds each RFC 2231 continuation segment.
const rfc2231ChunkLen = 60

// rfc2231Param encodes a UTF-8 parameter value per RFC 2231 as key*=, or
// as key*0*=, key*1*=... continuations when the encoded value is long; the
// first segment carries the utf-8 charset prefix. Segments never split a %XX escape.
func rfc2231Param(key, value string) string {
	var chunks []string
	var cur strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		n := 1
		if !isAttrChar(c) {
			n = 3
		}
		if cur.Len()+n > rfc2231ChunkLen {
			chunks = append(chunks, cur.String())
			cur.Reset()
		}
		if n == 1 {
			cur.WriteByte(c)
		} else {
			fmt.Fprintf(&cur, "%%%02X", c)
		}
	}
	chunks = append(chunks, cur.String())
	if len(chunks) == 1 {
		return key + "*=utf-8''" + chunks[0]
	}
	parts := make([]string, len(chunks))
	for i, c := range chunks {
		if i == 0 {
			c = "utf-8''" + c
		}
		parts[i] = fmt.Sprintf("%s*%d*=%s", key, i, c)
	}
	return strings.Join(parts, "; ")
}

// isAttrChar reports whether c may appear unescaped in an RFC 2231
// extended value (attribute-char: token characters except *, ' and %).
func isAttrChar(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("!#$&+-.^_`|~", c) >= 0
}

// base64LineWrapper wraps base64 output at 76 characters per line
type base64LineWrapper struct {
	Writer  io.Writer
	lineLen int
}

func (w *base64LineWrapper) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		// Calculate how much we can write before needing a line break
		remaining := 76 - w.lineLen
		if remaining <= 0 {
			if _, err := w.Writer.Write([]byte("\r\n")); err != nil {
				return n, err
			}
			w.lineLen = 0
			remaining = 76
		}

		toWrite := len(p)
		if toWrite > remaining {
			toWrite = remaining
		}

		written, err := w.Writer.Write(p[:toWrite])
		n += written
		w.lineLen += written
		if err != nil {
			return n, err
		}

		p = p[toWrite:]
	}
	return n, nil
}
