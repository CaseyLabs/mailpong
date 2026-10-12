package sync

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/caseylabs/mailven/internal/logging"
	msgcharset "github.com/emersion/go-message/charset"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/htmlindex"
)

// decodeQuotedPrintableIfNeeded detects and decodes quoted-printable content
// in a part that doesn't declare it, as a fallback for mislabelled mail.
// go-message has already decoded a declared quoted-printable or base64 body,
// so those are returned as is: decoding again would mangle text containing
// "=XX" sequences or lines ending in "=".
func decodeQuotedPrintableIfNeeded(content []byte, transferEncoding string) []byte {
	switch strings.ToLower(strings.TrimSpace(transferEncoding)) {
	case "quoted-printable", "base64":
		return content
	}
	// Quick check: if content doesn't contain "=3D" or "=\n" patterns, it's likely not QP-encoded
	contentStr := string(content)
	if !strings.Contains(contentStr, "=3D") && !strings.Contains(contentStr, "=\n") && !strings.Contains(contentStr, "=\r\n") {
		return content
	}

	log := logging.WithComponent("quoted-printable")
	log.Debug().Msg("Detected potential quoted-printable encoding, attempting decode")

	// Try to decode as quoted-printable
	reader := quotedprintable.NewReader(bytes.NewReader(content))
	decoded, err := io.ReadAll(reader)
	if err != nil {
		log.Debug().Err(err).Msg("Quoted-printable decode failed, returning original content")
		return content
	}

	log.Debug().
		Int("originalLen", len(content)).
		Int("decodedLen", len(decoded)).
		Bool("stillHasQP", strings.Contains(string(decoded), "=3D")).
		Msg("Quoted-printable decode successful")

	return decoded
}

// decodeCharset converts content from the specified charset to UTF-8
// It handles mislabeled encodings by validating UTF-8 and auto-detecting if invalid
func decodeCharset(content []byte, declaredCharset string) string {
	log := logging.WithComponent("charset")
	log.Debug().Str("declaredCharset", declaredCharset).Int("contentLen", len(content)).Msg("Attempting charset decode")

	// If declared charset is UTF-8/ASCII or empty, validate the content
	if declaredCharset == "" || strings.EqualFold(declaredCharset, "utf-8") || strings.EqualFold(declaredCharset, "us-ascii") {
		// Check if content is actually valid UTF-8
		if utf8.Valid(content) {
			// Even if "valid UTF-8", check if it looks like misencoded text
			// by looking for high concentration of replacement chars or CJK Extension B chars
			str := string(content)
			if !looksLikeGibberish(str) {
				log.Debug().Str("declaredCharset", declaredCharset).Msg("Content is valid UTF-8")
				return str
			}
			log.Warn().Str("declaredCharset", declaredCharset).Msg("Content is valid UTF-8 but looks like gibberish, trying Chinese encodings")
		} else {
			log.Warn().Str("declaredCharset", declaredCharset).Msg("Content is NOT valid UTF-8, auto-detecting encoding")
		}

		// Try auto-detection first
		encoding, name, _ := charset.DetermineEncoding(content, "text/html")
		log.Debug().Str("detectedEncoding", name).Msg("Auto-detected encoding")

		decoded, err := encoding.NewDecoder().Bytes(content)
		if err == nil && !looksLikeGibberish(string(decoded)) {
			log.Debug().Str("detectedEncoding", name).Msg("Successfully decoded using auto-detected encoding")
			return string(decoded)
		}

		// Auto-detection failed or produced gibberish - try common Chinese encodings
		chineseEncodings := []string{"gb18030", "gbk", "gb2312", "big5", "euc-tw"}
		for _, encName := range chineseEncodings {
			enc, err := htmlindex.Get(encName)
			if err != nil {
				continue
			}
			decoded, err := enc.NewDecoder().Bytes(content)
			if err == nil && utf8.Valid(decoded) && !looksLikeGibberish(string(decoded)) {
				log.Debug().Str("triedEncoding", encName).Msg("Successfully decoded using Chinese encoding fallback")
				return string(decoded)
			}
		}

		log.Warn().Msg("All charset detection attempts failed, returning as-is")
		return string(content)
	}

	// Declared charset is something other than UTF-8 - decode it
	log.Debug().Str("declaredCharset", declaredCharset).Msg("Decoding from declared charset")

	enc, err := htmlindex.Get(declaredCharset)
	if err != nil {
		log.Warn().Err(err).Str("declaredCharset", declaredCharset).Msg("Unknown charset, trying aliases")
		// Try common aliases
		aliases := map[string]string{
			"gb2312": "gbk", // GB2312 is often actually GBK
			"x-gbk":  "gbk",
			"big5":   "big5",
			"x-big5": "big5",
		}
		if alias, ok := aliases[strings.ToLower(declaredCharset)]; ok {
			enc, err = htmlindex.Get(alias)
		}
		if err != nil {
			log.Warn().Err(err).Str("declaredCharset", declaredCharset).Msg("Unknown charset, returning as-is")
			return string(content)
		}
	}

	// Decode to UTF-8
	decoded, err := enc.NewDecoder().Bytes(content)
	if err != nil {
		log.Warn().Err(err).Str("declaredCharset", declaredCharset).Msg("Charset decoding failed, returning as-is")
		return string(content)
	}

	log.Debug().Str("declaredCharset", declaredCharset).Msg("Successfully decoded charset to UTF-8")
	return string(decoded)
}

