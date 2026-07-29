package fb

import (
	"net/url"
	"strings"
)

// urls.go normalises every URL that comes out of a payload.
//
// Facebook answers a signed-out request from www.facebook.com and then writes
// web.facebook.com into every URL inside the payload, and m.facebook.com into
// mobileUrl. Left alone that gives records whose url fields point at a host the
// reader never asked for, and a graph whose keys split across two hosts for one
// object.

// canonHost is the host every facebook.com URL is rewritten to.
const canonHost = "www.facebook.com"

// fbHosts are the hosts that mean facebook.com.
var fbHosts = map[string]bool{
	"www.facebook.com":    true,
	"web.facebook.com":    true,
	"m.facebook.com":      true,
	"mbasic.facebook.com": true,
	"facebook.com":        true,
	"free.facebook.com":   true,
	"d.facebook.com":      true,
}

// trackingParams are stripped from every facebook.com URL. They are per-render
// and per-viewer, so keeping them makes two reads of one object look like two
// objects and puts a click trail in the store.
var trackingParams = map[string]bool{
	"__cft__[0]": true, "__cft__[1]": true, "__tn__": true,
	"fref": true, "mibextid": true, "rdid": true, "ref": true,
	"refsrc": true, "hc_ref": true, "share_url": true, "eav": true,
	"paipv": true, "_rdr": true, "notif_t": true, "notif_id": true,
	"acontext": true, "source": true, "sfnsn": true, "wtsid": true,
	"idorvanity": true, "extid": true, "locale2": true,
}

// keepParams survive normalisation because they identify the object rather than
// the click that reached it.
var keepParams = map[string]bool{
	"fbid": true, "set": true, "story_fbid": true, "id": true,
	"v": true, "comment_id": true, "reply_comment_id": true,
	"type": true, "tab": true, "q": true, "sk": true,
}

// canonURL rewrites a facebook.com URL to the canonical host with the tracking
// parameters removed. A URL on another host is returned as it came, because
// rewriting a CDN or an external link would be a lie about what was fetched.
func canonURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	if !fbHosts[strings.ToLower(u.Host)] {
		return raw
	}
	u.Host = canonHost
	u.Scheme = "https"
	if q := u.Query(); len(q) > 0 {
		out := url.Values{}
		for k, vs := range q {
			if keepParams[k] {
				out[k] = vs
				continue
			}
			if trackingParams[k] || strings.HasPrefix(k, "__cft__") {
				continue
			}
			// An unrecognised parameter is kept. Dropping the ones we do not
			// know would quietly lose a route parameter the next time Facebook
			// adds one.
			out[k] = vs
		}
		u.RawQuery = out.Encode()
	}
	u.Fragment = ""
	return u.String()
}

// absURL resolves an href against facebook.com and then canonicalises it.
//
// The directory is server-rendered markup from an older Facebook and its
// anchors are a mix of absolute web.facebook.com URLs and bare paths, so both
// have to come out the same for a row to be comparable to itself.
func absURL(href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
		return ""
	}
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		href = "https://" + canonHost + href
	}
	return canonURL(href)
}

// unshim turns an l.facebook.com/l.php redirect into the destination it wraps.
//
// Payloads usually carry the destination alongside as external_url, and the
// parsers prefer that. This is the fallback for the places that ship only the
// shim, and for a URL a reader passes in off a page they copied.
func unshim(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	h := strings.ToLower(u.Host)
	if h != "l.facebook.com" && h != "lm.facebook.com" && h != "l.messenger.com" {
		return raw
	}
	if dst := u.Query().Get("u"); dst != "" {
		return dst
	}
	return raw
}

// handleOf reads the vanity handle out of a profile URL, or "" when the URL
// names the profile by id instead.
func handleOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || !fbHosts[strings.ToLower(u.Host)] {
		return ""
	}
	p := strings.Trim(u.Path, "/")
	if p == "" || strings.Contains(p, "/") {
		return ""
	}
	if p == "profile.php" || allDigits(p) {
		return ""
	}
	return p
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
