package fb

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// fakeGraph is an in-memory grapher for hermetic walk tests. It mirrors the
// anonymous SSR surface: actors own feeds of posts, posts carry their owner and
// preview comments, and a commenter is a name with no traversable id. Resolve
// reuses the real pure classifier so seeds behave exactly as in production.
//
// fail injects failures keyed by "method:handle": a (*T, error) method returns
// the error directly, an iterator method yields it as the first pair. A seed
// failure (depth 0) is fatal; a deeper failure becomes a note.
type fakeGraph struct {
	pages    map[string]*Page
	profiles map[string]*Profile
	groups   map[string]*Group
	posts    map[string]*Post
	feeds    map[string][]Post
	comments map[string][]Comment
	fail     map[string]error
}

// fakeHandle normalizes any id or URL to the lookup key the corpus uses, the
// same string the real client would dedup on.
func fakeHandle(idOrURL string) string {
	id := fbid.Classify(idOrURL)
	switch id.Kind {
	case fbid.KindProfile:
		return firstNonEmpty(id.ProfileID, id.Slug)
	case fbid.KindGroup:
		return firstNonEmpty(id.GroupID, id.Slug)
	case fbid.KindPost:
		return id.PostID
	default:
		return firstNonEmpty(id.PageID, id.Slug, idOrURL)
	}
}

func (g *fakeGraph) Resolve(_ context.Context, input string) (Identity, error) {
	return fbid.Classify(input), nil
}

func (g *fakeGraph) Page(_ context.Context, idOrURL string) (*Page, error) {
	h := fakeHandle(idOrURL)
	if err := g.fail["Page:"+h]; err != nil {
		return nil, err
	}
	if p, ok := g.pages[h]; ok {
		return p, nil
	}
	return nil, errors.New("no such page: " + h)
}

func (g *fakeGraph) Profile(_ context.Context, idOrURL string) (*Profile, error) {
	h := fakeHandle(idOrURL)
	if err := g.fail["Profile:"+h]; err != nil {
		return nil, err
	}
	if p, ok := g.profiles[h]; ok {
		return p, nil
	}
	return nil, errors.New("no such profile: " + h)
}

func (g *fakeGraph) Group(_ context.Context, idOrURL string) (*Group, error) {
	h := fakeHandle(idOrURL)
	if err := g.fail["Group:"+h]; err != nil {
		return nil, err
	}
	if gr, ok := g.groups[h]; ok {
		return gr, nil
	}
	return nil, errors.New("no such group: " + h)
}

func (g *fakeGraph) Post(_ context.Context, idOrURL string, _ PostOptions) (*Post, error) {
	h := fakeHandle(idOrURL)
	if err := g.fail["Post:"+h]; err != nil {
		return nil, err
	}
	if p, ok := g.posts[h]; ok {
		return p, nil
	}
	return nil, errors.New("no such post: " + h)
}

func (g *fakeGraph) PagePosts(_ context.Context, idOrURL string, _ ListOptions) iter.Seq2[Post, error] {
	return g.feed("PagePosts", idOrURL)
}

func (g *fakeGraph) ProfilePosts(_ context.Context, idOrURL string, _ ListOptions) iter.Seq2[Post, error] {
	return g.feed("ProfilePosts", idOrURL)
}

func (g *fakeGraph) GroupPosts(_ context.Context, idOrURL string, _ ListOptions) iter.Seq2[Post, error] {
	return g.feed("GroupPosts", idOrURL)
}

func (g *fakeGraph) feed(method, idOrURL string) iter.Seq2[Post, error] {
	h := fakeHandle(idOrURL)
	return func(yield func(Post, error) bool) {
		if err := g.fail[method+":"+h]; err != nil {
			yield(Post{}, err)
			return
		}
		for _, p := range g.feeds[h] {
			if !yield(p, nil) {
				return
			}
		}
	}
}

