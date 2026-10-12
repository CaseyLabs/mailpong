// Package carddav — vCard builder for the write path (Phase 2b.2.b).
//
// BuildVCard turns a contact.Record into a vCard byte slice ready for PUT to
// a CardDAV server. When the record has a non-empty `vcard_raw` (preserved by
// the parser on every sync), we parse it first so unknown properties survive
// the round-trip — only the standard field set is rewritten from the Record's
// current state. When `vcard_raw` is empty (e.g., a future local→carddav
// promote with no prior server-side state), we synthesize a minimal vCard
// 3.0 card from scratch.
//
// The field mapping mirrors parseVCard in client.go so build/parse round-trip
// is lossless for the standard fields.
package carddav

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/caseylabs/mailven/internal/contact"
	"github.com/emersion/go-vcard"
)

// BuildVCard renders a contact.Record into vCard wire bytes. When originalRaw
// is non-empty, unknown properties (e.g., X-FOO) in the original are preserved
// verbatim in the output. When empty, a minimal vCard 3.0 card is built.
//
// The standard field set this builder writes:
//
//	FN, N, NICKNAME, BDAY, ORG, TITLE, NOTE, CATEGORIES, EMAIL+TYPE, TEL+TYPE,
//	ADR+TYPE (structured), URL+TYPE, IMPP+TYPE, PHOTO.
//
// PHOTO is emitted inline (vCard 3.0 dialect: `PHOTO;ENCODING=b;TYPE=...:<base64>`)
// when rec.PhotoData is set, or as a URI reference when only rec.PhotoURL is
// set (the original field is reused when the URL is unchanged). When both are
// empty, the PHOTO field is deleted (so a removed photo is removed on the
// server).
//
// Detail the Record doesn't model survives an edit: N keeps its middle
// names, prefixes and suffixes, and each EMAIL/TEL/ADR/URL/IMPP whose value
// is unchanged keeps its original params and group (PREF, extra TYPEs, and
// the itemN. group an X-ABLabel is attached to). When the user changed an
// entry's type, its TYPE is replaced but a PREF type is kept.
//
// Other binary-laden fields (KEY, SOUND, LOGO) are NOT in the standard set we
// mutate; they pass through unchanged when present in originalRaw.
func BuildVCard(rec *contact.Record, originalRaw string) ([]byte, error) {
	if rec == nil {
		return nil, fmt.Errorf("BuildVCard: nil record")
	}

	card, err := startingCard(originalRaw)
	if err != nil {
		return nil, fmt.Errorf("BuildVCard: parse original: %w", err)
	}

	orig := map[string]*origFields{}
	for _, k := range []string{vcard.FieldEmail, vcard.FieldTelephone, vcard.FieldAddress, vcard.FieldURL, vcard.FieldIMPP} {
		orig[k] = &origFields{fields: card[k]}
	}
	origName := card.Name()
	origPhoto := card.Get(vcard.FieldPhoto)

	// Wipe the standard fields the Record owns; unknown fields (X-*, KEY,
	// SOUND, CATEGORIES dialect, etc.) stay because we only touch known keys.
	// PHOTO is now in the owned set — wiped + re-emitted below (or stays
	// deleted when the record has no photo, naturally removing it on save).
	for _, k := range []string{
		vcard.FieldFormattedName,
		vcard.FieldName,
		vcard.FieldNickname,
		vcard.FieldBirthday,
		vcard.FieldOrganization,
		vcard.FieldTitle,
		vcard.FieldNote,
		vcard.FieldCategories,
		vcard.FieldEmail,
		vcard.FieldTelephone,
		vcard.FieldAddress,
		vcard.FieldURL,
		vcard.FieldIMPP,
		vcard.FieldPhoto,
	} {
		delete(card, k)
	}

	// Re-populate from the Record. Use SetValue for single-value scalars;
	// Add for multi-value lists (so PREF/TYPE on the first entry is the
	// natural primary indicator).
	if fn := strings.TrimSpace(rec.Fn); fn != "" {
		card.SetValue(vcard.FieldFormattedName, fn)
	}
	if rec.NFamily != "" || rec.NGiven != "" {
		name := &vcard.Name{}
		if origName != nil {
			*name = *origName
		}
		name.FamilyName = rec.NFamily
		name.GivenName = rec.NGiven
		card.SetName(name)
	}
	if nick := strings.TrimSpace(rec.Nickname); nick != "" {
		card.SetValue(vcard.FieldNickname, nick)
	}
	if bday := strings.TrimSpace(rec.Bday); bday != "" {
		card.SetValue(vcard.FieldBirthday, bday)
	}
	if org := strings.TrimSpace(rec.Org); org != "" {
		card.SetValue(vcard.FieldOrganization, org)
	}
	if title := strings.TrimSpace(rec.Title); title != "" {
		card.SetValue(vcard.FieldTitle, title)
	}
	if note := strings.TrimSpace(rec.Note); note != "" {
		card.SetValue(vcard.FieldNote, note)
	}
	if len(rec.Categories) > 0 {
		card.SetCategories(rec.Categories)
	}

	// PHOTO — emit inline base64 in vCard 3.0 dialect when PhotoData is set,
	// else a URI reference when PhotoURL is set. Neither = no PHOTO field
	// (removes a previous photo because we wiped FieldPhoto above).
	if url := strings.TrimSpace(rec.PhotoURL); url != "" && strings.TrimSpace(rec.PhotoData) == "" {
		if origPhoto != nil && strings.TrimSpace(origPhoto.Value) == url {
			card.Add(vcard.FieldPhoto, origPhoto)
		} else {
			card.Add(vcard.FieldPhoto, &vcard.Field{
				Value:  url,
				Params: vcard.Params{"VALUE": []string{"uri"}},
			})
		}
	}
	if data := strings.TrimSpace(rec.PhotoData); data != "" {
		mediaType := strings.TrimSpace(rec.PhotoMediaType)
		// Derive vCard 3.0 TYPE param from media type ("image/jpeg" → "JPEG").
		typeSuffix := "JPEG" // safe default; most servers accept it
		if mediaType != "" {
			if i := strings.LastIndex(mediaType, "/"); i >= 0 {
				typeSuffix = strings.ToUpper(mediaType[i+1:])
			}
		}
		card.Add(vcard.FieldPhoto, &vcard.Field{
			Value: data,
			Params: vcard.Params{
				"ENCODING": []string{"b"},
				"TYPE":     []string{typeSuffix},
			},
		})
	}

	for _, e := range rec.Emails {
		val := strings.TrimSpace(e.Email)
		if val == "" {
			continue
		}
		orig[vcard.FieldEmail].add(card, vcard.FieldEmail, val, e.EmailType, strings.EqualFold)
	}
	for _, p := range rec.Phones {
		val := strings.TrimSpace(p.Number)
		if val == "" {
			continue
		}
		orig[vcard.FieldTelephone].add(card, vcard.FieldTelephone, val, p.PhoneType, equal)
	}
	for _, a := range rec.Addresses {
		if isEmptyAddress(a) {
			continue
		}
		field, src := orig[vcard.FieldAddress].entry(addressKey(a), a.AddrType, equal)
		addr := &vcard.Address{
			Field:         field,
			StreetAddress: a.Street,
			Locality:      a.City,
			Region:        a.Region,
			PostalCode:    a.Postcode,
			Country:       a.Country,
		}
		// Keep the PO box and extended address the Record doesn't model.
		if src != nil {
			if parts := strings.Split(src.Value, ";"); len(parts) >= 7 {
				addr.PostOfficeBox, addr.ExtendedAddress = parts[0], parts[1]
			}
		}
		card.AddAddress(addr)
	}
	for _, u := range rec.URLs {
		val := strings.TrimSpace(u.URL)
		if val == "" {
			continue
		}
		orig[vcard.FieldURL].add(card, vcard.FieldURL, val, u.URLType, equal)
	}
	for _, i := range rec.IMPPs {
		val := strings.TrimSpace(i.Handle)
		if val == "" {
			continue
		}
		orig[vcard.FieldIMPP].add(card, vcard.FieldIMPP, val, i.IMPPType, equal)
	}

	dropOrphanLabels(card)

	// VERSION is required by go-vcard's encoder. Default to 3.0 for broad
	// server compatibility unless the original card carried 4.0.
	if card.Value(vcard.FieldVersion) == "" {
		card.SetValue(vcard.FieldVersion, "3.0")
	}
	// UID — keep the original if present; synthesize from rec.ID when not.
	if card.Value(vcard.FieldUID) == "" && rec.ID != "" {
		card.SetValue(vcard.FieldUID, rec.ID)
	}

	var buf bytes.Buffer
	if err := vcard.NewEncoder(&buf).Encode(card); err != nil {
		return nil, fmt.Errorf("BuildVCard: encode: %w", err)
	}
	return buf.Bytes(), nil
}

