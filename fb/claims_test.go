package fb

import (
	"testing"

	"github.com/tamnd/facebook-cli/pkg/graph"
)

// claims_test.go covers the walk's plumbing, which is the part that spends
// requests. Extraction is tested against fixtures in edges_test.go; here the
// question is which of those claims turn into a fetch and what URL that fetch
// uses, because getting either wrong costs a request per node and the budget is
// the whole point.

func TestAPostOnTheFrontierCarriesItsAuthor(t *testing.T) {
	// Facebook's post permalink needs the author's id as well as the story's,
	// and answers 404 without it. The author is only ever in the authored
	// claim, so the frontier has to read the claims twice: once for authors,
	// once for nodes.
	c := Claims{Edges: []graph.Edge{
		{From: "fb://profile/100044561550831", Predicate: graph.Authored, To: "fb://post/1587860636042640"},
		{From: "fb://post/1587860636042640", Predicate: graph.Attaches, To: "fb://photo/1587860609375976"},
	}}
	got := map[string]string{}
	for _, n := range frontierOf(c, map[string]bool{}) {
		got[n.uri] = n.ref
	}
	want := "https://www.facebook.com/permalink.php?story_fbid=1587860636042640&id=100044561550831"
	if got["fb://post/1587860636042640"] != want {
		t.Errorf("post ref = %q, want %q", got["fb://post/1587860636042640"], want)
	}
	if got["fb://photo/1587860609375976"] != "https://www.facebook.com/photo/?fbid=1587860609375976" {
		t.Errorf("photo ref = %q", got["fb://photo/1587860609375976"])
	}
}

func TestAPostNobodyClaimedIsNotWorthARequest(t *testing.T) {
	// A photo permalink names its containing story without saying who wrote it,
	// which happens on every photo read. Fetching that post spends a request to
	// be told 404, so it comes back with no ref and the walk skips it.
	c := Claims{Edges: []graph.Edge{
		{From: "fb://post/1587860636042640", Predicate: graph.Attaches, To: "fb://photo/1587860609375976"},
	}}
	for _, n := range frontierOf(c, map[string]bool{}) {
		if n.uri == "fb://post/1587860636042640" && n.ref != "" {
			t.Errorf("an authorless post got a ref: %q", n.ref)
		}
	}
}

func TestTheFrontierSkipsWhatHasNoPage(t *testing.T) {
	// A comment has no permalink fb can read, an album ships nothing signed
	// out, and an external URL is somebody else's site. All three are nodes and
	// none of them is a fetch.
	c := Claims{Edges: []graph.Edge{
		{From: "fb://comment/1032857456032245", Predicate: graph.CommentsOn, To: "fb://photo/1587860609375976"},
		{From: "fb://photo/1587860609375976", Predicate: graph.InAlbum, To: "fb://album/416661013162614"},
		{From: "fb://post/1", Predicate: graph.LinksTo, To: graph.External("https://www.nasa.gov/")},
	}}
	for _, n := range frontierOf(c, map[string]bool{}) {
		kind, _, _ := graph.Split(n.uri)
		switch kind {
		case graph.KindComment, graph.KindAlbum, graph.KindExternal:
			t.Errorf("%s is on the frontier and there is nothing to fetch", n.uri)
		}
	}
}

func TestANodeIsOnlyEverQueuedOnce(t *testing.T) {
	// A page names its own owner in half its claims. Queuing that profile eight
	// times is eight requests for one answer.
	c := Claims{Edges: []graph.Edge{
		{From: "fb://profile/100044561550831", Predicate: graph.Owns, To: "fb://photo/1"},
		{From: "fb://profile/100044561550831", Predicate: graph.Owns, To: "fb://photo/2"},
		{From: "fb://profile/100044561550831", Predicate: graph.Covers, To: "fb://photo/1"},
	}}
	seen := map[string]bool{}
	front := frontierOf(c, seen)
	if len(front) != 3 {
		t.Errorf("frontier has %d nodes, want the owner and its two photos: %v", len(front), front)
	}
	// A second read that names the same things adds nothing.
	if again := frontierOf(c, seen); len(again) != 0 {
		t.Errorf("a repeat read queued %d nodes that were already fetched", len(again))
	}
}