func (g *fakeGraph) Comments(_ context.Context, postURL string, _ CommentOptions) iter.Seq2[Comment, error] {
	h := fakeHandle(postURL)
	return func(yield func(Comment, error) bool) {
		if err := g.fail["Comments:"+h]; err != nil {
			yield(Comment{}, err)
			return
		}
		for _, c := range g.comments[h] {
			if !yield(c, nil) {
				return
			}
		}
	}
}

var _ grapher = (*fakeGraph)(nil)

// newFakeGraph builds a small star: a page (nasa), a group (python), a profile
// (42), each with a feed; a standalone post seed (123); and preview comments
// under posts 100 and 123.
func newFakeGraph() *fakeGraph {
	return &fakeGraph{
		pages: map[string]*Page{
			"nasa": {PageID: "nasa", Slug: "nasa", Name: "NASA", URL: "https://www.facebook.com/nasa"},
		},
		groups: map[string]*Group{
			"python": {GroupID: "python", Slug: "python", Name: "Python", URL: "https://www.facebook.com/groups/python"},
		},
		profiles: map[string]*Profile{
			"42": {ProfileID: "42", Name: "Ada", URL: "https://www.facebook.com/profile.php?id=42"},
		},
		posts: map[string]*Post{
			"123": {PostID: "123", OwnerID: "nasa", OwnerType: "page", OwnerName: "NASA", Text: "seed post", Permalink: "https://www.facebook.com/nasa/posts/123"},
		},
		feeds: map[string][]Post{
			"nasa": {
				{PostID: "100", OwnerID: "nasa", OwnerType: "page", OwnerName: "NASA", Text: "p100", Permalink: "https://www.facebook.com/nasa/posts/100"},
				{PostID: "101", OwnerID: "nasa", OwnerType: "page", OwnerName: "NASA", Text: "p101", Permalink: "https://www.facebook.com/nasa/posts/101"},
			},
			"python": {
				{PostID: "200", OwnerID: "python", OwnerType: "group", OwnerName: "Python", Text: "g200", Permalink: "https://www.facebook.com/groups/python/posts/200"},
			},
			"42": {
				{PostID: "300", OwnerID: "42", OwnerType: "profile", OwnerName: "Ada", Text: "u300", Permalink: "https://www.facebook.com/42/posts/300"},
			},
		},
		comments: map[string][]Comment{
			"100": {
				{CommentID: "1", PostID: "100", AuthorName: "Alice", Text: "first"},
				{CommentID: "2", PostID: "100", AuthorName: "Bob", Text: "second"},
			},
			"123": {
				{CommentID: "1", PostID: "123", AuthorName: "Carol", Text: "nice"},
			},
		},
		fail: map[string]error{},
	}
}

// runWalk runs a walk over g, collecting emitted nodes and any notes.
func runWalk(t *testing.T, g *fakeGraph, seeds []string, opts WalkOptions) ([]*Node, []string, error) {
	t.Helper()
	var nodes []*Node
	var notes []string
	opts.Note = func(s string) { notes = append(notes, s) }
	ss := make([]Seed, 0, len(seeds))
	for _, s := range seeds {
		ss = append(ss, Seed{Raw: s})
	}
	err := NewWalker(g).Walk(context.Background(), ss, opts, func(n *Node) error {
		nodes = append(nodes, n)
		return nil
	})
	return nodes, notes, err
}

func countKind(nodes []*Node, kind NodeKind) int {
	n := 0
	for _, nd := range nodes {
		if nd.Kind == kind {
			n++
		}
	}
	return n
}

func findNode(nodes []*Node, kind NodeKind, endpoint string) *Node {
	for _, n := range nodes {
		if n.Kind == kind && n.Endpoint() == endpoint {
			return n
		}
	}
	return nil
}

