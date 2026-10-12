package message

import (
	"slices"
	"testing"

	"github.com/caseylabs/mailven/internal/cid"
)

func TestMarkEmbedded(t *testing.T) {
	logo := &Attachment{ID: "logo", IsInline: true, ContentID: "image001.png@01DA0000.12345678"}
	banner := &Attachment{ID: "banner", IsInline: true, ContentID: "banner@x"}
	unreferenced := &Attachment{ID: "unref", IsInline: true, ContentID: "orphan@x"}
	inlineNoCID := &Attachment{ID: "inline-nocid", IsInline: true}
	pdf := &Attachment{ID: "pdf"}
	pdfWithCID := &Attachment{ID: "pdf-cid", ContentID: "banner@x"}

	all := []*Attachment{logo, banner, unreferenced, inlineNoCID, pdf, pdfWithCID}

	cases := []struct {
		name string
		html string
		want []string
	}{
		{
			name: "no html keeps everything",
			html: "",
			want: []string{"logo", "banner", "unref", "inline-nocid", "pdf", "pdf-cid"},
		},
		{
			name: "html without cid keeps everything",
			html: `<p>Hi</p><img src="https://example.com/a.png">`,
			want: []string{"logo", "banner", "unref", "inline-nocid", "pdf", "pdf-cid"},
		},
		{
			name: "referenced inline parts are hidden",
			html: `<img src="cid:image001.png@01DA0000.12345678"><img src='cid:banner@x'>`,
			want: []string{"unref", "inline-nocid", "pdf", "pdf-cid"},
		},
		{
			name: "only the referenced inline part is hidden, not a non-inline part sharing its cid",
			html: `<img src="cid:banner@x">`,
			want: []string{"logo", "unref", "inline-nocid", "pdf", "pdf-cid"},
		},
		{
			name: "content-id is case sensitive",
			html: `<img src="cid:BANNER@X">`,
			want: []string{"logo", "banner", "unref", "inline-nocid", "pdf", "pdf-cid"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			listed := MarkEmbedded(all, c.html)
			var ids []string
			for _, a := range all {
				if !a.Embedded {
					ids = append(ids, a.ID)
				}
			}
			if !slices.Equal(ids, c.want) {
				t.Errorf("got %v, want %v", ids, c.want)
			}
			if listed != (len(c.want) > 0) {
				t.Errorf("MarkEmbedded() = %v, want %v", listed, len(c.want) > 0)
			}
		})
	}
}

// TestEmbeddedCIDs also checks that cid.Embedded, which replies use, agrees
// with MarkEmbedded: a reply re-attaches exactly the parts the viewer hides.
func TestEmbeddedCIDs(t *testing.T) {
	cases := []struct {
		name string
		html string
		cid  string
		want bool
	}{
		{"double quoted", `<img src="cid:part1">`, "part1", true},
		{"single quoted", `<img src='cid:part1'>`, "part1", true},
		{"unquoted attribute", `<img src=cid:part1 alt="">`, "part1", true},
		{"uppercase scheme and attribute", `<IMG SRC="CID:part1">`, "part1", true},
		{"spaces around equals", `<img src = "cid:part1">`, "part1", true},
		{"css url is not resolved by the viewer", `<td style="background:url(cid:part1)">`, "part1", false},
		{"bare text", `cid:part1`, "part1", false},
		{"prefix of another cid", `<img src="cid:part10">`, "part1", false},
		{"prefix then whole match", `<img src="cid:part10"><img src="cid:part1">`, "part1", true},
		{"content-id is case sensitive", `<img src="cid:PART1">`, "part1", false},
		{"absent", `<p>no images</p>`, "part1", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, got := cid.Embedded(c.html)[c.cid]; got != c.want {
				t.Errorf("cid.Embedded(%q) has %q = %v, want %v", c.html, c.cid, got, c.want)
			}
			att := &Attachment{IsInline: true, ContentID: c.cid}
			if listed := MarkEmbedded([]*Attachment{att}, c.html); listed == c.want {
				t.Errorf("MarkEmbedded(%q) lists %q = %v, want %v", c.html, c.cid, listed, !c.want)
			}
		})
	}
}
