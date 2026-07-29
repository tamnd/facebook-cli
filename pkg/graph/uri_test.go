package graph

import "testing"

// uri_test.go pins the three identity rules. Each one exists because breaking it
// produces a store that looks fine and is wrong, which is the worst kind of
// wrong to find later.

func TestAHandleIsNeverAnIdentity(t *testing.T) {
	for _, id := range []string{"NASA", "nasa", "zuck", "profile.php"} {
		if got := URI(KindProfile, id); got != "" {
			t.Errorf("URI(profile, %q) = %q, and a Page can change its handle tomorrow", id, got)
		}
	}
	if got := URI(KindProfile, "100044561550831"); got != "fb://profile/100044561550831" {
		t.Errorf("URI(profile, 100044561550831) = %q", got)
	}
}

func TestAPfbidIsNeverANode(t *testing.T) {
	// A pfbid is signed and per-render, so Monday's and Tuesday's are two names
	// for one person and a store keyed on them grows a duplicate a day.
	for _, id := range []string{
		"pfbid031Q5ygEzfmJxMGXSQDjwbN6hZUc9vuXVuEDWWHPpu6HZk3NQRzMD5iwKP613DaAP6l",
		"pfbid02nHshyUHVP3E31ZwC7J8s5cqncL8PRg88aXG6RU5JDkqXGQywc9Ei85fWBq458aSMl",
	} {
		if got := URI(KindProfile, id); got != "" {
			t.Errorf("URI(profile, pfbid...) = %q", got)
		}
		if got := URI(KindPost, id); got != "" {
			t.Errorf("URI(post, pfbid...) = %q", got)
		}
	}
}

func TestOneLinkIsOneNodeHoweverItWasShared(t *testing.T) {
	// Every spelling here is the same destination, and three nodes for one link
	// is a graph that cannot answer "who else linked this".
	same := []string{
		"https://go.nasa.gov/3RWg5e8",
		"http://go.nasa.gov/3RWg5e8",
		"https://go.nasa.gov/3RWg5e8#top",
		"https://GO.NASA.GOV/3RWg5e8",
		"https://go.nasa.gov/3RWg5e8?utm_source=facebook&fbclid=IwAR123",
	}
	want := External(same[0])
	if want == "" {
		t.Fatal("External produced nothing for a plain URL")
	}
	for _, u := range same[1:] {
		if got := External(u); got != want {
			t.Errorf("External(%s) = %s, want the same node as %s", u, got, same[0])
		}
	}
	// The query is kept when it is not tracking, because on plenty of sites the
	// query is the page.
	if External("https://example.com/x?id=1") == External("https://example.com/x?id=2") {
		t.Error("two pages that differ only in a meaningful query parameter became one node")
	}
}

func TestSplitIsTheInverse(t *testing.T) {
	kind, id, ok := Split("fb://post/1587860636042640")
	if !ok || kind != KindPost || id != "1587860636042640" {
		t.Errorf("Split gave %q %q %v", kind, id, ok)
	}
	for _, bad := range []string{"", "1587860636042640", "https://facebook.com/NASA", "fb://post", "fb://"} {
		if _, _, ok := Split(bad); ok {
			t.Errorf("Split(%q) said yes", bad)
		}
	}
}

func TestAnUnknownKindGetsNoURI(t *testing.T) {
	// A __typename fb has not met keeps its raw string on the ref, and a raw
	// string is not a namespace anybody agreed on.
	if got := URI("MarketplaceListing", "123456789"); got != "" {
		t.Errorf("URI on an unmapped kind = %q", got)
	}
}

func TestASetKeepsTwoSourcesApartAndOneSourceTogether(t *testing.T) {
	var s Set
	base := Edge{From: "fb://profile/4", Predicate: Authored, To: "fb://post/1", Source: "https://www.facebook.com/zuck"}
	s.Add(base)
	s.Add(base) // the same page says it three times, which teaches nobody anything
	s.Add(base)
	if s.Len() != 1 {
		t.Errorf("one source asserting one claim three times gave %d rows", s.Len())
	}
	other := base
	other.Source = "https://www.facebook.com/NASA/posts/1"
	s.Add(other)
	if s.Len() != 2 {
		t.Errorf("two sources asserting the same claim gave %d rows, and a disagreement has to stay queryable", s.Len())
	}
	// An edge with nothing at one end is not a smaller edge.
	s.Add(Edge{From: "fb://profile/4", Predicate: Mentions, To: ""})
	if s.Len() != 2 {
		t.Error("an edge to nothing was recorded")
	}
}
