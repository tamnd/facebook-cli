package fb

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// config.go turns the global flags from spec 3004 doc 05 section 1 into a
// client.
//
// The flag that needs explaining is --tier, which is two flags in one string.
// A number caps what a run may use, so --tier 0 reads the way a machine with no
// cookies would even when cookies were imported, which is how a Tier 0 claim in
// the spec gets checked. A name pins one surface, so --tier og answers from the
// meta head alone and says nothing it could only have got from the Relay store.
// Anything else is a usage error that lists the values, because a typo that
// quietly renders the default is worse than a refusal.

// Config is every knob the commands and the ops share.
type Config struct {
	Tier     string
	Limit    int
	Rate     time.Duration
	Retries  int
	Timeout  time.Duration
	NoCache  bool
	CacheTTL time.Duration
	DataDir  string
	Cookies  string
	Proxy    string
	Verbose  func(format string, args ...any)
}

// Defaults are doc 05 section 1 as values.
func Defaults() Config {
	return Config{
		Rate:     time.Second,
		Retries:  2,
		Timeout:  30 * time.Second,
		CacheTTL: 15 * time.Minute,
	}
}

// pins maps the --tier names to the surfaces they pin to.
var pins = map[string]string{
	"comet":   surfaceComet,
	"graphql": surfaceGraphQL,
	"og":      surfaceOpenGraph,
	"embed":   surfaceEmbed,
	"session": surfaceSession,
}

// parseTier reads --tier into a cap and a pin.
//
// The cap defaults to 1 when cookies were imported and 0 when they were not, so
// the common case needs no flag at all.
func parseTier(s string, haveCookies bool) (capTier int, pin string, err error) {
	capTier = 0
	if haveCookies {
		capTier = 1
	}
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "auto" {
		return capTier, "", nil
	}
	if n, convErr := strconv.Atoi(s); convErr == nil {
		if n < 0 || n > 1 {
			return 0, "", usage("--tier %s is not a tier fb has: use 0 or 1, or one of comet, graphql, og, embed, session", s)
		}
		return min(n, capTier), "", nil
	}
	surface, ok := pins[s]
	if !ok {
		return 0, "", usage("--tier %s is not a tier or a surface: use 0, 1, comet, graphql, og, embed or session", s)
	}
	if surface == surfaceSession && !haveCookies {
		return 0, "", needAuth("--tier session needs cookies: run fb auth import first")
	}
	return capTier, surface, nil
}

// DefaultDataDir is where the cache, the store and the cookies live.
func DefaultDataDir() string {
	if dir := os.Getenv("FB_DATA_DIR"); dir != "" {
		return dir
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return ".fb"
	}
	return filepath.Join(base, "fb")
}

// client builds the HTTP client this config describes.
func (c Config) client() (*Client, error) {
	cl := NewClient()
	if c.Rate > 0 {
		cl.Rate = c.Rate
	}
	if c.Retries >= 0 {
		cl.Retries = c.Retries
	}
	if c.Timeout > 0 {
		cl.HTTP.Timeout = c.Timeout
	}
	cl.Cookies = c.Cookies
	cl.Verbose = c.Verbose
	if c.Proxy != "" {
		u, err := url.Parse(c.Proxy)
		if err != nil {
			return nil, usage("--proxy %s is not a URL: %v", c.Proxy, err)
		}
		cl.HTTP.Transport = &http.Transport{Proxy: http.ProxyURL(u)}
	}
	if !c.NoCache {
		dir := c.DataDir
		if dir == "" {
			dir = DefaultDataDir()
		}
		cache, err := NewCache(filepath.Join(dir, "cache"), c.CacheTTL)
		if err != nil {
			// A cache that will not open is not a reason to refuse to read, so
			// it is reported and the run goes on without one.
			c.log("cache disabled: %v", err)
		} else {
			cl.Cache = cache
		}
	}
	return cl, nil
}

func (c Config) log(format string, args ...any) {
	if c.Verbose != nil {
		c.Verbose(format, args...)
	}
}
