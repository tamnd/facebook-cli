package fb

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// cache.go keeps fetched pages on disk so that a session of reading does not
// fetch the same profile eight times.
//
// The entry keeps the final URL and the status beside the body, not just the
// bytes. A replay on surface 2 needs the lsd and spin tokens that came with the
// page the descriptor was harvested from, and those are in the body, so keeping
// the body alone would be enough for surface 1 and useless for surface 2. What
// would not be enough is keeping the body without the final URL: the redirect
// is how a deleted post is told from a live one.

// Entry is one cached response.
type Entry struct {
	Body     []byte    `json:"-"`
	FinalURL string    `json:"final_url"`
	Status   int       `json:"status"`
	At       time.Time `json:"at"`
	URL      string    `json:"url"`
}

// Cache is a directory of responses keyed by normalised URL.
type Cache struct {
	Dir string
	TTL time.Duration

	mu sync.Mutex
}

// NewCache opens the cache directory, creating it if it is not there.
func NewCache(dir string, ttl time.Duration) (*Cache, error) {
	if dir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &Cache{Dir: dir, TTL: ttl}, nil
}

// DefaultCacheDir is where the cache lives when nobody says otherwise.
func DefaultCacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "fb")
	}
	return ""
}

func (c *Cache) key(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:16])
}

// Get returns a cached entry when it is present and not stale.
func (c *Cache) Get(url string) (Entry, bool) {
	if c == nil {
		return Entry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	base := filepath.Join(c.Dir, c.key(url))
	metaBytes, err := os.ReadFile(base + ".json")
	if err != nil {
		return Entry{}, false
	}
	var e Entry
	if err := json.Unmarshal(metaBytes, &e); err != nil {
		return Entry{}, false
	}
	if time.Since(e.At) > c.TTL {
		return Entry{}, false
	}
	body, err := os.ReadFile(base + ".html")
	if err != nil {
		return Entry{}, false
	}
	e.Body = body
	return e, true
}

// Put writes an entry. A cache write that fails is not an error the caller
// needs: the read already succeeded, and the next run just fetches again.
func (c *Cache) Put(url string, e Entry) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e.URL = url
	if e.At.IsZero() {
		e.At = time.Now()
	}
	base := filepath.Join(c.Dir, c.key(url))
	meta, err := json.Marshal(e)
	if err != nil {
		return
	}
	if err := os.WriteFile(base+".html", e.Body, 0o644); err != nil {
		return
	}
	_ = os.WriteFile(base+".json", meta, 0o644)
}

// Clear empties the cache.
func (c *Cache) Clear() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.Remove(filepath.Join(c.Dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// Size is what is in the cache: how many bytes across how many files. `fb
// cache` prints it, which is the whole reason a read-only tool has a command
// that talks about disk at all.
func (c *Cache) Size() (bytes int64, files int) {
	if c == nil {
		return 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		return 0, 0
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		bytes += info.Size()
		files++
	}
	return bytes, files
}
