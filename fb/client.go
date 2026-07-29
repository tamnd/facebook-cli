package fb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// client.go is the only thing in fb that touches the network.
//
// Two rules shape it, and both were measured rather than assumed (spec 3004
// doc 01 sections 1 and 3.2):
//
// Sec-Fetch-Mode: navigate is the gate. Without it facebook.com answers 400
// with a 1542-byte error page, whatever the user agent says. With it, the
// honest user agent below gets the full Comet page. So fb does not pretend to
// be a browser: it sends its own name and the header the server actually
// checks.
//
// The refusal code 1675004 arrives as HTTP 200 with the message "Rate limit
// exceeded" and is not a rate limit. It is an allowlist refusal for two
// operations, and it is never retried.

// Version is stamped by the linker at release time.
var Version = "dev"

// userAgent is the honest one. Facebook does not check it, so there is nothing
// to gain from lying and a bug report is worth more when it names the build.
func userAgent() string {
	return "fb/" + Version + " (+https://github.com/tamnd/facebook-cli)"
}

// errTooManyRedirects ends a www to web bounce. Five is generous: the honest
// chain is one hop.
var errTooManyRedirects = errors.New("too many redirects")

// Client fetches pages and replays operations.
type Client struct {
	HTTP    *http.Client
	Cache   *Cache
	Cookies string        // the raw Cookie header for Tier 1, empty at Tier 0
	Rate    time.Duration // minimum gap between requests
	Retries int
	Verbose func(format string, args ...any)

	mu   sync.Mutex
	last time.Time
	rnd  *rand.Rand
}

// NewClient builds a client with the defaults from doc 06 section 2.4: one
// request per second, two retries.
func NewClient() *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout: 45 * time.Second,
			// A redirect is often the answer, so the final URL is kept and the
			// chain is followed no further than a browser would.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errTooManyRedirects
				}
				return nil
			},
		},
		Rate:    time.Second,
		Retries: 2,
		rnd:     rand.New(rand.NewSource(1)),
	}
}

// tier reports which tier this client is operating at.
func (c *Client) tier() int {
	if c.Cookies != "" {
		return 1
	}
	return 0
}

// wait spaces requests out. One bucket across every surface, because Facebook
// counts them together and so should we.
func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	gap := c.Rate - time.Since(c.last)
	c.last = time.Now().Add(max(gap, 0))
	c.mu.Unlock()
	if gap <= 0 {
		return nil
	}
	t := time.NewTimer(gap)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// setPageHeaders sends what a navigation sends. The list is the whole gate and
// doc 01 section 1 has the bisect: dropping Sec-Fetch-Mode alone is enough to
// turn the full page into a 400.
func (c *Client) setPageHeaders(req *http.Request) {
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	// Not cosmetic: the like and RSVP counts in og:description are formatted
	// English, and the parser is pinned to that locale. Under another locale it
	// finds nothing rather than a wrong number.
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	if c.Cookies != "" {
		req.Header.Set("Cookie", c.Cookies)
	}
}

// Get fetches a page, through the cache when there is one.
func (c *Client) Get(ctx context.Context, rawURL string) (*Page, error) {
	rawURL = canonURL(rawURL)
	if c.Cache != nil {
		if e, ok := c.Cache.Get(rawURL); ok {
			c.log("cache hit %s", rawURL)
			return parsePage(rawURL, e.FinalURL, e.Status, e.Body), nil
		}
	}
	body, finalURL, status, err := c.fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if c.Cache != nil {
		c.Cache.Put(rawURL, Entry{Body: body, FinalURL: finalURL, Status: status, At: time.Now()})
	}
	return parsePage(rawURL, finalURL, status, body), nil
}

