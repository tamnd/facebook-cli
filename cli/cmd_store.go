package cli

import (
	"context"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/facebook-cli/fb"
	"github.com/tamnd/facebook-cli/pkg/rdf"
)

// cmd_store.go is the half of the tool that keeps something: `fb crawl` fills a
// store, `fb db stats` and `fb query` read it, `fb export` writes it out, and
// `fb archive` writes one read down in full.
//
// None of these are registered as operations, so none of them are reachable
// over `fb serve` or `fb mcp`. They touch this machine's disk, and a disk
// command on a network port is a different tool with a different threat model
// (doc 05 section 5).

func storeCommands() []kit.Command {
	return []kit.Command{
		newCrawlCmd(),
		newDBCmd(),
		newQueryCmd(),
		newExportCmd(),
		newArchiveCmd(),
	}
}

// storePath resolves --store against the data directory.
func (a *App) storePath(flag string) string {
	if flag != "" {
		return flag
	}
	return fb.DefaultStorePath(a.cfg.DataDir)
}

func newCrawlCmd() kit.Command {
	var store, letter string
	var depth, budget int
	var seedDirectory, resolveOpaque bool
	return kit.Command{
		Use:   "crawl <seed>...",
		Short: "Walk the graph from seeds and keep what it says",
		Long: "crawl reads a seed, files what it claimed, then reads what those claims named, and keeps " +
			"going until the depth or the budget runs out. The budget is in requests rather than nodes, " +
			"because requests are the unit the rate limits are written in and a walk that promises a " +
			"number of nodes spends an unknown number of requests. A cache hit is not a request and " +
			"does not count against it. The frontier is ordered by what a fetch is worth, measured " +
			"rather than guessed, so a budget that runs out has been spent on the pages that say the " +
			"most. Every crawl leaves a manifest naming what it spent and everything that refused.",
		Args: kit.MinimumNArgs(0),
		Flags: func(f *kit.FlagSet) {
			f.IntVar(&depth, "depth", 1, "how many hops out from the seeds")
			f.IntVar(&budget, "budget", 200, "how many requests the crawl may spend")
			f.StringVar(&store, "store", "", "the store to write into (default: store.db under --data-dir)")
			f.BoolVar(&seedDirectory, "seed-directory", false, "seed from the Pages directory as well")
			f.StringVar(&letter, "letter", "", "which letter of the directory to seed from")
			f.BoolVar(&resolveOpaque, "resolve-opaque", false, "spend a request each on the pfbid authors a group feed gives, so their posts get an author")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			if len(args) == 0 && !seedDirectory {
				return a.done(fb.Usage("crawl needs somewhere to start: name a seed, or pass --seed-directory"))
			}
			if len(args) > 0 {
				a.target = args[0]
			}
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			st, err := fb.OpenStore(a.storePath(store))
			if err != nil {
				return a.done(err)
			}
			defer func() { _ = st.Close() }()

			m, err := eng.Crawl(a.ctx(), st, fb.CrawlOptions{
				Seeds: args, Depth: depth, Budget: budget,
				SeedDirectory: seedDirectory, Letter: letter, ResolveOpaque: resolveOpaque,
				Progress: a.logf,
			})
			// The manifest is written whatever happened. A crawl that was
			// cancelled halfway is exactly the crawl somebody most wants the
			// account of.
			path, werr := fb.WriteManifest(a.cfg.DataDir, m)
			if err != nil {
				return a.done(err)
			}
			if werr != nil {
				a.warnOnce("could not write the crawl manifest: " + werr.Error())
			}
			for _, r := range m.Refusals {
				a.warnOnce("refused " + r.Ref + ": " + r.Reason)
			}
			return a.done(a.emitOne(manifestRow(m, path)))
		},
	}
}

func newDBCmd() kit.Command {
	var store string
	return kit.Command{
		Use:   "db",
		Short: "Inspect the store a crawl filled",
		Sub: []kit.Command{{
			Use:   "stats",
			Short: "What is in the store, counted three ways",
			Long: "stats counts the nodes by kind, the claims by predicate and the read log by surface and " +
				"status. The read log is the section to look at first: a crawl that spent its whole " +
				"budget on 404s and a crawl that worked look identical in the other two.",
			Args: kit.NoArgs,
			Flags: func(f *kit.FlagSet) {
				f.StringVar(&store, "store", "", "the store to read (default: store.db under --data-dir)")
			},
			Run: func(ctx context.Context, args []string) error {
				a := appFromCtx(ctx)
				st, err := fb.OpenStoreRO(a.storePath(store))
				if err != nil {
					return a.done(err)
				}
				defer func() { _ = st.Close() }()
				stats, err := st.Stats()
				if err != nil {
					return a.done(err)
				}
				rows := make([]Row, 0, len(stats))
				for _, s := range stats {
					rows = append(rows, statRow(s))
				}
				return a.done(a.emit(rows))
			},
		}},
	}
}

