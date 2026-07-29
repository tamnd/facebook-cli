package fb

import (
	"context"
	"strings"
	"testing"
)

// The reads themselves are covered by the parser tests against captured pages
// and by the live suite. What is tested here is the part of the engine that
// decides things: which URL a reference turns into, what --tier allows, and
// whether an operation can be paged at all.

func TestTabURL(t *testing.T) {
	for _, tt := range []struct {
		ref, tab, want string
	}{
		{"nasa", "photos", "https://www.facebook.com/nasa/photos"},
		{"@nasa", "videos", "https://www.facebook.com/nasa/videos"},
		{"nasa", "about", "https://www.facebook.com/nasa/about"},
		{"100064346136437", "photos", "https://www.facebook.com/profile.php?id=100064346136437&sk=photos"},
	} {
		if got := tabURL(tt.ref, tt.tab); got != tt.want {
			t.Errorf("tabURL(%q, %q) = %q, want %q", tt.ref, tt.tab, got, tt.want)
		}
	}
}

func TestParseTier(t *testing.T) {
	for _, tt := range []struct {
		in      string
		cookies bool
		cap     int
		pin     string
		wantErr bool
	}{
		{"", false, 0, "", false},
		{"", true, 1, "", false},
		{"auto", true, 1, "", false},
		{"0", true, 0, "", false},  // cap below what the cookies allow
		{"1", false, 0, "", false}, // asking for more than there is, capped
		{"1", true, 1, "", false},  //
		{"og", false, 0, surfaceOpenGraph, false},
		{"COMET", false, 0, surfaceComet, false},
		{"session", false, 0, "", true}, // no cookies to pin to
		{"session", true, 1, surfaceSession, false},
		{"2", true, 0, "", true},
		{"-1", true, 0, "", true},
		{"html", false, 0, "", true},
	} {
		capTier, pin, err := parseTier(tt.in, tt.cookies)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseTier(%q, %v) error = %v, want error %v", tt.in, tt.cookies, err, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if capTier != tt.cap || pin != tt.pin {
			t.Errorf("parseTier(%q, %v) = %d, %q, want %d, %q", tt.in, tt.cookies, capTier, pin, tt.cap, tt.pin)
		}
	}
}

func TestAllowFollowsTier(t *testing.T) {
	tier0 := &Engine{tierCap: 0}
	if !tier0.allow(surfaceComet) {
		t.Error("tier 0 should allow the Comet page")
	}
	if tier0.allow(surfaceSession) {
		t.Error("tier 0 should not allow the session surface")
	}

	tier1 := &Engine{tierCap: 1}
	if !tier1.allow(surfaceSession) {
		t.Error("tier 1 should allow the session surface")
	}

	pinned := &Engine{tierCap: 0, pin: surfaceOpenGraph}
	if pinned.allow(surfaceComet) || pinned.allow(surfaceGraphQL) {
		t.Error("a pin should shut out every surface but the pinned one")
	}
	if !pinned.allow(surfaceOpenGraph) {
		t.Error("a pin should allow the surface it names")
	}
	if (&Engine{tierCap: 1}).allow("s99") {
		t.Error("a surface fb does not have should never be allowed")
	}
}

func TestAdvanceRefusesWhatItCannotPage(t *testing.T) {
	e := &Engine{tierCap: 0}

	// An operation whose variables carry no cursor has no next page, and saying
	// so is the difference between a short list and a wrong one.
	_, err := e.advance(context.Background(), &Page{}, Preload{Op: "SomeQuery", Variables: map[string]any{"id": "1"}}, "abc", 5)
	if err == nil || !strings.Contains(err.Error(), "no cursor") {
		t.Errorf("advance without a cursor variable: %v", err)
	}

	// A pinned surface that is not GraphQL cannot page either.
	pinned := &Engine{tierCap: 0, pin: surfaceOpenGraph}
	_, err = pinned.advance(context.Background(), &Page{}, Preload{Op: "SomeQuery", Variables: map[string]any{"cursor": ""}}, "abc", 5)
	if err == nil || !strings.Contains(err.Error(), "--tier") {
		t.Errorf("advance under a pin: %v", err)
	}
}

func TestLimitFallsBack(t *testing.T) {
	e := &Engine{}
	if got := e.limit(0, 8); got != 8 {
		t.Errorf("limit with nothing set = %d, want 8", got)
	}
	e.cfg.Limit = 3
	if got := e.limit(0, 8); got != 3 {
		t.Errorf("--limit should beat the fallback, got %d", got)
	}
	if got := e.limit(50, 8); got != 50 {
		t.Errorf("the per-call limit should beat --limit, got %d", got)
	}
}

func TestTrimSection(t *testing.T) {
	s := Section{Kind: "photos"}
	for i := range 8 {
		s.Photos = append(s.Photos, Photo{ID: string(rune('a' + i))})
	}
	trimSection(&s, 3)
	if len(s.Photos) != 3 {
		t.Fatalf("trimmed to %d photos, want 3", len(s.Photos))
	}
	if !s.More {
		t.Error("a trimmed section has more to give and should say so")
	}
}

func TestMergePrefersTheFirstPage(t *testing.T) {
	first := &Page{Docs: map[string]*Document{"A": {Op: "A"}}}
	second := &Page{Docs: map[string]*Document{"A": {Op: "A2"}, "B": {Op: "B"}}}
	docs := map[string]*Document{}
	merge(docs, first)
	merge(docs, second)
	if docs["A"].Op != "A" {
		t.Error("the page fetched first should win, so the preferred route stays preferred")
	}
	if docs["B"] == nil {
		t.Error("the second page should still add what the first did not have")
	}
}
