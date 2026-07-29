package fb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tamnd/facebook-cli/pkg/graph"
	_ "modernc.org/sqlite" // the pure-Go driver, so a build needs no cgo
)

// store.go is the crawl's memory: one SQLite file, three tables, spec 3004 doc
// 04 section 5.
//
// The schema is small on purpose and one column in it is the whole design. A
// claim's primary key includes the source, so two pages saying the same thing
// are two rows rather than one row written twice. That is what makes the store
// worth keeping: when the profile page and the post permalink disagree about
// who wrote something, a store keyed without the source has already thrown away
// the evidence that there was a disagreement.
//
// `reads` is the audit log. A record says which URLs it came from and this is
// the table that says what those URLs actually answered, so a thin record can
// be checked against the 404 that made it thin months later.
//
// SQLite because the answer to "what does this store say" should be SQL that
// somebody already knows rather than a query language this tool invented. `fb
// query` hands the string straight to the database.

// schema is doc 04 section 5, verbatim, plus the two indexes that make a walk
// backwards cheap. A claim is looked up by its subject most of the time, which
// the primary key already covers, and by its object whenever somebody asks who
// links to a thing, which it does not.
const schema = `
CREATE TABLE IF NOT EXISTS nodes  (uri TEXT PRIMARY KEY, kind TEXT, record JSON, first_seen INT, last_seen INT);
CREATE TABLE IF NOT EXISTS claims (from_uri TEXT, predicate TEXT, to_uri TEXT, source TEXT, surface TEXT, tier INT,
                                   seen_at INT, PRIMARY KEY (from_uri, predicate, to_uri, source));
CREATE TABLE IF NOT EXISTS reads  (url TEXT, surface TEXT, status INT, bytes INT, at INT, error TEXT);
CREATE INDEX IF NOT EXISTS claims_to ON claims (to_uri);
CREATE INDEX IF NOT EXISTS reads_at ON reads (at);
`

// Store is an open store.
type Store struct {
	db   *sql.DB
	Path string
}

// DefaultStorePath is where the store lives when nobody passes --store.
func DefaultStorePath(dataDir string) string {
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}
	return filepath.Join(dataDir, "store.db")
}

