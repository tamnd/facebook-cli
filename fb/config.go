package fb

import (
	"os"
	"path/filepath"
	"strings"
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

var userAgents = []string{
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
}

// Config is the resolved runtime configuration for a Client.
type Config struct {
	Cookie     string
	CookieFile string
	Surface    Surface
	Delay      time.Duration
	Retries    int
	Timeout    time.Duration
	Workers    int
	UserAgent  string
	Proxy      string
	Lang       string
	CacheDir   string
	NoCache    bool
	CacheTTL   time.Duration
	DataDir    string
	Verbose    int
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

func configHome() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// resolveCookie loads the cookie header from, in order: the explicit string, a
// cookie file, the FACEBOOK_COOKIE env var, the FACEBOOK_COOKIE_FILE env var. It
// auto-detects raw-header, Netscape cookies.txt, and JSON export formats.
func resolveCookie(cfg Config) (string, error) {
	if c := strings.TrimSpace(cfg.Cookie); c != "" {
		return normalizeCookie(c), nil
	}
	file := strings.TrimSpace(cfg.CookieFile)
	if file == "" {
		if c := strings.TrimSpace(os.Getenv("FACEBOOK_COOKIE")); c != "" {
			return normalizeCookie(c), nil
		}
		file = strings.TrimSpace(os.Getenv("FACEBOOK_COOKIE_FILE"))
	}
	if file == "" {
		return "", nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return "", codeErr(ExitGeneric, "read cookie file: %v", err)
	}
	return parseCookieFile(string(b)), nil
}
