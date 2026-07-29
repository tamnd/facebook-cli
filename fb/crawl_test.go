package fb

import (
	"reflect"
	"testing"

	"github.com/tamnd/facebook-cli/pkg/graph"
)

// crawl_test.go covers the parts of a walk that need no network: what order the
// frontier goes in, and finding the refs a crawl would spend a request on.

// TestTheFrontierSpendsTheBudgetOnWhatSaysTheMost. The order is the whole
// difference between a budget that ran out having learned something and one
// that spent itself on group pages.
func TestTheFrontierSpendsTheBudgetOnWhatSaysTheMost(t *testing.T) {
	in := []node{
		{uri: "fb://group/1"},
		{uri: "fb://video/2"},
		{uri: "fb://post/3"},
		{uri: "fb://profile/4"},
		{uri: "fb://photo/5"},
	}
	want := []string{"fb://post/3", "fb://photo/5", "fb://profile/4", "fb://video/2", "fb://group/1"}
	got := sortFrontier(in)
	for i, w := range want {
		if got[i].uri != w {
			t.Fatalf("position %d is %s, want %s", i, got[i].uri, w)
		}
	}
	if in[0].uri != "fb://group/1" {
		t.Error("sortFrontier reordered the caller's slice, which is the record of what the level was")
	}
}

// TestAProfilePictureGoesLast. Both of a page's cover claims point at photos
// whose permalinks ship nothing signed out, and ranking them as photos spends
// the first two requests of every page crawl on two refusals.
func TestAProfilePictureGoesLast(t *testing.T) {
	in := []node{
		{uri: "fb://photo/1496429661852405", via: graph.Covers},
		{uri: "fb://photo/416661036495945", via: graph.Covers},
		{uri: "fb://post/1587860636042640", via: graph.Authored},
		{uri: "fb://photo/1587860609375976", via: graph.Attaches},
	}
	got := sortFrontier(in)
	if got[0].uri != "fb://post/1587860636042640" || got[1].uri != "fb://photo/1587860609375976" {
		t.Fatalf("the story and its photo are not first: %v", got)
	}
	for _, n := range got[2:] {
		if n.via != graph.Covers {
			t.Errorf("%s is behind a profile picture", n.uri)
		}
	}
}

// TestTiesKeepTheOrderThePageAssertedThem. Two photos are worth the same, and
// the page put one of them first for a reason.
func TestTiesKeepTheOrderThePageAssertedThem(t *testing.T) {
	in := []node{{uri: "fb://photo/1"}, {uri: "fb://photo/2"}, {uri: "fb://photo/3"}}
	got := sortFrontier(in)
	for i, n := range got {
		if n.uri != in[i].uri {
			t.Fatalf("position %d is %s, want %s", i, n.uri, in[i].uri)
		}
	}
}

// TestAPfbidIsFoundWhereverItSitsOnTheRecord is what --resolve-opaque works
// from. A group feed puts the pfbid on every post's author, three levels down
// from the record, and a field list would miss the next place Facebook puts one.
func TestAPfbidIsFoundWhereverItSitsOnTheRecord(t *testing.T) {
	g := Group{
		ID: "1",
		Posts: []Post{
			{ID: "2", Author: Ref{Kind: graph.KindProfile, ID: "pfbid0abc", Opaque: true, URL: "https://www.facebook.com/x"}},
			{ID: "3", Author: Ref{Kind: graph.KindProfile, ID: "1234"}},
		},
	}
	v := reflect.New(reflect.TypeOf(g)).Elem()
	v.Set(reflect.ValueOf(g))
	refs := opaqueRefs(v)
	if len(refs) != 1 {
		t.Fatalf("found %d opaque refs, want 1", len(refs))
	}
	// Settable, or the resolve has nothing to write the numeric id into.
	r := refs[0].Interface().(Ref)
	r.ID, r.Opaque = "999", false
	refs[0].Set(reflect.ValueOf(r))
	patched := v.Interface().(Group)
	if patched.Posts[0].Author.ID != "999" {
		t.Fatalf("author id is %q after patching, want 999", patched.Posts[0].Author.ID)
	}
}

// TestAPatchedRecordAssertsTheClaimItCouldNotBefore is the point of the flag:
// the author claim exists once the pfbid has a number behind it.
func TestAPatchedRecordAssertsTheClaimItCouldNotBefore(t *testing.T) {
	before := Group{ID: "1", Posts: []Post{{
		ID:     "2",
		Author: Ref{Kind: graph.KindProfile, ID: "pfbid0abc", Opaque: true},
	}}}
	after := Group{ID: "1", Posts: []Post{{
		ID:     "2",
		Author: Ref{Kind: graph.KindProfile, ID: "999"},
	}}}
	authored := func(g Group) bool {
		for _, e := range edgesOf(g, nil) {
			if e.Predicate == graph.Authored && e.From == "fb://profile/999" {
				return true
			}
		}
		return false
	}
	if authored(before) {
		t.Fatal("a pfbid author was filed under a name that will not survive the week")
	}
	if !authored(after) {
		t.Fatal("a resolved author still has no authored claim, so --resolve-opaque bought nothing")
	}
}

// TestARecordNobodyTaughtEdgesOfKeepsWhatItCameWith. Losing every claim on a
// record because this switch has not heard of its type would be a silent loss.
func TestARecordNobodyTaughtEdgesOfKeepsWhatItCameWith(t *testing.T) {
	had := []graph.Edge{{From: "fb://profile/1", Predicate: graph.Mentions, To: "fb://profile/2"}}
	got := edgesOf(Directory{}, had)
	if len(got) != 1 {
		t.Fatalf("kept %d claims, want the 1 it came with", len(got))
	}
}

// TestWritingAReadFilesTheNodesNobodyFetched. The frontier of the next crawl is
// made of these, and a store holding only what was fetched cannot say what it
// has not looked at.
func TestWritingAReadFilesTheNodesNobodyFetched(t *testing.T) {
	st := openTestStore(t)
	c := Claims{
		URI:  "fb://profile/1",
		Kind: "page",
		Edges: []graph.Edge{
			{From: "fb://profile/1", Predicate: graph.Authored, To: "fb://post/2", Source: "u"},
			{From: "fb://post/2", Predicate: graph.Mentions, To: "fb://profile/3", Source: "u"},
			{From: "fb://post/2", Predicate: graph.LinksTo, To: "not a uri", Source: "u"},
		},
	}
	if err := writeClaims(st, c); err != nil {
		t.Fatalf("write: %v", err)
	}
	nodes, claims, _, err := st.Counts()
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if nodes != 3 {
		t.Errorf("filed %d nodes, want 3: the read itself, the post and the profile it named", nodes)
	}
	if claims != 3 {
		t.Errorf("filed %d claims, want 3", claims)
	}
}
