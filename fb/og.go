package fb

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// opengraph.go reads surface 3: the meta tags in the head of the same HTML
// surface 1 came in.
//
// It is a small surface and it would be easy to skip, except that it is the
// only place three facts appear signed out. A Page's like and talking-about
// counts are in og:description and nowhere in the Relay store. An event's
// interested and going counts are in og:description and nowhere in the Relay
// store either. And al:ios:url carries the numeric id for a group or profile
// whose URL is a slug, which saves a resolve request.
//
// The counts here are the rendered ones, so they are rounded: an event says
// "2.3K people interested". Both the text and the number are kept, and the
// number says it is approximate rather than pretending to be exact.

// Head is what the meta tags said.
type Head struct {
	Type        string `json:"og_type,omitempty"`
	Title       string `json:"og_title,omitempty"`
	Description string `json:"og_description,omitempty"`
	URL         string `json:"og_url,omitempty"`
	Image       string `json:"og_image,omitempty"`
	ImageAlt    string `json:"og_image_alt,omitempty"`
	Locale      string `json:"og_locale,omitempty"`
	AppURI      string `json:"app_uri,omitempty"` // al:ios:url, e.g. fb://profile/100044561550831
	AppID       string `json:"app_id,omitempty"`  // the numeric id inside it
}

// parseHead reads the meta tags.
//
// Only the head is parsed, not the document: a Comet page is close to two
// megabytes and all of it after </head> is script. Cutting there turns this
// from the most expensive parse in the tool into the cheapest.
func parseHead(html []byte) Head {
	head := html
	if i := bytes.Index(html, []byte("</head>")); i > 0 {
		head = html[:i+7]
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(head))
	if err != nil {
		return Head{}
	}
	meta := map[string]string{}
	doc.Find("meta").Each(func(_ int, s *goquery.Selection) {
		content, ok := s.Attr("content")
		if !ok {
			return
		}
		// Facebook writes og: tags with property and twitter: tags with name,
		// and both matter, so both keys land in the same map.
		if k, ok := s.Attr("property"); ok {
			meta[k] = content
		}
		if k, ok := s.Attr("name"); ok {
			meta[k] = content
		}
	})
	h := Head{
		Type:        meta["og:type"],
		Title:       meta["og:title"],
		Description: meta["og:description"],
		URL:         canonURL(meta["og:url"]),
		Image:       meta["og:image"],
		ImageAlt:    meta["og:image:alt"],
		Locale:      meta["og:locale"],
		AppURI:      firstNonEmpty(meta["al:ios:url"], meta["al:android:url"]),
	}
	if h.Description == "" {
		h.Description = meta["description"]
	}
	if h.Title == "" {
		h.Title = meta["twitter:title"]
	}
	h.AppID = appURIID(h.AppURI)
	return h
}

// appURIID pulls the numeric id out of an fb:// app link. Facebook writes it
// two ways, fb://group/123 and fb://group/?id=123, so both are read.
func appURIID(uri string) string {
	if uri == "" {
		return ""
	}
	if i := strings.Index(uri, "id="); i >= 0 {
		id := uri[i+3:]
		if j := strings.IndexAny(id, "&?"); j >= 0 {
			id = id[:j]
		}
		if allDigits(id) {
			return id
		}
	}
	last := uri[strings.LastIndex(uri, "/")+1:]
	if allDigits(last) {
		return last
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// The count sentences Facebook writes into og:description. They are English on
// a locale-neutral request, which is how fb asks, and a locale that changes the
// sentence just means the count is not found rather than found wrong.
var (
	reLikes        = regexp.MustCompile(`([\d.,KMB]+) likes`)
	reTalkingAbout = regexp.MustCompile(`([\d.,KMB]+) talking about this`)
	reWereHere     = regexp.MustCompile(`([\d.,KMB]+) were here`)
	reInterested   = regexp.MustCompile(`([\d.,KMB]+) people interested`)
	reGoing        = regexp.MustCompile(`([\d.,KMB]+) people going`)
)

// approxCount reads a rendered count, which is either exact ("28,660,812") or
// rounded ("2.3K"). The bool says which, because a rounded count that is
// reported as exact is worse than no count.
func approxCount(s string) (n int, exact bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" {
		return 0, false
	}
	mult := 1
	switch s[len(s)-1] {
	case 'K':
		mult, s = 1_000, s[:len(s)-1]
	case 'M':
		mult, s = 1_000_000, s[:len(s)-1]
	case 'B':
		mult, s = 1_000_000_000, s[:len(s)-1]
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return int(f * float64(mult)), mult == 1
}

// headCount finds one count sentence in a description.
func headCount(desc string, re *regexp.Regexp) (n int, exact bool) {
	m := re.FindStringSubmatch(desc)
	if m == nil {
		return 0, false
	}
	return approxCount(m[1])
}
