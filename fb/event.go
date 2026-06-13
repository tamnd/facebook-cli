package fb

import (
	"context"
	"iter"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Event fetches one public event.
func (c *Client) Event(ctx context.Context, idOrURL string) (*Event, error) {
	id := fbid.Classify(idOrURL)
	eid := id.EventID
	if eid == "" {
		eid = id.Slug
	}
	doc, err := c.GetDoc(ctx, "https://mbasic.facebook.com/events/"+eid)
	if err != nil {
		return nil, err
	}
	return parseEvent(doc, eid), nil
}

func parseEvent(doc *goquery.Document, eid string) *Event {
	body := cleanText(doc.Find("body").Text())
	startText := cleanText(doc.Find("abbr, time").First().Text())
	return &Event{
		EventID:         eid,
		Name:            firstNonEmpty(cleanText(doc.Find("h1").First().Text()), stripTitleSuffix(cleanText(doc.Find("title").First().Text()))),
		Description:     truncate(cleanText(doc.Find(`div[dir="auto"]`).First().Text()), 1000),
		StartText:       startText,
		StartAt:         parseTime(startText, time.Now()),
		GoingCount:      findCountNear(body, "going"),
		InterestedCount: findCountNear(body, "interested"),
		Online:          strings.Contains(strings.ToLower(body), "online event"),
		URL:             "https://www.facebook.com/events/" + eid,
		FetchedAt:       time.Now(),
	}
}

// Events streams a page's events.
func (c *Client) Events(ctx context.Context, idOrURL string, opt ListOptions) iter.Seq2[Event, error] {
	id := fbid.Classify(idOrURL)
	slug := firstNonEmpty(id.Slug, id.PageID)
	feed := "https://mbasic.facebook.com/" + slug + "/upcoming_events"
	return func(yield func(Event, error) bool) {
		emitted := 0
		seen := map[string]bool{}
		doc, err := c.GetDoc(ctx, feed)
		if err != nil {
			yield(Event{}, err)
			return
		}
		doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
			href := attr(a, "href")
			ev := fbid.Classify(href)
			if ev.EventID == "" || seen[ev.EventID] {
				return
			}
			seen[ev.EventID] = true
			e := Event{
				EventID:   ev.EventID,
				Name:      cleanText(a.Text()),
				URL:       "https://www.facebook.com/events/" + ev.EventID,
				FetchedAt: time.Now(),
			}
			if !yield(e, nil) {
				return
			}
			emitted++
		})
	}
}
