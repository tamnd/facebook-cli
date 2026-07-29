package fb

import (
	"strings"
	"testing"

	"github.com/tamnd/facebook-cli/pkg/graph"
)

// edges_test.go checks the claim extraction against the captured pages, because
// the whole argument for the graph plane is a number: how much one request
// already knows. A regression here is not a broken test, it is the tool getting
// less useful per request.

// claims indexes edges by predicate for the assertions below.
func claims(edges []graph.Edge) map[string][]graph.Edge {
	out := map[string][]graph.Edge{}
	for _, e := range edges {
		out[e.Predicate] = append(out[e.Predicate], e)
	}
	return out
}

// claimsPerRead is what each captured page is worth, measured on 2026-07-29.
//
// These are floors and not equalities, so a parser that starts finding more does
// not fail the suite. A parser that starts finding less does, and that is the
// regression worth catching: the argument for the whole graph plane is how much
// one request already knows, and a quiet drop from fifteen claims to four is the
// tool getting less useful without getting more broken.
var claimsPerRead = map[string]int{
	"photo": 12, "post": 15, "profile": 7, "event": 6,
	"reel": 4, "videos tab": 21, "photos tab": 8, "events tab": 8,
}

func atLeast(t *testing.T, what string, got int) {
	t.Helper()
	if want := claimsPerRead[what]; got < want {
		t.Errorf("one %s read produced %d claims and used to produce %d", what, got, want)
	}
}

func TestAPhotoPermalinkIsTheBestValueRead(t *testing.T) {
	p := parsePhoto(fixtureDocs(t, "photo"))
	edges := PhotoEdges(p)
	atLeast(t, "photo", len(edges))
	by := claims(edges)
	for _, want := range []string{graph.Owns, graph.InAlbum, graph.CommentsOn, graph.Commented} {
		if len(by[want]) == 0 {
			t.Errorf("no %s claim off the photo permalink", want)
		}
	}
	// Every claim carries where it came from, or the store cannot tell two
	// surfaces disagreeing from one surface saying it twice.
	for _, e := range edges {
		if e.From == "" || e.To == "" {
			t.Errorf("claim with a missing end: %+v", e)
		}
		if !strings.HasPrefix(e.From, "fb://") || !strings.HasPrefix(e.To, "fb://") {
			t.Errorf("claim that is not between two URIs: %+v", e)
		}
	}
}

func TestAPostKnowsWhoWroteItAndWhoItNamed(t *testing.T) {
	post := parsePost(fixtureDocs(t, "post_perm"))
	edges := PostEdges(post)
	atLeast(t, "post", len(edges))
	by := claims(edges)
	if len(by[graph.Authored]) != 1 {
		t.Errorf("a post has exactly one author, got %d: %v", len(by[graph.Authored]), by[graph.Authored])
	}
	if len(by[graph.Commented]) == 0 {
		t.Error("the permalink ships comments and each one has an author, so there should be commented claims")
	}
	// The author claim points the right way: profile authored post, not the
	// other way about.
	a := by[graph.Authored][0]
	if !strings.HasPrefix(a.From, "fb://profile/") && !strings.HasPrefix(a.From, "fb://page/") {
		t.Errorf("authored runs from a profile, got %s", a.From)
	}
	if !strings.HasPrefix(a.To, "fb://post/") {
		t.Errorf("authored runs to a post, got %s", a.To)
	}
}

func TestAProfileReadKnowsItsDelegatePage(t *testing.T) {
	p := parseProfile(fixtureDocs(t, "page_nasa"))
	by := claims(ProfileEdges(p))
	if len(by[graph.DelegatesTo]) == 0 {
		t.Fatal("NASA has a delegate page and the claim is missing")
	}
	d := by[graph.DelegatesTo][0]
	if d.From == d.To {
		t.Error("a profile and its delegate page are two nodes, not one")
	}
}

