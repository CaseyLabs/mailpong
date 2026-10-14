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
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

// NormalizeEnvelopeAddress converts only the domain to IDNA ASCII. Display
// names, groups, whitespace and control characters are not SMTP mailboxes.
func NormalizeEnvelopeAddress(address string) (string, error) {
	if address == "" || strings.ContainsAny(address, "\r\n\x00") || strings.TrimSpace(address) != address || !utf8.ValidString(address) {
		return "", fmt.Errorf("invalid envelope address")
	}
	parsed, err := mail.ParseAddress(address)
	if err != nil || parsed.Name != "" || strings.ContainsAny(address, "<>") {
		return "", fmt.Errorf("invalid envelope address %q", address)
	}
	at := strings.LastIndexByte(address, '@')
	if at <= 0 || at == len(address)-1 {
		return "", fmt.Errorf("invalid envelope address %q", address)
	}
	if at > 64 {
		return "", fmt.Errorf("envelope local part exceeds 64 bytes")
	}
	domain := address[at+1:]
	if !strings.HasPrefix(domain, "[") {
		domain, err = idna.Lookup.ToASCII(domain)
		if err != nil || domain == "" {
			return "", fmt.Errorf("invalid address domain in %q", address)
		}
		for _, label := range strings.Split(domain, ".") {
			if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return "", fmt.Errorf("invalid address domain in %q", address)
			}
		}
	}
	if len(domain) > 255 || at+1+len(domain) > 254 {
		return "", fmt.Errorf("envelope address is too long")
	}
	return address[:at+1] + domain, nil
}

func validMessageIDs(value string) bool {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return false
	}
	for _, field := range fields {
		if len(field) < 5 || field[0] != '<' || field[len(field)-1] != '>' {
			return false
		}
		inner := field[1 : len(field)-1]
		if strings.ContainsAny(inner, "<>\r\n\t ") || strings.Count(inner, "@") != 1 {
			return false
		}
		// Message-ID syntax resembles an address, but its id-left is not an
		// SMTP local part and does not have the envelope's 64-byte limit.
		parsed, err := mail.ParseAddress(inner)
		if err != nil || parsed.Name != "" || parsed.Address != inner {
			return false
		}
	}
	return true
}

// ValidateMessage checks the exact final MIME bytes, without rewriting signed
// content. Gmail's limits are separate from the RFC physical line ceiling.
func ValidateMessage(msg []byte, gmail bool) error {
	for _, line := range bytes.Split(msg, []byte("\n")) {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if len(line) > 998 {
			return fmt.Errorf("message contains a physical line longer than 998 bytes; shorten the header or content")
		}
		if bytes.ContainsAny(line, "\r\x00") {
			return fmt.Errorf("message contains an invalid control character")
		}
	}
	split := bytes.Index(msg, []byte("\r\n\r\n"))
	if split < 0 {
		return fmt.Errorf("message is missing the MIME header separator")
	}
	h, err := validateHeaderBlock(msg[:split], gmail)
	if err != nil {
		return err
	}
	for _, name := range []string{"Date", "From", "Message-ID"} {
		if len(h.Values(name)) != 1 {
			return fmt.Errorf("message requires exactly one %s header", name)
		}
	}
	if _, err := mail.ParseDate(h.Get("Date")); err != nil {
		return fmt.Errorf("invalid Date header")
	}
	from, err := mail.ParseAddressList(h.Get("From"))
	if err != nil || len(from) != 1 {
		return fmt.Errorf("message requires one valid From address")
	}
	if _, err := NormalizeEnvelopeAddress(from[0].Address); err != nil {
		return fmt.Errorf("invalid From header: %w", err)
	}
	if !validMessageIDs(h.Get("Message-ID")) || len(strings.Fields(h.Get("Message-ID"))) != 1 {
		return fmt.Errorf("invalid Message-ID header")
	}
	for _, name := range []string{"In-Reply-To", "References"} {
		if len(h.Values(name)) > 1 || (len(h.Values(name)) == 1 && !validMessageIDs(h.Get(name))) {
			return fmt.Errorf("invalid %s header", name)
		}
	}
	for _, name := range []string{"Bcc", "Received", "Return-Path", "Authentication-Results"} {
		if len(h.Values(name)) != 0 {
			return fmt.Errorf("outgoing message must not contain %s", name)
		}
	}
	for _, name := range []string{"To", "Cc", "Reply-To", "Subject"} {
		if len(h.Values(name)) > 1 {
			return fmt.Errorf("duplicate %s header", name)
		}
		if value := h.Get(name); name != "Subject" && value != "" && !(name == "To" && value == "undisclosed-recipients:;") {
			addresses, err := mail.ParseAddressList(value)
			if err != nil || len(addresses) == 0 {
				return fmt.Errorf("invalid %s header", name)
			}
			for _, address := range addresses {
				if _, err := NormalizeEnvelopeAddress(address.Address); err != nil {
					return fmt.Errorf("invalid %s header: %w", name, err)
				}
			}
		}
	}
	return validateMIME(h, msg[split+4:], 0)
}

