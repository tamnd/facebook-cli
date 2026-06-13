package fb

import (
	"context"
	"iter"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Group fetches a group's metadata.
func (c *Client) Group(ctx context.Context, idOrURL string) (*Group, error) {
	id := fbid.Classify(idOrURL)
	gid := id.GroupID
	if gid == "" {
		gid = id.Slug
	}
	doc, err := c.GetDoc(ctx, "https://mbasic.facebook.com/groups/"+gid)
	if err != nil {
		return nil, err
	}
	return parseGroup(doc, gid), nil
}

func parseGroup(doc *goquery.Document, gid string) *Group {
	body := cleanText(doc.Find("body").Text())
	low := strings.ToLower(body)
	privacy := "public"
	switch {
	case strings.Contains(low, "private group"):
		privacy = "private"
	case strings.Contains(low, "public group"):
		privacy = "public"
	}
	return &Group{
		GroupID:      gid,
		Slug:         gid,
		Name:         firstNonEmpty(cleanText(doc.Find("h1").First().Text()), stripTitleSuffix(cleanText(doc.Find("title").First().Text()))),
		Description:  truncate(extractAbout(doc, ""), 1000),
		Privacy:      privacy,
		MembersCount: firstNonZero(findCountNear(body, "members"), findCountNear(body, "total members")),
		URL:          "https://www.facebook.com/groups/" + gid,
		FetchedAt:    time.Now(),
	}
}

// GroupPosts streams a group's feed.
func (c *Client) GroupPosts(ctx context.Context, idOrURL string, opt ListOptions) iter.Seq2[Post, error] {
	id := fbid.Classify(idOrURL)
	gid := id.GroupID
	if gid == "" {
		gid = id.Slug
	}
	return c.walkFeed(ctx, "https://www.facebook.com/groups/"+gid, gid, "group", opt)
}
