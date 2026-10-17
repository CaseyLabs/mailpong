package message

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ChatText is a message body reduced to what a chat bubble shows. It is
// computed on read and never persisted.
type ChatText struct {
	// Text is the new content of the message: quoted history, signature and
	// mobile footers removed. It falls back to the full body when stripping
	// would leave nothing.
	Text string `json:"text"`
	// Quoted holds the stripped quoted history and signature, in body order.
	Quoted string `json:"quoted,omitempty"`
	// HasQuoted reports whether Quoted is non-empty.
	HasQuoted bool `json:"hasQuoted"`
	// IsRich marks layout-heavy HTML (newsletters, receipts) or very long
	// text that should render as a collapsed card instead of a bubble.
	IsRich bool `json:"isRich"`
	// Reader is the sanitized content of a rich message for the chat reader
	// view; nil when the message is not rich or yields no blocks.
	Reader []ReaderBlock `json:"reader,omitempty"`
}

const (
	// richImageCount is the number of visible images that makes HTML rich.
	richImageCount = 3
	// richTextRunes is the stripped text length that makes a message rich.
	richTextRunes = 4000
	// attributionMaxLines bounds how many lines an "On … wrote:" line may wrap over.
	attributionMaxLines = 3
	// signatureMaxLines bounds the non-blank lines after a "-- " delimiter,
	// so a "--" used as a separator mid-message is not taken as a signature.
	signatureMaxLines = 12
	// headerBlockWindow is how many lines after "From:" are searched for
	// the rest of an Outlook-style header block.
	headerBlockWindow = 5
)

var (
	wroteRe          = regexp.MustCompile(`(?i)\bwrote:$`)
	originalMsgRe    = regexp.MustCompile(`(?i)^-{2,}\s*original message\s*-{2,}$`)
	headerFromRe     = regexp.MustCompile(`(?i)^\*?from:\*?\s`)
	headerFieldRe    = regexp.MustCompile(`(?i)^\*?(sent|date|to|subject):\*?\s`)
	separatorRe      = regexp.MustCompile(`^[_\-=]{8,}$`)
	forwardMarkerRe  = regexp.MustCompile(`(?i)(forwarded message|begin forwarded message)`)
	signatureDelimRe = regexp.MustCompile(`^--\s*$`)
	mobileFooterRe   = regexp.MustCompile(`(?i)^(sent from my \S.*|sent from (outlook|mail|yahoo mail)\b.*|get outlook for \S.*)$`)
	blankRunRe       = regexp.MustCompile(`\n{3,}`)
	spaceRunRe       = regexp.MustCompile(`[ \t\f\v\x{00a0}]+`)
	invisibleRe      = regexp.MustCompile(`[\x{200b}-\x{200d}\x{034f}\x{feff}\x{00ad}]`)
)

// ExtractChatText splits a message body into its new content and its quoted
// history. HTML is preferred when present; plain text is used when there is
// no HTML or the HTML has no text.
func ExtractChatText(bodyText, bodyHTML string) ChatText {
	return extractChatText(bodyText, bodyHTML, false)
}

// ExtractViewChatText is ExtractChatText for a verified or decrypted body: its
// reader blocks keep "cid:<id>" pictures, which the view resolves from the
// result's inline attachments.
func ExtractViewChatText(bodyText, bodyHTML string) ChatText {
	return extractChatText(bodyText, bodyHTML, true)
}

func extractChatText(bodyText, bodyHTML string, keepCID bool) ChatText {
	if strings.TrimSpace(bodyHTML) != "" {
		if ct, ok := chatTextFromHTML(bodyHTML, keepCID); ok {
			return ct
		}
	}
	lines := splitLines(bodyText)
	ct, _ := splitChatLines(lines, len(lines))
	if ct.IsRich = isLongText(ct.Text); ct.IsRich {
		ct.Reader = readerFromText(ct.Text)
	}
	return ct
}

// isLongText reports whether stripped text is long enough to show as a card.
func isLongText(s string) bool {
	return utf8.RuneCountInString(s) > richTextRunes
}

// chatTextFromHTML renders HTML to quote-marked lines and applies the text
// rules. ok is false when the HTML has no text.
func chatTextFromHTML(bodyHTML string, keepCID bool) (ChatText, bool) {
	doc, err := html.Parse(strings.NewReader(bodyHTML))
	if err != nil {
		return ChatText{}, false
	}
	r := &htmlRenderer{cut: -1}
	r.walk(doc)
	r.flush()
	cut := len(r.lines)
	if r.cut >= 0 {
		cut = r.cut
	}
	ct, hi := splitChatLines(r.lines, cut)
	if ct.Text == "" {
		return ChatText{}, false
	}
	if ct.IsRich = r.isRich(hi) || isLongText(ct.Text); ct.IsRich {
		ct.Reader = readerFromHTML(doc, keepCID)
	}
	return ct, true
}

