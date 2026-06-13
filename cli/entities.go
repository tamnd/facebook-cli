package cli

import (
	"context"
	"iter"

	"github.com/spf13/cobra"
	"github.com/tamnd/facebook-cli/fb"
)

// emitSeq streams an iter.Seq2 through a row projector, honoring the limit and
// returning the first hard error. Per-item nil errors stop the stream cleanly.
func emitSeq[T any](a *App, seq iter.Seq2[T, error], row func(*T) Row) error {
	for v, err := range seq {
		if err != nil {
			return err
		}
		val := v
		if e := a.Out.Emit(row(&val)); e != nil {
			return e
		}
	}
	return nil
}

func (a *App) emitRaw(ctx context.Context, url string) error {
	b, err := a.Client.GetRaw(ctx, url)
	if err != nil {
		return err
	}
	return a.Out.Raw(b)
}

func newPageCmd(a *App) *cobra.Command {
	var posts, about, photos, videos, events bool
	cmd := &cobra.Command{
		Use:   "page <slug|id|url>...",
		Short: "A Facebook Page, fully resolved",
		Args:  cobra.MinimumNArgs(1),
		Example: `  fb page nasa
  fb page nasa --posts --limit 20
  fb page nasa --photos -o jsonl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			ctx := cmd.Context()
			for _, arg := range readArgsOrStdin(args) {
				if a.g.raw {
					if err := a.emitRaw(ctx, arg); err != nil {
						return err
					}
					continue
				}
				switch {
				case posts:
					if err := emitSeq(a, a.Client.PagePosts(ctx, arg, a.listOpts()), postRow); err != nil {
						return err
					}
				case photos:
					if err := emitSeq(a, a.Client.Photos(ctx, arg, a.listOpts()), photoRow); err != nil {
						return err
					}
				case videos:
					if err := emitSeq(a, a.Client.Videos(ctx, arg, a.listOpts()), videoRow); err != nil {
						return err
					}
				case events:
					if err := emitSeq(a, a.Client.Events(ctx, arg, a.listOpts()), eventRow); err != nil {
						return err
					}
				default:
					_ = about
					p, err := a.Client.Page(ctx, arg)
					if err != nil {
						return err
					}
					if err := a.Out.Emit(pageRow(p)); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&posts, "posts", false, "stream the Page's feed")
	cmd.Flags().BoolVar(&about, "about", false, "metadata only (default)")
	cmd.Flags().BoolVar(&photos, "photos", false, "stream the Page's photos")
	cmd.Flags().BoolVar(&videos, "videos", false, "stream the Page's videos")
	cmd.Flags().BoolVar(&events, "events", false, "stream the Page's events")
	return cmd
}

func newProfileCmd(a *App) *cobra.Command {
	var posts, about, photos bool
	cmd := &cobra.Command{
		Use:   "profile <user|id|url>",
		Short: "A person's public profile",
		Args:  cobra.ExactArgs(1),
		Example: `  fb profile zuck
  fb profile zuck --posts`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			ctx := cmd.Context()
			arg := args[0]
			if a.g.raw {
				return a.emitRaw(ctx, arg)
			}
			if posts {
				return emitSeq(a, a.Client.ProfilePosts(ctx, arg, a.listOpts()), postRow)
			}
			_, _ = about, photos
			p, err := a.Client.Profile(ctx, arg)
			if err != nil {
				return err
			}
			return a.Out.Emit(profileRow(p))
		},
	}
	cmd.Flags().BoolVar(&posts, "posts", false, "stream the profile's public timeline")
	cmd.Flags().BoolVar(&about, "about", false, "intro/work/education/places")
	cmd.Flags().BoolVar(&photos, "photos", false, "stream the profile's photos")
	return cmd
}

func newGroupCmd(a *App) *cobra.Command {
	var posts, about bool
	cmd := &cobra.Command{
		Use:   "group <id|slug|url>",
		Short: "A group and its feed",
		Args:  cobra.ExactArgs(1),
		Example: `  fb group 123456789
  fb group 123456789 --posts --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			ctx := cmd.Context()
			arg := args[0]
			if a.g.raw {
				return a.emitRaw(ctx, arg)
			}
			if posts {
				return emitSeq(a, a.Client.GroupPosts(ctx, arg, a.listOpts()), postRow)
			}
			_ = about
			g, err := a.Client.Group(ctx, arg)
			if err != nil {
				return err
			}
			return a.Out.Emit(groupRow(g))
		},
	}
	cmd.Flags().BoolVar(&posts, "posts", false, "stream the group feed")
	cmd.Flags().BoolVar(&about, "about", false, "metadata only (default)")
	return cmd
}

var _ = fb.SurfaceAuto
