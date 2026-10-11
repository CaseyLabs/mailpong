package carddav

import (
	"strings"
	"testing"

	"github.com/caseylabs/mailpong/internal/contact"
	"github.com/emersion/go-vcard"
)

// preserveOriginal is an Apple-style card with detail the Record doesn't model.
var preserveOriginal = strings.Join([]string{
	"BEGIN:VCARD",
	"VERSION:3.0",
	"UID:u1",
	"FN:Dr. Jane Q. Doe Jr.",
	"N:Doe;Jane;Quinn;Dr.;Jr.",
	"PHOTO;VALUE=uri:https://example.com/jane.jpg",
	"item1.EMAIL;TYPE=INTERNET,HOME,PREF:jane@example.com",
	"item1.X-ABLabel:Personal",
	"item2.EMAIL;TYPE=INTERNET:old@example.com",
	"item2.X-ABLabel:Old",
	"TEL;TYPE=CELL,VOICE,PREF:+1 555 0100",
	"ADR;TYPE=WORK:PO 9;Suite 4;1 Main St;Springfield;IL;62701;USA",
	"END:VCARD",
	"",
}, "\r\n")

func decodeCard(t *testing.T, out []byte) vcard.Card {
	t.Helper()
	card, err := vcard.NewDecoder(strings.NewReader(string(out))).Decode()
	if err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	return card
}

func TestBuildVCardPreservesUnmodeledDetail(t *testing.T) {
	base := func() *contact.Record {
		return &contact.Record{
			ID: "u1", Fn: "Dr. Jane Q. Doe Jr.", NFamily: "Doe", NGiven: "Jane",
			PhotoURL: "https://example.com/jane.jpg",
			Emails:   []contact.RecordEmail{{Email: "jane@example.com", EmailType: "internet"}},
			Phones:   []contact.RecordPhone{{Number: "+1 555 0100", PhoneType: "cell"}},
			Addresses: []contact.RecordAddress{{
				AddrType: "work", Street: "1 Main St", City: "Springfield",
				Region: "IL", Postcode: "62701", Country: "USA",
			}},
		}
	}

	tests := []struct {
		name  string
		edit  func(r *contact.Record)
		check func(t *testing.T, c vcard.Card)
	}{
		{"N middle, prefix and suffix kept", func(r *contact.Record) { r.NGiven = "Janet" }, func(t *testing.T, c vcard.Card) {
			n := c.Name()
			if n.GivenName != "Janet" || n.AdditionalName != "Quinn" || n.HonorificPrefix != "Dr." || n.HonorificSuffix != "Jr." {
				t.Errorf("N = %+v", n)
			}
		}},
		{"URI photo kept", func(*contact.Record) {}, func(t *testing.T, c vcard.Card) {
			if p := c.Get(vcard.FieldPhoto); p == nil || p.Value != "https://example.com/jane.jpg" {
				t.Errorf("PHOTO = %+v", p)
			}
		}},
		{"changed URI photo written as uri", func(r *contact.Record) { r.PhotoURL = "https://example.com/new.jpg" }, func(t *testing.T, c vcard.Card) {
			p := c.Get(vcard.FieldPhoto)
			if p == nil || p.Value != "https://example.com/new.jpg" || !strings.EqualFold(p.Params.Get("VALUE"), "uri") {
				t.Errorf("PHOTO = %+v", p)
			}
		}},
		{"unchanged email keeps types, PREF and label group", func(*contact.Record) {}, func(t *testing.T, c vcard.Card) {
			e := c.Get(vcard.FieldEmail)
			if e.Group != "item1" || !e.Params.HasType("home") || !e.Params.HasType("pref") {
				t.Errorf("EMAIL = %+v", e)
			}
			labels := c["X-ABLABEL"]
			if len(labels) != 1 || labels[0].Value != "Personal" {
				t.Errorf("X-ABLabel = %+v, want only the Personal label", labels)
			}
		}},
		{"retyped phone keeps PREF", func(r *contact.Record) { r.Phones[0].PhoneType = "work" }, func(t *testing.T, c vcard.Card) {
			p := c.Get(vcard.FieldTelephone)
			if !p.Params.HasType("work") || !p.Params.HasType("pref") || p.Params.HasType("cell") {
				t.Errorf("TEL params = %v", p.Params)
			}
		}},
		{"address keeps PO box and extended", func(r *contact.Record) {}, func(t *testing.T, c vcard.Card) {
			a := c.Addresses()[0]
			if a.PostOfficeBox != "PO 9" || a.ExtendedAddress != "Suite 4" || !a.Params.HasType("work") {
				t.Errorf("ADR = %+v params %v", a, a.Params)
			}
		}},
		{"new email gets fresh type", func(r *contact.Record) {
			r.Emails = append(r.Emails, contact.RecordEmail{Email: "new@example.com", EmailType: "work"})
		}, func(t *testing.T, c vcard.Card) {
			e := c[vcard.FieldEmail][1]
			if e.Group != "" || e.Params.Get(vcard.ParamType) != "WORK" {
				t.Errorf("new EMAIL = %+v", e)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := base()
			tt.edit(rec)
			out, err := BuildVCard(rec, preserveOriginal)
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, decodeCard(t, out))
		})
	}
}
