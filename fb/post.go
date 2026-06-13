package fb

import (
	"context"

	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Post fetches a single post's detail from its crawler page.
func (c *Client) Post(ctx context.Context, idOrURL string, opt PostOptions) (*Post, error) {
	id := fbid.Classify(idOrURL)
	target := wwwURL(idOrURL, id)
	doc, err := c.getHTML(ctx, target)
	if err != nil {
		return nil, err
	}
	return parsePostSSR(doc, id, target), nil
}
