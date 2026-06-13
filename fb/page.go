package fb

import (
	"context"
	"iter"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Page fetches a Page's metadata from the mbasic about surface.
func (c *Client) Page(ctx context.Context, idOrURL string) (*Page, error) {
	id := fbid.Classify(idOrURL)
	slug := id.Slug
	if slug == "" {
		slug = id.PageID
	}
	doc, err := c.GetDoc(ctx, "https://mbasic.facebook.com/"+slug)
	if err != nil {
		return nil, err
	}
	return parsePage(doc, slug), nil
}

func parsePage(doc *goquery.Document, slug string) *Page {
	body := cleanText(doc.Find("body").Text())
	name := firstNonEmpty(
		cleanText(doc.Find("h1").First().Text()),
		stripTitleSuffix(cleanText(doc.Find("title").First().Text())),
		cleanText(doc.Find("strong").First().Text()),
	)
	return &Page{
		PageID:         slug,
		Slug:           slug,
		Name:           name,
		About:          truncate(extractAbout(doc, body), 1000),
		Category:       extractCategory(doc),
		LikesCount:     findCountNear(body, "people like this"),
		FollowersCount: firstNonZero(findCountNear(body, "people follow this"), findCountNear(body, "followers")),
		Verified:       strings.Contains(strings.ToLower(body), "verified"),
		Website:        findFirstExternalLink(doc),
		Phone:          findPhone(body),
		AvatarURL:      findAvatar(doc),
		URL:            "https://www.facebook.com/" + slug,
		FetchedAt:      time.Now(),
	}
}

// PagePosts streams a Page's feed, following the mbasic "See more" links.
func (c *Client) PagePosts(ctx context.Context, idOrURL string, opt ListOptions) iter.Seq2[Post, error] {
	id := fbid.Classify(idOrURL)
	slug := id.Slug
	if slug == "" {
		slug = id.PageID
	}
	return c.walkFeed(ctx, "https://mbasic.facebook.com/"+slug, slug, "page", opt)
}

// walkFeed is shared by page/profile/group feeds: it parses post links from a
// feed page, fetches each post's detail, then follows the next-page link.
func (c *Client) walkFeed(ctx context.Context, feedURL, ownerID, ownerType string, opt ListOptions) iter.Seq2[Post, error] {
	return func(yield func(Post, error) bool) {
		emitted := 0
		seen := map[string]bool{}
		next := feedURL
		for next != "" {
			doc, err := c.GetDoc(ctx, next)
			if err != nil {
				yield(Post{}, err)
				return
			}
			links := parseFeedPostLinks(doc, next)
			for _, link := range links {
				pid := fbid.Classify(link).PostID
				if pid == "" || seen[pid] {
					continue
				}
				seen[pid] = true
				post, perr := c.Post(ctx, link, PostOptions{NoDetail: false})
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
			next = parseNextPageLink(doc, next)
		}
	}
}

func parseFeedPostLinks(doc *goquery.Document, pageURL string) []string {
	seen := map[string]bool{}
	var links []string
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href := attr(a, "href")
		if !looksLikePostLink(href) {
			return
		}
		full := fbid.ToMBasic(resolveRelative(pageURL, href))
		pid := fbid.Classify(full).PostID
		if pid == "" || seen[pid] {
			return
		}
		seen[pid] = true
		links = append(links, full)
	})
	return links
}

func parseNextPageLink(doc *goquery.Document, pageURL string) string {
	var next string
	doc.Find("a[href]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		txt := strings.ToLower(cleanText(a.Text()))
		if strings.Contains(txt, "see more posts") ||
			strings.Contains(txt, "more stories") ||
			strings.Contains(txt, "show more") ||
			txt == "see more" {
			next = fbid.ToMBasic(resolveRelative(pageURL, attr(a, "href")))
			return false
		}
		return true
	})
	return next
}

func looksLikePostLink(href string) bool {
	if href == "" {
		return false
	}
	return strings.Contains(href, "story_fbid=") ||
		strings.Contains(href, "/posts/") ||
		strings.Contains(href, "permalink.php") ||
		strings.Contains(href, "story.php")
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

func extractCategory(doc *goquery.Document) string {
	cat := ""
	doc.Find("a[href]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		if strings.Contains(attr(a, "href"), "/pages/category/") {
			cat = cleanText(a.Text())
			return false
		}
		return true
	})
	return cat
}

func findFirstExternalLink(doc *goquery.Document) string {
	link := ""
	doc.Find("a[href]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		href := attr(a, "href")
		if strings.Contains(href, "lm.facebook.com/l.php") || strings.Contains(href, "l.facebook.com/l.php") {
			if u := extractRedirectTarget(href); u != "" {
				link = u
				return false
			}
		}
		return true
	})
	return link
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
