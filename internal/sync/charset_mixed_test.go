package sync

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
)

// TestDecodeMIMEWordMultilingual covers RFC 2047 encoded words in several
// scripts and charsets, as found in Subject and From headers.
func TestDecodeMIMEWordMultilingual(t *testing.T) {
	sjis, err := japanese.ShiftJIS.NewEncoder().String("日本語の件名")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, in, want string
	}{
		{"utf-8 base64 CJK", "=?UTF-8?B?5Lit5paH?=", "中文"},
		{"utf-8 q accents", "=?UTF-8?Q?caf=C3=A9_cr=C3=A8me?=", "café crème"},
		{"iso-8859-1 q", "=?ISO-8859-1?Q?Andr=E9_M=FCller?=", "André Müller"},
		{"shift_jis base64", "=?Shift_JIS?B?" + base64.StdEncoding.EncodeToString([]byte(sjis)) + "?=", "日本語の件名"},
		{"rtl utf-8", "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte("שלום עולם")) + "?=", "שלום עולם"},
		{"emoji", "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte("🎉 party")) + "?=", "🎉 party"},
		{"plain ascii", "Hello", "Hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeMIMEWord(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParseMessageBodyMixedCharsets checks that parts in different charsets
// in one message each decode to the right text.
func TestParseMessageBodyMixedCharsets(t *testing.T) {
	latin1, err := charmap.ISO8859_1.NewEncoder().String("Grüße aus Zürich")
	if err != nil {
		t.Fatal(err)
	}
	sjis, err := japanese.ShiftJIS.NewEncoder().String("こんにちは世界")
	if err != nil {
		t.Fatal(err)
	}
	utf8Text := "你好 שלום مرحبا 🎉 café"

	tests := []struct {
		name, ctype, charset, enc, body, want string
		html                                  bool
	}{
		{"iso-8859-1 plain", "text/plain", "iso-8859-1", "8bit", latin1, "Grüße aus Zürich", false},
		{"shift_jis plain", "text/plain", "shift_jis", "base64", base64.StdEncoding.EncodeToString([]byte(sjis)), "こんにちは世界", false},
		{"shift_jis html", "text/html", "shift_jis", "base64", base64.StdEncoding.EncodeToString([]byte("<p>" + sjis + "</p>")), "こんにちは世界", true},
		{"utf-8 mixed scripts", "text/plain", "utf-8", "base64", base64.StdEncoding.EncodeToString([]byte(utf8Text)), utf8Text, false},
	}
	e := &Engine{log: zerolog.Nop()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := "From: =?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte("山田 太郎")) + "?= <t@example.com>\r\n" +
				"Subject: test\r\nMIME-Version: 1.0\r\n" +
				"Content-Type: " + tt.ctype + "; charset=" + tt.charset + "\r\n" +
				"Content-Transfer-Encoding: " + tt.enc + "\r\n\r\n" + tt.body + "\r\n"
			text, html, _ := e.parseMessageBody([]byte(raw))
			got := text
			if tt.html {
				got = html
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("got %q, want it to contain %q", got, tt.want)
			}
		})
	}
}
