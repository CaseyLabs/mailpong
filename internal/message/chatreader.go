package message

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Reader block kinds.
const (
	ReaderHeading = "heading"
	ReaderText    = "text"
	ReaderItem    = "item"
	ReaderImage   = "image"
)

const (
	// readerMaxBlocks bounds the blocks kept for one message.
	readerMaxBlocks = 400
	// readerMinImagePx is the smallest declared image side kept as content;
	// smaller images are icons and spacers.
	readerMinImagePx = 48
	// readerMaxDataURI bounds an inline data: image source, and
	// readerMaxDataTotal all of a message's inline sources together.
	readerMaxDataURI   = 256 << 10
	readerMaxDataTotal = 1 << 20
	// readerCellSep joins the cells of a data table row.
	readerCellSep = " · "
)

// ReaderBlock is one block of a rich message reduced to sanitized content:
// a heading, paragraph or list item made of text runs, or an image.
type ReaderBlock struct {
	Kind  string       `json:"kind"`
	Spans []ReaderSpan `json:"spans,omitempty"`
	// Src, Alt and Href describe an image block; Href is its enclosing link.
	Src  string `json:"src,omitempty"`
	Alt  string `json:"alt,omitempty"`
	Href string `json:"href,omitempty"`
}

// ReaderSpan is a run of text, linked when Href is set. Href is always an
// http, https or mailto URL.
type ReaderSpan struct {
	Text string `json:"text"`
	Href string `json:"href,omitempty"`
}

// readerFromHTML reduces a parsed HTML document to reader blocks. Layout
// tables, hidden elements (preheaders), tracking pixels, icons, scripts and
// quoted history are dropped.
//
// keepCID keeps images sourced from a "cid:" content ID as "cid:<id>" blocks
// for the view to resolve from the message's own inline attachments; without
// it they are dropped, as the stored body has no attachment data to show.
func readerFromHTML(doc *html.Node, keepCID bool) []ReaderBlock {
	w := &readerWalker{kind: ReaderText, keepCID: keepCID}
	w.walk(doc)
	w.flush()
	return w.blocks
}

// readerFromText splits plain text into paragraph blocks.
func readerFromText(text string) []ReaderBlock {
	var blocks []ReaderBlock
	text = blankRunRe.ReplaceAllString(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	for _, para := range strings.Split(text, "\n\n") {
		if para = strings.TrimSpace(para); para == "" {
			continue
		}
		blocks = append(blocks, ReaderBlock{Kind: ReaderText, Spans: []ReaderSpan{{Text: para}}})
		if len(blocks) == readerMaxBlocks {
			break
		}
	}
	return blocks
}

type readerWalker struct {
	blocks    []ReaderBlock
	kind      string // kind of the block being built
	spans     []ReaderSpan
	href      string // sanitized enclosing link, or ""
	pre       int    // <pre> nesting
	dataTable int    // data (non-layout) table nesting
	cellStart bool   // the next text starts a data table cell
	dataBytes int    // bytes of inline data: image sources kept
	keepCID   bool   // keep cid: image sources (decrypted view results)
	stopped   bool   // an Outlook reply marker or the block limit was reached
}

func (w *readerWalker) walk(n *html.Node) {
	if w.stopped {
		return
	}
	if n.Type == html.TextNode {
		w.text(n.Data)
		return
	}
	if n.Type != html.ElementNode {
		w.walkChildren(n)
		return
	}
	if skipAtoms[n.DataAtom] || isReaderHidden(n) || isQuoteBlock(n) {
		return
	}
	if isOutlookReplyMarker(n) {
		w.stopped = true
		return
	}

	switch n.DataAtom {
	case atom.Br:
		w.write("\n")
		return
	case atom.Img:
		w.image(n)
		return
	case atom.A:
		prev := w.href
		w.href = safeReaderHref(attr(n, "href"))
		w.walkChildren(n)
		w.href = prev
		return
	case atom.Table:
		if !isLayoutTable(n) {
			w.dataTable++
			defer func() { w.dataTable-- }()
		}
	case atom.Td, atom.Th:
		if w.dataTable > 0 {
			w.cellStart = true
			w.walkChildren(n)
			return
		}
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		w.block(n, ReaderHeading)
		return
	case atom.Li:
		w.block(n, ReaderItem)
		return
	case atom.Pre:
		w.pre++
		defer func() { w.pre-- }()
	}

	if !blockAtoms[n.DataAtom] && n.DataAtom != atom.Blockquote && n.DataAtom != atom.Td && n.DataAtom != atom.Th {
		w.walkChildren(n)
		return
	}
	w.block(n, w.kind)
}

// block walks n as its own block of the given kind.
func (w *readerWalker) block(n *html.Node, kind string) {
	w.flush()
	prev := w.kind
	w.kind = kind
	w.walkChildren(n)
	w.flush()
	w.kind = prev
}

func (w *readerWalker) walkChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		w.walk(c)
	}
}