// looksLikeGibberish checks if a string appears to be misencoded text
// by looking for telltale signs of encoding problems
func looksLikeGibberish(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Count problematic characters
	var replacementCount, cjkExtBCount, total int
	for _, r := range s {
		total++
		if r == '\ufffd' { // Unicode replacement character
			replacementCount++
		}
		// CJK Extension B range (U+20000-U+2A6DF) often indicates misencoding
		// These are rare characters that shouldn't appear frequently in normal text
		if r >= 0x20000 && r <= 0x2A6DF {
			cjkExtBCount++
		}
	}

	// If more than 10% replacement characters, it's gibberish
	if total > 10 && float64(replacementCount)/float64(total) > 0.1 {
		return true
	}

	// If more than 5% CJK Extension B characters, likely misencoded
	// (these are extremely rare in normal Chinese text)
	if total > 20 && float64(cjkExtBCount)/float64(total) > 0.05 {
		return true
	}

	return false
}

// extractCharsetFromHTML extracts charset from HTML meta tags
// This is used as a fallback when Content-Type header doesn't specify charset
func extractCharsetFromHTML(html []byte) string {
	log := logging.WithComponent("charset")

	// Only check the first 1024 bytes for performance (meta tags are always near the top)
	searchBytes := html
	if len(html) > 1024 {
		searchBytes = html[:1024]
	}

	// Log the first 200 bytes for debugging (to see what meta tags are present)
	preview := searchBytes
	if len(preview) > 200 {
		preview = preview[:200]
	}
	log.Debug().
		Str("htmlPreview", string(preview)).
		Int("searchLen", len(searchBytes)).
		Msg("Searching for charset in HTML")

	// Pattern 1: <meta charset="...">
	re1 := regexp.MustCompile(`(?i)<meta[^>]+charset=["']?([^"'\s>]+)`)
	if match := re1.FindSubmatch(searchBytes); len(match) > 1 {
		charset := string(match[1])
		log.Debug().Str("charset", charset).Msg("Found charset via meta charset attribute")
		return charset
	}

	// Pattern 2: <meta http-equiv="Content-Type" content="text/html; charset=...">
	re2 := regexp.MustCompile(`(?i)<meta[^>]+content=["'][^"']*charset=([^"'\s;]+)`)
	if match := re2.FindSubmatch(searchBytes); len(match) > 1 {
		charset := string(match[1])
		log.Debug().Str("charset", charset).Msg("Found charset via meta http-equiv")
		return charset
	}

	log.Debug().Msg("No charset found in HTML meta tags")
	return ""
}

// decodeMIMEWord decodes RFC 2047 encoded words (e.g., =?UTF-8?B?5Lit5paH?=)
// used for non-ASCII filenames and headers
func decodeMIMEWord(s string) string {
	if s == "" {
		return s
	}
	// Use mime.WordDecoder with charset fallback support
	dec := &mime.WordDecoder{
		CharsetReader: func(charsetName string, r io.Reader) (io.Reader, error) {
			// First try the go-message charset package
			if reader, err := msgcharset.Reader(charsetName, r); err == nil {
				return reader, nil
			}
			// Fall back to htmlindex for broader charset support (GB2312, GBK, Big5, etc.)
			enc, err := htmlindex.Get(charsetName)
			if err != nil {
				return nil, fmt.Errorf("unknown charset: %s", charsetName)
			}
			return enc.NewDecoder().Reader(r), nil
		},
	}
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		// If decoding fails, return original string
		return s
	}
	return decoded
}
