package fb

import (
	"context"
	"net/http"

	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Identity re-exports the fbid type for library consumers.
type Identity = fbid.Identity

// Classify maps an input to a typed Identity without any network access.
func Classify(input string) Identity { return fbid.Classify(input) }

// Resolve classifies an input and, when it is a short link (fb.watch, fb.me,
// share/...), follows one redirect to fully classify it.
func (c *Client) Resolve(ctx context.Context, input string) (Identity, error) {
	id := fbid.Classify(input)
	if !id.ShortLink {
		return id, nil
	}
	final, err := c.followRedirect(ctx, id.CanonicalURL)
	if err != nil || final == "" {
		return id, nil // keep the short-link classification on failure
	}
	resolved := fbid.Classify(final)
	resolved.Input = input
	return resolved, nil
}

// followRedirect issues a GET that follows redirects and returns the final URL.
func (c *Client) followRedirect(ctx context.Context, raw string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.uas[0])
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	c.rateLimit()
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return resp.Request.URL.String(), nil
}
