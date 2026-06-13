package cli

import (
	"github.com/spf13/cobra"
	"github.com/tamnd/facebook-cli/fb"
)

func newPostCmd(a *App) *cobra.Command {
	var comments, replies, reactions, noDetail bool
	cmd := &cobra.Command{
		Use:   "post <url|id>...",
		Short: "One or more posts, fully resolved",
		Args:  cobra.MinimumNArgs(1),
		Example: `  fb post "https://www.facebook.com/nasa/posts/pfbid0xyz"
  fb post <url> --comments --replies
  fb post <url> --reactions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer a.Out.Flush()
			ctx := cmd.Context()
			opt := fb.PostOptions{Comments: comments, Replies: replies, Reactions: reactions, NoDetail: noDetail}
			for _, arg := range readArgsOrStdin(args) {
				if a.g.raw {
					if err := a.emitRaw(ctx, arg); err != nil {
						return err
					}
					continue
				}
				p, err := a.Client.Post(ctx, arg, opt)
				if err != nil {
					return err
				}
				if err := a.Out.Emit(postRow(p)); err != nil {
					return err
				}
				if comments {
					if err := emitSeq(a, a.Client.Comments(ctx, arg, fb.CommentOptions{Replies: replies, Limit: a.Limit}), commentRow); err != nil {
						return err
					}
				}
				if reactions {
					sum, list := a.Client.Reactions(ctx, arg)
					if err := a.Out.Emit(reactionSummaryRow(sum)); err != nil {
						return err
					}
					if err := emitSeq(a, list, reactionRow); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&comments, "comments", false, "also stream the comment thread")
	cmd.Flags().BoolVar(&replies, "replies", false, "expand nested replies")
	cmd.Flags().BoolVar(&reactions, "reactions", false, "emit the reaction breakdown")
	cmd.Flags().BoolVar(&noDetail, "no-detail", false, "counters only, faster")
	return cmd
}

func newCommentsCmd(a *App) *cobra.Command {
	var replies bool
	var order string
	cmd := &cobra.Command{
		Use:   "comments <post-url|id>",
		Short: "Every comment and reply on a post",
		Args:  cobra.ExactArgs(1),
		Example: `  fb comments <post-url>
  fb comments <post-url> --replies --limit 500 -o jsonl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer a.Out.Flush()
			opt := fb.CommentOptions{Replies: replies, Order: order, Limit: a.Limit}
			return emitSeq(a, a.Client.Comments(cmd.Context(), args[0], opt), commentRow)
		},
	}
	cmd.Flags().BoolVar(&replies, "replies", false, "expand all nested replies")
	cmd.Flags().StringVar(&order, "order", "chrono", "chrono|ranked")
	return cmd
}

func newReactionsCmd(a *App) *cobra.Command {
	var list bool
	var typ string
	cmd := &cobra.Command{
		Use:   "reactions <post-url|id>",
		Short: "Who reacted and how",
		Args:  cobra.ExactArgs(1),
		Example: `  fb reactions <post-url>
  fb reactions <post-url> --list --type love`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer a.Out.Flush()
			sum, seq := a.Client.Reactions(cmd.Context(), args[0])
			if !list {
				return a.Out.Emit(reactionSummaryRow(sum))
			}
			return emitSeq(a, seq, func(r *fb.Reaction) Row {
				if typ != "" && r.Type != typ {
					return Row{Cols: []string{"type"}, Vals: []string{""}, Value: r}
				}
				return reactionRow(r)
			})
		},
	}
	cmd.Flags().BoolVar(&list, "list", false, "emit every reactor as a row")
	cmd.Flags().StringVar(&typ, "type", "", "filter to one reaction type")
	return cmd
}
