package fb

import (
	"bytes"
	"fmt"
	"strings"
)

// page.go is one fetched Comet page, both of its planes, and what it says about
// itself.
//
// Surface 1 answers 200 for a page that does not exist, so the HTTP status is
// not the not-found signal and a client that trusts it emits empty records for
// deleted posts. Spec 3004 doc 02 section 9 sets the order this file follows:
// the final URL first, then which operations came back, then the errors array,
// and the status code last.

// Page is a fetched HTML page with its Relay documents and its meta head.
type Page struct {
	URL      string // the URL that was requested
	FinalURL string // where the redirects ended, which is often the answer
	Status   int
	HTML     []byte
	Docs     map[string]*Document
	Head     Head
	Preloads []Preload // the operations this page shipped, for replay
}

// parsePage runs both planes over fetched bytes.
func parsePage(url, finalURL string, status int, html []byte) *Page {
	blocks := dataSJS(html)
	decoded := decodeBlocks(blocks)
	return &Page{
		URL:      url,
		FinalURL: finalURL,
		Status:   status,
		HTML:     html,
		Docs:     documents(relayPayloads(decoded)),
		Head:     parseHead(html),
		Preloads: preloads(decoded),
	}
}

// has reports whether the page shipped an operation's results.
func (p *Page) has(op string) bool {
	if p == nil {
		return false
	}
	d, ok := p.Docs[op]
	return ok && d != nil && len(d.Data) > 0
}

// ops lists the operations the page shipped, for `fb explain` and for the
// message on a page fb could not read.
func (p *Page) ops() []string {
	out := make([]string, 0, len(p.Docs))
	for op := range p.Docs {
		out = append(out, op)
	}
	return out
}

// loginWall reports whether Facebook answered with the log-in page.
//
// It is not an error page: it is a 200 with a real Comet payload in it, whose
// only operation is the log-in form. Treated as a parse failure it looks like
// "this profile has no name"; treated as what it is, it is exit 4.
func (p *Page) loginWall() bool {
	if p == nil {
		return false
	}
	if strings.Contains(p.FinalURL, "/login") || strings.Contains(p.FinalURL, "/checkpoint") {
		return true
	}
	if len(p.Docs) == 0 {
		return false
	}
	// The log-in query rides along on every signed-out page, so it only means a
	// wall when it is the only thing that came back.
	for op := range p.Docs {
		if op != "useCometLogInFormQuery" && op != "CometHomeRootQuery" {
			return false
		}
	}
	return true
}

// blockMarker is the sentence Facebook's security interstitial renders. It is
// matched on the body rather than on a status code because the interstitial
// answers 200.
const blockMarker = "blocked by our security systems"

// blocked reports whether Facebook answered with its security interstitial.
//
// It is a 200 with no Relay payload and one sentence in the body. Every letter
// page of every directory answers this way signed out, and without this check
// it looks like a page that parsed to nothing.
func (p *Page) blocked() bool {
	return p != nil && bytes.Contains(p.HTML, []byte(blockMarker))
}

// classify turns a fetched page into the error it represents, or nil when the
// page is readable.
//
// wanted names the operations the caller needs. A page that shipped none of
// them and no log-in wall either is a not-found, whatever its status was.
func (p *Page) classify(what string, wanted ...string) error {
	if p == nil {
		return notFound(what, "no response")
	}
	if p.blocked() {
		return needAuth("Facebook blocked the request for %s: import a session with `fb auth import`", what)
	}
	if p.loginWall() {
		return needAuth("Facebook answered every request for %s with the log-in page: import a session with `fb auth import`", what)
	}
	for _, op := range wanted {
		if p.has(op) {
			return nil
		}
	}
	if len(wanted) == 0 && len(p.Docs) > 0 {
		return nil
	}
	// A deleted or private object redirects to the home page or to a "content
	// not available" route, and that redirect is the clearest signal there is.
	if p.FinalURL != "" && p.FinalURL != p.URL {
		switch {
		case strings.HasSuffix(p.FinalURL, "facebook.com/"),
			strings.Contains(p.FinalURL, "/content_not_found"),
			strings.Contains(p.FinalURL, "unsupportedbrowser"):
			return notFound(what, "the page redirected to "+p.FinalURL)
		}
	}
	if p.Status >= 400 {
		return notFound(what, fmt.Sprintf("HTTP %d", p.Status))
	}
	if len(p.Docs) == 0 {
		return notFound(what, "the page carried no Relay results")
	}
	return notFound(what, "the page shipped "+strings.Join(p.ops(), ", ")+" and none of the operations "+what+" needs")
}