// fetch does one page request with the retry loop around it, and one fallback
// to the mirror when www and web will not stop handing the request to each
// other.
func (c *Client) fetch(ctx context.Context, rawURL string) ([]byte, string, int, error) {
	body, finalURL, status, err := c.attempt(ctx, rawURL)
	if !errors.Is(err, errTooManyRedirects) {
		return body, finalURL, status, err
	}
	mirror := mirrorOf(rawURL)
	if mirror == "" {
		return body, finalURL, status, err
	}
	c.log("redirect loop on %s, asking %s directly", rawURL, mirror)
	return c.attempt(ctx, mirror)
}

// attempt does the request with the retry loop around it.
func (c *Client) attempt(ctx context.Context, rawURL string) (body []byte, finalURL string, status int, err error) {
	for attempt := 0; ; attempt++ {
		if err := c.wait(ctx); err != nil {
			return nil, "", 0, err
		}
		body, finalURL, status, err = c.once(ctx, rawURL)
		if err == nil && status < 400 {
			return body, finalURL, status, nil
		}
		if attempt >= c.Retries || !retryable(status, err) {
			break
		}
		c.log("retry %d for %s (status %d)", attempt+1, rawURL, status)
		if err := c.backoff(ctx, attempt); err != nil {
			return nil, "", 0, err
		}
	}
	if err != nil {
		if ne := AsNetwork(err); ne != nil {
			return nil, "", 0, ne
		}
		return nil, "", 0, err
	}
	if e := asHTTPStatus(status, rawURL, string(truncate(body))); e != nil {
		return nil, "", status, e
	}
	// A 4xx that is not a 404 still carries a body worth classifying, so it is
	// handed back rather than turned into an error here.
	return body, finalURL, status, nil
}

func (c *Client) once(ctx context.Context, rawURL string) ([]byte, string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", 0, usage("%s is not a URL fb can fetch: %v", rawURL, err)
	}
	c.setPageHeaders(req)
	c.log("GET %s", rawURL)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", resp.StatusCode, err
	}
	final := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}
	return body, final, resp.StatusCode, nil
}

// retryable says whether trying again could plausibly help. A 429 and a 5xx
// could. A 400 means the request was wrong and repeating it just makes fb a
// worse citizen.
func retryable(status int, err error) bool {
	if err != nil {
		// A bounce repeats, so the same request costs the same argument again.
		// fetch has a better answer for it than waiting.
		if errors.Is(err, errTooManyRedirects) {
			return false
		}
		return AsNetwork(err) != nil
	}
	return status == http.StatusTooManyRequests || status >= 500
}

// backoff waits, with jitter, so that several fb processes started by the same
// script do not retry in lockstep.
func (c *Client) backoff(ctx context.Context, attempt int) error {
	base := time.Duration(1<<attempt) * time.Second
	c.mu.Lock()
	jitter := time.Duration(c.rnd.Int63n(int64(500 * time.Millisecond)))
	c.mu.Unlock()
	t := time.NewTimer(base + jitter)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c *Client) log(format string, args ...any) {
	if c.Verbose != nil {
		c.Verbose(format, args...)
	}
}

func truncate(b []byte) []byte {
	if len(b) > 400 {
		return b[:400]
	}
	return b
}

// pageURL builds a facebook.com URL from a path and query pairs.
func pageURL(path string, query ...string) string {
	u := url.URL{Scheme: "https", Host: canonHost, Path: path}
	if len(query) >= 2 {
		q := url.Values{}
		for i := 0; i+1 < len(query); i += 2 {
			if query[i+1] != "" {
				q.Set(query[i], query[i+1])
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// profileURL is the page for a handle or a numeric id. Facebook serves a
// numeric id from /profile.php?id= and a handle from /{handle}, and getting
// that backwards is a 404 rather than a redirect.
func profileURL(ref string) string {
	ref = strings.TrimPrefix(ref, "@")
	if allDigits(ref) {
		return pageURL("/profile.php", "id", ref)
	}
	return pageURL("/" + ref)
}

func fmtErr(format string, args ...any) error { return fmt.Errorf(format, args...) }
