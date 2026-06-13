package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/tamnd/facebook-cli/fb"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

func newSearchCmd(a *App) *cobra.Command {
	var typ string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search across pages, profiles, groups, posts, photos, videos, events",
		Args:  cobra.ExactArgs(1),
		Example: `  fb search "climate"
  fb search "climate" --type page --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer a.Out.Flush()
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
			defer a.Out.Flush()
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
			defer a.Out.Flush()
			id, err := a.Client.Resolve(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return a.Out.Emit(identityRow(id))
		},
	}
	return cmd
}
