package fb

import (
	"bytes"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// directory.go parses the Pages directory, which is the only surface in this
// tool with no Relay payload at all.
//
// /directory/pages/ is a plain server-rendered document from an older Facebook:
// a list of anchors, no data-sjs blocks, nothing to stitch. So this is the one
// parser that reads markup, and it reads it with goquery rather than a regex
// because the rows are nested and an anchor list is exactly what goquery is
// good at.
//
// What the directory will actually give you, checked in July 2026: the index
// page answers and lists A to Z, and every letter page under it answers 200
// with the body "This message contains content that has been blocked by our
// security systems". That is true for /directory/pages/, /directory/people/ and
// /directory/places/ alike, and it is true with a browser user agent, so it is
// not the honest-UA gate that surface 1 has. The index is real and the rows
// behind it are not reachable signed out. `fb directory` says so rather than
// returning an empty list, because an empty list reads as "Facebook has no
// pages starting with A".

// Directory is one page of the Pages directory: the letter index, and the rows
// behind it when they are reachable.
type Directory struct {
	Envelope
	URL     string           `json:"url"`
	Letter  string           `json:"letter,omitempty"`
	Index   []Tab            `json:"index,omitempty"`
	Entries []DirectoryEntry `json:"entries,omitempty"`
	Blocked bool             `json:"blocked,omitempty"`
}

// parseDirectory reads a directory page.
func parseDirectory(url string, html []byte) Directory {
	d := Directory{URL: canonURL(url), Letter: directoryLetter(url)}
	d.addSurface(surfaceDirectory)
	if bytes.Contains(html, []byte(blockMarker)) {
		d.Blocked = true
		d.miss(surfaceDirectory, "Facebook served its security interstitial for this letter")
		return d
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return d
	}
	seenIndex := map[string]bool{}
	seenEntry := map[string]bool{}
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		name := strings.TrimSpace(a.Text())
		if name == "" {
			return
		}
		u := absURL(href)
		switch {
		case isDirectoryURL(u):
			// A locale switcher points at the same directory on a different
			// host, so an index link only counts when it stays on ours.
			if !strings.HasPrefix(u, "https://www.facebook.com/") || seenIndex[u] {
				return
			}
			seenIndex[u] = true
			d.Index = append(d.Index, Tab{Name: name, URL: u})
		case isProfileURL(u):
			if seenEntry[u] {
				return
			}
			seenEntry[u] = true
			d.Entries = append(d.Entries, DirectoryEntry{
				Name:   name,
				URL:    u,
				ID:     directoryID(u),
				Letter: d.Letter,
			})
		}
	})
	return d
}

// directoryLetter pulls the letter off a directory URL, so a row knows which
// page it came from.
func directoryLetter(url string) string {
	i := strings.Index(url, "/directory/")
	if i < 0 {
		return ""
	}
	parts := strings.Split(strings.Trim(url[i+len("/directory/"):], "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func isDirectoryURL(u string) bool { return strings.Contains(u, "/directory/") }

// isProfileURL reports whether a URL names a page rather than Facebook's own
// chrome.
//
// The directory's footer and header are anchors on facebook.com too, so a
// prefix test is not enough. Everything Facebook itself owns lives under a
// known path, and a row is what is left.
func isProfileURL(u string) bool {
	const host = "https://www.facebook.com/"
	if !strings.HasPrefix(u, host) {
		return false
	}
	path := strings.TrimPrefix(u, host)
	if path == "" || strings.HasPrefix(path, "?") || strings.HasPrefix(path, "#") {
		return false
	}
	first := path
	if i := strings.IndexAny(first, "/?#"); i >= 0 {
		first = first[:i]
	}
	switch first {
	case "login", "reg", "r.php", "recover", "lite", "watch", "help", "policies",
		"privacy", "careers", "ad_campaign", "settings", "allactivity", "legal",
		"terms", "cookies", "business", "gaming", "marketplace", "groups",
		"events", "directory", "pages", "profile.php", "people", "places",
		"bookmarks", "campaign", "notes", "video", "games", "fundraisers":
		return false
	}
	return true
}

// directoryID reads the numeric id out of a /pages/name/123 URL. A modern
// vanity URL has no id in it, and an entry without one is still a valid row:
// the URL is the key.
func directoryID(u string) string {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(u, "https://www.facebook.com/"), "/"), "/")
	last := parts[len(parts)-1]
	if last == "" || !allDigits(last) {
		return ""
	}
	return last
}
