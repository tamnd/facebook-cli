package fb

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// archive.go writes one read down in full: the bytes, everything extracted from
// them, the headers that were sent, and the record that came out.
//
// This is the command that produced spec 3004's evidence, and it is how a bug
// report should be filed. A report that says "the photo parser missed the album"
// is a conversation; the same report with the directory this writes attached is
// a fixture.
//
// Two rules it keeps. It never reads the cache, because the point is what
// Facebook is serving now rather than what it served twenty minutes ago. And it
// writes outside the cache directory and never expires, because an archive that
// a TTL sweeps away is not an archive.

// Archived is one capture, for the row `fb archive` prints.
type Archived struct {
	Dir      string    `json:"dir"`
	URL      string    `json:"url"`
	FinalURL string    `json:"final_url,omitempty"`
	Status   int       `json:"status"`
	Bytes    int       `json:"bytes"`
	Ops      []string  `json:"operations,omitempty"`
	Kind     string    `json:"kind,omitempty"`
	Claims   int       `json:"claims"`
	At       time.Time `json:"at"`
}

// capture is what meta.json holds: everything about the request and the
// response that is not the body.
type capture struct {
	URL      string              `json:"url"`
	FinalURL string              `json:"final_url,omitempty"`
	Status   int                 `json:"status"`
	Bytes    int                 `json:"bytes"`
	Tier     int                 `json:"tier"`
	At       time.Time           `json:"at"`
	Tool     string              `json:"tool"`
	Headers  map[string][]string `json:"request_headers"`
	Ops      []string            `json:"operations,omitempty"`
	Preloads []Preload           `json:"preloads,omitempty"`
	Head     Head                `json:"head"`
}

// Archive captures whatever ref names into dir.
func (e *Engine) Archive(ctx context.Context, ref, dir string) (Archived, error) {
	// The capture is two steps over one read: fetch the bytes, then parse them
	// through the ordinary reader so record.json is the record fb would really
	// produce. That works because the fetch writes into the cache and the parse
	// reads it back. With the cache off there is no way to hand the second step
	// the first step's bytes, so it would fetch again and the record beside
	// page.html would describe a page nobody kept.
	if e.c.Cache == nil {
		return Archived{}, usage("archive needs the cache on: it is how the parse sees the same bytes the capture kept. It never answers from the cache, so --no-cache buys nothing here")
	}
	url := archiveURL(ref)
	p, err := e.c.Fetch(ctx, url)
	if err != nil {
		return Archived{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Archived{}, err
	}
	out := Archived{
		Dir: dir, URL: p.URL, FinalURL: p.FinalURL, Status: p.Status,
		Bytes: len(p.HTML), At: time.Now().UTC(),
	}
	for op := range p.Docs {
		out.Ops = append(out.Ops, op)
	}
	slices.Sort(out.Ops)

	if err := os.WriteFile(filepath.Join(dir, "page.html"), p.HTML, 0o644); err != nil {
		return out, err
	}
	// The Relay payloads go in one file per operation rather than one big one.
	// A fixture is usually about a single operation, and a reviewer reading a
	// diff of CometPhotoRootContentQuery should not have to scroll past the
	// navigation chrome to find it.
	if len(p.Docs) > 0 {
		relay := filepath.Join(dir, "relay")
		if err := os.MkdirAll(relay, 0o755); err != nil {
			return out, err
		}
		for op, d := range p.Docs {
			if err := writeJSON(filepath.Join(relay, safeName(op)+".json"), d.Data); err != nil {
				return out, err
			}
		}
	}
	if err := writeJSON(filepath.Join(dir, "meta.json"), capture{
		URL: p.URL, FinalURL: p.FinalURL, Status: p.Status, Bytes: len(p.HTML),
		Tier: e.c.tier(), At: out.At, Tool: userAgent(),
		Headers: archiveHeaders(e.c), Ops: out.Ops, Preloads: p.Preloads, Head: p.Head,
	}); err != nil {
		return out, err
	}

	// The record last, and a failure here is not a failed archive. A page fb
	// cannot parse is the most useful thing this command ever captures, and
	// refusing to keep the bytes because the parser choked on them would throw
	// away the bug report.
	c, cerr := e.Edges(ctx, ref)
	if cerr != nil {
		return out, writeJSON(filepath.Join(dir, "record.json"), map[string]string{
			"error": cerr.Error(),
			"note":  "fb could not parse this capture, which is what the page.html beside it is for",
		})
	}
	out.Kind, out.Claims = c.Kind, len(c.Edges)
	return out, writeJSON(filepath.Join(dir, "record.json"), c)
}

// archiveURL is the URL a reference points at. A bare handle or id has no URL
// of its own, so it becomes the profile page, which is what it names.
func archiveURL(ref string) string {
	if r := fbid.Parse(ref); r.URL != "" {
		return r.URL
	}
	return profileURL(ref)
}

// archiveHeaders is the header set a page request sends, recorded as sent.
//
// The cookie is the exception and it is dropped. An archive is a thing people
// attach to bug reports, and a session cookie in one is a Facebook account
// handed to a stranger. That it was sent at all is in the tier field.
func archiveHeaders(c *Client) map[string][]string {
	req, err := http.NewRequest(http.MethodGet, "https://"+canonHost+"/", nil)
	if err != nil {
		return nil
	}
	c.setPageHeaders(req)
	out := map[string][]string{}
	for k, v := range req.Header {
		if strings.EqualFold(k, "Cookie") {
			out[k] = []string{"(redacted: an archive is a thing people attach to bug reports)"}
			continue
		}
		out[k] = v
	}
	return out
}

// ArchiveDir is where a capture goes when nobody passes --dir: one directory
// per capture, named for what was captured and when, so two captures of the
// same page a week apart sit beside each other and can be diffed.
func ArchiveDir(dataDir, ref string, at time.Time) string {
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}
	return filepath.Join(dataDir, "archive", at.UTC().Format("20060102-150405")+"-"+safeName(ref))
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// safeName turns a reference into something that is a filename on every system
// this runs on.
func safeName(s string) string {
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, canonHost)
	s = strings.Trim(s, "/")
	if s == "" {
		s = "page"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 80 {
		out = out[:80]
	}
	if out == "" {
		return "page"
	}
	return out
}
