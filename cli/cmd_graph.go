package cli

import (
	"context"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/facebook-cli/pkg/graph"
)

// cmd_graph.go is the graph plane at the command line: `fb edges` for one read's
// claims, `fb graph` for one read plus its neighbours.
//
// Both print claims rather than records, which is the difference worth keeping
// straight. A record is what a page said about itself. A claim is what it said
// about somebody else, and it is the half that is worth storing, because it is
// the half nobody has to fetch to get.

func newEdgesCmd() kit.Command {
	return kit.Command{
		Use:   "edges <ref>",
		Short: "The claims one read already makes",
		Long: "edges reads whatever the reference names and prints what it asserted about everything " +
			"else: who wrote it, who it mentioned, what it linked to, which album a photo is in, who " +
			"commented. Nothing is walked, so this is one read's worth of requests and no more. A " +
			"photo permalink is the best value of anything in the tool, because it ships the " +
			"containing story, the album neighbours and the comments the permalink chose to send.",
		Args: kit.ExactArgs(1),
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			c, err := eng.Edges(a.ctx(), args[0])
			if err != nil {
				return a.done(err)
			}
			a.warnMissed(c.Envelope)
			rows := make([]Row, 0, len(c.Edges))
			for _, e := range c.Edges {
				rows = append(rows, edgeRow(e))
			}
			return a.done(a.emit(rows))
		},
	}
}

func newGraphCmd() kit.Command {
	var depth, budget int
	return kit.Command{
		Use:   "graph <ref>",
		Short: "The claims, and then the claims of everything they named",
		Long: "graph is edges plus a walk. --depth says how many hops out to go and --budget caps the " +
			"requests, because requests are the unit the rate limits are written in and a walk that " +
			"promises a number of nodes spends an unknown number of requests. A node that will not " +
			"read is recorded as a miss and the walk carries on: an album has no page signed out, a " +
			"group behind the wall stays behind it, and neither is a reason to stop.",
		Args: kit.ExactArgs(1),
		Flags: func(f *kit.FlagSet) {
			f.IntVar(&depth, "depth", 1, "how many hops out from the seed")
			f.IntVar(&budget, "budget", 25, "how many requests the walk may spend")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			claims, err := eng.Graph(a.ctx(), args[0], depth, budget)
			if err != nil {
				return a.done(err)
			}
			// A walk reads pages that overlap, so the same page asserts the
			// same claim more than once. The set drops the repeats and keeps
			// two sources saying one thing apart, which is the whole reason
			// the source is part of a claim's identity.
			var seen graph.Set
			for _, c := range claims {
				a.warnMissed(c.Envelope)
				for _, e := range c.Edges {
					seen.Add(e)
				}
			}
			rows := make([]Row, 0, seen.Len())
			for _, e := range seen.Edges() {
				rows = append(rows, edgeRow(e))
			}
			return a.done(a.emit(rows))
		},
	}
}
