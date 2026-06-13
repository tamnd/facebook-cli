package fb

import (
	"encoding/json"
	"regexp"
	"strings"
)

// normalizeCookie collapses whitespace in a raw Cookie header value.
func normalizeCookie(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "Cookie:")
	s = strings.TrimPrefix(s, "cookie:")
	return strings.TrimSpace(s)
}

// parseCookieFile detects the cookie file format and returns a Cookie header
// value. Three formats are accepted: a raw header line, a Netscape cookies.txt,
// and a JSON array exported by a browser extension.
func parseCookieFile(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	// JSON export: [{"name":"c_user","value":"..."}, ...].
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		if h := parseJSONCookies(trimmed); h != "" {
			return h
		}
	}
	// Netscape cookies.txt: tab-separated lines, # comments.
	if looksNetscape(trimmed) {
		return parseNetscapeCookies(trimmed)
	}
	// Otherwise treat the whole thing as a raw header.
	return normalizeCookie(firstLine(trimmed))
}

type jsonCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
}

func parseJSONCookies(s string) string {
	var arr []jsonCookie
	if err := json.Unmarshal([]byte(s), &arr); err == nil && len(arr) > 0 {
		return joinCookies(arr)
	}
	var one jsonCookie
	if err := json.Unmarshal([]byte(s), &one); err == nil && one.Name != "" {
		return one.Name + "=" + one.Value
	}
	return ""
}

func joinCookies(arr []jsonCookie) string {
	var parts []string
	for _, c := range arr {
		if c.Domain != "" && !strings.Contains(c.Domain, "facebook.com") {
			continue
		}
		if c.Name == "" {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

var netscapeLine = regexp.MustCompile(`^[^\t]+\t[^\t]*\t[^\t]*\t[^\t]*\t[^\t]*\t[^\t]+\t`)

func looksNetscape(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if netscapeLine.MatchString(line+"\t") || strings.Count(line, "\t") >= 6 {
			return true
		}
		return false
	}
	return false
}

func parseNetscapeCookies(s string) string {
	var parts []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		if !strings.Contains(fields[0], "facebook.com") {
			continue
		}
		name, value := fields[5], fields[6]
		if name == "" {
			continue
		}
		parts = append(parts, name+"="+value)
	}
	return strings.Join(parts, "; ")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// cookieUser extracts the c_user value (the acting uid) from a cookie header.
func cookieUser(header string) string {
	for _, p := range strings.Split(header, ";") {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "c_user=") {
			return strings.TrimPrefix(p, "c_user=")
		}
	}
	return ""
}