// splitChatLines strips quoted history, signature and mobile footer from
// lines. Lines from cut on are already known to be quoted. It returns the end
// of the kept range so callers can tell where the new content stops.
func splitChatLines(lines []string, cut int) (ChatText, int) {
	if c := findCut(lines[:cut]); c < cut {
		cut = c
	}
	regions := quoteRegions(lines[:cut])

	first, last := -1, -1
	for i := range cut {
		if isBlank(lines[i]) || inRegions(regions, i) {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if first < 0 {
		return fullText(lines), len(lines)
	}

	lo, hi := first, last+1
	if isInline(regions, first, last) {
		lo, hi = 0, cut
	}

	// A mobile footer is dropped, unless a signature above it moves both to Quoted.
	tailStart := hi
	if f := lastNonBlank(lines, lo, hi); f > lo && mobileFooterRe.MatchString(strings.TrimSpace(lines[f])) {
		hi, tailStart = f, f+1
	}
	if s := signatureStart(lines, lo, hi); s >= 0 {
		hi, tailStart = s, s
	}

	text := joinLines(lines[lo:hi])
	if text == "" {
		return fullText(lines), len(lines)
	}
	var quoted []string
	for _, part := range [][]string{lines[:lo], lines[tailStart:]} {
		if q := joinLines(part); q != "" {
			quoted = append(quoted, q)
		}
	}
	q := strings.Join(quoted, "\n\n")
	return ChatText{Text: text, Quoted: q, HasQuoted: q != ""}, hi
}

// fullText is the fallback when stripping would leave nothing.
func fullText(lines []string) ChatText {
	return ChatText{Text: joinLines(lines)}
}

// findCut returns the first line of a trailing quoted block that is not
// marked with ">" (Outlook "Original Message" or From/Sent/To header blocks),
// or len(lines) when there is none.
func findCut(lines []string) int {
	for i, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case originalMsgRe.MatchString(t):
			return i
		case headerFromRe.MatchString(t) && isHeaderBlock(lines, i) && !afterForwardMarker(lines, i):
			return skipBackSeparator(lines, i)
		}
	}
	return len(lines)
}

// isHeaderBlock reports whether the "From:" line at i is followed by at least
// two other header fields.
func isHeaderBlock(lines []string, i int) bool {
	fields := 0
	for j := i + 1; j < len(lines) && j <= i+headerBlockWindow; j++ {
		if headerFieldRe.MatchString(strings.TrimSpace(lines[j])) {
			fields++
		}
	}
	return fields >= 2
}

// afterForwardMarker reports whether the nearest non-blank line before i is a
// forwarded-message marker, whose header block belongs to the content.
func afterForwardMarker(lines []string, i int) bool {
	p := lastNonBlank(lines, 0, i)
	return p >= 0 && forwardMarkerRe.MatchString(lines[p])
}

// skipBackSeparator moves a cut at i back over blank lines and one separator
// line (Outlook's "_____" rule) directly above it.
func skipBackSeparator(lines []string, i int) int {
	p := lastNonBlank(lines, 0, i)
	if p >= 0 && separatorRe.MatchString(strings.TrimSpace(lines[p])) {
		return p
	}
	return i
}

// quoteRegions returns [start, end) ranges of ">" quoted runs, each extended
// back over the attribution directly above it.
func quoteRegions(lines []string) [][2]int {
	var regions [][2]int
	prevEnd := 0
	for i := 0; i < len(lines); i++ {
		if !isQuote(lines[i]) {
			continue
		}
		start := attributionStart(lines, i, prevEnd)
		end := i + 1
		for j := end; j < len(lines) && (isQuote(lines[j]) || isBlank(lines[j])); j++ {
			if isQuote(lines[j]) {
				end = j + 1
			}
		}
		regions = append(regions, [2]int{start, end})
		prevEnd, i = end, end-1
	}
	return regions
}

// attributionStart extends a quote run starting at i back over its
// attribution: the nearest line above ending with ":" ("On … wrote:",
// "Le … a écrit :"), plus the lines it wraps from (an "On …" start for
// "wrote:", or lines Gmail broke before "<email>"). lo is the end of the
// previous region. It returns i when there is no attribution.
func attributionStart(lines []string, i, lo int) int {
	p := lastNonBlank(lines, lo, i)
	if p < 0 || isQuote(lines[p]) || !strings.HasSuffix(strings.TrimSpace(lines[p]), ":") {
		return i
	}
	wrote := wroteRe.MatchString(strings.TrimSpace(lines[p]))
	s := p
	for k := p - 1; k >= lo && k > p-attributionMaxLines && !isBlank(lines[k]) && !isQuote(lines[k]); k-- {
		t := strings.TrimSpace(lines[k])
		if wrote && strings.HasPrefix(t, "On ") {
			return k
		}
		if s == k+1 && strings.HasSuffix(t, "<") {
			s = k
		}
	}
	return s
}

