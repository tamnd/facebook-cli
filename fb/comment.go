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
			for _, cm := range parseComments(doc, id.PostID) {
				if cm.CommentID == "" || seen[cm.CommentID] {
					continue
				}
				seen[cm.CommentID] = true
				if !yield(cm.Comment, nil) {
					return
				}
				emitted++
				if opt.Limit > 0 && emitted >= opt.Limit {
					return
				}
				// Expand the reply thread inline, attributing each reply to its
				// parent through ParentID. Replies count toward the same limit.
				if opt.Replies && cm.ReplyURL != "" {
					for _, rp := range c.replies(ctx, cm.ReplyURL, id.PostID, cm.CommentID, seen) {
						if rp.err != nil {
							yield(Comment{}, rp.err)
							return
						}
						if !yield(rp.Comment, nil) {
							return
						}
						emitted++
						if opt.Limit > 0 && emitted >= opt.Limit {
							return
						}
					}
				}
			}
			next = parseMoreCommentsLink(doc, next)
		}
	}
}

// replies walks a single comment's reply thread to exhaustion, returning each
// reply with ParentID set to the parent comment. Replies already seen on the
// main thread are skipped so a comment is never emitted twice.
func (c *Client) replies(ctx context.Context, threadURL, postID, parentID string, seen map[string]bool) []replyResult {
	var out []replyResult
	next := threadURL
	for next != "" {
		doc, err := c.GetDoc(ctx, next)
		if err != nil {
			out = append(out, replyResult{err: err})
			return out
		}
		for _, cm := range parseComments(doc, postID) {
			if cm.CommentID == "" || cm.CommentID == parentID || seen[cm.CommentID] {
				continue
			}
			seen[cm.CommentID] = true
			cm.ParentID = parentID
			out = append(out, replyResult{Comment: cm.Comment})
		}
		next = parseMoreCommentsLink(doc, next)
	}
	return out
}

type replyResult struct {
	Comment
	err error
}

// commentNode pairs a parsed comment with the link to its reply thread, when
// mbasic renders one ("View N replies").
type commentNode struct {
	Comment
	ReplyURL string
}

func parseComments(doc *goquery.Document, postID string) []commentNode {
	var out []commentNode
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
		out = append(out, commentNode{
			Comment: Comment{
				CommentID:     cid,
				PostID:        postID,
				AuthorName:    author,
				AuthorURL:     authorURL,
				AuthorID:      fbid.Classify(authorURL).ProfileID,
				Text:          text,
				CreatedAtText: timeText,
				CreatedAt:     parseTime(timeText, time.Now()),
				FetchedAt:     time.Now(),
			},
			ReplyURL: parseReplyThreadLink(s),
		})
	})
	return out
}

// parseReplyThreadLink finds the link to a comment's reply thread inside its
// container. mbasic labels it "View N replies" / "N replies" and points it at a
// comment/replies endpoint.
func parseReplyThreadLink(s *goquery.Selection) string {
	var link string
	s.Find("a[href]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		href := attr(a, "href")
		txt := strings.ToLower(cleanText(a.Text()))
		if strings.Contains(href, "comment/replies") ||
			strings.Contains(href, "/replies/") ||
			(strings.Contains(txt, "repl") && strings.Contains(href, "comment_id")) {
			link = fbid.ToMBasic(resolveRelative("https://mbasic.facebook.com/", href))
			return false
		}
		return true
	})
	return link
}

func isCommentID(id string) bool {
	// Facebook comment containers are numeric ids, often long.
	return len(id) >= 8 && isNumeric(id)
}

func parseMoreCommentsLink(doc *goquery.Document, pageURL string) string {
	var next string
	doc.Find("a[href]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		txt := strings.ToLower(cleanText(a.Text()))
		href := attr(a, "href")
		if strings.Contains(txt, "view more comments") ||
			strings.Contains(txt, "view previous comments") ||
			strings.Contains(txt, "more comments") ||
			// href fallback: mbasic's comment cursor links carry these params even
			// when the label is localized or rendered as an icon.
			strings.Contains(href, "ucfx") ||
			(strings.Contains(href, "comment") && strings.Contains(href, "cursor")) {
			next = fbid.ToMBasic(resolveRelative(pageURL, href))
			return false
		}
		return true
	})
	return next
}