func validateHeaderBlock(block []byte, gmail bool) (textproto.MIMEHeader, error) {
	if !utf8.Valid(block) {
		return nil, fmt.Errorf("invalid UTF-8 in MIME headers")
	}
	if gmail && len(block)+2 > 512000 {
		return nil, fmt.Errorf("Gmail headers exceed 512000 bytes")
	}
	h := textproto.MIMEHeader{}
	name, value := "", ""
	count := 0
	flush := func() error {
		if name == "" {
			return nil
		}
		if gmail {
			if len(name) > 1000 {
				return fmt.Errorf("Gmail header name exceeds 1000 bytes")
			}
			if name != "To" && name != "Cc" && name != "References" && len(value) > 32768 {
				return fmt.Errorf("Gmail %s header exceeds 32768 bytes", name)
			}
			limit := 0
			switch name {
			case "Subject":
				limit = 998
			case "Message-Id":
				limit = 500
			}
			if limit > 0 && utf8.RuneCountInString(value) > limit {
				return fmt.Errorf("Gmail %s header exceeds %d characters; shorten it before sending", name, limit)
			}
		}
		h.Add(name, strings.TrimSpace(value))
		return nil
	}
	for _, line := range strings.Split(string(block), "\r\n") {
		if strings.ContainsAny(line, "\r\n\x00") {
			return nil, fmt.Errorf("invalid header line ending")
		}
		for _, c := range line {
			if (c < 32 && c != '\t') || c == 127 {
				return nil, fmt.Errorf("invalid header control character")
			}
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if name == "" {
				return nil, fmt.Errorf("header continuation without a field")
			}
			value += line
			continue
		}
		if err := flush(); err != nil {
			return nil, err
		}
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			return nil, fmt.Errorf("malformed MIME header")
		}
		for _, c := range line[:colon] {
			if c < 33 || c > 126 || c == ':' {
				return nil, fmt.Errorf("invalid header name")
			}
		}
		name, value = textproto.CanonicalMIMEHeaderKey(line[:colon]), line[colon+1:]
		count++
		if gmail && count > 5000 {
			return nil, fmt.Errorf("Gmail message exceeds 5000 header fields")
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return h, nil
}

func validateMIME(h textproto.MIMEHeader, body []byte, depth int) error {
	if depth > 30 {
		return fmt.Errorf("MIME nesting is too deep")
	}
	if len(h.Values("Content-Type")) > 1 {
		return fmt.Errorf("duplicate Content-Type header")
	}
	if len(h.Values("Content-Transfer-Encoding")) > 1 {
		return fmt.Errorf("duplicate Content-Transfer-Encoding header")
	}
	if disposition := h.Get("Content-Disposition"); disposition != "" {
		if _, _, err := mime.ParseMediaType(disposition); err != nil {
			return fmt.Errorf("invalid Content-Disposition: %w", err)
		}
	}
	contentType := h.Get("Content-Type")
	if contentType == "" {
		contentType = "text/plain"
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("invalid MIME Content-Type: %w", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		var decoded io.Reader
		switch strings.ToLower(h.Get("Content-Transfer-Encoding")) {
		case "base64":
			decoded = base64.NewDecoder(base64.StdEncoding, bytes.NewReader(body))
		case "quoted-printable":
			decoded = quotedprintable.NewReader(bytes.NewReader(body))
		case "", "7bit", "8bit", "binary":
			return nil
		default:
			return fmt.Errorf("unsupported MIME Content-Transfer-Encoding")
		}
		if _, err := io.Copy(io.Discard, decoded); err != nil {
			return fmt.Errorf("invalid encoded MIME body: %w", err)
		}
		return nil
	}
	boundary := params["boundary"]
	if boundary == "" {
		return fmt.Errorf("multipart MIME is missing its boundary")
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	parts := 0
	for {
		part, err := reader.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("malformed multipart MIME: %w", err)
		}
		parts++
		data, err := io.ReadAll(part)
		if err != nil {
			return fmt.Errorf("malformed MIME part: %w", err)
		}
		if err := validateMIME(part.Header, data, depth+1); err != nil {
			return err
		}
	}
	if parts == 0 {
		return fmt.Errorf("multipart MIME has no parts")
	}
	return nil
}

func (c *Client) preflight(from string, to []string, msg []byte) (string, []string, error) {
	sender, err := NormalizeEnvelopeAddress(from)
	if err != nil {
		return "", nil, fmt.Errorf("sender: %w", err)
	}
	if len(to) == 0 {
		return "", nil, fmt.Errorf("no envelope recipients specified")
	}
	recipients := make([]string, len(to))
	requiresUTF8 := !isASCII(sender)
	gmail := strings.EqualFold(c.config.Host, "smtp.gmail.com") || strings.EqualFold(c.config.Host, "smtp-relay.gmail.com")
	for i, recipient := range to {
		recipients[i], err = NormalizeEnvelopeAddress(recipient)
		if err != nil {
			return "", nil, fmt.Errorf("recipient: %w", err)
		}
		requiresUTF8 = requiresUTF8 || !isASCII(recipients[i])
		domain := recipients[i][strings.LastIndexByte(recipients[i], '@')+1:]
		gmail = gmail || strings.EqualFold(domain, "gmail.com") || strings.EqualFold(domain, "googlemail.com")
	}
	if err := ValidateMessage(msg, gmail); err != nil {
		return "", nil, err
	}
	// Raw international headers also require SMTPUTF8 (RFC 6531); encoded
	// subjects/display names and Unicode bodies do not.
	headerEnd := bytes.Index(msg, []byte("\r\n\r\n"))
	requiresUTF8 = requiresUTF8 || !isASCII(string(msg[:headerEnd]))
	if ok, _ := c.client.Extension("SMTPUTF8"); requiresUTF8 && !ok {
		return "", nil, fmt.Errorf("this message requires SMTPUTF8, which the sending server does not advertise; use an ASCII mailbox or another configured account")
	}
	if ok, parameter := c.client.Extension("SIZE"); ok && strings.TrimSpace(parameter) != "" {
		limit, err := strconv.ParseUint(strings.TrimSpace(parameter), 10, 64)
		if err != nil {
			return "", nil, fmt.Errorf("sending server advertised an invalid SIZE limit")
		}
		// SIZE 0 explicitly declares no fixed maximum, not verified unlimited delivery.
		if limit > 0 && uint64(messageWireSize(msg)) > limit {
			return "", nil, fmt.Errorf("encoded message is %d bytes, exceeding the server SIZE limit of %d bytes; reduce attachments or content (draft retained)", messageWireSize(msg), limit)
		}
	}
	return sender, recipients, nil
}

// net/textproto's DATA writer canonicalizes LF and appends a final CRLF.
// SIZE includes those octets, excluding dot-stuffing and the DATA terminator.
func messageWireSize(msg []byte) int {
	size := len(msg)
	for i, b := range msg {
		if b == '\n' && (i == 0 || msg[i-1] != '\r') {
			size++
		}
	}
	if len(msg) == 0 || msg[len(msg)-1] != '\n' {
		size += 2
	}
	return size
}