func TestParseEdges(t *testing.T) {
	cases := []struct {
		spec string
		want []Edge
	}{
		{"", []Edge{EdgePosts, EdgeAuthor}},                               // default is content
		{"content", []Edge{EdgePosts, EdgeAuthor}},                        // preset
		{"threads", []Edge{EdgePosts, EdgeComments}},                      // preset
		{"all", []Edge{EdgePosts, EdgeAuthor, EdgeComments}},              // preset
		{"posts,comments", []Edge{EdgePosts, EdgeComments}},               // explicit list
		{"content,comments", []Edge{EdgePosts, EdgeAuthor, EdgeComments}}, // preset + edge
		{" author , comments ", []Edge{EdgeAuthor, EdgeComments}},         // whitespace tolerated
	}
	for _, tc := range cases {
		set, err := ParseEdges(tc.spec)
		if err != nil {
			t.Fatalf("ParseEdges(%q) error: %v", tc.spec, err)
		}
		want := newEdgeSet(tc.want...)
		if set.String() != want.String() {
			t.Errorf("ParseEdges(%q) = %q, want %q", tc.spec, set, want)
		}
	}
}

func TestParseEdgesUnknown(t *testing.T) {
	if _, err := ParseEdges("nope"); err == nil {
		t.Fatal("ParseEdges(\"nope\") should error")
	}
	if _, err := ParseEdges("posts,bogus"); err == nil {
		t.Fatal("ParseEdges with a bogus token should error")
	}
}

func TestEdgeFollows(t *testing.T) {
	actors := []NodeKind{NodePage, NodeProfile, NodeGroup}
	for _, k := range actors {
		if !EdgePosts.follows(k) {
			t.Errorf("posts should follow from actor %s", k)
		}
		if EdgeAuthor.follows(k) || EdgeComments.follows(k) {
			t.Errorf("author/comments should not follow from actor %s", k)
		}
	}
	if EdgePosts.follows(NodePost) {
		t.Error("posts should not follow from a post")
	}
	if !EdgeAuthor.follows(NodePost) || !EdgeComments.follows(NodePost) {
		t.Error("author and comments should follow from a post")
	}
	if EdgeComments.follows(NodeComment) {
		t.Error("nothing should follow from a comment")
	}
}

