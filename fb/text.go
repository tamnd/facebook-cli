package fb

import (
	"unicode/utf16"
)

// text.go parses TextWithEntities, which is every piece of user text on
// Facebook: a post body, a comment, an event description, a bio, a context row.
//
// The object is a string plus six kinds of range. Two carry data worth keeping:
// ranges, which holds mentions and links with the entity behind each, and
// inline_style_ranges, which is how a category line marks which half is the
// category. The other four were empty on every capture; they are read and
// dropped, and the census reports it if one ever arrives with something in it.

// Text is a string with what Facebook knows about the things inside it.
type Text struct {
	Text     string  `json:"text"`
	Ranges   []Range `json:"ranges,omitempty"`
	Mentions []Ref   `json:"mentions,omitempty"`
	Links    []Link  `json:"links,omitempty"`
	Styles   []Style `json:"styles,omitempty"`
}

// Range is one span of the text with the entity it points at.
type Range struct {
	Offset int  `json:"offset"`
	Length int  `json:"length"`
	Entity *Ref `json:"entity,omitempty"`
}

// Link is an external destination named in the text, already unwrapped from the
// l.facebook.com redirect shim.
type Link struct {
	URL     string `json:"url"`
	Display string `json:"display,omitempty"`
	Offset  int    `json:"offset"`
	Length  int    `json:"length"`
}

// Style is a formatting span: BOLD, ITALIC and friends.
type Style struct {
	Style  string `json:"style"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

// Empty reports whether there is nothing here worth emitting.
func (t Text) Empty() bool { return t.Text == "" && len(t.Ranges) == 0 }

// parseText reads a TextWithEntities object.
func parseText(v any) Text {
	m, _ := v.(map[string]any)
	if m == nil {
		return Text{}
	}
	t := Text{Text: digStr(m, "text")}
	units := utf16.Encode([]rune(t.Text))
	for _, r := range digMaps(m, "ranges") {
		off, ln := digInt(r, "offset"), digInt(r, "length")
		ent := digMap(r, "entity")
		rng := Range{Offset: off, Length: ln}
		if ent != nil {
			ref := parseRef(ent)
			rng.Entity = &ref
			switch digStr(ent, "__typename") {
			case "ExternalUrl":
				dst := externalURL(ent)
				// The ref built from the entity carries the shim, because that
				// is what the url field said. The destination is the fact worth
				// keeping, so it replaces it on both the ref and the link.
				ref.URL = dst
				rng.Entity = &ref
				t.Links = append(t.Links, Link{
					URL:     dst,
					Display: slice16(units, off, ln),
					Offset:  off,
					Length:  ln,
				})
			case "User", "Page", "Group", "Event":
				t.Mentions = append(t.Mentions, ref)
			}
		}
		t.Ranges = append(t.Ranges, rng)
	}
	for _, s := range digMaps(m, "inline_style_ranges") {
		t.Styles = append(t.Styles, Style{
			Style:  digStr(s, "inline_style"),
			Offset: digInt(s, "offset"),
			Length: digInt(s, "length"),
		})
	}
	return t
}

// externalURL finds the destination behind an ExternalUrl entity.
//
// The name external_url promises the unwrapped destination and does not deliver
// it: on the NASA intro tiles all three of external_url, web_link.url and url
// were the same l.facebook.com shim, differing only in the per-render signature
// after h=. So every candidate is unshimmed and the first one that comes out
// somewhere other than the redirector wins. The shim is never stored: the
// signature is per-render and per-viewer, so keeping it makes two reads of one
// link look like two links.
func externalURL(ent map[string]any) string {
	var shim string
	for _, raw := range []string{
		digStr(ent, "external_url"),
		digStr(ent, "web_link", "url"),
		digStr(ent, "url"),
	} {
		if raw == "" {
			continue
		}
		if u := unshim(raw); u != raw {
			return u
		}
		if shim == "" {
			shim = raw
		}
	}
	return shim
}

// slice16 cuts a span out of text using UTF-16 code units, which is what
// Facebook's offsets count.
//
// Counting runes agrees with counting code units right up until an emoji
// appears, and Facebook text is full of emoji. Getting this wrong shifts every
// mention after the first emoji by one.
func slice16(units []uint16, offset, length int) string {
	if offset < 0 || length <= 0 || offset >= len(units) {
		return ""
	}
	end := offset + length
	if end > len(units) {
		end = len(units)
	}
	return string(utf16.Decode(units[offset:end]))
}
