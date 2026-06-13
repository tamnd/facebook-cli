package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tamnd/facebook-cli/fb"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// newArchiveCmd walks a Page's feed and writes it as an incremental tree of
// Markdown files: one file per post (with its comments) under year/month
// directories, plus a generated README.md index. Re-running only fetches posts
// that are not already on disk, so an archive grows over time.
func newArchiveCmd(a *App) *cobra.Command {
	var out string
	var comments, replies, force bool
	cmd := &cobra.Command{
		Use:   "archive <page>...",
		Short: "Archive a Page's posts and comments as incremental Markdown",
		Args:  cobra.MinimumNArgs(1),
		Long: `Archive a Page as a browsable tree of Markdown files.

Each post becomes <out>/<page>/YYYY/MM/YYYY-MM-DD_slug.md with its text, images,
links, and comments, and <out>/<page>/README.md indexes them by month. The
archive is incremental: a post already on disk is skipped, so re-running only
pulls what is new.`,
		Example: `  fb archive aivietnam.edu.vn --comments
  fb archive nasa --out ~/data -n 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			for _, arg := range args {
				if err := a.archivePage(ctx, arg, out, comments, replies, force); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", defaultArchiveDir(), "root directory for the archive")
	cmd.Flags().BoolVar(&comments, "comments", true, "fetch and embed each post's comments")
	cmd.Flags().BoolVar(&replies, "replies", false, "expand reply threads under comments")
	cmd.Flags().BoolVar(&force, "force", false, "re-fetch and overwrite posts already on disk")
	return cmd
}

func defaultArchiveDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "data")
}

func (a *App) archivePage(ctx context.Context, arg, out string, comments, replies, force bool) error {
	id := fbid.Classify(arg)
	slug := id.Slug
	if slug == "" {
		slug = id.PageID
	}
	if slug == "" {
		return fmt.Errorf("could not resolve a page from %q", arg)
	}

	page, err := a.Client.Page(ctx, arg)
	if err != nil {
		return err
	}

	dir := filepath.Join(out, slug)
	ar, err := fb.OpenArchive(dir, page)
	if err != nil {
		return err
	}

	a.progress("archiving %s into %s", page.Name, dir)
	added, skipped := 0, 0
	for p, perr := range a.Client.PagePosts(ctx, arg, a.listOpts()) {
		if perr != nil {
			return perr
		}
		if !force && ar.Has(p.PostID) {
			skipped++
			continue
		}
		var cs []fb.Comment
		if comments && p.CommentCount != 0 {
			for c, cerr := range a.Client.Comments(ctx, p.Permalink, fb.CommentOptions{Limit: a.Limit, Replies: replies}) {
				if cerr != nil {
					break // a bad comment thread should not lose the post
				}
				cs = append(cs, c)
			}
		}
		rel, werr := ar.WritePost(p, cs)
		if werr != nil {
			return werr
		}
		added++
		a.progress("  + %s", rel)
		if ferr := ar.Flush(); ferr != nil { // persist after each post: incremental-safe
			return ferr
		}
	}
	if err := ar.Flush(); err != nil {
		return err
	}
	a.progress("done: %d new, %d already archived, %d total", added, skipped, ar.Count())
	return nil
}
