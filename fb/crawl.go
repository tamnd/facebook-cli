package fb

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"github.com/tamnd/facebook-cli/pkg/graph"
)

// crawl.go walks the graph from seeds and writes what it finds into a store.
//
// The budget is in requests, and it is counted rather than estimated. Every
// request fb makes goes through the client's read log, so the crawl installs a
// counter there and stops when the count reaches the budget. That is the only
// honest way to spend a number somebody typed: a crawl that promised 200 nodes
// would spend an unknown number of requests, and requests are the unit the rate
// limits are written in.
//
// A cache hit is not a request and does not count, which is why a second crawl
// over the same seeds gets much further on the same budget.
//
// The frontier is ordered by how much a fetch is worth, measured against the
// captured pages in testdata rather than guessed at (doc 04 section 4). A post
// permalink brings fifteen claims and a group page brings two, so with a budget
// that runs out, spending it on posts leaves a better store behind.

// CrawlOptions is everything `fb crawl` was asked for.
type CrawlOptions struct {
	Seeds         []string
	Depth         int
	Budget        int
	ResolveOpaque bool
	SeedDirectory bool
	Letter        string
	Progress      func(format string, args ...any)
}

// Refusal is one node the crawl could not read.
type Refusal struct {
	URI    string `json:"uri,omitempty"`
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
}

