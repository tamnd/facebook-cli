package fb

import (
	"context"
	"iter"

	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Comments streams the preview comments a post's crawler page exposes. The
// crawler surface renders a handful of comments per post, not the full thread,
// so this is a preview rather than an exhaustive walk.
func (c *Client) Comments(ctx context.Context, postURL string, opt CommentOptions) iter.Seq2[Comment, error] {
	id := fbid.Classify(postURL)
	target := wwwURL(postURL, id)
	return func(yield func(Comment, error) bool) {
		doc, err := c.getHTML(ctx, target)
		if err != nil {
			yield(Comment{}, err)
			return
		}
		emitted := 0
		for _, cm := range parseCommentsSSR(doc, id.PostID) {
			if !yield(cm, nil) {
				return
			}
			emitted++
			if opt.Limit > 0 && emitted >= opt.Limit {
				return
			}
		}
	}
}
