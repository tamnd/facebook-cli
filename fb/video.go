package fb

import (
	"context"
	"iter"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

var hdSrcRe = regexp.MustCompile(`(?:hd_src|sd_src|playable_url(?:_quality_hd)?)["']?\s*[:=]\s*["']([^"']+)["']`)

// Video fetches one video or reel, including any discoverable stream URLs.
func (c *Client) Video(ctx context.Context, idOrURL string) (*Video, error) {
	id := fbid.Classify(idOrURL)
	target := id.MBasicURL
	if id.VideoID != "" {
		target = "https://mbasic.facebook.com/watch/?v=" + id.VideoID
	}
	body, err := c.GetRaw(ctx, target)
	if err != nil {
		return nil, err
	}
	doc, derr := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if derr != nil {
		return nil, derr
	}
	v := &Video{
		VideoID:       id.VideoID,
		Title:         firstNonEmpty(cleanText(doc.Find("h1").First().Text()), stripTitleSuffix(cleanText(doc.Find("title").First().Text()))),
		Description:   truncate(cleanText(doc.Find(`div[dir="auto"]`).First().Text()), 1000),
		Permalink:     fbid.ToCanonical(target),
		IsReel:        strings.Contains(target, "/reel/"),
		Streams:       extractStreams(body),
		FetchedAt:     time.Now(),
		CreatedAtText: cleanText(doc.Find("abbr").First().Text()),
	}
	v.CreatedAt = parseTime(v.CreatedAtText, time.Now())
	return v, nil
}

func extractStreams(body []byte) []Stream {
	var out []Stream
	seen := map[string]bool{}
	for _, m := range hdSrcRe.FindAllStringSubmatch(string(body), -1) {
		u := strings.ReplaceAll(m[1], `\/`, "/")
		u = strings.ReplaceAll(u, `%`, "%")
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, Stream{URL: u, MIME: "video/mp4"})
	}
	return out
}

// Videos streams a page's videos and reels.
func (c *Client) Videos(ctx context.Context, idOrURL string, opt ListOptions) iter.Seq2[Video, error] {
	id := fbid.Classify(idOrURL)
	slug := firstNonEmpty(id.Slug, id.PageID)
	feed := "https://mbasic.facebook.com/" + slug + "/videos"
	return func(yield func(Video, error) bool) {
		emitted := 0
		seen := map[string]bool{}
		next := feed
		for next != "" {
			doc, err := c.GetDoc(ctx, next)
			if err != nil {
				yield(Video{}, err)
				return
			}
			vids := parseVideoLinks(doc, slug)
			for _, v := range vids {
				if seen[v.VideoID] {
					continue
				}
				seen[v.VideoID] = true
				if !yield(v, nil) {
					return
				}
				emitted++
				if opt.Limit > 0 && emitted >= opt.Limit {
					return
				}
			}
			next = parseVideoNext(doc, next)
		}
	}
}

func parseVideoLinks(doc *goquery.Document, owner string) []Video {
	var out []Video
	seen := map[string]bool{}
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href := attr(a, "href")
		vc := fbid.Classify(href)
		if vc.VideoID == "" || seen[vc.VideoID] {
			return
		}
		seen[vc.VideoID] = true
		out = append(out, Video{
			VideoID:   vc.VideoID,
			OwnerID:   owner,
			Title:     cleanText(a.Text()),
			Permalink: fbid.ToCanonical(resolveRelative("https://mbasic.facebook.com/", href)),
			IsReel:    strings.Contains(href, "/reel/"),
			FetchedAt: time.Now(),
		})
	})
	return out
}

func parseVideoNext(doc *goquery.Document, pageURL string) string {
	var next string
	doc.Find("a[href]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		txt := strings.ToLower(cleanText(a.Text()))
		if strings.Contains(txt, "see more") || strings.Contains(txt, "more videos") || strings.Contains(txt, "show more") {
			next = fbid.ToMBasic(resolveRelative(pageURL, attr(a, "href")))
			return false
		}
		return true
	})
	return next
}
