package fb

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Client speaks the Facebook consumer web surfaces (mbasic by default, m. for
// GraphQL enrichment), rate-limited, cookie-aware, and resilient.
type Client struct {
	cfg     Config
	http    *http.Client
	cookie  string
	cache   *Cache
	uas     []string
	mu      sync.Mutex
	lastReq time.Time
}

// NewClient builds a Client from cfg, resolving cookies and the proxy.
func NewClient(cfg Config) (*Client, error) {
	cookie, err := resolveCookie(cfg)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		MaxIdleConns:        16,
		MaxConnsPerHost:     cfg.Workers + 2,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	if cfg.Proxy != "" {
		pu, perr := url.Parse(cfg.Proxy)
		if perr != nil {
			return nil, codeErr(ExitUsage, "bad proxy URL: %v", perr)
		}
		transport.Proxy = http.ProxyURL(pu)
	}
	uas := userAgents
	if cfg.UserAgent != "" {
		uas = []string{cfg.UserAgent}
	}
	return &Client{
		cfg:    cfg,
		cookie: cookie,
		uas:    uas,
		http: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
		},
		cache: NewCache(cfg.CacheDir, !cfg.NoCache, cfg.CacheTTL),
	}, nil
}

// Whoami reports the acting user id from the cookie, if any.
func (c *Client) Whoami() (uid string, authenticated bool) {
	uid = cookieUser(c.cookie)
	return uid, uid != ""
}

// Authenticated reports whether a session cookie is present.
func (c *Client) Authenticated() bool { return c.cookie != "" }

// surfaceHost rewrites a Facebook URL to the configured surface.
func (c *Client) surfaceURL(raw string) string {
	switch c.cfg.Surface {
	case SurfaceMobile:
		return fbid.ToMobile(raw)
	default:
		return fbid.ToMBasic(raw)
	}
}

// GetDoc fetches a Facebook URL on the mbasic surface and returns a parsed
// document. Login walls and error shells are mapped to typed CodeErrors.
func (c *Client) GetDoc(ctx context.Context, rawURL string) (*goquery.Document, error) {
	body, _, err := c.getBytes(ctx, c.surfaceURL(rawURL))
	if err != nil {
		return nil, err
	}
	return goquery.NewDocumentFromReader(strings.NewReader(string(body)))
}

// GetRaw returns the raw response body for --raw.
func (c *Client) GetRaw(ctx context.Context, rawURL string) ([]byte, error) {
	b, _, err := c.getBytes(ctx, c.surfaceURL(rawURL))
	return b, err
}

func (c *Client) getBytes(ctx context.Context, target string) ([]byte, int, error) {
	if b, ok := c.cache.Get(target); ok {
		c.logf(2, "cache hit %s", target)
		if w := wallOrShell(b); w != nil {
			return b, 200, w
		}
		return b, 200, nil
	}
	body, code, err := c.fetch(ctx, target)
	if err != nil {
		return body, code, err
	}
	if w := wallOrShell(body); w != nil {
		return body, code, w
	}
	c.cache.Put(target, body)
	return body, code, nil
}

func wallOrShell(body []byte) *CodeError {
	if isLoginWall(body) {
		return errLoginWall()
	}
	if isErrorShell(body) {
		return codeErr(ExitNotFound, "facebook returned an error page; content may be unavailable or anonymous access is limited")
	}
	return nil
}

func (c *Client) fetch(ctx context.Context, target string) ([]byte, int, error) {
	attempts := c.cfg.Retries
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		c.rateLimit()
		body, code, err := c.doGet(ctx, target)
		if err != nil {
			if ctx.Err() != nil {
				return nil, 0, ctx.Err()
			}
			lastErr = err
			if attempt == attempts {
				return nil, 0, codeErr(ExitNetwork, "request failed after %d attempts: %v", attempts, err)
			}
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		switch {
		case code == 404:
			return body, code, errNotFound(target)
		case code == 429 || code == 503:
			if attempt == attempts {
				return body, code, codeErr(ExitRateLimit, "rate limited (HTTP %d) after %d attempts", code, attempts)
			}
			time.Sleep(time.Duration(attempt*attempt) * 3 * time.Second)
			continue
		case code >= 500:
			if attempt == attempts {
				return body, code, codeErr(ExitNetwork, "server error HTTP %d", code)
			}
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}
		return body, code, nil
	}
	return nil, 0, codeErr(ExitNetwork, "all attempts failed: %v", lastErr)
}

func (c *Client) doGet(ctx context.Context, target string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.uas[rand.Intn(len(c.uas))])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	lang := c.cfg.Lang
	if lang == "" {
		lang = "en-US"
	}
	req.Header.Set("Accept-Language", lang+","+strings.SplitN(lang, "-", 2)[0]+";q=0.9")
	req.Header.Set("Referer", "https://mbasic.facebook.com/")
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	c.logf(2, "GET %s", target)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func (c *Client) rateLimit() {
	if c.cfg.Delay <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if since := time.Since(c.lastReq); since < c.cfg.Delay {
		time.Sleep(c.cfg.Delay - since)
	}
	c.lastReq = time.Now()
}

func (c *Client) logf(level int, format string, args ...any) {
	if c.cfg.Verbose >= level {
		fmt.Fprintf(os.Stderr, "[fb] "+format+"\n", args...)
	}
}

func isLoginWall(body []byte) bool {
	s := strings.ToLower(string(body))
	return strings.Contains(s, "log in to facebook") ||
		strings.Contains(s, "login to facebook") ||
		strings.Contains(s, "you must log in") ||
		strings.Contains(s, "enter mobile number or email to continue") ||
		strings.Contains(s, "you must log in to continue")
}

// errorTitleWords is the localized word for "Error" that mbasic uses as the
// <title> of its generic error / browser-not-supported shell. Matching the
// title keeps shell detection language-agnostic: facebook.com geolocates the
// response, so an English token alone misses the same page served in Vietnamese
// or any other locale.
var errorTitleWords = map[string]bool{
	"error": true, "lỗi": true, "erreur": true, "fehler": true,
	"errore": true, "erro": true, "ошибка": true, "错误": true,
	"錯誤": true, "エラー": true, "오류": true, "خطأ": true,
	"hata": true, "fout": true, "fel": true, "virhe": true,
}

func isErrorShell(body []byte) bool {
	s := strings.ToLower(string(body))
	if strings.Contains(s, "sorry, something went wrong") ||
		strings.Contains(s, "the link you followed may be broken") ||
		strings.Contains(s, "this page isn't available") {
		return true
	}
	// Localized error / browser-not-supported shell: the <title> is the
	// localized word for "Error", optionally followed by "Facebook". A real
	// page's title is the entity name, so this never fires on actual content.
	title := strings.TrimSpace(strings.TrimSuffix(extractTitle(s), "facebook"))
	title = strings.TrimSpace(title)
	return errorTitleWords[title]
}

func extractTitle(lowerHTML string) string {
	start := strings.Index(lowerHTML, "<title>")
	if start < 0 {
		return ""
	}
	start += len("<title>")
	end := strings.Index(lowerHTML[start:], "</title>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(lowerHTML[start : start+end])
}
