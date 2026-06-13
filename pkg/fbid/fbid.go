// Package fbid classifies and normalizes the many Facebook identifier and URL
// forms into a single typed Identity. It is pure (no network) so it can be
// imported and tested on its own; short-link resolution that needs a redirect
// lives in the fb package.
package fbid

import (
	"net/url"
	"regexp"
	"strings"
)

// Kind is the entity class an input resolves to.
type Kind string

const (
	KindUnknown Kind = "unknown"
	KindPage    Kind = "page"
	KindProfile Kind = "profile"
	KindGroup   Kind = "group"
	KindPost    Kind = "post"
	KindPhoto   Kind = "photo"
	KindVideo   Kind = "video"
	KindEvent   Kind = "event"
)

// Identity is the normalized result of classifying an input.
type Identity struct {
	Input        string `json:"input"`
	Kind         Kind   `json:"kind"`
	PageID       string `json:"page_id,omitempty"`
	ProfileID    string `json:"profile_id,omitempty"`
	GroupID      string `json:"group_id,omitempty"`
	PostID       string `json:"post_id,omitempty"`
	OwnerID      string `json:"owner_id,omitempty"`
	PhotoID      string `json:"photo_id,omitempty"`
	VideoID      string `json:"video_id,omitempty"`
	EventID      string `json:"event_id,omitempty"`
	Slug         string `json:"slug,omitempty"`
	CanonicalURL string `json:"canonical_url,omitempty"`
	MBasicURL    string `json:"mbasic_url,omitempty"`
	// ShortLink is true when the input is a fb.watch/fb.me/share link that needs
	// a redirect to fully classify; the fb package resolves it.
	ShortLink bool `json:"short_link,omitempty"`
}

var (
	numericRe  = regexp.MustCompile(`^\d+$`)
	pfbidRe    = regexp.MustCompile(`pfbid[0-9A-Za-z]+`)
	storyFbid  = regexp.MustCompile(`(?:story_fbid|fbid)=([0-9A-Za-z]+)`)
	idParam    = regexp.MustCompile(`[?&]id=(\d+)`)
	postsPath  = regexp.MustCompile(`/posts/([0-9A-Za-z]+)`)
	groupsPath = regexp.MustCompile(`/groups/([^/?&]+)`)
	videoVPar  = regexp.MustCompile(`[?&]v=(\d+)`)
	reelPath   = regexp.MustCompile(`/reel/(\d+)`)
	watchVideo = regexp.MustCompile(`/videos/(\d+)`)
	photoFbid  = regexp.MustCompile(`(?:photo\.php\?fbid=|/photo/\?fbid=|[?&]fbid=)(\d+)`)
	eventsPath = regexp.MustCompile(`/events/(\d+)`)
)

// reserved path segments that are never a page/profile slug.
var reserved = map[string]bool{
	"profile.php": true, "groups": true, "events": true, "watch": true,
	"reel": true, "photo.php": true, "photo": true, "permalink.php": true,
	"story.php": true, "media": true, "sharer": true, "sharer.php": true,
	"search": true, "marketplace": true, "gaming": true, "pages": true,
	"login": true, "login.php": true, "recover": true, "checkpoint": true,
	"help": true, "settings": true, "policies": true, "l.php": true,
}

