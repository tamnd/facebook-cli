package fb

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	countRe    = regexp.MustCompile(`([\d][\d.,]*)\s*([KkMmBb])?`)
	wsRe       = regexp.MustCompile(`\s+`)
	relTimeRe  = regexp.MustCompile(`(?i)(\d+)\s*(m|min|mins|minute|minutes|h|hr|hrs|hour|hours|d|day|days|w|wk|wks|week|weeks|mo|month|months|y|yr|yrs|year|years)\s*(ago)?`)
	justNowRe  = regexp.MustCompile(`(?i)^just now|^a few seconds`)
	absDateFmt = []string{
		"January 2, 2006 at 3:04 PM",
		"January 2 at 3:04 PM",
		"January 2, 2006",
		"2 January 2006 at 15:04",
		"2 January at 15:04",
		"Jan 2, 2006",
		"Jan 2 at 3:04 PM",
		"01/02/2006",
		"2006-01-02",
	}
)

// cleanText collapses whitespace and trims.
func cleanText(s string) string {
	return strings.TrimSpace(wsRe.ReplaceAllString(s, " "))
}

// truncate cuts a string to n runes.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// parseCount parses Facebook's human counters ("1.2K", "3.4M", "1,234",
// "12 likes") into an int64. Returns 0 when nothing parses.
func parseCount(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	m := countRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	num := strings.ReplaceAll(m[1], ",", "")
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(m[2]) {
	case "k":
		f *= 1e3
	case "m":
		f *= 1e6
	case "b":
		f *= 1e9
	}
	return int64(f)
}

// parseTime turns a Facebook timestamp string into a best-effort time.Time. It
// handles relative ("3h", "2 days ago", "just now") and absolute forms. The
// reference now is supplied so the function stays testable; callers pass
// time.Now(). A zero time means nothing parsed.
func parseTime(s string, now time.Time) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if justNowRe.MatchString(s) {
		return now
	}
	low := strings.ToLower(s)
	switch {
	case strings.HasPrefix(low, "yesterday"):
		return now.AddDate(0, 0, -1)
	case strings.HasPrefix(low, "today"):
		return now
	}
	if m := relTimeRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		switch unitClass(m[2]) {
		case "minute":
			return now.Add(-time.Duration(n) * time.Minute)
		case "hour":
			return now.Add(-time.Duration(n) * time.Hour)
		case "day":
			return now.AddDate(0, 0, -n)
		case "week":
			return now.AddDate(0, 0, -7*n)
		case "month":
			return now.AddDate(0, -n, 0)
		case "year":
			return now.AddDate(-n, 0, 0)
		}
	}
	for _, f := range absDateFmt {
		if t, err := time.Parse(f, s); err == nil {
			if t.Year() == 0 {
				t = t.AddDate(now.Year(), 0, 0)
			}
			return t
		}
	}
	return time.Time{}
}

func unitClass(u string) string {
	u = strings.ToLower(u)
	switch {
	case strings.HasPrefix(u, "mo"):
		return "month"
	case strings.HasPrefix(u, "m"):
		return "minute"
	case strings.HasPrefix(u, "h"):
		return "hour"
	case strings.HasPrefix(u, "d"):
		return "day"
	case strings.HasPrefix(u, "w"):
		return "week"
	case strings.HasPrefix(u, "y"):
		return "year"
	}
	return ""
}

// firstNonEmpty returns the first non-empty trimmed string.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

// findCountNear looks for "<number> <label>" or "<label> <number>" near a label
// in a blob of text.
func findCountNear(body, label string) int64 {
	low := strings.ToLower(body)
	idx := strings.Index(low, strings.ToLower(label))
	if idx < 0 {
		return 0
	}
	start := idx - 24
	if start < 0 {
		start = 0
	}
	window := body[start:idx]
	if m := countRe.FindAllString(window, -1); len(m) > 0 {
		return parseCount(m[len(m)-1])
	}
	// try after the label
	end := idx + len(label) + 24
	if end > len(body) {
		end = len(body)
	}
	after := body[idx+len(label) : end]
	return parseCount(after)
}

// attr returns a node attribute or "".
func attr(s *goquery.Selection, name string) string {
	v, _ := s.Attr(name)
	return v
}
