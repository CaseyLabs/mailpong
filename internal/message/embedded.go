package message

import "github.com/caseylabs/mailpong/internal/cid"

// MarkEmbedded sets Embedded on the inline parts whose Content-ID bodyHTML
// embeds as an image source (src="cid:<id>"), the references the viewer
// resolves. Those render in the body, so they are neither listed as
// attachments nor earn the paperclip. Non-inline parts and inline parts the
// body never shows stay listed. It reports whether any attachment is listed.
func MarkEmbedded(atts []*Attachment, bodyHTML string) bool {
	embedded := cid.Embedded(bodyHTML)
	listed := false
	for _, att := range atts {
		_, ok := embedded[att.ContentID]
		att.Embedded = ok && att.IsInline
		listed = listed || !att.Embedded
	}
	return listed
}