func TestWalkPostsFromPage(t *testing.T) {
	g := newFakeGraph()
	nodes, notes, err := runWalk(t, g, []string{"nasa"}, WalkOptions{Depth: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Errorf("unexpected notes: %v", notes)
	}
	if len(nodes) != 3 {
		t.Fatalf("got %d nodes, want 3 (page + 2 posts)", len(nodes))
	}
	if nodes[0].Kind != NodePage || nodes[0].Depth != 0 {
		t.Errorf("first node = %s d%d, want page d0", nodes[0].Kind, nodes[0].Depth)
	}
	if countKind(nodes, NodePost) != 2 {
		t.Errorf("want 2 post nodes, got %d", countKind(nodes, NodePost))
	}
	for _, n := range nodes[1:] {
		if n.Via != EdgePosts || n.Depth != 1 {
			t.Errorf("post node via %q d%d, want posts d1", n.Via, n.Depth)
		}
	}
}

func TestWalkFromGroup(t *testing.T) {
	g := newFakeGraph()
	nodes, _, err := runWalk(t, g, []string{"https://www.facebook.com/groups/python"}, WalkOptions{Depth: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2 (group + 1 post)", len(nodes))
	}
	if nodes[0].Kind != NodeGroup {
		t.Errorf("first node = %s, want group", nodes[0].Kind)
	}
	if p := findNode(nodes, NodePost, "200"); p == nil {
		t.Error("group post 200 not reached")
	}
}

func TestWalkAuthorFromPostSeed(t *testing.T) {
	// The real author hop: a post seed reaches its owner, then (one hop further)
	// the rest of that owner's feed.
	g := newFakeGraph()
	nodes, _, err := runWalk(t, g, []string{"https://www.facebook.com/nasa/posts/123"}, WalkOptions{Depth: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 4 {
		t.Fatalf("got %d nodes, want 4 (seed post + author page + 2 feed posts)", len(nodes))
	}
	if nodes[0].Kind != NodePost || nodes[0].Endpoint() != "123" {
		t.Errorf("seed = %s/%s, want post/123", nodes[0].Kind, nodes[0].Endpoint())
	}
	author := findNode(nodes, NodePage, "nasa")
	if author == nil {
		t.Fatal("author page nasa not reached")
	}
	if author.Via != EdgeAuthor || author.Depth != 1 {
		t.Errorf("author node via %q d%d, want author d1", author.Via, author.Depth)
	}
	for _, id := range []string{"100", "101"} {
		p := findNode(nodes, NodePost, id)
		if p == nil {
			t.Fatalf("feed post %s not reached via author", id)
		}
		if p.Via != EdgePosts || p.Depth != 2 {
			t.Errorf("feed post %s via %q d%d, want posts d2", id, p.Via, p.Depth)
		}
	}
}

func TestWalkThreadsFromPostSeed(t *testing.T) {
	g := newFakeGraph()
	nodes, _, err := runWalk(t, g, []string{"https://www.facebook.com/nasa/posts/123"},
		WalkOptions{Depth: 1, Edges: edgePresets["threads"].clone()})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2 (post + 1 comment)", len(nodes))
	}
	c := findNode(nodes, NodeComment, "123:1")
	if c == nil {
		t.Fatal("comment 123:1 not reached")
	}
	if c.Via != EdgeComments || c.Depth != 1 {
		t.Errorf("comment via %q d%d, want comments d1", c.Via, c.Depth)
	}
}

func TestWalkThreadsNeedsDepth2FromActor(t *testing.T) {
	// From an actor seed, comments sit two hops down: actor -> post -> comment.
	g := newFakeGraph()
	threads := edgePresets["threads"].clone()

	d1, _, err := runWalk(t, g, []string{"nasa"}, WalkOptions{Depth: 1, Edges: threads})
	if err != nil {
		t.Fatal(err)
	}
	if countKind(d1, NodeComment) != 0 {
		t.Errorf("depth 1 from actor should reach no comments, got %d", countKind(d1, NodeComment))
	}

	d2, _, err := runWalk(t, g, []string{"nasa"}, WalkOptions{Depth: 2, Edges: threads})
	if err != nil {
		t.Fatal(err)
	}
	// page + post100 + post101 + 2 comments under post100.
	if len(d2) != 5 {
		t.Fatalf("depth 2 from actor: got %d nodes, want 5", len(d2))
	}
	if countKind(d2, NodeComment) != 2 {
		t.Errorf("want 2 comments at depth 2, got %d", countKind(d2, NodeComment))
	}
}

func TestWalkAuthorDedupsToContainer(t *testing.T) {
	// Following all edges from an actor, each feed post's author points back at
	// the seed actor; those stubs must dedup, not duplicate the actor node.
	g := newFakeGraph()
	nodes, _, err := runWalk(t, g, []string{"nasa"}, WalkOptions{Depth: 2, Edges: edgePresets["all"].clone()})
	if err != nil {
		t.Fatal(err)
	}
	if countKind(nodes, NodePage) != 1 {
		t.Errorf("want exactly 1 page node (deduped), got %d", countKind(nodes, NodePage))
	}
	// page + post100 + post101 + 2 comments.
	if len(nodes) != 5 {
		t.Fatalf("got %d nodes, want 5", len(nodes))
	}
}

func TestWalkBudgetStops(t *testing.T) {
	g := newFakeGraph()
	nodes, _, err := runWalk(t, g, []string{"nasa"}, WalkOptions{Depth: 2, Max: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("Max=2 should stop at 2 nodes, got %d", len(nodes))
	}
}

func TestWalkFanoutCaps(t *testing.T) {
	g := newFakeGraph()
	nodes, _, err := runWalk(t, g, []string{"nasa"}, WalkOptions{Depth: 1, Fanout: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("Fanout=1 should yield page + 1 post = 2 nodes, got %d", len(nodes))
	}
	if countKind(nodes, NodePost) != 1 {
		t.Errorf("want 1 post under fanout 1, got %d", countKind(nodes, NodePost))
	}
}

func TestWalkDepthZeroSeedsOnly(t *testing.T) {
	g := newFakeGraph()
	nodes, _, err := runWalk(t, g, []string{"nasa", "https://www.facebook.com/groups/python"}, WalkOptions{Depth: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("depth 0 should emit only the 2 seeds, got %d", len(nodes))
	}
	if findNode(nodes, NodePage, "nasa") == nil || findNode(nodes, NodeGroup, "python") == nil {
		t.Error("both seeds should be present")
	}
}

func TestWalkSeedNotFoundFatal(t *testing.T) {
	g := newFakeGraph()
	g.fail["Page:nasa"] = errors.New("boom")
	nodes, _, err := runWalk(t, g, []string{"nasa"}, WalkOptions{Depth: 1})
	if err == nil {
		t.Fatal("a seed that cannot be fetched should fail the walk")
	}
	if len(nodes) != 0 {
		t.Errorf("no nodes should be emitted on a fatal seed, got %d", len(nodes))
	}
}

func TestWalkUnsupportedSeedFatal(t *testing.T) {
	g := newFakeGraph()
	_, _, err := runWalk(t, g, []string{"https://www.facebook.com/photo.php?fbid=99"}, WalkOptions{Depth: 1})
	if err == nil {
		t.Fatal("a photo seed should fail: discover only walks pages, profiles, groups, posts")
	}
}

func TestWalkDeeperErrorDegrades(t *testing.T) {
	// A feed that fails below the seed becomes a note; the walk keeps the seed.
	g := newFakeGraph()
	g.fail["PagePosts:nasa"] = errors.New("rate limited")
	nodes, notes, err := runWalk(t, g, []string{"nasa"}, WalkOptions{Depth: 1})
	if err != nil {
		t.Fatalf("a deeper failure should not be fatal: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Kind != NodePage {
		t.Fatalf("want just the page node, got %d nodes", len(nodes))
	}
	if len(notes) != 1 {
		t.Errorf("want 1 note for the failed feed, got %d: %v", len(notes), notes)
	}
}

func TestWalkDeeperCommentErrorDegrades(t *testing.T) {
	g := newFakeGraph()
	g.fail["Comments:123"] = errors.New("comments unavailable")
	nodes, notes, err := runWalk(t, g, []string{"https://www.facebook.com/nasa/posts/123"},
		WalkOptions{Depth: 1, Edges: edgePresets["threads"].clone()})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Kind != NodePost {
		t.Fatalf("want just the post node, got %d nodes", len(nodes))
	}
	if len(notes) != 1 {
		t.Errorf("want 1 note for the failed comments, got %d", len(notes))
	}
}

func TestWalkMultipleSeeds(t *testing.T) {
	g := newFakeGraph()
	nodes, _, err := runWalk(t, g,
		[]string{"nasa", "https://www.facebook.com/groups/python", "profile.php?id=42"},
		WalkOptions{Depth: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 3 {
		t.Fatalf("got %d nodes, want 3 seeds", len(nodes))
	}
	if findNode(nodes, NodeProfile, "42") == nil {
		t.Error("profile seed 42 not reached")
	}
}

func TestWalkCommentsAreLeaves(t *testing.T) {
	// Even at a generous depth, a comment never expands into more nodes.
	g := newFakeGraph()
	nodes, _, err := runWalk(t, g, []string{"https://www.facebook.com/nasa/posts/123"},
		WalkOptions{Depth: 5, Edges: edgePresets["threads"].clone()})
	if err != nil {
		t.Fatal(err)
	}
	// post 123 -> 1 comment, and the comment is a leaf; nothing further.
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2 (post + leaf comment)", len(nodes))
	}
	if countKind(nodes, NodeComment) != 1 {
		t.Errorf("want 1 comment, got %d", countKind(nodes, NodeComment))
	}
}

func TestWalkProfileFeed(t *testing.T) {
	g := newFakeGraph()
	nodes, _, err := runWalk(t, g, []string{"profile.php?id=42"}, WalkOptions{Depth: 1})
	if err != nil {
		t.Fatal(err)
	}
	if nodes[0].Kind != NodeProfile {
		t.Fatalf("first node = %s, want profile", nodes[0].Kind)
	}
	if findNode(nodes, NodePost, "300") == nil {
		t.Error("profile post 300 not reached via ProfilePosts")
	}
}
