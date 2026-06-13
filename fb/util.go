package fb

import (
	"net/url"
	"strings"
)

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