// startingCard returns the parsed original or a fresh empty Card.
func startingCard(originalRaw string) (vcard.Card, error) {
	if strings.TrimSpace(originalRaw) == "" {
		return vcard.Card{}, nil
	}
	dec := vcard.NewDecoder(strings.NewReader(originalRaw))
	card, err := dec.Decode()
	if err != nil {
		return nil, err
	}
	return card, nil
}

// typeParams returns vCard Params carrying a single TYPE param, or nil when
// the type is empty. Uppercased for wire-level conventionality (vCard TYPEs
// are case-insensitive but the canonical form is upper).
func typeParams(t string) vcard.Params {
	t = strings.TrimSpace(t)
	if t == "" {
		return nil
	}
	return vcard.Params{vcard.ParamType: []string{strings.ToUpper(t)}}
}

// origFields holds one property's fields from the original card so rebuilt
// entries can reuse the params and group of the field they came from.
type origFields struct {
	fields []*vcard.Field
	used   []bool
}

// entry returns a field for value: a copy of the first unused original
// field whose value matches (an ADR matches on addressKey), with its TYPE
// replaced only when recType differs from the type the parser read from
// it, or a fresh field with recType when nothing matches. The matched
// original is returned too (nil when none matched).
func (o *origFields) entry(value, recType string, eq func(a, b string) bool) (field, src *vcard.Field) {
	if o.used == nil {
		o.used = make([]bool, len(o.fields))
	}
	for i, f := range o.fields {
		if o.used[i] || !eq(fieldKey(f), value) {
			continue
		}
		o.used[i] = true
		params := make(vcard.Params, len(f.Params))
		for k, v := range f.Params {
			params[k] = append([]string(nil), v...)
		}
		if !strings.EqualFold(firstFieldType(f), strings.TrimSpace(recType)) {
			var types []string
			if t := strings.TrimSpace(recType); t != "" {
				types = append(types, strings.ToUpper(t))
			}
			for _, t := range f.Params[vcard.ParamType] {
				if strings.EqualFold(t, "pref") {
					types = append(types, t)
				}
			}
			delete(params, vcard.ParamType)
			if len(types) > 0 {
				params[vcard.ParamType] = types
			}
		}
		return &vcard.Field{Value: value, Params: params, Group: f.Group}, f
	}
	return &vcard.Field{Value: value, Params: typeParams(recType)}, nil
}

