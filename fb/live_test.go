//go:build live

package fb

import (
	"context"
	"os"
	"testing"
	"time"
)

// live_test.go reads facebook.com.
//
//	go test ./fb -tags live -v
//
// It is behind a build tag because it is slow, it is rate limited, and it fails
// for reasons that have nothing to do with the code: a page gets taken down, a
// checkpoint appears, somebody runs it from an IP Facebook does not like. None
// of that should turn a red mark on a pull request.
//
// What it is for is the failure the fixture suite cannot see. Every other test
// in this package reads bytes that were captured once and never change, so all
// of them keep passing on the day Facebook renames a field. This one is the
// only thing that notices, and the point is to notice early rather than from a
// bug report.
//
// So it asserts shapes, not values. NASA's follower count is different every
// day and a test that pins it is a test that fails every day; a test that says
// the count is present and is more than a million says the field is still being
// found, which is the thing that breaks. The same goes for names, ids and URLs:
// what matters is that they are there and look like what they are.
//
// FB_COOKIES in the environment raises it to Tier 1, and the tests that need a
// session skip without it. Nothing here writes a cookie anywhere.

// liveEngine builds an engine with the cache off, because a cached answer is
// the fixture suite again and this is the one place where a real request is the
// whole point.
func liveEngine(t *testing.T) *Engine {
	t.Helper()
	cfg := Defaults()
	cfg.NoCache = true
	cfg.Rate = 2 * time.Second
	cfg.Cookies = os.Getenv("FB_COOKIES")
	if testing.Verbose() {
		cfg.Verbose = t.Logf
	}
	e, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	return e
}

// liveCtx bounds a read, so a hung connection fails the test rather than the
// whole run.
func liveCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// needSession skips a test that cannot work signed out.
func needSession(t *testing.T) {
	t.Helper()
	if os.Getenv("FB_COOKIES") == "" {
		t.Skip("needs FB_COOKIES: this read is Tier 1")
	}
}

// TestLiveProfile is the load-bearing one. Every other read starts from a page
// fetch, so if the Comet blocks stop being found here they stop being found
// everywhere, and this is the test that says so first.
func TestLiveProfile(t *testing.T) {
	p, err := liveEngine(t).Profile(liveCtx(t), "nasa", ProfileOptions{})
	if err != nil {
		t.Fatalf("read nasa: %v", err)
	}
	if !numeric(p.ID) {
		t.Errorf("id = %q, want a numeric node id", p.ID)
	}
	if p.Name == "" {
		t.Error("no name")
	}
	if p.Kind != "page" {
		t.Errorf("kind = %q, want page", p.Kind)
	}
	// A number this size only comes from the Comet store or the meta head, so
	// a zero here means both are being missed rather than that NASA lost its
	// audience overnight.
	if p.Followers < 1_000_000 && p.Likes < 1_000_000 {
		t.Errorf("followers %d and likes %d, and one of them should be in the millions", p.Followers, p.Likes)
	}
	if p.Avatar.URI == "" {
		t.Error("no avatar, so the CDN URLs are not being picked up")
	}
	if len(p.Surfaces) == 0 {
		t.Error("the envelope names no surface, so nothing is recording where this came from")
	}
	if p.FetchedAt.IsZero() {
		t.Error("no fetched_at, and every record gets one")
	}
}

// TestLiveProfileAbout reads the second page, which is where the whole About
// section lives and where a rename would show up as an empty record rather than
// an error.
func TestLiveProfileAbout(t *testing.T) {
	p, err := liveEngine(t).Profile(liveCtx(t), "nasa", ProfileOptions{About: true, NoPosts: true})
	if err != nil {
		t.Fatalf("read nasa about: %v", err)
	}
	if len(p.Category) == 0 && p.Bio.Empty() && len(p.Websites) == 0 {
		t.Error("the About page produced no category, no bio and no website, so the section is not being parsed")
	}
}

// TestLiveFeed. Signed out, Facebook ships exactly one post with the page, and
// the record is expected to say so in Missed rather than to look like a page
// that has posted once.
func TestLiveFeed(t *testing.T) {
	f, err := liveEngine(t).Feed(liveCtx(t), "nasa", 5)
	if err != nil {
		t.Fatalf("read the nasa feed: %v", err)
	}
	if len(f.Posts) == 0 {
		t.Fatal("no posts at all")
	}
	p := f.Posts[0]
	if p.ID == "" || p.URL == "" {
		t.Errorf("the first post has id %q url %q", p.ID, p.URL)
	}
	if p.CreatedAt.IsZero() {
		t.Error("no timestamp on the first post")
	}
	if os.Getenv("FB_COOKIES") == "" && len(f.Posts) == 1 && len(f.Missed) == 0 {
		t.Error("one post signed out and nothing in Missed to say why, so the envelope is not being filled in")
	}
}

