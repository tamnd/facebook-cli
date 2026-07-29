// Package fbid classifies a Facebook reference without making a request.
//
// Facebook has more shapes of identifier than any other site in this tool
// series: numeric ids for six different kinds of object, vanity handles, two
// base64 keys that decode to something useful, an opaque signed token that
// decodes to nothing, and a dozen URL routes that wrap all of them. Spec 3004
// doc 04 section 1 has the table this file implements.
//
// It is its own package because classification is useful on its own. Anybody
// with a folder of Facebook links and a question about what is in it should be
// able to import this and get an answer without pulling in an HTTP client.
//
// The one rule worth stating: a bare number is not classified. 100044561550831
// is a profile and 1587860636042640 is a post and nothing in the digits says
// which, so Parse returns KindNumeric and the caller that knows what it asked
// for coerces it. Guessing here would put wrong kinds in a graph.
package fbid

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// The kinds Parse returns.
const (
	KindProfile   = "profile"
	KindPage      = "page"
	KindPost      = "post"
	KindPhoto     = "photo"
	KindVideo     = "video"
	KindReel      = "reel"
	KindGroup     = "group"
	KindEvent     = "event"
	KindAlbum     = "album"
	KindComment   = "comment"
	KindStory     = "story"     // a base64 story key
	KindFeedback  = "feedback"  // a base64 feedback key
	KindHandle    = "handle"    // a vanity name, which names a profile
	KindNumeric   = "numeric"   // digits, and nothing says what they name
	KindOpaque    = "opaque"    // a pfbid
	KindShare     = "share"     // a short link that redirects to one of the above
	KindDirectory = "directory" // a directory index or letter page
	KindSearch    = "search"
	KindUnknown   = "unknown"
)

// Ref is what a reference names, as far as it can be known without asking
// Facebook.
//
// The fields are all optional because the shapes carry different things: a
// permalink gives an author and a post, a story key gives an author and up to
// two object ids, a pfbid gives nothing but itself.
type Ref struct {
	Input    string `json:"input"`
	Kind     string `json:"kind"`
	ID       string `json:"id,omitempty"`
	Handle   string `json:"handle,omitempty"`
	AuthorID string `json:"author_id,omitempty"`
	PostID   string `json:"post_id,omitempty"`
	PhotoID  string `json:"photo_id,omitempty"`
	VideoID  string `json:"video_id,omitempty"`
	Set      string `json:"set,omitempty"`
	Tab      string `json:"tab,omitempty"`
	Letter   string `json:"letter,omitempty"`
	Query    string `json:"query,omitempty"`
	// URL is the page fb would fetch to read this, when the reference is enough
	// to build one. A bare post id is not, which is the whole reason `fb post`
	// has an --author flag.
	URL string `json:"url,omitempty"`
	// Command is the fb command that handles this reference, which is what `fb
	// explain` prints.
	Command string `json:"command,omitempty"`
	// Opaque marks a token that is per-render and per-viewer, so it is never a
	// graph key however stable it looks.
	Opaque bool `json:"opaque,omitempty"`
	// Decoded is the plaintext behind a base64 key, kept because seeing it is
	// most of the value of decoding it.
	Decoded string `json:"decoded,omitempty"`
	Note    string `json:"note,omitempty"`
}

// Host is the host every URL Ref is built against.
const Host = "https://www.facebook.com"

// ErrEmpty is returned for a reference that is not there at all.
var ErrEmpty = errors.New("no reference given")

// Parse classifies anything: a URL, an id, a handle, a base64 key, a token.
// It never makes a request and it never fails: an unrecognised string comes
// back as KindUnknown, which is a fact about the string rather than an error.
func Parse(s string) Ref {
	raw := strings.TrimSpace(s)
	r := Ref{Input: raw, Kind: KindUnknown}
	if raw == "" {
		r.Note = "there is nothing here to classify"
		return r
	}
	if looksLikeURL(raw) {
		return parseURL(raw)
	}
	if strings.HasPrefix(raw, "@") {
		return handleRef(raw, strings.TrimPrefix(raw, "@"))
	}
	if strings.HasPrefix(raw, "pfbid") {
		r.Kind = KindOpaque
		r.Opaque = true
		r.ID = raw
		r.Note = "a pfbid is per-render and is not a stable id: resolve it by fetching the permalink"
		return r
	}
	// An album set token is "a." and the album id, and it is the only shape with
	// a dot that is not a handle.
	if rest, ok := strings.CutPrefix(raw, "a."); ok && allDigits(rest) {
		r.Kind = KindAlbum
		r.ID = rest
		r.Set = raw
		r.Command = "photos --album"
		r.URL = Host + "/media/set/?set=" + url.QueryEscape(raw)
		return r
	}
	if allDigits(raw) {
		r.Kind = KindNumeric
		r.ID = raw
		r.Note = "a number alone does not say what it names: fb page, fb post --author, fb group, fb event and fb photo all take one"
		if ProfileShaped(raw) {
			r.Note = "15 digits starting 1000 is the shape of a profile id, but only a fetch confirms it"
		}
		return r
	}
	if key, ok := decodeKey(raw); ok {
		return key
	}
	if isHandle(raw) {
		return handleRef(raw, raw)
	}
	r.Note = "this is not a shape fb recognises"
	return r
}

