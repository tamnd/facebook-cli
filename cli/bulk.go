package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tamnd/facebook-cli/fb"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// newSeedCmd turns a root (page/profile/group/search) into a stream of post URLs
// on stdout, one per line, ready to pipe into `fb crawl`.
func newSeedCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed <page|profile|group|search> <arg>",
		Short: "Expand a root into a stream of URLs for crawling",
		Args:  cobra.MinimumNArgs(2),
		Example: `  fb seed page nasa | fb crawl
  fb seed search "climate" --type page | fb crawl --db fb.db`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			kind, arg := args[0], args[1]
			w := bufio.NewWriter(os.Stdout)
			defer func() { _ = w.Flush() }()
			emit := func(url string) bool {
				_, _ = fmt.Fprintln(w, url)
				return true
			}
			switch kind {
			case "page":
				for p, err := range a.Client.PagePosts(ctx, arg, a.listOpts()) {
					if err != nil {
						return err
					}
					emit(p.Permalink)
				}
			case "profile":
				for p, err := range a.Client.ProfilePosts(ctx, arg, a.listOpts()) {
					if err != nil {
						return err
					}
					emit(p.Permalink)
				}
			case "group":
				for p, err := range a.Client.GroupPosts(ctx, arg, a.listOpts()) {
					if err != nil {
						return err
					}
					emit(p.Permalink)
				}
			case "search":
				typ, _ := cmd.Flags().GetString("type")
				for r, err := range a.Client.Search(ctx, arg, fb.SearchOptions{Type: typ, Limit: a.Limit}) {
					if err != nil {
						return err
					}
					emit(r.URL)
				}
			default:
				return fmt.Errorf("unknown seed root %q (want page|profile|group|search)", kind)
			}
			return nil
		},
	}
	cmd.Flags().String("type", "all", "search type when seeding from search")
	return cmd
}

// newCrawlCmd reads URLs from stdin or --from, fetches each to a full record,
// and optionally upserts into a SQLite store.
func newCrawlCmd(a *App) *cobra.Command {
	var from, dbPath string
	var comments, reactions bool
	cmd := &cobra.Command{
		Use:   "crawl",
		Short: "Fetch a stream of URLs into full records (and optionally a DB)",
		Example: `  fb seed page nasa | fb crawl --db nasa.db --comments
  fb crawl --from queue.txt --db fb.db`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			ctx := cmd.Context()
			urls, err := crawlInputs(from)
			if err != nil {
				return err
			}
			var store *fb.Store
			if dbPath != "" {
				store, err = fb.OpenStore(dbPath)
				if err != nil {
					return err
				}
				defer func() { _ = store.Close() }()
			}
			count := 0
			for _, u := range urls {
				if a.Limit > 0 && count >= a.Limit {
					break
				}
				if err := a.crawlOne(ctx, u, store, comments, reactions); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "[fb] skip %s: %v\n", u, err)
					continue
				}
				count++
			}
			if store != nil {
				_, _ = fmt.Fprintf(os.Stderr, "[fb] crawled %d records into %s\n", count, dbPath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "read URLs from a file instead of stdin")
	cmd.Flags().StringVar(&dbPath, "db", "", "upsert records into this SQLite store")
	cmd.Flags().BoolVar(&comments, "comments", false, "also fetch each post's comments")
	cmd.Flags().BoolVar(&reactions, "reactions", false, "also fetch reactions")
	return cmd
}

func (a *App) crawlOne(ctx context.Context, u string, store *fb.Store, comments, reactions bool) error {
	id := fbid.Classify(u)
	switch id.Kind {
	case fbid.KindPage:
		p, err := a.Client.Page(ctx, u)
		if err != nil {
			return err
		}
		if err := a.Out.Emit(pageRow(p)); err != nil {
			return err
		}
		if store != nil {
			return store.Upsert("pages", p.PageID, p)
		}
	case fbid.KindProfile:
		p, err := a.Client.Profile(ctx, u)
		if err != nil {
			return err
		}
		if err := a.Out.Emit(profileRow(p)); err != nil {
			return err
		}
		if store != nil {
			return store.Upsert("profiles", p.ProfileID, p)
		}
	case fbid.KindGroup:
		g, err := a.Client.Group(ctx, u)
		if err != nil {
			return err
		}
		if err := a.Out.Emit(groupRow(g)); err != nil {
			return err
		}
		if store != nil {
			return store.Upsert("groups", g.GroupID, g)
		}
	default:
		p, err := a.Client.Post(ctx, u, fb.PostOptions{Comments: comments, Reactions: reactions})
		if err != nil {
			return err
		}
		if err := a.Out.Emit(postRow(p)); err != nil {
			return err
		}
		if store != nil {
			if err := store.Upsert("posts", p.PostID, p); err != nil {
				return err
			}
		}
		if comments {
			for c, cerr := range a.Client.Comments(ctx, u, fb.CommentOptions{Limit: a.Limit}) {
				if cerr != nil {
					break
				}
				if store != nil {
					_ = store.Upsert("comments", c.CommentID, c)
				}
			}
		}
	}
	return nil
}

func crawlInputs(from string) ([]string, error) {
	var r *bufio.Scanner
	if from != "" {
		f, err := os.Open(from)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		r = bufio.NewScanner(f)
	} else {
		r = bufio.NewScanner(os.Stdin)
	}
	r.Buffer(make([]byte, 1024*1024), 1024*1024)
	var out []string
	for r.Scan() {
		line := strings.TrimSpace(r.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// accept jsonl by pulling a url/permalink field heuristically
		out = append(out, line)
	}
	return out, nil
}

func newDBCmd(a *App) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Query the local SQLite store",
	}
	cmd.PersistentFlags().StringVar(&dbPath, "db", "fb.db", "path to the SQLite store")

	query := &cobra.Command{
		Use:     "query <sql>",
		Short:   "Run SQL against the store",
		Args:    cobra.ExactArgs(1),
		Example: `  fb db --db nasa.db query "select owner_name, count(*) from posts group by 1"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			store, err := fb.OpenStore(dbPath)
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()
			cols, rows, err := store.Query(args[0])
			if err != nil {
				return err
			}
			for _, r := range rows {
				if err := a.Out.Emit(Row{Cols: cols, Vals: r, Value: rowMap(cols, r)}); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.AddCommand(query)
	return cmd
}

func rowMap(cols, vals []string) map[string]string {
	m := make(map[string]string, len(cols))
	for i, c := range cols {
		if i < len(vals) {
			m[c] = vals[i]
		}
	}
	return m
}
