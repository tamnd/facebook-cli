package fb

import (
	"os"
	"path/filepath"
	"time"
)

// Surface selects which Facebook host answers a request.
type Surface string

const (
	SurfaceAuto   Surface = "auto"
	SurfaceMBasic Surface = "mbasic"
	SurfaceMobile Surface = "mobile"
)

// Default request parameters. Facebook is stricter than the other targets, so
// the default delay is deliberately conservative.
const (
	DefaultDelay   = 2 * time.Second
	DefaultRetries = 4
	DefaultTimeout = 30 * time.Second
	DefaultWorkers = 2
)

// userAgents is the crawler identity Facebook answers with server-rendered HTML.
// Presenting as Googlebot is what unlocks anonymous, no-browser access.
var userAgents = []string{
	"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
}

// Config is the resolved runtime configuration for a Client.
type Config struct {
	Surface   Surface
	Delay     time.Duration
	Retries   int
	Timeout   time.Duration
	Workers   int
	UserAgent string
	Proxy     string
	Lang      string
	CacheDir  string
	NoCache   bool
	CacheTTL  time.Duration
	DataDir   string
	Verbose   int
}

// DefaultConfig returns the built-in defaults with XDG paths filled in.
func DefaultConfig() Config {
	return Config{
		Surface:  SurfaceAuto,
		Delay:    DefaultDelay,
		Retries:  DefaultRetries,
		Timeout:  DefaultTimeout,
		Workers:  DefaultWorkers,
		Lang:     "en-US",
		CacheDir: filepath.Join(cacheHome(), "fb"),
		CacheTTL: time.Hour,
		DataDir:  filepath.Join(dataHome(), "fb"),
	}
}

func cacheHome() string {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache")
}

func dataHome() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}
