package cli

import (
	"context"
	"strconv"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/facebook-cli/fb"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// cmd_meta.go prints what fb knows without asking Facebook anything.
//
// The tables come out of the Go values that drive the reads rather than out of
// a document, because a routing table that lives in prose drifts from the code
// that implements it. An invariant test checks both directions.

func metaCommands() []kit.Command {
	return []kit.Command{
		newSurfacesCmd(),
		newTiersCmd(),
		newRoutesCmd(),
		newIDCmd(),
		newExplainCmd(),
	}
}

func newSurfacesCmd() kit.Command {
	return kit.Command{
		Use:   "surfaces",
		Short: "The surfaces fb reads, and what each one answers",
		Args:  kit.NoArgs,
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			rows := make([]Row, 0, len(fb.Surfaces))
			for _, s := range fb.Surfaces {
				rows = append(rows, surfaceRow(s))
			}
			return a.emit(rows)
		},
	}
}

// newTiersCmd prints the two tiers and what each one costs. There is no third:
// fb has no app id, no developer app and no API token, by design.
func newTiersCmd() kit.Command {
	return kit.Command{
		Use:   "tiers",
		Short: "The two tiers, and what each one needs",
		Args:  kit.NoArgs,
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			tiers := []tierInfo{
				{0, "public", "nothing at all", a.cfg.Cookies == ""},
				{1, "session", "two cookies from a browser you are signed into", a.cfg.Cookies != ""},
			}
			rows := make([]Row, 0, len(tiers))
			for _, t := range tiers {
				rows = append(rows, Row{
					Cols:  []string{"tier", "name", "needs", "active"},
					Vals:  []string{strconv.Itoa(t.Tier), t.Name, t.Needs, yn(t.Active)},
					Value: t,
				})
			}
			return a.emit(rows)
		},
	}
}

// tierInfo is the shape `fb tiers` prints. It is a named type with tags rather
// than an anonymous struct so the JSON keys match the columns.
type tierInfo struct {
	Tier   int    `json:"tier"`
	Name   string `json:"name"`
	Needs  string `json:"needs"`
	Active bool   `json:"active"`
}

// newRoutesCmd prints the command-to-operation table, and --questions prints
// the same routing the other way round: by what somebody wants to know rather
// than by what they type.
func newRoutesCmd() kit.Command {
	var questions bool
	return kit.Command{
		Use:   "routes",
		Short: "Which surface and operation answers which command, at each tier",
		Args:  kit.NoArgs,
		Flags: func(f *kit.FlagSet) {
			f.BoolVar(&questions, "questions", false, "print the routing by question rather than by command")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			if questions {
				rows := make([]Row, 0, len(fb.Routes))
				for _, r := range fb.Routes {
					rows = append(rows, routeRow(r))
				}
				return a.emit(rows)
			}
			rows := make([]Row, 0, len(fb.Operations))
			for _, o := range fb.Operations {
				rows = append(rows, operationRow(o))
			}
			return a.emit(rows)
		},
	}
}

// newExplainCmd says what fb would do with a URL without doing any of it: which
// command handles it, which surface and operation answer it, and at which tier.
// It is the fastest way to find out whether this tool reads a link.
func newExplainCmd() kit.Command {
	return kit.Command{
		Use:   "explain <url>",
		Short: "Say what fb would do with a URL, without asking Facebook",
		Args:  kit.ExactArgs(1),
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			r := fbid.Parse(args[0])
			if r.Command == "" {
				return mapErr(fb.NoResults("fb has no command for %s: `fb id` says what it is", args[0]))
			}
			var rows []Row
			for _, o := range fb.Operations {
				if o.Command == r.Command || strings.HasPrefix(o.Command, r.Command+" ") {
					rows = append(rows, explainRow(r, o))
				}
			}
			if len(rows) == 0 {
				rows = append(rows, explainRow(r, fb.Operation{Command: r.Command}))
			}
			return a.emit(rows)
		},
	}
}

// newIDCmd classifies a reference without spending a request. It is the fastest
// way to answer "what is this thing" for the half-dozen shapes Facebook writes
// an id in, including the two base64 ones.
func newIDCmd() kit.Command {
	var resolve bool
	return kit.Command{
		Use:   "id <anything>",
		Short: "Classify a Facebook id, handle, story key or URL",
		Long: "id decodes what you paste: a numeric id, a handle, a permalink, a story key, a pfbid, a " +
			"base64 node id. It makes no request, so it costs nothing and works offline. --resolve " +
			"spends one, to turn a handle into the numeric id behind it, a pfbid into the story id, or " +
			"a share link into whatever it redirects to.",
		Args: kit.ExactArgs(1),
		Flags: func(f *kit.FlagSet) {
			f.BoolVar(&resolve, "resolve", false, "spend one request to turn a handle, a pfbid or a share link into a numeric id")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			r := fbid.Parse(args[0])
			if resolve {
				e, err := a.engine()
				if err != nil {
					return a.fail(args[0], err)
				}
				r, err = e.Resolve(a.ctx(), args[0])
				if err != nil {
					return a.fail(args[0], err)
				}
			}
			return a.emitOne(Row{
				Cols:  []string{"kind", "id", "handle", "post_id", "author_id", "command", "url"},
				Vals:  []string{r.Kind, r.ID, r.Handle, r.PostID, r.AuthorID, r.Command, r.URL},
				Value: r,
			})
		},
	}
}
