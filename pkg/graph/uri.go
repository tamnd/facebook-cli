// Package graph is the fb:// URI space and the claims made in it.
//
// It knows nothing about Facebook's payloads and nothing about HTTP. What it
// knows is how to name a thing so that two runs a week apart, over two different
// surfaces, agree on the name. Everything that reads Facebook lives in package
// fb and calls into here.
package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

// The kinds a URI can name. They are the same strings package fb puts in a
// Ref's Kind, which is deliberate: a ref is half a URI already.
const (
	KindProfile  = "profile"
	KindPage     = "page"
	KindPost     = "post"
	KindComment  = "comment"
	KindPhoto    = "photo"
	KindVideo    = "video"
	KindGroup    = "group"
	KindEvent    = "event"
	KindAlbum    = "album"
	KindPlace    = "place"
	KindExternal = "external"
)

// URI builds fb://kind/id, and returns "" when it will not.
//
// Three rules, in the order of how much trouble each one saves.
//
// Numeric ids only. A handle is an alias rather than an identity: NASA and nasa
// are one profile, and Facebook lets a Page change its handle, so a store keyed
// on handles gets two nodes for one Page the week somebody renames theirs.
//
// A pfbid is never a URI. It is per-render and per-viewer, so the pfbid from
// Monday and the pfbid from Tuesday for the same post are two nodes for one
// thing, and a store that does that is worse than a store with a gap in it.
// Callers with only a pfbid emit an unresolved node instead: see Unresolved.
//
// An external URL is hashed. Two posts linking the same go.nasa.gov shortlink
// converge on one node, and the key is a hex string rather than a URL with a
// query in it.
func URI(kind, id string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	id = strings.TrimSpace(id)
	if kind == "" || id == "" {
		return ""
	}
	if kind == KindExternal {
		return External(id)
	}
	if !numeric(id) {
		return ""
	}
	switch kind {
	case KindProfile, KindPage, KindPost, KindComment, KindPhoto,
		KindVideo, KindGroup, KindEvent, KindAlbum, KindPlace:
		return "fb://" + kind + "/" + id
	}
	return ""
}

// External is the URI for a link off Facebook: fb://external/{sha256 of the
// normalised URL}.
//
// Normalising first is what makes it converge. The same link arrives with and
// without a trailing slash, with http and with https, and with utm parameters
// bolted on by whoever shared it, and three spellings of one destination is
// three nodes that should have been one.
func External(raw string) string {
	n := normaliseURL(raw)
	if n == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(n))
	return "fb://external/" + hex.EncodeToString(sum[:])
}

// Split takes a URI apart again. ok is false for anything that is not one.
func Split(uri string) (kind, id string, ok bool) {
	rest, found := strings.CutPrefix(uri, "fb://")
	if !found {
		return "", "", false
	}
	kind, id, found = strings.Cut(rest, "/")
	if !found || kind == "" || id == "" {
		return "", "", false
	}
	return kind, id, true
}

// Unresolved is a node fb knows about and cannot name: a profile or a post that
// arrived carrying only a pfbid.
//
// It is a record rather than a URI because the whole point is that it has no
// URI. `fb crawl --resolve-opaque` turns these into real nodes at one request
// each, and until then they are visible as what they are instead of being
// dropped or given a name that will not survive the week.
type Unresolved struct {
	URI      *string `json:"uri"` // always nil, so the shape matches a resolved node
	OpaqueID string  `json:"opaque_id"`
	Kind     string  `json:"kind,omitempty"`
	Name     string  `json:"name,omitempty"`
	URL      string  `json:"url,omitempty"`
	Resolved bool    `json:"resolved"` // always false
}

// numeric reports whether every byte is a digit. A real Facebook id is all
// digits; a handle, a pfbid and a base64 key are not.
func numeric(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// trackingParams are the query keys stripped before hashing an external URL.
// Each one is added by the sharer rather than by the destination, so two links
// that differ only here are one link.
var trackingParams = map[string]bool{
	"utm_source": true, "utm_medium": true, "utm_campaign": true,
	"utm_term": true, "utm_content": true, "utm_id": true,
	"fbclid": true, "gclid": true, "mc_cid": true, "mc_eid": true,
	"ref": true, "ref_src": true, "ref_url": true, "s": true,
}

// normaliseURL is the spelling of a URL that gets hashed.
//
// Lowercase scheme and host, http promoted to https, the default port dropped,
// tracking parameters dropped, the fragment dropped, a bare trailing slash on
// the root dropped. What is deliberately kept is the rest of the query, because
// on plenty of sites the query is the page.
func normaliseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		// Not a URL fb can take apart. Hash what it was given rather than
		// dropping the claim: an unparseable link is still a link, and a node
		// nobody can join is better than a mention nobody can see.
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme == "http" {
		u.Scheme = "https"
	}
	u.Host = strings.ToLower(u.Host)
	u.Host = strings.TrimSuffix(u.Host, ":443")
	u.Fragment = ""
	u.RawFragment = ""
	if q := u.Query(); len(q) > 0 {
		for k := range q {
			if trackingParams[strings.ToLower(k)] {
				q.Del(k)
			}
		}
		u.RawQuery = q.Encode()
	}
	if u.Path == "/" {
		u.Path = ""
	}
	return u.String()
}