// isInline reports whether quoted regions interleave with the new content,
// as in inline or mixed replies, where the full text must be kept.
func isInline(regions [][2]int, first, last int) bool {
	before, after := false, false
	for _, r := range regions {
		switch {
		case r[1] <= first:
			before = true
		case r[0] > last:
			after = true
		default:
			return true
		}
	}
	return before && after
}

// signatureStart returns the last "-- " delimiter in (lo, hi) followed by at
// most signatureMaxLines non-blank lines, or -1.
func signatureStart(lines []string, lo, hi int) int {
	after := 0
	for i := hi - 1; i > lo && after <= signatureMaxLines; i-- {
		if signatureDelimRe.MatchString(lines[i]) {
			return i
		}
		if !isBlank(lines[i]) {
			after++
		}
	}
	return -1
}

func inRegions(regions [][2]int, i int) bool {
	for _, r := range regions {
		if i >= r[0] && i < r[1] {
			return true
		}
	}
	return false
}

func isQuote(l string) bool {
	return strings.HasPrefix(strings.TrimLeft(l, " \t"), ">")
}

func isBlank(l string) bool {
	return strings.TrimSpace(l) == ""
}

// lastNonBlank returns the last non-blank line index in [lo, hi), or -1.
func lastNonBlank(lines []string, lo, hi int) int {
	for i := hi - 1; i >= lo; i-- {
		if !isBlank(lines[i]) {
			return i
		}
	}
	return -1
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(strings.ReplaceAll(s, "\r", "\n"), "\n")
}

// joinLines joins lines, trims trailing spaces and collapses blank runs.
func joinLines(lines []string) string {
	trimmed := make([]string, len(lines))
	for i, l := range lines {
		trimmed[i] = strings.TrimRight(l, " \t")
	}
	s := blankRunRe.ReplaceAllString(strings.Join(trimmed, "\n"), "\n\n")
	return strings.Trim(s, "\n")
}

// htmlRenderer converts HTML to text lines, prefixing blockquote content with
// "> " per nesting level and recording rich-content signals.
type htmlRenderer struct {
	lines    []string
	cur      strings.Builder
	curDepth int
	depth    int // blockquote nesting
	pre      int // <pre> nesting
	sig      int // signature block nesting
	cut      int // first line from an Outlook reply marker on, or -1
	signals  []richSignal
}

type richSignal struct {
	line  int
	table bool // layout table (else visible image)
}

// Block elements that end the current line.
var blockAtoms = map[atom.Atom]bool{
	atom.P: true, atom.Div: true, atom.Li: true, atom.Ul: true, atom.Ol: true,
	atom.Tr: true, atom.Table: true, atom.H1: true, atom.H2: true, atom.H3: true,
	atom.H4: true, atom.H5: true, atom.H6: true, atom.Hr: true, atom.Pre: true,
	atom.Section: true, atom.Article: true, atom.Header: true, atom.Footer: true,
	atom.Center: true, atom.Dl: true, atom.Dt: true, atom.Dd: true,
}

// Elements whose content is never shown.
var skipAtoms = map[atom.Atom]bool{
	atom.Head: true, atom.Script: true, atom.Style: true, atom.Title: true,
	atom.Template: true, atom.Noscript: true,
}

func (r *htmlRenderer) walk(n *html.Node) {
	if n.Type == html.TextNode {
		r.text(n.Data)
		return
	}
	if n.Type != html.ElementNode {
		r.walkChildren(n)
		return
	}
	if skipAtoms[n.DataAtom] || isHidden(n) {
		return
	}
	if r.cut < 0 && isOutlookReplyMarker(n) {
		r.flush()
		r.cut = len(r.lines)
	}

	switch n.DataAtom {
	case atom.Br:
		r.newline()
		return
	case atom.Img:
		if !isTrackingPixel(n) {
			r.signal(false)
		}
		return
	case atom.Table:
		if isLayoutTable(n) {
			r.signal(true)
		}
	case atom.Td, atom.Th:
		r.write(" ")
	}

	if isSignatureBlock(n) {
		r.sig++
		defer func() { r.sig-- }()
	}

	quote := n.DataAtom == atom.Blockquote
	if !quote && !blockAtoms[n.DataAtom] {
		r.walkChildren(n)
		return
	}
	r.flush()
	if n.DataAtom == atom.Li {
		r.write("• ")
	}
	if quote {
		r.depth++
		defer func() { r.depth-- }()
	}
	if n.DataAtom == atom.Pre {
		r.pre++
		defer func() { r.pre-- }()
	}
	r.walkChildren(n)
	r.flush()
}

