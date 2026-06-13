package fb

import (
	"context"
	"iter"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Photo fetches one photo's metadata.
func (c *Client) Photo(ctx context.Context, idOrURL string) (*Photo, error) {
	id := fbid.Classify(idOrURL)
	target := id.MBasicURL
	if id.PhotoID != "" {
		target = "https://mbasic.facebook.com/photo.php?fbid=" + id.PhotoID
	}
	doc, err := c.GetDoc(ctx, target)
	if err != nil {
		return nil, err
	}
	return parsePhoto(doc, id), nil
}

func parsePhoto(doc *goquery.Document, id fbid.Identity) *Photo {
	full := ""
	doc.Find("img").EachWithBreak(func(_ int, img *goquery.Selection) bool {
		s := attr(img, "src")
		if strings.Contains(s, "scontent") || strings.Contains(s, "fbcdn") {
			full = s
			return false
		}
		return true
	})
	return &Photo{
		PhotoID:   id.PhotoID,
		OwnerID:   id.OwnerID,
		Caption:   truncate(cleanText(doc.Find(`div[dir="auto"], figcaption`).First().Text()), 500),
		FullURL:   full,
		FetchedAt: time.Now(),
	}
}

// Photos streams a page or profile's photos.
func (c *Client) Photos(ctx context.Context, idOrURL string, opt ListOptions) iter.Seq2[Photo, error] {
	id := fbid.Classify(idOrURL)
	slug := firstNonEmpty(id.Slug, id.PageID, id.ProfileID)
	feed := "https://mbasic.facebook.com/" + slug + "/photos"
	return func(yield func(Photo, error) bool) {
		emitted := 0
		seen := map[string]bool{}
		next := feed
		for next != "" {
			doc, err := c.GetDoc(ctx, next)
			if err != nil {
				yield(Photo{}, err)
				return
			}
			photos := parsePhotoLinks(doc, slug)
			for _, p := range photos {
				if seen[p.PhotoID] {
					continue
				}
				seen[p.PhotoID] = true
				if !yield(p, nil) {
					return
				}
				emitted++
				if opt.Limit > 0 && emitted >= opt.Limit {
					return
				}
			}
			next = parsePhotoNext(doc, next)
		}
	}
}

func parsePhotoLinks(doc *goquery.Document, owner string) []Photo {
	var out []Photo
	seen := map[string]bool{}
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href := attr(a, "href")
		pid := fbid.Classify(href).PhotoID
		if pid == "" || seen[pid] {
			return
		}
		seen[pid] = true
		thumb := attr(a.Find("img").First(), "src")
		out = append(out, Photo{
			PhotoID:   pid,
			OwnerID:   owner,
			ThumbURL:  thumb,
			FetchedAt: time.Now(),
		})
	})
	return out
}

func parsePhotoNext(doc *goquery.Document, pageURL string) string {
	var next string
	doc.Find("a[href]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		txt := strings.ToLower(cleanText(a.Text()))
		if strings.Contains(txt, "see more") || strings.Contains(txt, "more photos") || strings.Contains(txt, "show more") {
			next = fbid.ToMBasic(resolveRelative(pageURL, attr(a, "href")))
			return false
		}
		return true
	})
	return next
}
