package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tamnd/facebook-cli/fb"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// defaultDiscoverBudget caps a streaming walk when -n is not given, so
// `fb discover <id>` always terminates instead of spidering Facebook forever.
const defaultDiscoverBudget = 500

// newDiscoverCmd is the breadth-first graph walk. Where each single command
// answers one question about one object (a page's feed, a post's comments),
// discover chains them: from a seed it follows the object's edges, and from each
// neighbor it follows theirs, hop by hop, streaming one record per node.
func newDiscoverCmd(a *App) *cobra.Command {
	var (
		depth  int
		fanout int
		follow string
	)
	cmd := &cobra.Command{
		Use:     "discover <id|url>...",
		Aliases: []string{"walk", "graph"},
		Short:   "Breadth-first walk of the graph linked from a page, profile, group, or post",
		Long: `Walk the graph of connected Facebook objects, breadth first, starting from one
or more seeds. A seed is anything fb can resolve: a page slug, a profile, a
group, or a post URL.

The graph has two actor kinds (page, profile, group) and a post, joined by three
edges:

  posts     an actor's recent posts (actor to post)
  author    the actor that posted a post (post to actor)
  comments  a post's preview comments (post to comment; a leaf)

--follow chooses which edges to traverse. It takes a preset or a comma-separated
edge list:

  content   posts + author (the default; actors and their posts, and from a post
            seed back to its author and on to the rest of their feed)
  threads   posts + comments (posts and the preview comments under them)
  all       every edge

--depth is how many hops to follow (default 1; 0 emits only the seeds). Because
comments sit one hop below their post, threads needs --depth 2 from an actor seed
(or seed a post directly). --fanout caps neighbors per edge (default 25). The
walk streams nodes and stops after -n nodes (default 500).

fb reads anonymously from the same server-rendered pages Facebook serves to
search engines, so a walk sees recent posts and preview comments rather than full
history. A page that does not render for the crawler becomes a one-line note and
the walk carries on.

fb discover streams to stdout. To keep a walk, pipe it:
  fb discover nasa --depth 2 -o jsonl > graph.jsonl
For a single actor's feed use fb feed; to build per-type files or a database, see
fb seed and fb crawl.`,
		Args: cobra.MinimumNArgs(1),
		Example: `  fb discover nasa
  fb discover nasa --depth 2 -o jsonl > graph.jsonl
  fb discover "https://www.facebook.com/nasa/posts/123" --depth 2
  fb discover python --follow threads --depth 2
  fb search "climate" -o url | fb discover - --depth 1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			edges, err := fb.ParseEdges(follow)
			if err != nil {
				return err
			}
			seeds := toSeeds(readArgsOrStdin(args))
			if len(seeds) == 0 {
				return fmt.Errorf("no seeds given")
			}
			budget := a.Limit
			if budget <= 0 {
				budget = defaultDiscoverBudget
			}
			opts := fb.WalkOptions{
				Depth:  depth,
				Max:    budget,
				Fanout: fanout,
				Edges:  edges,
				Note: func(s string) {
					if a.g == nil || !a.g.quiet {
						_, _ = fmt.Fprintf(os.Stderr, "[fb] note: %s\n", s)
					}
				},
			}
			return a.Client.Walk(cmd.Context(), seeds, opts, func(n *fb.Node) error {
				return a.Out.Emit(nodeRow(n))
			})
		},
	}
	f := cmd.Flags()
	f.IntVar(&depth, "depth", 1, "hops to follow from each seed (0 = seeds only)")
	f.IntVar(&fanout, "fanout", 25, "max neighbors to follow per edge (0 = unlimited)")
	f.StringVar(&follow, "follow", "content", "edges to follow ("+fb.EdgeHelp()+")")
	return cmd
}

// toSeeds wraps raw arguments as walk seeds. Classification happens at walk time
// via the client's resolver, so a seed that resolves to none of page/profile/
// group/post fails the walk like any bad id.
func toSeeds(args []string) []fb.Seed {
	seeds := make([]fb.Seed, 0, len(args))
	for _, s := range args {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		seeds = append(seeds, fb.Seed{Raw: s})
	}
	return seeds
}

// nodeRow renders a graph node discovered by `fb discover`. It reuses each
// object's own column projection and prepends how the node was reached, while
// the full typed node rides in Value for json, jsonl, yaml, and templates.
func nodeRow(n *fb.Node) Row {
	base := nodeBaseRow(n)
	cols := append([]string{"depth", "via", "kind"}, base.Cols...)
	vals := append([]string{strconv.Itoa(n.Depth), string(n.Via), string(n.Kind)}, base.Vals...)
	return Row{Cols: cols, Vals: vals, Value: n}
}

// nodeBaseRow projects a node's payload through the matching entity renderer.
func nodeBaseRow(n *fb.Node) Row {
	switch n.Kind {
	case fb.NodePage:
		return pageRow(n.Page)
	case fb.NodeProfile:
		return profileRow(n.Profile)
	case fb.NodeGroup:
		return groupRow(n.Group)
	case fb.NodeComment:
		return commentRow(n.Comment)
	default:
		return postRow(n.Post)
	}
}

func newSearchCmd(a *App) *cobra.Command {
	var typ string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search across pages, profiles, groups, posts, photos, videos, events",
		Args:  cobra.ExactArgs(1),
		Example: `  fb search "climate"
  fb search "climate" --type page --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			opt := fb.SearchOptions{Type: typ, Limit: a.Limit, Since: a.since, Until: a.until}
			return emitSeq(a, a.Client.Search(cmd.Context(), args[0], opt), searchRow)
		},
	}
	cmd.Flags().StringVar(&typ, "type", "all", "page|profile|group|post|photo|video|event|all")
	return cmd
}

func newFeedCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "feed <slug|id>...",
		Short:   "Stream the feed of any handle (page/profile/group)",
		Args:    cobra.MinimumNArgs(1),
		Example: `  fb feed nasa zuck --limit 20 -o jsonl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			ctx := cmd.Context()
			for _, arg := range readArgsOrStdin(args) {
				if err := a.feedFor(ctx, arg); err != nil {
					return err
				}
			}
			return nil
		},
	}
	return cmd
}

func (a *App) feedFor(ctx context.Context, arg string) error {
	id := fbid.Classify(arg)
	switch id.Kind {
	case fbid.KindGroup:
		return emitSeq(a, a.Client.GroupPosts(ctx, arg, a.listOpts()), postRow)
	case fbid.KindProfile:
		return emitSeq(a, a.Client.ProfilePosts(ctx, arg, a.listOpts()), postRow)
	default:
		return emitSeq(a, a.Client.PagePosts(ctx, arg, a.listOpts()), postRow)
	}
}

func newIDCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "id <thing>",
		Short: "Classify any Facebook id or URL",
		Args:  cobra.ExactArgs(1),
		Example: `  fb id nasa
  fb id "https://fb.watch/xxxxx"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			id, err := a.Client.Resolve(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return a.Out.Emit(identityRow(id))
		},
	}
	return cmd
}