func (r *htmlRenderer) walkChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		r.walk(c)
	}
}

// text appends a text node, collapsing whitespace outside <pre>.
func (r *htmlRenderer) text(s string) {
	if r.pre > 0 {
		parts := strings.Split(s, "\n")
		for i, p := range parts {
			if i > 0 {
				r.newline()
			}
			r.write(p)
		}
		return
	}
	s = invisibleRe.ReplaceAllString(s, "")
	r.write(spaceRunRe.ReplaceAllString(strings.ReplaceAll(s, "\n", " "), " "))
}

func (r *htmlRenderer) write(s string) {
	if s == "" {
		return
	}
	if r.cur.Len() == 0 {
		s = strings.TrimLeft(s, " ")
		if s == "" {
			return
		}
		r.curDepth = r.depth
	}
	r.cur.WriteString(s)
}

// flush ends the current line if it has content.
func (r *htmlRenderer) flush() {
	if r.cur.Len() == 0 {
		return
	}
	r.newline()
}

// newline ends the current line, even if empty (a <br> on its own is a blank line).
func (r *htmlRenderer) newline() {
	line := strings.TrimRight(r.cur.String(), " ")
	depth := r.curDepth
	if r.cur.Len() == 0 {
		depth = r.depth
	}
	if line != "" && depth > 0 {
		line = strings.Repeat("> ", depth) + line
	}
	r.lines = append(r.lines, line)
	r.cur.Reset()
}

// signal records a rich-content element outside quoted history and
// signatures (logo tables and images there don't make a reply rich).
func (r *htmlRenderer) signal(table bool) {
	if r.depth > 0 || r.sig > 0 || r.cut >= 0 {
		return
	}
	r.signals = append(r.signals, richSignal{line: len(r.lines), table: table})
}

// isRich reports whether a layout table or enough images occur within the new
// content, which ends at line hi.
func (r *htmlRenderer) isRich(hi int) bool {
	tables, images := 0, 0
	for _, s := range r.signals {
		// A signal at line hi sits between the last kept line and the
		// stripped part; count it only when nothing was stripped after it.
		if s.line > hi || s.line == hi && lastNonBlank(r.lines, hi, len(r.lines)) >= 0 {
			continue
		}
		if s.table {
			tables++
			continue
		}
		images++
	}
	return tables > 0 || images >= richImageCount
}

// isHidden matches elements styled invisible, such as newsletter preheaders.
func isHidden(n *html.Node) bool {
	style := attr(n, "style")
	if style == "" {
		return false
	}
	style = strings.ToLower(strings.ReplaceAll(style, " ", ""))
	return strings.Contains(style, "display:none") || strings.Contains(style, "mso-hide:all")
}

// isOutlookReplyMarker matches Outlook's reply/forward header divs, after
// which everything is quoted history.
func isOutlookReplyMarker(n *html.Node) bool {
	id := attr(n, "id")
	return id == "divRplyFwdMsg" || id == "appendonsend"
}

// isSignatureBlock matches the signature containers Gmail, Thunderbird and
// Outlook insert.
func isSignatureBlock(n *html.Node) bool {
	if id := attr(n, "id"); id == "Signature" || id == "x_Signature" {
		return true
	}
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == "gmail_signature" || c == "moz-signature" {
			return true
		}
	}
	return false
}

// isLayoutTable matches tables used for page layout rather than data:
// presentation role, full or page width, or a table nested inside.
func isLayoutTable(n *html.Node) bool {
	if strings.EqualFold(attr(n, "role"), "presentation") {
		return true
	}
	if strings.TrimSpace(attr(n, "width")) == "100%" {
		return true
	}
	if px, ok := pixelAttr(n, "width"); ok && px >= 500 {
		return true
	}
	return containsTable(n)
}

func containsTable(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == atom.Table {
			return true
		}
		if containsTable(c) {
			return true
		}
	}
	return false
}

// isTrackingPixel matches 1×1-style images that are not visible content.
func isTrackingPixel(n *html.Node) bool {
	w, wok := pixelAttr(n, "width")
	h, hok := pixelAttr(n, "height")
	return wok && w <= 2 || hok && h <= 2
}

// pixelAttr parses a numeric size attribute such as width="600" or "600px".
func pixelAttr(n *html.Node, key string) (int, bool) {
	v, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(attr(n, key)), "px"))
	return v, err == nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