// add appends the entry for value to card under key.
func (o *origFields) add(card vcard.Card, key, value, recType string, eq func(a, b string) bool) {
	f, _ := o.entry(value, recType, eq)
	card.Add(key, f)
}

// fieldKey is the value an original field is matched on: the trimmed value,
// or for an ADR the same key addressKey builds from a record address.
func fieldKey(f *vcard.Field) string {
	parts := strings.Split(f.Value, ";")
	if len(parts) < 7 {
		return strings.TrimSpace(f.Value)
	}
	return addressKey(contact.RecordAddress{
		Street:   strings.TrimSpace(parts[2]),
		City:     strings.TrimSpace(parts[3]),
		Region:   strings.TrimSpace(parts[4]),
		Postcode: strings.TrimSpace(parts[5]),
		Country:  strings.TrimSpace(parts[6]),
	})
}

// addressKey joins the address parts the Record models.
func addressKey(a contact.RecordAddress) string {
	return strings.Join([]string{a.Street, a.City, a.Region, a.Postcode, a.Country}, "\x00")
}

func equal(a, b string) bool { return a == b }

// dropOrphanLabels removes X-ABLabel fields whose itemN. group no longer
// holds any other field, such as the label of a deleted email.
func dropOrphanLabels(card vcard.Card) {
	const label = "X-ABLABEL"
	groups := map[string]bool{}
	for k, fields := range card {
		if k == label {
			continue
		}
		for _, f := range fields {
			if f.Group != "" {
				groups[strings.ToLower(f.Group)] = true
			}
		}
	}
	kept := card[label][:0]
	for _, f := range card[label] {
		if f.Group == "" || groups[strings.ToLower(f.Group)] {
			kept = append(kept, f)
		}
	}
	if len(kept) == 0 {
		delete(card, label)
		return
	}
	card[label] = kept
}

// isEmptyAddress reports whether all structured parts of the address are blank.
func isEmptyAddress(a contact.RecordAddress) bool {
	return a.Street == "" && a.City == "" && a.Region == "" && a.Postcode == "" && a.Country == ""
}