// ProfileShaped reports whether a numeric id has the shape Facebook gives a
// person or a Page profile: fifteen digits starting 1000. It is a hint and it
// is documented as one, because Facebook has never promised it.
func ProfileShaped(id string) bool {
	return len(id) == 15 && strings.HasPrefix(id, "1000") && allDigits(id)
}

func handleRef(input, handle string) Ref {
	return Ref{
		Input:   input,
		Kind:    KindHandle,
		Handle:  handle,
		URL:     Host + "/" + handle,
		Command: "page",
		Note:    "a handle is an alias and not an identity: it names a profile until the profile changes it",
	}
}

// decodeKey reads the two base64 shapes that carry something.
//
// A feedback key is base64 of "feedback:{post_id}". A story key is base64 of
// "S:_I{author}:{a}:{b}", where the middle segment is either the post id again
// or the literal VK, which is what Facebook writes when the story is about a
// video or an event rather than a post. Both were read out of the captures
// rather than a document.
func decodeKey(s string) (Ref, bool) {
	plain, ok := base64Any(s)
	if !ok {
		return Ref{}, false
	}
	r := Ref{Input: s, Decoded: plain}
	if id, ok := strings.CutPrefix(plain, "feedback:"); ok && allDigits(id) {
		r.Kind = KindFeedback
		r.ID = id
		r.PostID = id
		r.Command = "post --author"
		r.Note = `base64 of "feedback:` + id + `", which is the post's comment thread`
		return r, true
	}
	if body, ok := strings.CutPrefix(plain, "S:_I"); ok {
		parts := strings.Split(body, ":")
		if len(parts) < 2 || !allDigits(parts[0]) {
			return Ref{}, false
		}
		r.Kind = KindStory
		r.AuthorID = parts[0]
		r.Note = `base64 of "` + plain + `": the author id in it names a profile nobody fetched`
		r.Command = "post --author"
		switch parts[1] {
		case "VK":
			// VK is Facebook's marker for a story about a video or an event, and
			// the id after it is the video or the event, not a post.
			if len(parts) > 2 && allDigits(parts[2]) {
				r.ID = parts[2]
				r.VideoID = parts[2]
			}
			r.Command = "video"
			r.Note = `base64 of "` + plain + `": VK means the story is about a video or an event, so the last id is not a post id`
		default:
			if allDigits(parts[1]) {
				r.PostID = parts[1]
				r.ID = parts[1]
			}
			if len(parts) > 2 && allDigits(parts[2]) && parts[2] != r.PostID {
				r.PhotoID = parts[2]
			}
		}
		return r, true
	}
	return Ref{}, false
}

// base64Any decodes the four spellings Facebook uses, padded and not, standard
// and URL-safe, and reports whether the result is printable text. A blob that
// decodes to bytes is not a key, it is a coincidence.
func base64Any(s string) (string, bool) {
	if len(s) < 8 {
		return "", false
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		b, err := enc.DecodeString(s)
		if err != nil || len(b) == 0 {
			continue
		}
		if !printable(b) {
			continue
		}
		return string(b), true
	}
	return "", false
}