func TestAnEventKnowsWhoHostsItAndWhereItIs(t *testing.T) {
	e := parseEvent(fixtureDocs(t, "event"), parseHead(fixture(t, "event")))
	by := claims(EventEdges(e))
	if len(by[graph.Hosts]) == 0 {
		t.Error("an event permalink names its host")
	}
	if e.Place != nil && e.Place.ID != "" && len(by[graph.LocatedAt]) == 0 {
		t.Error("the event has a place with an id and no located_at claim came out")
	}
}

// TestAGroupFeedFilesItsPostsInTheGroup also records why a group read is the
// worst value in the tool: two claims off a whole page.
//
// It is not a parser problem. A group post is written by a person rather than
// by a Page, and Facebook gives a person's profile in a group feed as a pfbid,
// which is per-render and never a node. So the post is filed in the group and
// nobody is filed as having written it, and `fb crawl --resolve-opaque` is the
// only thing that closes the gap, at one request per author.
func TestAGroupFeedFilesItsPostsInTheGroup(t *testing.T) {
	g := parseGroup(fixtureDocs(t, "group_big"))
	if len(g.Posts) == 0 {
		t.Skip("the captured group page ships no posts")
	}
	by := claims(GroupEdges(g))
	if len(by[graph.PostedIn]) == 0 {
		t.Error("a post read off a group feed was posted in that group and should say so")
	}
	for _, p := range g.Posts {
		if !p.Author.Opaque {
			return // a group whose posters are not all pfbids: authored should be there
		}
	}
	if len(by[graph.Authored]) != 0 {
		t.Error("every author in this capture is a pfbid, so no authored claim can be made")
	}
}

// TestATabIsWorthMoreThanItLooks checks the section reads, which produce the
// most claims per request of anything in the tool: a videos tab is one request
// and twenty-one claims, because a grid of twenty videos is twenty owns claims
// and nobody had to fetch a single one of them.
func TestATabIsWorthMoreThanItLooks(t *testing.T) {
	owner := Ref{Kind: graph.KindProfile, ID: "100044561550831", Name: "NASA"}
	for _, c := range []struct{ fixture, what string }{
		{"page_videos", "videos tab"},
		{"page_photos", "photos tab"},
		{"page_events", "events tab"},
	} {
		edges := SectionEdges(parseSection(fixtureDocs(t, c.fixture), owner))
		atLeast(t, c.what, len(edges))
		for _, e := range edges {
			if e.From != "fb://profile/100044561550831" && e.Predicate != graph.InAlbum && e.Predicate != graph.LocatedAt {
				t.Errorf("a tab's claims run from its owner, got %+v", e)
			}
		}
	}
}

// TestNothingOpaqueBecomesANode is the rule that keeps the store from growing a
// second node for everybody every time Facebook re-renders a page.
func TestNothingOpaqueBecomesANode(t *testing.T) {
	fixtures := []struct {
		name  string
		edges func(t *testing.T, n string) []graph.Edge
	}{
		{"photo", func(t *testing.T, n string) []graph.Edge { return PhotoEdges(parsePhoto(fixtureDocs(t, n))) }},
		{"post_perm", func(t *testing.T, n string) []graph.Edge { return PostEdges(parsePost(fixtureDocs(t, n))) }},
		{"page_nasa", func(t *testing.T, n string) []graph.Edge { return ProfileEdges(parseProfile(fixtureDocs(t, n))) }},
		{"group_big", func(t *testing.T, n string) []graph.Edge { return GroupEdges(parseGroup(fixtureDocs(t, n))) }},
	}
	for _, f := range fixtures {
		for _, e := range f.edges(t, f.name) {
			for _, u := range []string{e.From, e.To} {
				if strings.Contains(u, "pfbid") {
					t.Errorf("%s produced a URI with a pfbid in it, which is a different node on every render: %s", f.name, u)
				}
			}
		}
	}
}
