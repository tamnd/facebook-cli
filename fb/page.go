package fb

import (
	"context"
	"iter"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Page fetches a Page's metadata from its crawler page.
func (c *Client) Page(ctx context.Context, idOrURL string) (*Page, error) {
	id := fbid.Classify(idOrURL)
	slug := id.Slug
	if slug == "" {
		slug = id.PageID
	}
	doc, err := c.getHTML(ctx, "https://www.facebook.com/"+slug+"/")
	if err != nil {
		return nil, err
	}
	return parsePageSSR(doc, slug), nil
}

// PagePosts streams the posts the Page's crawler page exposes (its most recent
// stories), fetching each post's detail.
func (c *Client) PagePosts(ctx context.Context, idOrURL string, opt ListOptions) iter.Seq2[Post, error] {
	id := fbid.Classify(idOrURL)
	slug := id.Slug
	if slug == "" {
		slug = id.PageID
	}
	return c.walkFeed(ctx, "https://www.facebook.com/"+slug+"/", slug, "page", opt)
}

// walkFeed is shared by page/profile/group feeds: it reads the post permalinks
// embedded in the crawler page and fetches each post's detail. The crawler
// surface exposes the most recent stories rather than the full history.
func (c *Client) walkFeed(ctx context.Context, feedURL, ownerID, ownerType string, opt ListOptions) iter.Seq2[Post, error] {
	return func(yield func(Post, error) bool) {
		doc, err := c.getHTML(ctx, feedURL)
		if err != nil {
			yield(Post{}, err)
			return
		}
		emitted := 0
		seen := map[string]bool{}
		for _, link := range findPostPermalinks(doc) {
			pid := fbid.Classify(link).PostID
			if pid == "" || seen[pid] {
				continue
			}
			seen[pid] = true
			post, perr := c.Post(ctx, link, PostOptions{})
			if perr != nil {
				// a single bad post should not abort the whole feed
				c.logf(1, "skip post %s: %v", pid, perr)
				continue
			}
			if post.OwnerID == "" {
				post.OwnerID = ownerID
			}
			if post.OwnerType == "" {
				post.OwnerType = ownerType
			}
			if !opt.Until.IsZero() && !post.CreatedAt.IsZero() && post.CreatedAt.After(opt.Until) {
				continue
			}
			if !opt.Since.IsZero() && !post.CreatedAt.IsZero() && post.CreatedAt.Before(opt.Since) {
				return // feeds are reverse-chronological; stop once we cross the floor
			}
			if !yield(*post, nil) {
				return
			}
			emitted++
			if opt.Limit > 0 && emitted >= opt.Limit {
				return
			}
		}
	}
}

func stripTitleSuffix(s string) string {
	for _, sep := range []string{" | Facebook", " - Facebook", " | Posts", " | "} {
		if i := strings.Index(s, sep); i > 0 {
			return strings.TrimSpace(s[:i])
		}
	}
	return s
}

func extractAbout(doc *goquery.Document, body string) string {
	if v := cleanText(doc.Find(`div#bio, div[data-sigil="profile-intro-card-bio"]`).First().Text()); v != "" {
		return v
	}
	return body
}

func findAvatar(doc *goquery.Document) string {
	src := ""
	doc.Find("img").EachWithBreak(func(_ int, img *goquery.Selection) bool {
		s := attr(img, "src")
		if strings.Contains(s, "scontent") || strings.Contains(s, "fbcdn") {
			src = s
			return false
		}
		return true
	})
	return src
}

func firstNonZero(vals ...int64) int64 {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}