// Manifest is the account of one crawl.
//
// It exists because a crawl that hit a wall and a crawl that ran out of budget
// leave identical stores and are not the same thing at all. The store says what
// was learned; the manifest says what it cost and what refused.
type Manifest struct {
	Seeds      []string  `json:"seeds"`
	Depth      int       `json:"depth"`
	Budget     int       `json:"budget"`
	Spent      int       `json:"spent"`
	NodesRead  int       `json:"nodes_read"`
	Nodes      int       `json:"nodes"`
	Claims     int       `json:"claims"`
	Surfaces   []string  `json:"surfaces,omitempty"`
	Refusals   []Refusal `json:"refusals,omitempty"`
	Stopped    string    `json:"stopped"`
	Store      string    `json:"store,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

// defaultBudget is what a crawl spends when nobody says. It is a couple of
// minutes at one request a second, which is long enough to be useful and short
// enough that a mistyped seed is not an afternoon.
const defaultBudget = 200

// Crawl walks from the seeds and writes nodes, claims and the read log into st.
func (e *Engine) Crawl(ctx context.Context, st *Store, opt CrawlOptions) (Manifest, error) {
	if opt.Budget <= 0 {
		opt.Budget = defaultBudget
	}
	m := Manifest{
		Seeds: opt.Seeds, Depth: opt.Depth, Budget: opt.Budget,
		Stopped: "frontier", StartedAt: time.Now().UTC(),
	}
	if st != nil {
		m.Store = st.Path
	}

	// The counter is the budget. It sits on the client's read log, so it counts
	// what actually went out rather than what the walk meant to spend.
	spent := 0
	prev := e.c.Log
	e.c.Log = func(r Read) {
		spent++
		if st != nil {
			_ = st.LogRead(r)
		}
		if prev != nil {
			prev(r)
		}
	}
	defer func() { e.c.Log = prev }()

	seeds := opt.Seeds
	if opt.SeedDirectory {
		found, err := e.seedFromDirectory(ctx, opt.Letter)
		if err != nil {
			m.Refusals = append(m.Refusals, Refusal{Ref: "directory " + opt.Letter, Reason: err.Error()})
		}
		seeds = append(seeds, found...)
		m.Seeds = seeds
	}
	if len(seeds) == 0 {
		m.FinishedAt = time.Now().UTC()
		m.Stopped = "no seeds"
		return m, noResults("a crawl needs somewhere to start: name a seed, or pass --seed-directory")
	}

	seen := map[string]bool{}
	surfaces := map[string]bool{}
	frontier := make([]node, 0, len(seeds))
	for _, s := range seeds {
		frontier = append(frontier, node{uri: s, ref: s})
	}

	for d := 0; d <= opt.Depth && len(frontier) > 0; d++ {
		var next []node
		for _, n := range sortFrontier(frontier) {
			if spent >= opt.Budget {
				m.Stopped = "budget"
				break
			}
			if err := ctx.Err(); err != nil {
				m.Stopped = "cancelled"
				m.FinishedAt = time.Now().UTC()
				m.Spent = spent
				return m, err
			}
			if n.ref == "" {
				continue
			}
			c, err := e.Edges(ctx, n.ref)
			if err != nil {
				m.Refusals = append(m.Refusals, Refusal{URI: n.uri, Ref: n.ref, Reason: err.Error()})
				e.crawlLog(opt, "refused %s: %v", n.ref, err)
				continue
			}
			if opt.ResolveOpaque {
				c = e.resolveOpaque(ctx, c)
			}
			seen[c.URI] = true
			m.NodesRead++
			for _, s := range c.Surfaces {
				surfaces[s] = true
			}
			if err := writeClaims(st, c); err != nil {
				m.FinishedAt = time.Now().UTC()
				m.Spent = spent
				return m, err
			}
			e.crawlLog(opt, "read %s: %d claims, %d of %d requests spent", c.URI, len(c.Edges), spent, opt.Budget)
			if d < opt.Depth {
				next = append(next, frontierOf(c, seen)...)
			}
		}
		if m.Stopped == "budget" {
			break
		}
		frontier = next
		// The order of these two matters. The last level queues nothing, so an
		// empty frontier there means the depth was reached rather than that the
		// graph ran out.
		if d == opt.Depth {
			m.Stopped = "depth"
		} else if len(frontier) == 0 {
			m.Stopped = "frontier"
		}
	}

	m.Spent = spent
	for s := range surfaces {
		m.Surfaces = append(m.Surfaces, s)
	}
	sort.Strings(m.Surfaces)
	if st != nil {
		nodes, claims, _, err := st.Counts()
		if err != nil {
			return m, err
		}
		m.Nodes, m.Claims = nodes, claims
	}
	m.FinishedAt = time.Now().UTC()
	return m, nil
}

// WriteManifest writes the account of a crawl beside the store and returns
// where it went.
//
// One file per crawl, named for when it started and what it started from, so
// two crawls of the same seeds a week apart sit next to each other. The store
// says what was learned and only this says what it cost, which is the
// difference between a crawl that hit a wall and one that ran out of budget.
func WriteManifest(dataDir string, m Manifest) (string, error) {
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}
	dir := filepath.Join(dataDir, "crawls")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	seed := "seeds"
	if len(m.Seeds) > 0 {
		seed = safeName(m.Seeds[0])
	}
	path := filepath.Join(dir, m.StartedAt.Format("20060102-150405")+"-"+seed+".json")
	return path, writeJSON(path, m)
}

func (e *Engine) crawlLog(opt CrawlOptions, format string, args ...any) {
	if opt.Progress != nil {
		opt.Progress(format, args...)
	}
}

// seedFromDirectory turns the Pages directory into seeds.
//
// Signed out this is nearly always a refusal, because Facebook answers every
// letter page with its security interstitial, and that refusal is on the
// manifest rather than swallowed. With a session it is a few hundred Pages for
// one request, which is the cheapest seeding there is.
func (e *Engine) seedFromDirectory(ctx context.Context, letter string) ([]string, error) {
	d, err := e.Discover(ctx, letter)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, en := range d.Entries {
		switch {
		case en.ID != "":
			out = append(out, en.ID)
		case en.URL != "":
			out = append(out, en.URL)
		}
	}
	return out, nil
}

// writeClaims puts one read into the store: the node it was about, every node
// it named, and the claims themselves.
//
// The named nodes go in without a record on purpose. A node fb has heard of and
// never fetched is a real thing to know about, it is what the next crawl's
// frontier is made of, and a store that only holds what was fetched cannot tell
// you what it has not looked at yet.
func writeClaims(st *Store, c Claims) error {
	if st == nil {
		return nil
	}
	if err := st.PutNode(c.URI, c.Kind, c.Record); err != nil {
		return err
	}
	for _, ed := range c.Edges {
		for _, uri := range [2]string{ed.From, ed.To} {
			if uri == "" || uri == c.URI {
				continue
			}
			kind, _, ok := graph.Split(uri)
			if !ok {
				continue
			}
			if err := st.PutNode(uri, kind, nil); err != nil {
				return err
			}
		}
	}
	_, err := st.PutClaims(c.Edges)
	return err
}

// worth is the measured claims per request from doc 04 section 4. It is the
// frontier's priority, so a budget that runs out has been spent on the pages
// that say the most.
//
// via is the predicate that put the node on the frontier, and it matters in one
// case. A profile picture and a cover photo are photos by kind, and their
// permalinks ship no media at all to a signed-out reader, so a page crawl that
// ranks them with the photos in its stories spends its first two requests on
// two refusals. They stay on the frontier rather than being skipped, because
// with a session they read like any other photo; they go last.
func worth(kind, via string) int {
	if via == graph.Covers {
		return 1
	}
	switch kind {
	case graph.KindPost:
		return 15
	case graph.KindPhoto:
		return 12
	case graph.KindProfile, graph.KindPage:
		return 7
	case graph.KindEvent:
		return 6
	case graph.KindVideo:
		return 4
	case graph.KindGroup:
		return 2
	}
	return 1
}

// sortFrontier orders one level of the walk by what a fetch is worth, keeping
// the order a page asserted things in for ties. It copies rather than sorting
// in place, because the caller's slice is the record of what the level was.
func sortFrontier(in []node) []node {
	out := make([]node, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		ki, _, _ := graph.Split(out[i].uri)
		kj, _, _ := graph.Split(out[j].uri)
		return worth(ki, out[i].via) > worth(kj, out[j].via)
	})
	return out
}

// refType is what the opaque walk is looking for.
var refType = reflect.TypeOf(Ref{})

// resolveOpaque spends one request per unnameable ref on a record and rebuilds
// the claims with what came back.
//
// This is the group problem and nothing else. A group post is written by a
// person, Facebook gives a person in a group feed as a pfbid, and a pfbid is
// per-render so it is never a node. The claim is dropped rather than filed
// under a name that will not survive the week, which leaves a group post in the
// store with nobody having written it. One request per author closes that, and
// it is off by default because it is the most expensive flag in the tool.
func (e *Engine) resolveOpaque(ctx context.Context, c Claims) Claims {
	if c.Record == nil {
		return c
	}
	// A copy, because the record arrives as an interface holding a value and
	// nothing inside one is addressable. It is a shallow copy, so a ref inside
	// a slice is patched in the caller's backing array as well as in this one.
	// That is fine here and would not be somewhere else: the crawl holds the
	// only reference to this record and goes on to use the patched one.
	v := reflect.New(reflect.TypeOf(c.Record)).Elem()
	v.Set(reflect.ValueOf(c.Record))
	refs := opaqueRefs(v)
	if len(refs) == 0 {
		return c
	}
	// One request per author rather than per mention: a busy group feed names
	// the same person on half the posts.
	done := map[string]string{}
	patched := false
	for _, slot := range refs {
		r := slot.Interface().(Ref)
		if r.URL == "" {
			continue
		}
		id, asked := done[r.URL]
		if !asked {
			out, err := e.Resolve(ctx, r.URL)
			if err == nil && numeric(out.ID) {
				id = out.ID
			}
			done[r.URL] = id
		}
		if id == "" {
			continue
		}
		r.ID, r.Opaque = id, false
		slot.Set(reflect.ValueOf(r))
		patched = true
	}
	if !patched {
		return c
	}
	c.Record = v.Interface()
	c.Edges = edgesOf(c.Record, c.Edges)
	return c
}

// opaqueRefs collects every settable Ref on a record that arrived carrying a
// pfbid. It walks with reflection rather than a field list per record type,
// because the list would be six type switches that go stale the first time
// somebody adds a field.
func opaqueRefs(v reflect.Value) []reflect.Value {
	var out []reflect.Value
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Struct:
			if v.Type() == refType {
				if v.CanSet() && v.Interface().(Ref).Opaque {
					out = append(out, v)
				}
				return
			}
			for i := range v.NumField() {
				walk(v.Field(i))
			}
		case reflect.Slice, reflect.Array:
			for i := range v.Len() {
				walk(v.Index(i))
			}
		case reflect.Pointer, reflect.Interface:
			if !v.IsNil() {
				walk(v.Elem())
			}
		}
	}
	walk(v)
	return out
}

// edgesOf re-derives the claims for a record that has been patched. The
// fallback is the claims it already had, so a record type nobody taught this
// about keeps what it came with rather than losing everything.
func edgesOf(record any, fallback []graph.Edge) []graph.Edge {
	switch r := record.(type) {
	case Profile:
		return ProfileEdges(r)
	case Post:
		return PostEdges(r)
	case Photo:
		return PhotoEdges(r)
	case Video:
		return VideoEdges(r)
	case Group:
		return GroupEdges(r)
	case Event:
		return EventEdges(r)
	}
	return fallback
}