// OpenStore opens the store for writing, creating the file and the schema when
// they are not there yet.
func OpenStore(path string) (*Store, error) {
	if path == "" {
		return nil, usage("a store needs a path: pass --store or let fb use the one under --data-dir")
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	// WAL so that a `fb query` in another terminal reads while a crawl writes,
	// and a busy timeout so that the two do not collide over the same page.
	db, err := sql.Open("sqlite", dsn(path, "_pragma=journal_mode(wal)", "_pragma=busy_timeout(5000)"))
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &Store{db: db, Path: path}, nil
}

// OpenStoreRO opens an existing store read-only, which is what `fb query` gets.
//
// The query is arbitrary SQL somebody typed. Opening the file read-only means a
// finger slip that says `delete from claims` is refused by SQLite rather than
// by a check in this package that somebody will eventually find a way around.
func OpenStoreRO(path string) (*Store, error) {
	if path == "" {
		return nil, usage("a store needs a path: pass --store or let fb use the one under --data-dir")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, notFound("store "+path, "there is no store there yet, and `fb crawl` is what makes one")
	}
	db, err := sql.Open("sqlite", dsn(path, "mode=ro", "_pragma=busy_timeout(5000)"))
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &Store{db: db, Path: path}, nil
}

// dsn builds the file: URI SQLite wants. The path goes through url escaping
// because a data directory with a question mark in it would otherwise be read
// as the start of the options.
func dsn(path string, opts ...string) string {
	p := filepath.ToSlash(path)
	p = strings.ReplaceAll(p, "?", "%3f")
	p = strings.ReplaceAll(p, "#", "%23")
	return "file:" + p + "?" + strings.Join(opts, "&")
}

// Close closes the store.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB exposes the handle for the few callers that need one, such as a test.
func (s *Store) DB() *sql.DB { return s.db }

// PutNode writes a node, keeping the record it already had when this sighting
// brought none.
//
// Most nodes are met twice: once as the object of somebody else's claim, with a
// name and nothing else, and once as a page that was actually read. Whichever
// order those happen in, the read is the one worth keeping, so a sighting with
// no record leaves the stored one alone and first_seen never moves.
func (s *Store) PutNode(uri, kind string, record any) error {
	if uri == "" {
		return nil
	}
	var rec any
	if record != nil {
		b, err := json.Marshal(record)
		if err != nil {
			return err
		}
		rec = string(b)
	}
	now := time.Now().Unix()
	_, err := s.db.Exec(`
INSERT INTO nodes (uri, kind, record, first_seen, last_seen) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(uri) DO UPDATE SET
  kind      = CASE WHEN excluded.kind <> '' THEN excluded.kind ELSE nodes.kind END,
  record    = coalesce(excluded.record, nodes.record),
  last_seen = excluded.last_seen`, uri, kind, rec, now, now)
	return err
}

// PutClaims writes a batch of claims in one transaction.
//
// A repeat of a claim from the same source updates seen_at and nothing else. It
// is the same statement by the same page, so overwriting it with today's date
// is the truth: the page still says it.
func (s *Store) PutClaims(edges []graph.Edge) (int, error) {
	if len(edges) == 0 {
		return 0, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	st, err := tx.Prepare(`
INSERT INTO claims (from_uri, predicate, to_uri, source, surface, tier, seen_at) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(from_uri, predicate, to_uri, source) DO UPDATE SET
  surface = excluded.surface, tier = excluded.tier, seen_at = excluded.seen_at`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = st.Close() }()
	now := time.Now().Unix()
	n := 0
	for _, e := range edges {
		if e.From == "" || e.Predicate == "" || e.To == "" {
			continue
		}
		if _, err := st.Exec(e.From, e.Predicate, e.To, e.Source, e.Surface, e.Tier, now); err != nil {
			return n, err
		}
		n++
	}
	return n, tx.Commit()
}

// Read is one request and what came back. It is what fills the reads table, and
// the client hands one to whoever set Client.Log.
type Read struct {
	URL     string    `json:"url"`
	Surface string    `json:"surface"`
	Status  int       `json:"status"`
	Bytes   int       `json:"bytes"`
	At      time.Time `json:"at"`
	Error   string    `json:"error,omitempty"`
}

// LogRead appends to the audit log.
func (s *Store) LogRead(r Read) error {
	at := r.At
	if at.IsZero() {
		at = time.Now()
	}
	_, err := s.db.Exec(`INSERT INTO reads (url, surface, status, bytes, at, error) VALUES (?, ?, ?, ?, ?, ?)`,
		r.URL, r.Surface, r.Status, r.Bytes, at.Unix(), r.Error)
	return err
}

// Stat is one line of `fb db stats`.
type Stat struct {
	Section string `json:"section"`
	Key     string `json:"key"`
	Count   int    `json:"count"`
}

// Stats counts the store three ways: nodes by kind, claims by predicate, and
// the read log by surface and status.
//
// The read log is the one people look at first, because a crawl that spent its
// whole budget on 404s and a crawl that worked look identical in the other two
// sections and nothing alike in this one.
func (s *Store) Stats() ([]Stat, error) {
	var out []Stat
	sections := []struct{ name, query string }{
		{"nodes", `SELECT coalesce(nullif(kind,''),'(none)'), count(*) FROM nodes GROUP BY 1 ORDER BY 2 DESC, 1`},
		{"claims", `SELECT predicate, count(*) FROM claims GROUP BY 1 ORDER BY 2 DESC, 1`},
		{"reads", `SELECT surface || ' ' || status, count(*) FROM reads GROUP BY 1 ORDER BY 2 DESC, 1`},
	}
	for _, sec := range sections {
		rows, err := s.db.Query(sec.query)
		if err != nil {
			return nil, err
		}
		total := 0
		for rows.Next() {
			var st Stat
			if err := rows.Scan(&st.Key, &st.Count); err != nil {
				_ = rows.Close()
				return nil, err
			}
			st.Section = sec.name
			total += st.Count
			out = append(out, st)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
		out = append(out, Stat{Section: sec.name, Key: "all", Count: total})
	}
	return out, nil
}

// Counts is the three totals, for the crawl manifest.
func (s *Store) Counts() (nodes, claims, reads int, err error) {
	row := s.db.QueryRow(`SELECT (SELECT count(*) FROM nodes), (SELECT count(*) FROM claims), (SELECT count(*) FROM reads)`)
	err = row.Scan(&nodes, &claims, &reads)
	return
}

// Result is what a query answered: the column names the database gave, and the
// rows under them.
type Result struct {
	Cols []string         `json:"columns"`
	Rows []map[string]any `json:"rows"`
}

// Query runs SQL and returns the rows. No wrapper, no dialect of our own, and
// no attempt to guess what somebody meant.
func (s *Store) Query(ctx context.Context, q string) (*Result, error) {
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, usage("that query did not run: %v", err)
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := &Result{Cols: cols}
	for rows.Next() {
		cells := make([]any, len(cols))
		into := make([]any, len(cols))
		for i := range cells {
			into[i] = &cells[i]
		}
		if err := rows.Scan(into...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			// A TEXT column comes back as bytes, and bytes render as a base64
			// blob in JSON, which is a surprising way to be shown a URL.
			if b, ok := cells[i].([]byte); ok {
				m[c] = string(b)
				continue
			}
			m[c] = cells[i]
		}
		out.Rows = append(out.Rows, m)
	}
	return out, rows.Err()
}

// AllClaims is every claim in the store, in a fixed order.
//
// The order is the whole reason this is not `SELECT *`: an export that reorders
// itself between two runs cannot be diffed, and a diff is how somebody notices
// that a page started saying something different.
func (s *Store) AllClaims(ctx context.Context) ([]graph.Edge, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT from_uri, predicate, to_uri, source, surface, tier FROM claims
ORDER BY from_uri, predicate, to_uri, source`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []graph.Edge
	for rows.Next() {
		var e graph.Edge
		if err := rows.Scan(&e.From, &e.Predicate, &e.To, &e.Source, &e.Surface, &e.Tier); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Kinds is the kind stored for every node, which is what an export needs to
// write a type for a node nobody fetched.
func (s *Store) Kinds(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT uri, kind FROM nodes WHERE kind <> ''`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var uri, kind string
		if err := rows.Scan(&uri, &kind); err != nil {
			return nil, err
		}
		out[uri] = kind
	}
	return out, rows.Err()
}
