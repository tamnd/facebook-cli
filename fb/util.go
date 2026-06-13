package fb

import (
	"net/url"
	"regexp"
	"strings"
)

var phoneRe = regexp.MustCompile(`(?:\+?\d[\d\s().-]{7,}\d)`)

// findPhone pulls the first plausible phone number out of a text blob.
func findPhone(body string) string {
	if i := strings.Index(strings.ToLower(body), "call"); i >= 0 {
		window := body[i:min(i+40, len(body))]
		if m := phoneRe.FindString(window); m != "" {
			return strings.TrimSpace(m)
		}
	}
	return ""
}

// resolveRelative resolves href against base.
func resolveRelative(base, href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}
	bu, err := url.Parse(base)
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return bu.ResolveReference(ref).String()
}

// extractRedirectTarget pulls the real URL out of a Facebook l.php link wrapper.
func extractRedirectTarget(href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if t := u.Query().Get("u"); t != "" {
		if dec, derr := url.QueryUnescape(t); derr == nil {
			return dec
		}
		return t
	}
	return ""
}
