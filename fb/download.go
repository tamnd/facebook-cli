package fb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// download.go writes media to disk: surface 6, and the only place in fb that
// creates a file the user did not name.
//
// Two things make this more than an io.Copy.
//
// A CDN URL is signed and short-lived. The `oh=` and `oe=` parameters on every
// scontent URL are a signature and an expiry, and a URL read out of a cached
// record is a URL that has probably stopped working. Rather than let that
// surface as a 403 with no explanation, a record older than staleAfter is
// refused with the instruction that fixes it.
//
// A video is large enough that the connection will sometimes drop halfway. A
// partial file that looks complete is worse than no file, so a resumed download
// asks for the range it is missing and appends, and a server that ignores the
// range header is spotted rather than trusted.

// staleAfter is how long a signed CDN URL is treated as usable.
//
// The `oe=` expiry on a scontent URL is a few hours out, but the signature stops
// working sooner than that in practice and the failure is a 403 with an empty
// body. An hour is well inside the window, and re-reading the record costs one
// request.
const staleAfter = time.Hour

// Download writes one media URL to path, resuming a partial file if there is
// one. It returns the number of bytes written by this call.
func (c *Client) Download(ctx context.Context, rawURL, path string) (int64, error) {
	if rawURL == "" {
		return 0, usage("there is no media URL on this record to download")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, fmt.Errorf("make the directory for %s: %w", path, err)
	}
	var have int64
	if fi, err := os.Stat(path); err == nil {
		if !fi.Mode().IsRegular() {
			return 0, fmt.Errorf("%s: %w", path, errNotRegular)
		}
		have = fi.Size()
	}
	if err := c.wait(ctx); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, usage("%s is not a URL fb can fetch: %v", rawURL, err)
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "*/*")
	if have > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
	}
	c.log("GET %s", rawURL)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ne := AsNetwork(err); ne != nil {
			return 0, ne
		}
		return 0, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		// The file on disk is already the whole thing.
		c.logRead(rawURL, surfaceCDN, resp.StatusCode, 0, nil)
		return 0, nil
	case resp.StatusCode == http.StatusForbidden:
		c.logRead(rawURL, surfaceCDN, resp.StatusCode, 0, nil)
		return 0, needAuth("the CDN refused %s, which is what an expired signature looks like: read the record again to get a fresh URL", path)
	case resp.StatusCode >= 400:
		c.logRead(rawURL, surfaceCDN, resp.StatusCode, 0, nil)
		return 0, asHTTPStatus(resp.StatusCode, rawURL, "")
	}

	// A server that ignores the range header answers 200 with the whole file,
	// and appending that to what is already there produces a corrupt file that
	// looks fine. So the resume only appends when the server said it resumed.
	flags := os.O_CREATE | os.O_WRONLY
	if have > 0 && resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", path, err)
	}
	n, cerr := io.Copy(f, resp.Body)
	if err := f.Close(); err != nil && cerr == nil {
		cerr = err
	}
	c.logRead(rawURL, surfaceCDN, resp.StatusCode, int(n), cerr)
	if cerr != nil {
		return n, fmt.Errorf("write %s: %w", path, cerr)
	}
	return n, nil
}

// Downloaded is one file written, for the row a download command prints.
type Downloaded struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Bytes   int64  `json:"bytes"`
	Quality string `json:"quality,omitempty"`
	Source  string `json:"source"`
	Sidecar string `json:"sidecar,omitempty"`
}

// fresh refuses a record whose signed URLs have had time to expire.
func fresh(env Envelope, what string) error {
	if env.FetchedAt.IsZero() {
		return nil
	}
	if age := time.Since(env.FetchedAt); age > staleAfter {
		return usage("this %s was read %s ago and its media URLs are signed and short-lived: run it again with --no-cache", what, age.Round(time.Minute))
	}
	return nil
}

