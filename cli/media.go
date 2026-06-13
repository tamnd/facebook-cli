package cli

import (
	"github.com/spf13/cobra"
	"github.com/tamnd/facebook-cli/fb"
)

func newPhotosCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "photos <page|profile|url>",
		Short:   "Stream a Page or profile's photos",
		Args:    cobra.ExactArgs(1),
		Example: `  fb photos nasa --limit 100 -o jsonl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			return emitSeq(a, a.Client.Photos(cmd.Context(), args[0], a.listOpts()), photoRow)
		},
	}
	return cmd
}

func newPhotoCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "photo <fbid|url>",
		Short:   "One photo, full metadata",
		Args:    cobra.ExactArgs(1),
		Example: `  fb photo "fbid=10160000000000000"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			if a.g.raw {
				return a.emitRaw(cmd.Context(), args[0])
			}
			p, err := a.Client.Photo(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return a.Out.Emit(photoRow(p))
		},
	}
	return cmd
}

func newVideosCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "videos <page>",
		Short:   "Stream a Page's videos and reels",
		Args:    cobra.ExactArgs(1),
		Example: `  fb videos nasa -o jsonl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			return emitSeq(a, a.Client.Videos(cmd.Context(), args[0], a.listOpts()), videoRow)
		},
	}
	return cmd
}

func newVideoCmd(a *App) *cobra.Command {
	var streams bool
	cmd := &cobra.Command{
		Use:     "video <id|url>",
		Short:   "One video or reel",
		Args:    cobra.ExactArgs(1),
		Example: `  fb video "https://fb.watch/xxxxx" --streams -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			ctx := cmd.Context()
			if a.g.raw {
				return a.emitRaw(ctx, args[0])
			}
			// resolve short links to a video id first
			id, _ := a.Client.Resolve(ctx, args[0])
			target := args[0]
			if id.VideoID != "" {
				target = id.CanonicalURL
			}
			v, err := a.Client.Video(ctx, target)
			if err != nil {
				return err
			}
			if streams {
				for i := range v.Streams {
					if err := a.Out.Emit(streamRow(&v.Streams[i])); err != nil {
						return err
					}
				}
				return nil
			}
			return a.Out.Emit(videoRow(v))
		},
	}
	cmd.Flags().BoolVar(&streams, "streams", false, "emit playable source URLs")
	return cmd
}

func newEventsCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "events <page>",
		Short:   "A Page's public events",
		Args:    cobra.ExactArgs(1),
		Example: `  fb events nasa`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			return emitSeq(a, a.Client.Events(cmd.Context(), args[0], a.listOpts()), eventRow)
		},
	}
	return cmd
}

func newEventCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "event <id|url>",
		Short:   "One public event, full",
		Args:    cobra.ExactArgs(1),
		Example: `  fb event 1234567890`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = a.Out.Flush() }()
			if a.g.raw {
				return a.emitRaw(cmd.Context(), args[0])
			}
			e, err := a.Client.Event(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return a.Out.Emit(eventRow(e))
		},
	}
	return cmd
}

var _ = fb.ListOptions{}