func printable(b []byte) bool {
	for _, c := range b {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}

func looksLikeURL(s string) bool {
	if strings.Contains(s, "://") {
		return true
	}
	lower := strings.ToLower(s)
	for _, p := range []string{"www.facebook.com/", "facebook.com/", "fb.watch/", "fb.me/", "m.facebook.com/", "web.facebook.com/", "mbasic.facebook.com/"} {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// isHandle reports whether a string is shaped like a vanity name. Facebook
// allows letters, digits, dots and hyphens, and nothing else gets a URL.
func isHandle(s string) bool {
	if s == "" || len(s) > 60 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.' || c == '-' || c == '_':
		default:
			return false
		}
	}
	return true
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Profile coerces a reference to a profile, which is what every command that
// takes a page or a person needs. A handle and a numeric id both work, and they
// take different routes: Facebook serves a number from /profile.php?id= and a
// name from /{name}, and getting that backwards is a 404 rather than a redirect.
func Profile(s string) (Ref, error) {
	r := Parse(s)
	switch r.Kind {
	case KindProfile, KindPage:
		return r, nil
	case KindHandle:
		return r, nil
	case KindNumeric:
		r.Kind = KindProfile
		r.URL = Host + "/profile.php?id=" + r.ID
		r.Command = "page"
		return r, nil
	case KindStory:
		// A story key names its author, which is a profile nobody fetched.
		if r.AuthorID != "" {
			return Ref{Input: s, Kind: KindProfile, ID: r.AuthorID, URL: Host + "/profile.php?id=" + r.AuthorID, Command: "page"}, nil
		}
	case KindOpaque:
		return r, fmt.Errorf("%s is a pfbid, which is not a profile id: use the profile's handle or its numeric id", short(s))
	}
	if r.Handle != "" {
		return r, nil
	}
	return r, fmt.Errorf("%s does not name a profile: give a handle, a numeric id, or a facebook.com profile URL", short(s))
}

// Post coerces a reference to a post, with an author when one is needed.
//
// Facebook has no route that takes a post id on its own, so a bare id with no
// author is a usage error that names both ways to fix it rather than a request
// that was always going to 404.
func Post(s, author string) (Ref, error) {
	r := Parse(s)
	switch r.Kind {
	case KindPost:
		return r, nil
	case KindFeedback:
		r.Kind = KindPost
		r.ID = r.PostID
	case KindStory:
		if r.PostID == "" {
			return r, fmt.Errorf("%s is a story key for a video or an event, not a post: try fb video", short(s))
		}
		r.Kind = KindPost
		r.ID = r.PostID
		if author == "" && r.AuthorID != "" {
			author = r.AuthorID
		}
	case KindNumeric:
		r.Kind = KindPost
	case KindOpaque:
		if author == "" {
			return r, fmt.Errorf("a pfbid names a post only next to its author: pass --author, or give the whole permalink URL")
		}
		r.Kind = KindPost
	default:
		return r, fmt.Errorf("%s does not name a post: give a permalink URL, or a post id with --author", short(s))
	}
	if r.URL == "" {
		if author == "" {
			return r, fmt.Errorf("facebook has no route that takes a post id alone: pass --author, or give the whole permalink URL")
		}
		r.AuthorID = author
		if allDigits(author) {
			r.URL = Host + "/permalink.php?story_fbid=" + r.ID + "&id=" + author
		} else {
			r.URL = Host + "/" + strings.TrimPrefix(author, "@") + "/posts/" + r.ID
		}
	}
	r.Command = "post"
	return r, nil
}

// Photo coerces a reference to a photo permalink.
func Photo(s string) (Ref, error) {
	r := Parse(s)
	switch r.Kind {
	case KindPhoto:
		return r, nil
	case KindNumeric:
		r.Kind = KindPhoto
		r.PhotoID = r.ID
		r.URL = Host + "/photo/?fbid=" + r.ID
		r.Command = "photo"
		return r, nil
	case KindStory:
		if r.PhotoID != "" {
			return Ref{Input: s, Kind: KindPhoto, ID: r.PhotoID, PhotoID: r.PhotoID, URL: Host + "/photo/?fbid=" + r.PhotoID, Command: "photo"}, nil
		}
	}
	return r, fmt.Errorf("%s does not name a photo: give a /photo/?fbid= URL or a photo id", short(s))
}

// Video coerces a reference to a video or a reel. The same media id serves both
// routes, so the caller picks the route and this only has to find the id.
func Video(s string) (Ref, error) {
	r := Parse(s)
	switch r.Kind {
	case KindVideo, KindReel:
		return r, nil
	case KindNumeric, KindStory:
		id := r.ID
		if r.VideoID != "" {
			id = r.VideoID
		}
		if id == "" {
			break
		}
		return Ref{Input: s, Kind: KindVideo, ID: id, VideoID: id, URL: Host + "/watch/?v=" + id, Command: "video"}, nil
	}
	return r, fmt.Errorf("%s does not name a video: give a /watch/?v= URL, a /reel/ URL, or a video id", short(s))
}

// Group coerces a reference to a group. A slug works signed in and a numeric id
// works either way, and the difference is not visible here: it is the fetch that
// finds out.
func Group(s string) (Ref, error) {
	r := Parse(s)
	switch r.Kind {
	case KindGroup:
		return r, nil
	case KindNumeric, KindHandle:
		id := r.ID
		if id == "" {
			id = r.Handle
		}
		return Ref{Input: s, Kind: KindGroup, ID: id, URL: Host + "/groups/" + id, Command: "group"}, nil
	}
	return r, fmt.Errorf("%s does not name a group: give a numeric group id, a slug, or a /groups/ URL", short(s))
}

// Event coerces a reference to an event.
func Event(s string) (Ref, error) {
	r := Parse(s)
	switch r.Kind {
	case KindEvent:
		return r, nil
	case KindNumeric:
		return Ref{Input: s, Kind: KindEvent, ID: r.ID, URL: Host + "/events/" + r.ID, Command: "event"}, nil
	case KindStory:
		// An event's announcement story carries the event id after the VK
		// marker, which is how an event id turns up in a payload at all.
		if r.VideoID != "" {
			return Ref{Input: s, Kind: KindEvent, ID: r.VideoID, URL: Host + "/events/" + r.VideoID, Command: "event"}, nil
		}
	}
	return r, fmt.Errorf("%s does not name an event: give a numeric event id or a /events/ URL", short(s))
}

func short(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 48 {
		return s[:45] + "..."
	}
	if s == "" {
		return "an empty reference"
	}
	return s
}