// text appends a text node, collapsing whitespace outside <pre>.
func (w *readerWalker) text(s string) {
	s = invisibleRe.ReplaceAllString(s, "")
	if w.pre == 0 {
		s = spaceRunRe.ReplaceAllString(strings.NewReplacer("\r", " ", "\n", " ").Replace(s), " ")
	}
	if strings.TrimSpace(s) == "" {
		if s != "" {
			w.write(" ")
		}
		return
	}
	if w.cellStart {
		w.cellStart = false
		if w.hasText() {
			w.write(readerCellSep)
			s = strings.TrimLeft(s, " ")
		}
	}
	w.write(s)
}

// write appends s to the current block, merging runs with the same link.
func (w *readerWalker) write(s string) {
	if last := len(w.spans) - 1; last >= 0 && w.spans[last].Href == w.href {
		w.spans[last].Text += s
		return
	}
	w.spans = append(w.spans, ReaderSpan{Text: s, Href: w.href})
}

func (w *readerWalker) hasText() bool {
	for _, s := range w.spans {
		if strings.TrimSpace(s.Text) != "" {
			return true
		}
	}
	return false
}

// image adds a content image as its own block. Icons, spacers and images
// without a safe source are dropped; inside a link their alt text stays as
// the link's label.
func (w *readerWalker) image(n *html.Node) {
	if isTrackingPixel(n) {
		return
	}
	src := safeReaderImageSrc(attr(n, "src"), w.keepCID)
	alt := strings.TrimSpace(spaceRunRe.ReplaceAllString(invisibleRe.ReplaceAllString(attr(n, "alt"), ""), " "))
	if strings.HasPrefix(src, "data:") {
		if w.dataBytes+len(src) > readerMaxDataTotal {
			src = ""
		} else {
			w.dataBytes += len(src)
		}
	}
	if src == "" || isSmallImage(n) {
		if alt != "" && w.href != "" {
			w.text(" " + alt + " ")
		}
		return
	}
	w.flush()
	w.add(ReaderBlock{Kind: ReaderImage, Src: src, Alt: alt, Href: w.href})
}

// flush ends the current text block, dropping it when empty or when it
// repeats the previous block.
func (w *readerWalker) flush() {
	spans := trimSpans(w.spans)
	w.spans = nil
	w.cellStart = false
	if len(spans) == 0 {
		return
	}
	if last := len(w.blocks) - 1; last >= 0 && w.blocks[last].Kind != ReaderImage && spansText(w.blocks[last].Spans) == spansText(spans) {
		return
	}
	w.add(ReaderBlock{Kind: w.kind, Spans: spans})
}

func (w *readerWalker) add(b ReaderBlock) {
	w.blocks = append(w.blocks, b)
	if len(w.blocks) >= readerMaxBlocks {
		w.stopped = true
	}
}