// DownloadPhoto writes one photo and its sidecar into dir.
//
// The sidecar is the record, not a subset of it. A directory of JPEGs with no
// provenance is a directory of files nobody can cite, and the whole point of the
// envelope is that every record says which surface it came from and when.
func (e *Engine) DownloadPhoto(ctx context.Context, p Photo, dir string) (Downloaded, error) {
	if err := fresh(p.Envelope, "photo"); err != nil {
		return Downloaded{}, err
	}
	if p.Image.URI == "" {
		return Downloaded{}, noResults("photo %s carries no image URL", p.ID)
	}
	path := filepath.Join(dir, p.ID+extOf(p.Image.URI, ".jpg"))
	n, err := e.c.Download(ctx, p.Image.URI, path)
	if err != nil {
		return Downloaded{}, err
	}
	side, err := writeSidecar(path, p)
	if err != nil {
		return Downloaded{}, err
	}
	return Downloaded{ID: p.ID, Path: path, Bytes: n, Source: p.Image.URI, Sidecar: side}, nil
}

// DownloadVideo writes one video and its sidecar into dir.
//
// HD first, SD second, and the row says which arrived, because "I downloaded
// the video" and "I downloaded the 480p one because the HD URL was missing" are
// different facts. The dash manifest is not a fallback: it is a playlist, and
// writing it out as though it were the video would produce a file that plays
// nowhere.
func (e *Engine) DownloadVideo(ctx context.Context, v Video, dir string) (Downloaded, error) {
	if err := fresh(v.Envelope, "video"); err != nil {
		return Downloaded{}, err
	}
	url, quality := v.HDURL, "hd"
	if url == "" {
		url, quality = v.SDURL, "sd"
	}
	if url == "" {
		return Downloaded{}, noResults("video %s carries neither an HD nor an SD URL: the dash manifest is a playlist and not a file", v.ID)
	}
	path := filepath.Join(dir, v.ID+extOf(url, ".mp4"))
	n, err := e.c.Download(ctx, url, path)
	if err != nil {
		return Downloaded{}, err
	}
	side, err := writeSidecar(path, v)
	if err != nil {
		return Downloaded{}, err
	}
	return Downloaded{ID: v.ID, Path: path, Bytes: n, Quality: quality, Source: url, Sidecar: side}, nil
}

// writeSidecar writes the record beside the file it describes.
func writeSidecar(path string, record any) (string, error) {
	side := strings.TrimSuffix(path, filepath.Ext(path)) + ".json"
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode the sidecar for %s: %w", path, err)
	}
	if err := os.WriteFile(side, append(b, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", side, err)
	}
	return side, nil
}

// extOf works out what a CDN URL will actually serve.
//
// The path is not it. Facebook transcodes on the way out and does not rename:
// .../759547557_1027630960058911_276488844242050310_n.png?stp=dst-jpg_tt6
// answers with Content-Type image/jpeg and JPEG bytes, measured 2026-07-29. Take
// the path at its word and you write a .png that is a JPEG, which is a file half
// the tools on a machine open and the other half refuse.
//
// What is it is the `stp` parameter, where `dst-jpg` means the destination
// format is JPEG. So stp wins, the path is the fallback, and the argument is the
// fallback's fallback.
func extOf(rawURL, fallback string) string {
	if ext, ok := knownExt(dstFormat(rawURL)); ok {
		return ext
	}
	p := rawURL
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if ext, ok := knownExt(strings.TrimPrefix(filepath.Ext(p), ".")); ok {
		return ext
	}
	return fallback
}

// dstFormat pulls the format out of an stp parameter: `stp=dst-jpg_tt6` and
// `stp=dst-webp_s600x600` both name the format in the first underscore-separated
// piece, and everything after it is sizing.
func dstFormat(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	stp := u.Query().Get("stp")
	if stp == "" {
		return ""
	}
	head, _, _ := strings.Cut(stp, "_")
	f, ok := strings.CutPrefix(head, "dst-")
	if !ok {
		return ""
	}
	return f
}

// knownExt maps a format name onto the extension to write, and says no to
// anything not on the list rather than trusting a string from a URL to be a
// sensible thing to put on the end of a filename.
func knownExt(format string) (string, bool) {
	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		return ".jpg", true
	case "png":
		return ".png", true
	case "gif":
		return ".gif", true
	case "webp":
		return ".webp", true
	case "mp4":
		return ".mp4", true
	case "webm":
		return ".webm", true
	case "mov":
		return ".mov", true
	}
	return "", false
}

// errNotRegular is returned when the target path exists and is not a file, so a
// download never truncates a directory or a device node.
var errNotRegular = errors.New("the download target exists and is not a regular file")
