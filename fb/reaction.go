package fb

import (
	"context"
	"iter"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Reactions returns the aggregate reaction summary for a post and a streaming
// iterator over individual reactors. On the mbasic surface only the coarse total
// is reliably available to an anonymous crawler; the per-reactor list is not
// exposed.
func (c *Client) Reactions(ctx context.Context, postURL string) (*ReactionSummary, iter.Seq2[Reaction, error]) {
	id := fbid.Classify(postURL)
	target := id.MBasicURL
	if target == "" {
		target = fbid.ToMBasic(postURL)
	}
	summary := &ReactionSummary{PostID: id.PostID, Source: "mbasic", FetchedAt: time.Now()}
	doc, err := c.GetDoc(ctx, target)
	if err == nil {
		body := cleanText(doc.Find("body").Text())
		summary.Like = firstNonZero(findCountNear(body, "reactions"), findCountNear(body, "likes"))
		summary.Total = summary.Like
	}

	list := func(yield func(Reaction, error) bool) {
		// The reaction profile browser lists reactors with their reaction type.
		ruURL := "https://mbasic.facebook.com/ufi/reaction/profile/browser/?ft_ent_identifier=" + id.PostID
		seen := map[string]bool{}
		next := ruURL
		for next != "" {
			rdoc, rerr := c.GetDoc(ctx, next)
			if rerr != nil {
				yield(Reaction{}, rerr)
				return
			}
			reactors := parseReactors(rdoc, id.PostID)
			for _, r := range reactors {
				key := r.ReactorURL + "|" + r.Type
				if seen[key] {
					continue
				}
				seen[key] = true
				if !yield(r, nil) {
					return
				}
			}
			next = parseReactionNext(rdoc, next)
		}
	}
	return summary, list
}

func parseReactors(doc *goquery.Document, postID string) []Reaction {
	var out []Reaction
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href := attr(a, "href")
		name := cleanText(a.Text())
		if name == "" || !looksLikeProfileHref(href) {
			return
		}
		u := fbid.ToCanonical(resolveRelative("https://mbasic.facebook.com/", href))
		out = append(out, Reaction{
			PostID:      postID,
			Type:        "like",
			ReactorName: name,
			ReactorURL:  u,
			ReactorID:   fbid.Classify(u).ProfileID,
			FetchedAt:   time.Now(),
		})
	})
	return out
}

func looksLikeProfileHref(href string) bool {
	if href == "" {
		return false
	}
	return strings.Contains(href, "profile.php?id=") ||
		strings.HasPrefix(href, "/") && !strings.Contains(href, "reaction") && !strings.Contains(href, "ufi")
}

func parseReactionNext(doc *goquery.Document, pageURL string) string {
	var next string
	doc.Find("a[href]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		txt := strings.ToLower(cleanText(a.Text()))
		if strings.Contains(txt, "see more") || strings.Contains(txt, "more") {
			next = fbid.ToMBasic(resolveRelative(pageURL, attr(a, "href")))
			return false
		}
		return true
	})
	return next
}
