package graph

import "sort"

// Edge is a claim: subject, predicate, object, and who said so.
//
// The source is part of the identity of the claim rather than metadata on it.
// Two surfaces asserting the same edge stay two rows, so when they disagree the
// disagreement is queryable instead of being resolved silently by whichever ran
// last. That rule came from x-cli and it earned its keep there.
type Edge struct {
	From      string `json:"from"`
	Predicate string `json:"predicate"`
	To        string `json:"to"`
	Source    string `json:"source"`  // the URL that asserted it
	Surface   string `json:"surface"` // s1..s8
	Tier      int    `json:"tier"`
	// Note carries what the claim knew that the two URIs do not, like the name
	// on a profile nobody fetched. It is not part of the key.
	Note string `json:"note,omitempty"`
}

// Key is the claim's identity: everything but the note.
func (e Edge) Key() [4]string { return [4]string{e.From, e.Predicate, e.To, e.Source} }

// The predicates, from spec 3004 doc 04 section 3.1.
const (
	Authored    = "authored" // profile -> post
	Mentions    = "mentions" // post -> profile
	LinksTo     = "links_to" // post -> external
	Attaches    = "attaches" // post -> photo, video
	InAlbum     = "in_album" // photo -> album
	NextInAlbum = "next_in_album"
	CommentsOn  = "comments_on"  // comment -> post
	Commented   = "commented"    // profile -> comment
	PostedIn    = "posted_in"    // post -> group
	Hosts       = "hosts"        // profile -> event
	AnnouncedBy = "announced_by" // event -> post
	LocatedAt   = "located_at"   // event -> place
	InCity      = "in_city"      // place -> place
	Suggests    = "suggests"     // event -> event
	DelegatesTo = "delegates_to" // profile -> page
	Shares      = "shares"       // post -> post
	Owns        = "owns"         // profile -> photo, video
	TaggedIn    = "tagged_in"    // profile -> photo
	Covers      = "covers"       // profile, group, event -> photo
)

// Predicates is every predicate fb asserts, for `fb routes` and for the RDF
// mapping to check itself against.
var Predicates = []string{
	Authored, Mentions, LinksTo, Attaches, InAlbum, NextInAlbum,
	CommentsOn, Commented, PostedIn, Hosts, AnnouncedBy, LocatedAt,
	InCity, Suggests, DelegatesTo, Shares, Owns, TaggedIn, Covers,
}

// Set collects claims without duplicating them.
//
// One page asserts the same thing more than once as a matter of course: a post's
// author is in the story header, in every comment's parent, and in the feedback
// node, and a reader who sees `authored` three times learns nothing the first
// one did not tell them. Duplicates within a single source are noise; the same
// claim from two sources is the signal, and Key keeps those apart.
type Set struct {
	seen  map[[4]string]bool
	edges []Edge
}

// Add records a claim, ignoring it when either end has no URI. An edge to
// nothing is not a smaller edge, it is not an edge.
func (s *Set) Add(e Edge) {
	if e.From == "" || e.To == "" || e.Predicate == "" {
		return
	}
	if s.seen == nil {
		s.seen = map[[4]string]bool{}
	}
	if s.seen[e.Key()] {
		return
	}
	s.seen[e.Key()] = true
	s.edges = append(s.edges, e)
}

// Edges returns the claims in the order they were asserted, which is the order
// they appear on the page and the most useful order to read them in.
func (s *Set) Edges() []Edge { return s.edges }

// Len is how many claims one read produced.
func (s *Set) Len() int { return len(s.edges) }

// Nodes is every URI either end of a claim named, sorted. It is what the
// crawler's frontier is built from.
func (s *Set) Nodes() []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range s.edges {
		for _, u := range [2]string{e.From, e.To} {
			if u != "" && !seen[u] {
				seen[u] = true
				out = append(out, u)
			}
		}
	}
	sort.Strings(out)
	return out
}