func newQueryCmd() kit.Command {
	var store string
	return kit.Command{
		Use:   "query <sql>",
		Short: "Run SQL over the store",
		Long: "query hands the string straight to SQLite. There is no dialect of fb's own here on " +
			"purpose: the answer to what a store says should be a query somebody already knows how to " +
			"write. The store is opened read-only, so a finger slip that says delete is refused by the " +
			"database rather than by a check in this tool. The tables are nodes, claims and reads, and " +
			"`fb db stats` prints their shape.",
		Args: kit.ExactArgs(1),
		Flags: func(f *kit.FlagSet) {
			f.StringVar(&store, "store", "", "the store to read (default: store.db under --data-dir)")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			st, err := fb.OpenStoreRO(a.storePath(store))
			if err != nil {
				return a.done(err)
			}
			defer func() { _ = st.Close() }()
			res, err := st.Query(a.ctx(), args[0])
			if err != nil {
				return a.done(err)
			}
			if len(res.Rows) == 0 {
				return a.done(fb.NoResults("that query matched nothing"))
			}
			rows := make([]Row, 0, len(res.Rows))
			for _, r := range res.Rows {
				rows = append(rows, queryRow(res.Cols, r))
			}
			return a.done(a.emit(rows))
		},
	}
}

func newExportCmd() kit.Command {
	var store, format string
	var noProv bool
	return kit.Command{
		Use:   "export",
		Short: "Write the whole store as RDF",
		Long: "export writes every claim in the store in the same vocabulary `fb rdf` uses: schema.org " +
			"where schema.org has a term, the fb namespace where it does not, and the namespace " +
			"declared in the output rather than assumed. The kind a node was stored under is used " +
			"where there is one, so a Page that was actually read comes out as an Organization rather " +
			"than as a Person. The order is fixed, so two exports of the same store are the same bytes " +
			"and a diff means something changed.",
		Args: kit.NoArgs,
		Flags: func(f *kit.FlagSet) {
			f.StringVar(&format, "format", "nt", "nt, turtle or jsonld")
			f.StringVar(&store, "store", "", "the store to read (default: store.db under --data-dir)")
			f.BoolVar(&noProv, "no-provenance", false, "leave out which URL asserted each claim")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			if err := a.rawOutput("export"); err != nil {
				return a.done(err)
			}
			if !slices.Contains(rdf.Formats, format) {
				return a.done(fb.Usage("no rdf format %q; there is %s", format, strings.Join(rdf.Formats, ", ")))
			}
			st, err := fb.OpenStoreRO(a.storePath(store))
			if err != nil {
				return a.done(err)
			}
			defer func() { _ = st.Close() }()
			edges, err := st.AllClaims(a.ctx())
			if err != nil {
				return a.done(err)
			}
			if len(edges) == 0 {
				return a.done(fb.NoResults("the store has no claims in it yet: `fb crawl` is what fills one"))
			}
			kinds, err := st.Kinds(a.ctx())
			if err != nil {
				return a.done(err)
			}
			ts := rdf.Triples(edges, rdf.Options{NoProvenance: noProv, Types: true, Kinds: kinds})
			return a.done(rdf.Write(os.Stdout, ts, format))
		},
	}
}

func newArchiveCmd() kit.Command {
	var dir string
	return kit.Command{
		Use:   "archive <ref>",
		Short: "Write one read down in full, for the record",
		Long: "archive fetches a reference without the cache and writes the whole thing to disk: the " +
			"HTML, every Relay payload extracted from it one file per operation, the request headers " +
			"as sent with the cookie redacted, and the record fb parsed out of it. This is how a bug " +
			"report should be filed, and it is where the fixtures in this repository came from. A page " +
			"fb cannot parse still archives: the parse error goes in beside the bytes, which is the " +
			"case the command is most useful for.",
		Args: kit.ExactArgs(1),
		Flags: func(f *kit.FlagSet) {
			f.StringVar(&dir, "dir", "", "where to write the capture (default: a dated directory under --data-dir)")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			out := dir
			if out == "" {
				out = fb.ArchiveDir(a.cfg.DataDir, args[0], time.Now())
			}
			got, err := eng.Archive(a.ctx(), args[0], out)
			if err != nil {
				return a.done(err)
			}
			return a.done(a.emitOne(archivedRow(got)))
		},
	}
}
