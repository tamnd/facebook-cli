package fb

import (
	"context"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Post fetches a single post's full detail.
func (c *Client) Post(ctx context.Context, idOrURL string, opt PostOptions) (*Post, error) {
	id := fbid.Classify(idOrURL)
	target := id.MBasicURL
	if target == "" {
		target = fbid.ToMBasic(idOrURL)
	}
	doc, err := c.GetDoc(ctx, target)
	if err != nil {
		return nil, err
	}
	post := parsePostDetail(doc, id, target)
	return post, nil
}

func parsePostDetail(doc *goquery.Document, id fbid.Identity, target string) *Post {
	container := doc.Find("#m_story_permalink_view").First()
	if container.Length() == 0 {
		container = doc.Find("article").First()
	}
	if container.Length() == 0 {
		container = doc.Find("body")
	}

	text := extractPostText(container)
	body := cleanText(container.Text())

	post := &Post{
		PostID:        id.PostID,
		OwnerID:       id.OwnerID,
		Text:          text,
		CreatedAtText: extractTimeText(container),
		LikeCount:     firstNonZero(findCountNear(body, "reactions"), findCountNear(body, "likes"), findCountNear(body, " like")),
		CommentCount:  findCountNear(body, "comment"),
		ShareCount:    findCountNear(body, "share"),
		Permalink:     fbid.ToCanonical(target),
		MediaURLs:     extractMediaURLs(container),
		ExternalLinks: extractExternalLinks(container),
		FetchedAt:     time.Now(),
	}
	post.ReactionCount = post.LikeCount
	post.CreatedAt = parseTime(post.CreatedAtText, time.Now())
	post.OwnerName = firstNonEmpty(cleanText(container.Find("h3 a, strong a").First().Text()), cleanText(doc.Find("h3 a, strong a").First().Text()))
	return post
}

func extractPostText(sel *goquery.Selection) string {
	// mbasic wraps the body text in a div with dir="auto" or a p block.
	if v := cleanText(sel.Find(`div[data-ft] p, p`).First().Text()); v != "" {
		return v
	}
	if v := cleanText(sel.Find(`div[dir="auto"]`).First().Text()); v != "" {
		return v
	}
	return ""
}

func extractTimeText(sel *goquery.Selection) string {
	if v := cleanText(sel.Find("abbr").First().Text()); v != "" {
		return v
	}
	return ""
}

func extractMediaURLs(sel *goquery.Selection) []string {
	var out []string
	seen := map[string]bool{}
	sel.Find("img").Each(func(_ int, img *goquery.Selection) {
		s := attr(img, "src")
		if (strings.Contains(s, "scontent") || strings.Contains(s, "fbcdn")) && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	})
	return out
}

func extractExternalLinks(sel *goquery.Selection) []string {
	var out []string
	seen := map[string]bool{}
	sel.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href := attr(a, "href")
		if strings.Contains(href, "l.php") {
			if t := extractRedirectTarget(href); t != "" && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	})
	return out
}
