package fb

import (
	"context"
	"iter"

	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Group fetches a group's metadata from its crawler page.
func (c *Client) Group(ctx context.Context, idOrURL string) (*Group, error) {
	id := fbid.Classify(idOrURL)
	gid := id.GroupID
	if gid == "" {
		gid = id.Slug
	}
	doc, err := c.getHTML(ctx, "https://www.facebook.com/groups/"+gid+"/")
	if err != nil {
		return nil, err
	}
	return parseGroupSSR(doc, gid), nil
}

// GroupPosts streams the posts a group's crawler page exposes (its most recent
// stories), fetching each post's detail.
func (c *Client) GroupPosts(ctx context.Context, idOrURL string, opt ListOptions) iter.Seq2[Post, error] {
	id := fbid.Classify(idOrURL)
	gid := id.GroupID
	if gid == "" {
		gid = id.Slug
	}
	return c.walkFeed(ctx, "https://www.facebook.com/groups/"+gid+"/", gid, "group", opt)
}
