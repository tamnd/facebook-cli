package fb

import (
	"context"
	"iter"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Comments streams every comment on a post, following the "View more comments"
// pagination. When opt.Replies is set, reply threads are expanded inline.
func (c *Client) Comments(ctx context.Context, postURL string, opt CommentOptions) iter.Seq2[Comment, error] {
	id := fbid.Classify(postURL)
	target := id.MBasicURL
	if target == "" {
		target = fbid.ToMBasic(postURL)
	}
	return func(yield func(Comment, error) bool) {
		emitted := 0
		seen := map[string]bool{}
		next := target
		for next != "" {
			doc, err := c.GetDoc(ctx, next)
			if err != nil {
				yield(Comment{}, err)
				return
			}
			comments := parseComments(doc, id.PostID)
			for _, cm := range comments {
				if cm.CommentID == "" || seen[cm.CommentID] {
					continue
				}
				seen[cm.CommentID] = true
				if !yield(cm, nil) {
					return
				}
				emitted++
				if opt.Limit > 0 && emitted >= opt.Limit {
					return
				}
			}
			next = parseMoreCommentsLink(doc, next)
		}
	}
}

func parseComments(doc *goquery.Document, postID string) []Comment {
	var out []Comment
	// mbasic renders each comment under a div whose id is the comment id.
	doc.Find(`div[id]`).Each(func(_ int, s *goquery.Selection) {
		cid := attr(s, "id")
		if !isCommentID(cid) {
			return
		}
		author := cleanText(s.Find("h3 a, strong a").First().Text())
		text := cleanText(s.Find(`div[dir="auto"], h3 + div, span`).First().Text())
		if author == "" && text == "" {
			return
		}
		authorURL := fbid.ToCanonical(resolveRelative("https://mbasic.facebook.com/", attr(s.Find("h3 a, strong a").First(), "href")))
		timeText := cleanText(s.Find("abbr").First().Text())
		out = append(out, Comment{
			CommentID:     cid,
			PostID:        postID,
			AuthorName:    author,
			AuthorURL:     authorURL,
			AuthorID:      fbid.Classify(authorURL).ProfileID,
			Text:          text,
			CreatedAtText: timeText,
			CreatedAt:     parseTime(timeText, time.Now()),
			FetchedAt:     time.Now(),
		})
	})
	return out
}

func isCommentID(id string) bool {
	// Facebook comment containers are numeric ids, often long.
	return len(id) >= 8 && isNumeric(id)
}

func parseMoreCommentsLink(doc *goquery.Document, pageURL string) string {
	var next string
	doc.Find("a[href]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		txt := strings.ToLower(cleanText(a.Text()))
		if strings.Contains(txt, "view more comments") ||
			strings.Contains(txt, "view previous comments") ||
			strings.Contains(txt, "more comments") {
			next = fbid.ToMBasic(resolveRelative(pageURL, attr(a, "href")))
			return false
		}
		return true
	})
	return next
}
