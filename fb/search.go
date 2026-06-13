package fb

import (
	"context"
	"iter"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

var searchPaths = map[string]string{
	"page":    "pages",
	"profile": "people",
	"group":   "groups",
	"post":    "posts",
	"photo":   "photos",
	"video":   "videos",
	"event":   "events",
	"all":     "top",
}

// Search streams search results, optionally filtered to one entity type.
func (c *Client) Search(ctx context.Context, q string, opt SearchOptions) iter.Seq2[SearchResult, error] {
	typ := opt.Type
	if typ == "" {
		typ = "all"
	}
	seg, ok := searchPaths[typ]
	if !ok {
		seg = "top"
	}
	target := "https://mbasic.facebook.com/search/" + seg + "/?q=" + url.QueryEscape(q)
	return func(yield func(SearchResult, error) bool) {
		emitted := 0
		seen := map[string]bool{}
		next := target
		for next != "" {
			doc, err := c.GetDoc(ctx, next)
			if err != nil {
				yield(SearchResult{}, err)
				return
			}
			for _, r := range parseSearchResults(doc, q, typ) {
				if r.URL == "" || seen[r.URL] {
					continue
				}
				seen[r.URL] = true
				if !yield(r, nil) {
					return
				}
				emitted++
				if opt.Limit > 0 && emitted >= opt.Limit {
					return
				}
			}
			next = parseSearchNext(doc, next)
		}
	}
}

func parseSearchResults(doc *goquery.Document, q, typ string) []SearchResult {
	var out []SearchResult
	seen := map[string]bool{}
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href := attr(a, "href")
		title := cleanText(a.Text())
		if title == "" || len(title) < 2 {
			return
		}
		if !looksLikeSearchHit(href) {
			return
		}
		canon := fbid.ToCanonical(resolveRelative("https://mbasic.facebook.com/", href))
		if seen[canon] {
			return
		}
		seen[canon] = true
		id := fbid.Classify(canon)
		out = append(out, SearchResult{
			Query:      q,
			ResultType: resultType(typ, id),
			EntityID:   firstNonEmpty(id.PageID, id.ProfileID, id.GroupID, id.PostID, id.VideoID, id.EventID),
			Title:      truncate(title, 200),
			URL:        canon,
			FetchedAt:  time.Now(),
		})
	})
	return out
}

func resultType(typ string, id fbid.Identity) string {
	if typ != "" && typ != "all" {
		return typ
	}
	return string(id.Kind)
}

func looksLikeSearchHit(href string) bool {
	if href == "" || strings.HasPrefix(href, "#") {
		return false
	}
	if strings.Contains(href, "/search/") || strings.Contains(href, "login") || strings.Contains(href, "help") {
		return false
	}
	return strings.HasPrefix(href, "/") || strings.Contains(href, "facebook.com/")
}

func parseSearchNext(doc *goquery.Document, pageURL string) string {
	var next string
	doc.Find("a[href]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		txt := strings.ToLower(cleanText(a.Text()))
		if strings.Contains(txt, "see more results") || strings.Contains(txt, "see more") || strings.Contains(txt, "show more") {
			next = fbid.ToMBasic(resolveRelative(pageURL, attr(a, "href")))
			return false
		}
		return true
	})
	return next
}