// trimSpans trims spaces around line breaks and at both ends, collapses
// blank lines and drops empty runs.
func trimSpans(spans []ReaderSpan) []ReaderSpan {
	out := make([]ReaderSpan, 0, len(spans))
	for _, s := range spans {
		lines := strings.Split(s.Text, "\n")
		for i, l := range lines {
			if i > 0 {
				lines[i] = strings.TrimLeft(l, " ")
			}
			if i < len(lines)-1 {
				lines[i] = strings.TrimRight(lines[i], " ")
			}
		}
		s.Text = strings.Join(lines, "\n")
		if s.Text != "" {
			out = append(out, s)
		}
	}
	// Trim the ends across runs.
	for len(out) > 0 {
		out[0].Text = strings.TrimLeft(out[0].Text, " \n")
		if out[0].Text != "" {
			break
		}
		out = out[1:]
	}
	for len(out) > 0 {
		last := len(out) - 1
		out[last].Text = strings.TrimRight(out[last].Text, " \n")
		if out[last].Text != "" {
			break
		}
		out = out[:last]
	}
	for i := range out {
		out[i].Text = blankRunRe.ReplaceAllString(out[i].Text, "\n\n")
	}
	if strings.TrimSpace(spansText(out)) == "" {
		return nil
	}
	return out
}

func spansText(spans []ReaderSpan) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	return b.String()
}

// isReaderHidden matches elements styled invisible, including the preheader
// patterns that hide text by size or opacity.
func isReaderHidden(n *html.Node) bool {
	if hasAttr(n, "hidden") || isHidden(n) {
		return true
	}
	style := attr(n, "style")
	if style == "" {
		return false
	}
	var zeroHeight, overflowHidden bool
	for _, decl := range strings.Split(strings.ToLower(style), ";") {
		k, v, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "!important"))
		switch k {
		case "visibility":
			if v == "hidden" {
				return true
			}
		case "opacity", "font-size":
			if isZeroLength(v) {
				return true
			}
		case "max-height", "height":
			zeroHeight = zeroHeight || isZeroLength(v)
		case "overflow":
			overflowHidden = v == "hidden"
		}
	}
	return zeroHeight && overflowHidden
}

// isZeroLength matches CSS zero values such as "0", "0px" or "0.0".
func isZeroLength(v string) bool {
	v = strings.TrimSpace(v)
	for _, unit := range []string{"px", "pt", "em", "rem", "%"} {
		v = strings.TrimSuffix(v, unit)
	}
	return v != "" && strings.Trim(v, "0.") == ""
}

// isQuoteBlock matches quoted history containers.
func isQuoteBlock(n *html.Node) bool {
	if n.DataAtom == atom.Blockquote && strings.EqualFold(attr(n, "type"), "cite") {
		return true
	}
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == "gmail_quote" || c == "moz-cite-prefix" {
			return true
		}
	}
	return false
}

// isSmallImage matches images whose declared width or height marks an icon.
func isSmallImage(n *html.Node) bool {
	w, wok := pixelAttr(n, "width")
	h, hok := pixelAttr(n, "height")
	return wok && w < readerMinImagePx || hok && h < readerMinImagePx
}

// safeReaderHref returns raw when it is an http, https or mailto URL.
func safeReaderHref(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" {
			return ""
		}
		return u.String()
	case "mailto":
		return u.String()
	}
	return ""
}

// safeReaderImageSrc returns an http(s) image URL, or a bounded data: URI of
// a raster image type. Protocol-relative URLs become https. With keepCID a
// "cid:" source becomes "cid:<id>" (percent-decoded, angle brackets removed).
func safeReaderImageSrc(raw string, keepCID bool) string {
	raw = strings.TrimSpace(raw)
	if keepCID && len(raw) > 4 && strings.EqualFold(raw[:4], "cid:") {
		id := strings.TrimSpace(raw[4:])
		if dec, err := url.PathUnescape(id); err == nil {
			id = dec
		}
		id = strings.Trim(id, "<>")
		if id == "" || strings.ContainsAny(id, "\"'<> \t\r\n") {
			return ""
		}
		return "cid:" + id
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "data:") {
		for _, t := range []string{"data:image/png;", "data:image/jpeg;", "data:image/gif;", "data:image/webp;"} {
			if strings.HasPrefix(lower, t) && len(raw) <= readerMaxDataURI && !strings.ContainsAny(raw, "\"'<> ") {
				return raw
			}
		}
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	if s := strings.ToLower(u.Scheme); s != "http" && s != "https" {
		return ""
	}
	return u.String()
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}
