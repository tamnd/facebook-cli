package fb

import (
	"context"
	"iter"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Profile fetches a person's public profile.
func (c *Client) Profile(ctx context.Context, idOrURL string) (*Profile, error) {
	id := fbid.Classify(idOrURL)
	target := id.MBasicURL
	if id.ProfileID != "" && strings.HasPrefix(id.ProfileID, "pfbid") {
		target = "https://mbasic.facebook.com/" + id.ProfileID
	} else if id.ProfileID != "" && isNumeric(id.ProfileID) {
		target = "https://mbasic.facebook.com/profile.php?id=" + id.ProfileID
	} else if id.Slug != "" {
		target = "https://mbasic.facebook.com/" + id.Slug
	}
	doc, err := c.GetDoc(ctx, target)
	if err != nil {
		return nil, err
	}
	return parseProfile(doc, id), nil
}

func parseProfile(doc *goquery.Document, id fbid.Identity) *Profile {
	body := cleanText(doc.Find("body").Text())
	name := firstNonEmpty(
		cleanText(doc.Find("h1").First().Text()),
		stripTitleSuffix(cleanText(doc.Find("title").First().Text())),
		cleanText(doc.Find("strong").First().Text()),
	)
	pid := id.ProfileID
	if pid == "" {
		pid = id.Slug
	}
	return &Profile{
		ProfileID:      pid,
		Username:       id.Slug,
		Name:           name,
		Intro:          truncate(extractAbout(doc, ""), 500),
		FollowersCount: findCountNear(body, "followers"),
		FriendsCount:   findCountNear(body, "friends"),
		Verified:       strings.Contains(strings.ToLower(body), "verified"),
		Work:           findLabeled(doc, "Works at"),
		Education:      findLabeled(doc, "Studied at", "Studies at", "Went to"),
		Hometown:       findLabeled(doc, "From"),
		CurrentCity:    findLabeled(doc, "Lives in"),
		AvatarURL:      findAvatar(doc),
		URL:            fbid.ToCanonical(id.MBasicURL),
		FetchedAt:      time.Now(),
	}
}

// ProfilePosts streams a profile's public timeline.
func (c *Client) ProfilePosts(ctx context.Context, idOrURL string, opt ListOptions) iter.Seq2[Post, error] {
	id := fbid.Classify(idOrURL)
	owner := firstNonEmpty(id.ProfileID, id.Slug)
	return c.walkFeed(ctx, id.MBasicURL, owner, "profile", opt)
}

func findLabeled(doc *goquery.Document, labels ...string) string {
	out := ""
	doc.Find("div, span, td").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		txt := cleanText(s.Text())
		for _, label := range labels {
			if strings.HasPrefix(txt, label) && len(txt) > len(label)+1 {
				out = strings.TrimSpace(strings.TrimPrefix(txt, label))
				return false
			}
		}
		return true
	})
	return out
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