// Classify maps any Facebook id or URL to a typed Identity without any network
// access. Short links (fb.watch, fb.me, share/...) are flagged ShortLink so the
// caller can resolve the redirect.
func Classify(input string) Identity {
	id := Identity{Input: strings.TrimSpace(input), Kind: KindUnknown}
	raw := id.Input
	if raw == "" {
		return id
	}

	// A bare numeric id is ambiguous (page or profile); default to page since
	// that is the more common public target. Callers can override.
	if numericRe.MatchString(raw) {
		id.Kind = KindPage
		id.PageID = raw
		id.Slug = raw
		fillURLs(&id, raw)
		return id
	}

	// A bare slug (no slash, no scheme) is a page or profile vanity name.
	if !strings.Contains(raw, "/") && !strings.Contains(raw, "?") && !strings.Contains(raw, ".") {
		id.Kind = KindPage
		id.Slug = raw
		id.PageID = raw
		fillURLs(&id, raw)
		return id
	}

	u := parseLoose(raw)
	host := strings.ToLower(u.Host)
	pathQuery := u.Path
	if u.RawQuery != "" {
		pathQuery += "?" + u.RawQuery
	}

	// Short links that require a redirect to classify.
	if strings.Contains(host, "fb.watch") {
		id.Kind = KindVideo
		id.ShortLink = true
		id.CanonicalURL = raw
		id.MBasicURL = raw
		return id
	}
	if strings.Contains(host, "fb.me") || strings.HasPrefix(strings.TrimPrefix(u.Path, "/"), "share/") {
		id.ShortLink = true
		id.CanonicalURL = raw
		id.MBasicURL = raw
		return id
	}

	// Photo.
	if m := photoFbid.FindStringSubmatch(pathQuery); m != nil {
		id.Kind = KindPhoto
		id.PhotoID = m[1]
		if o := idParam.FindStringSubmatch(pathQuery); o != nil {
			id.OwnerID = o[1]
		}
		fillURLs(&id, strings.TrimPrefix(u.Path, "/"))
		return id
	}

	// Event.
	if m := eventsPath.FindStringSubmatch(u.Path); m != nil {
		id.Kind = KindEvent
		id.EventID = m[1]
		fillURLs(&id, "events/"+m[1])
		return id
	}

	// Video / reel.
	if m := reelPath.FindStringSubmatch(u.Path); m != nil {
		id.Kind = KindVideo
		id.VideoID = m[1]
		fillURLs(&id, "reel/"+m[1])
		return id
	}
	if m := videoVPar.FindStringSubmatch(pathQuery); m != nil {
		id.Kind = KindVideo
		id.VideoID = m[1]
		fillURLs(&id, "watch/?v="+m[1])
		return id
	}
	if m := watchVideo.FindStringSubmatch(u.Path); m != nil {
		id.Kind = KindVideo
		id.VideoID = m[1]
		fillURLs(&id, strings.TrimPrefix(u.Path, "/"))
		return id
	}

	// Group.
	if m := groupsPath.FindStringSubmatch(u.Path); m != nil {
		id.GroupID = m[1]
		// A post inside a group.
		if pm := postsPath.FindStringSubmatch(u.Path); pm != nil {
			id.Kind = KindPost
			id.PostID = pm[1]
			id.OwnerID = m[1]
			fillURLs(&id, strings.TrimPrefix(u.Path, "/"))
			return id
		}
		id.Kind = KindGroup
		id.Slug = m[1]
		fillURLs(&id, "groups/"+m[1])
		return id
	}

	// Permalink / story with story_fbid or fbid + id.
	if m := storyFbid.FindStringSubmatch(pathQuery); m != nil {
		id.Kind = KindPost
		id.PostID = m[1]
		if o := idParam.FindStringSubmatch(pathQuery); o != nil {
			id.OwnerID = o[1]
		}
		fillURLs(&id, strings.TrimPrefix(u.Path, "/")+queryString(u))
		return id
	}

	// /<owner>/posts/<id> form.
	if m := postsPath.FindStringSubmatch(u.Path); m != nil {
		id.Kind = KindPost
		id.PostID = m[1]
		if owner := firstSegment(u.Path); owner != "" && !reserved[owner] {
			id.OwnerID = owner
		}
		fillURLs(&id, strings.TrimPrefix(u.Path, "/"))
		return id
	}

	// profile.php?id=<uid>.
	if strings.Contains(u.Path, "profile.php") {
		if o := idParam.FindStringSubmatch(pathQuery); o != nil {
			id.Kind = KindProfile
			id.ProfileID = o[1]
			fillURLs(&id, "profile.php?id="+o[1])
			return id
		}
	}

	// A pfbid token anywhere is a profile token.
	if pfbidRe.MatchString(u.Path) {
		id.Kind = KindProfile
		id.ProfileID = pfbidRe.FindString(u.Path)
		id.Slug = id.ProfileID
		fillURLs(&id, id.ProfileID)
		return id
	}

	// Fall back: first non-reserved path segment is a page/profile slug.
	if seg := firstSegment(u.Path); seg != "" && !reserved[seg] {
		id.Kind = KindPage
		id.Slug = seg
		id.PageID = seg
		fillURLs(&id, seg)
		return id
	}

	id.CanonicalURL = raw
	id.MBasicURL = ToMBasic(raw)
	return id
}

func queryString(u *url.URL) string {
	if u.RawQuery == "" {
		return ""
	}
	return "?" + u.RawQuery
}

func firstSegment(p string) string {
	p = strings.TrimPrefix(p, "/")
	if i := strings.IndexByte(p, '/'); i >= 0 {
		p = p[:i]
	}
	return strings.ToLower(p)
}

func fillURLs(id *Identity, path string) {
	path = strings.TrimPrefix(path, "/")
	id.CanonicalURL = "https://www.facebook.com/" + path
	id.MBasicURL = "https://mbasic.facebook.com/" + path
}

// parseLoose parses a URL even when the scheme is missing.
func parseLoose(raw string) *url.URL {
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		if strings.HasPrefix(raw, "/") {
			raw = "https://www.facebook.com" + raw
		} else if strings.Contains(raw, "facebook.com") || strings.Contains(raw, "fb.watch") || strings.Contains(raw, "fb.me") {
			raw = "https://" + raw
		} else {
			raw = "https://www.facebook.com/" + raw
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return &url.URL{Path: raw}
	}
	return u
}

// ToMBasic rewrites any Facebook URL to the mbasic surface, preserving path and
// query. Non-Facebook hosts are returned unchanged.
func ToMBasic(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	u := parseLoose(raw)
	if strings.Contains(u.Host, "facebook.com") {
		u.Scheme = "https"
		u.Host = "mbasic.facebook.com"
	}
	return u.String()
}

// ToMobile rewrites a Facebook URL to the m.facebook.com surface.
func ToMobile(raw string) string {
	u := parseLoose(raw)
	if strings.Contains(u.Host, "facebook.com") {
		u.Scheme = "https"
		u.Host = "m.facebook.com"
	}
	return u.String()
}

// ToCanonical rewrites a Facebook URL to the www host users expect in output.
func ToCanonical(raw string) string {
	u := parseLoose(raw)
	if strings.Contains(u.Host, "facebook.com") {
		u.Scheme = "https"
		u.Host = "www.facebook.com"
	}
	return u.String()
}
