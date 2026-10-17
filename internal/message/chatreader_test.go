package message

import (
	"strings"
	"testing"
)

// readerString renders blocks one per line: "kind: text" with links as
// [text](href), and "image: src | alt | href" for images.
func readerString(blocks []ReaderBlock) string {
	lines := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if b.Kind == ReaderImage {
			lines = append(lines, strings.TrimRight("image: "+b.Src+" | "+b.Alt+" | "+b.Href, " |"))
			continue
		}
		var s strings.Builder
		for _, sp := range b.Spans {
			if sp.Href != "" {
				s.WriteString("[" + sp.Text + "](" + sp.Href + ")")
				continue
			}
			s.WriteString(sp.Text)
		}
		lines = append(lines, b.Kind+": "+s.String())
	}
	return strings.Join(lines, "\n")
}

func TestExtractChatTextReader(t *testing.T) {
	tests := []struct {
		name string
		text string
		html string
		want string // readerString of the blocks; "" means no reader blocks
	}{
		{
			name: "newsletter layout",
			html: `<html><head><style>p{color:red}</style><title>Digest</title></head><body>
<div style="display:none">Preheader one</div>
<span style="max-height:0;overflow:hidden;font-size:1px">Preheader two</span>
<span style="opacity: 0">Preheader three</span>
<table width="100%" role="presentation"><tr><td align="center">
  <table width="600"><tr><td>
    <a href="https://example.com/home"><img src="https://cdn.example.com/logo.png" width="32" height="32" alt="Trail Magazine"></a>
    <h1>Ten   autumn
      hikes</h1>
    <img src="https://cdn.example.com/hero.jpg" width="600" alt="A ridge at dawn">
    <p>Coastal paths and <a href="https://example.com/ridge">a ridge walk</a> this week.</p>
    <p>Second line<br>after a break.</p>
    <table role="presentation"><tr><td style="background:#333"><a href="https://example.com/read">Read more</a></td></tr></table>
    <script>alert(1)</script>
    <img src="https://t.example.com/open.gif" width="1" height="1">
  </td></tr></table>
  <p style="font-size:11px">You get this because you subscribed. <a href="https://example.com/unsub">Unsubscribe</a></p>
</td></tr></table></body></html>`,
			want: `text: [Trail Magazine](https://example.com/home)
heading: Ten autumn hikes
image: https://cdn.example.com/hero.jpg | A ridge at dawn
text: Coastal paths and [a ridge walk](https://example.com/ridge) this week.
text: Second line
after a break.
text: [Read more](https://example.com/read)
text: You get this because you subscribed. [Unsubscribe](https://example.com/unsub)`,
		},
		{
			name: "receipt data table joins cells per row",
			html: `<table width="100%"><tr><td><h2>Your order</h2>
<table><tr><th>Item</th><th>Price</th></tr>
<tr><td>Coffee</td><td>$4.00</td></tr>
<tr><td>Bagel</td><td> $3.50 </td></tr></table>
</td></tr></table>`,
			want: "heading: Your order\ntext: Item · Price\ntext: Coffee · $4.00\ntext: Bagel · $3.50",
		},
		{
			name: "unsafe links become plain text",
			html: `<table role="presentation"><tr><td>
<p><a href="javascript:alert(1)">Click</a> <a href="data:text/html,x">data</a> <a href="/relative">rel</a> <a href="mailto:hi@example.com">Mail us</a> <a href="HTTPS://Example.com/x">ok</a></p>
</td></tr></table>`,
			want: "text: Click data rel [Mail us](mailto:hi@example.com) [ok](https://Example.com/x)",
		},
		{
			name: "image sources are filtered",
			html: `<table role="presentation"><tr><td>
<img src="cid:logo@x" width="200" alt="cid image">
<img src="javascript:alert(1)" width="200">
<img src="data:image/svg+xml,<svg/>" width="200">
<img src="//cdn.example.com/a.png" alt="relative scheme">
<img src="data:image/png;base64,AAAA" width="100" alt="inline">
<a href="https://social.example.com/me"><img src="https://cdn.example.com/fb.png" width="24" alt="Follow us"></a>
<p>Gear for the season.</p>
</td></tr></table>`,
			want: "image: https://cdn.example.com/a.png | relative scheme\nimage: data:image/png;base64,AAAA | inline\ntext: [Follow us](https://social.example.com/me)\ntext: Gear for the season.",
		},
		{
			name: "inline images share a size budget",
			html: `<table role="presentation"><tr><td>` +
				strings.Repeat(`<img src="data:image/png;base64,`+strings.Repeat("A", 200<<10)+`" width="100"><p>Shot.</p>`, 6) +
				`</td></tr></table>`,
			want: strings.TrimSuffix(strings.Repeat("image: data:image/png;base64,"+strings.Repeat("A", 200<<10)+"\ntext: Shot.\n", 5), "\n"),
		},
		{
			name: "lists become items",
			html: `<table role="presentation"><tr><td><p>This week:</p><ul><li>Boots</li><li><a href="https://example.com/tents">Tents</a></li></ul></td></tr></table>`,
			want: "text: This week:\nitem: Boots\nitem: [Tents](https://example.com/tents)",
		},
		{
			name: "quoted history is dropped",
			html: `<table role="presentation"><tr><td><p>Forwarding this digest.</p></td></tr></table>
<div class="gmail_quote">On Monday Sam wrote:<blockquote>Old digest</blockquote></div>
<blockquote type="cite">Cited</blockquote>
<div id="divRplyFwdMsg">From: Sam</div><p>Outlook history</p>`,
			want: "text: Forwarding this digest.",
		},
		{
			name: "repeated block is kept once",
			html: `<table role="presentation"><tr><td><p>View in browser</p><div>View in browser</div><p>Body</p></td></tr></table>`,
			want: "text: View in browser\ntext: Body",
		},
		{
			name: "long plain text becomes paragraphs",
			text: strings.Repeat("word ", 900) + "\n\n\n\nSecond paragraph\nwith a line break.",
			want: "text: " + strings.TrimSpace(strings.Repeat("word ", 900)) + "\ntext: Second paragraph\nwith a line break.",
		},
		{
			name: "short message has no reader blocks",
			html: `<p>Lunch tomorrow?</p>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := ExtractChatText(tt.text, tt.html)
			if got := readerString(ct.Reader); got != tt.want {
				t.Errorf("reader =\n%s\nwant\n%s", got, tt.want)
			}
			if tt.want != "" && !ct.IsRich {
				t.Error("IsRich = false, want true")
			}
		})
	}
}

func TestIsReaderHidden(t *testing.T) {
	tests := []struct {
		html string
		want bool
	}{
		{`<div style="display: none">x</div>`, true},
		{`<div hidden>x</div>`, true},
		{`<div style="visibility:hidden">x</div>`, true},
		{`<div style="font-size:0px">x</div>`, true},
		{`<div style="opacity:0.0 !important">x</div>`, true},
		{`<div style="max-height:0;overflow:hidden">x</div>`, true},
		{`<div style="max-height:0">x</div>`, false},
		{`<div style="opacity:0.5;font-size:0.8em">x</div>`, false},
		{`<div style="height:100px;overflow:hidden">x</div>`, false},
	}
	for _, tt := range tests {
		ct := ExtractChatText("", `<table role="presentation"><tr><td>`+tt.html+`<p>shown</p></td></tr></table>`)
		hidden := !strings.Contains(readerString(ct.Reader), "text: x")
		if hidden != tt.want {
			t.Errorf("%s: hidden = %v, want %v (reader %q)", tt.html, hidden, tt.want, readerString(ct.Reader))
		}
	}
}

func TestExtractViewChatTextCID(t *testing.T) {
	body := `<table role="presentation"><tr><td><h1>Receipt</h1>
<img src="cid:logo@example.com" width="200" alt="Logo">
<img src="CID:%3Chero%40example.com%3E" alt="Hero">
<img src="cid:" alt="empty"><img src="cid:a b" alt="space">
<img src="cid:icon" width="16" height="16" alt="icon">
<a href="https://example.com/x"><img src="cid:linked"></a>
<p>` + strings.Repeat("Thanks for your order. ", 40) + `</p></td></tr></table>`
	tests := []struct {
		name string
		view bool
		want string
	}{
		{"view keeps cid pictures", true, "heading: Receipt\nimage: cid:logo@example.com | Logo\nimage: cid:hero@example.com | Hero\nimage: cid:linked |  | https://example.com/x"},
		{"stored body drops them", false, "heading: Receipt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := ExtractChatText("", body)
			if tt.view {
				ct = ExtractViewChatText("", body)
			}
			got := readerString(ct.Reader)
			if i := strings.Index(got, "\ntext:"); i >= 0 {
				got = got[:i]
			}
			if got != tt.want {
				t.Errorf("reader =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}