// TestLivePost reads a permalink whole, which is the only signed-out surface
// that carries comments.
func TestLivePost(t *testing.T) {
	f, err := liveEngine(t).Feed(liveCtx(t), "nasa", 1)
	if err != nil || len(f.Posts) == 0 {
		t.Skipf("no post to read: %v", err)
	}
	p, err := liveEngine(t).Post(liveCtx(t), f.Posts[0].URL, "")
	if err != nil {
		t.Fatalf("read %s: %v", f.Posts[0].URL, err)
	}
	if p.ID == "" {
		t.Error("no id")
	}
	if p.Author.Empty() {
		t.Error("no author, so nobody is claimed to have written it")
	}
	if p.Counts.Reactions == 0 && p.Counts.Comments == 0 && p.Counts.Shares == 0 {
		t.Error("no counts of any kind, and a NASA post has all three")
	}
}

// TestLivePhotoSection reads the photo tab. The permalinks it returns are what
// `fb photo` and `--download` work from, so an empty list here is the whole
// media plane going quiet.
func TestLivePhotoSection(t *testing.T) {
	s, err := liveEngine(t).Photos(liveCtx(t), "nasa", 8)
	if err != nil {
		t.Fatalf("read the nasa photos: %v", err)
	}
	if len(s.Photos) == 0 {
		t.Fatal("no photos")
	}
	for i, p := range s.Photos {
		if p.ID == "" {
			t.Errorf("photo %d has no id", i)
		}
		if p.Image.URI == "" {
			t.Errorf("photo %d has no image URI, so there is nothing to download", i)
		}
	}
}

// TestLiveVideoSection reads the video tab, which is two different queries
// stitched into one record and the most likely of the sections to half break.
func TestLiveVideoSection(t *testing.T) {
	s, err := liveEngine(t).Videos(liveCtx(t), "nasa", 8)
	if err != nil {
		t.Fatalf("read the nasa videos: %v", err)
	}
	if len(s.Videos) == 0 {
		t.Fatal("no videos")
	}
	for i, v := range s.Videos {
		if v.ID == "" || v.URL == "" {
			t.Errorf("video %d has id %q url %q", i, v.ID, v.URL)
		}
	}
}

// TestLiveEvents reads the events tab. Public events come and go, so an empty
// list is a skip and not a failure: what is being checked is that the cards
// parse when there are cards.
func TestLiveEvents(t *testing.T) {
	s, err := liveEngine(t).Events(liveCtx(t), "nasa", 8)
	if err != nil {
		t.Skipf("no events to read: %v", err)
	}
	if len(s.Events) == 0 {
		t.Skip("nasa has no upcoming events today")
	}
	for i, e := range s.Events {
		if e.ID == "" || e.Name == "" {
			t.Errorf("event %d has id %q name %q", i, e.ID, e.Name)
		}
	}
}

// TestLiveDirectory reads the Pages directory, which is the seed source a crawl
// starts from and the one surface with no profile behind it.
func TestLiveDirectory(t *testing.T) {
	d, err := liveEngine(t).Discover(liveCtx(t), "")
	if err != nil {
		t.Fatalf("read the directory index: %v", err)
	}
	if len(d.Index) == 0 {
		t.Fatal("the index names no letters, so there is nowhere to walk to")
	}
	for i, l := range d.Index {
		if l.URL == "" {
			t.Errorf("index row %d has no URL", i)
		}
	}
}

// TestLiveEdges checks the claim plane against a real read: a page that is
// really there should assert something about something else, and every claim
// should name two URIs fb can key on.
func TestLiveEdges(t *testing.T) {
	c, err := liveEngine(t).Edges(liveCtx(t), "nasa")
	if err != nil {
		t.Fatalf("read the nasa edges: %v", err)
	}
	if len(c.Edges) == 0 {
		t.Fatal("nasa asserted nothing about anything")
	}
	for i, e := range c.Edges {
		if e.From == "" || e.Predicate == "" || e.To == "" {
			t.Errorf("edge %d is %q %q %q", i, e.From, e.Predicate, e.To)
		}
		if e.Source == "" {
			t.Errorf("edge %d names no source, so nothing says where the claim came from", i)
		}
	}
}

// TestLiveReactionIdsAreAllKnown is the one test here that watches for a new
// thing rather than a broken one. The reaction table is seven ids written down
// by hand, and the day Facebook adds an eighth every breakdown quietly reports
// it under its raw id.
func TestLiveReactionIdsAreAllKnown(t *testing.T) {
	f, err := liveEngine(t).Feed(liveCtx(t), "nasa", 1)
	if err != nil || len(f.Posts) == 0 {
		t.Skipf("no post to read: %v", err)
	}
	for kind, n := range f.Posts[0].Counts.ByType {
		if numeric(kind) {
			t.Errorf("a reaction came back under the raw id %q with %d on it, so reactionNames needs a new row", kind, n)
		}
	}
}

// TestLiveGroupNeedsASession is the refusal, checked from the other side. A
// public group reads signed out; the discussion behind it does not, and the
// record should say which it was rather than coming back empty.
func TestLiveGroupNeedsASession(t *testing.T) {
	needSession(t)
	g, err := liveEngine(t).Group(liveCtx(t), "9218157864")
	if err != nil {
		t.Fatalf("read the group: %v", err)
	}
	if g.ID == "" || g.Name == "" {
		t.Errorf("group id %q name %q", g.ID, g.Name)
	}
}
