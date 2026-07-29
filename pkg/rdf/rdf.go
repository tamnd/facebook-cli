// Package rdf writes claims in a vocabulary something else can read.
//
// The mapping starts at schema.org rather than at an invented one. Facebook
// publishes OpenGraph on its own pages and OpenGraph is RDFa, so there is a
// vendor-blessed vocabulary here already; where og: runs out, schema.org
// carries on, and schema.org is what x-cli exports into, so a store built from
// both tools joins instead of sitting in two piles.
//
// Predicates with no schema.org equivalent go in the fb: namespace and the
// namespace is declared in the output rather than assumed. A consumer that has
// never heard of fb:inAlbum should be able to work out where to look without
// asking anybody.
//
// The package knows nothing about HTTP or about Facebook's page shapes. It
// takes claims and writes bytes.
package rdf

import (
	"sort"
	"strings"

	"github.com/tamnd/facebook-cli/pkg/graph"
)

// The namespaces every output declares.
const (
	NSSchema = "https://schema.org/"
	NSFB     = "https://tamnd.github.io/facebook-cli/ns#"
	NSProv   = "http://www.w3.org/ns/prov#"
	NSRDF    = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
)

// types maps an fb kind onto its class.
//
// A Page and a person share one __typename on Facebook and the difference is a
// field, so the caller passes the kind it decided on and this is where that
// decision turns into schema:Organization or schema:Person.
var types = map[string]string{
	graph.KindProfile: "Person",
	graph.KindPage:    "Organization",
	graph.KindPost:    "SocialMediaPosting",
	graph.KindComment: "Comment",
	graph.KindPhoto:   "ImageObject",
	graph.KindVideo:   "VideoObject",
	graph.KindEvent:   "Event",
	graph.KindGroup:   "Organization",
	graph.KindPlace:   "Place",
	graph.KindAlbum:   "ImageGallery",
}

// TypeOf is the class for a kind, and whether there is one at all. An external
// URL is a node with no class: it is somebody else's page and guessing what
// kind of page it is would be inventing a fact.
func TypeOf(kind string) (string, bool) {
	t, ok := types[kind]
	return t, ok
}

// term is how one fb predicate is written in RDF.
type term struct {
	// name is the local name, in schema.org unless fb is set.
	name string
	// fb says the term lives in the fb: namespace because schema.org has no
	// equivalent and borrowing a near-miss would be worse than declaring one.
	fb bool
	// invert says the triple runs the other way. fb writes `profile authored
	// post` because that is how a page reads; schema writes `post author
	// profile` because that is how schema.org defines it, and one of the two
	// has to turn round.
	invert bool
}

var terms = map[string]term{
	graph.Authored:    {name: "author", invert: true},
	graph.Mentions:    {name: "mentions"},
	graph.LinksTo:     {name: "citation"},
	graph.Attaches:    {name: "associatedMedia"},
	graph.CommentsOn:  {name: "parentItem"},
	graph.Commented:   {name: "author", invert: true},
	graph.Hosts:       {name: "organizer", invert: true},
	graph.LocatedAt:   {name: "location"},
	graph.InCity:      {name: "containedInPlace"},
	graph.Covers:      {name: "image"},
	graph.Owns:        {name: "creator", invert: true},
	graph.TaggedIn:    {name: "image", invert: true},
	graph.Shares:      {name: "sharedContent"},
	graph.PostedIn:    {name: "isPartOf"},
	graph.DelegatesTo: {name: "delegatesTo", fb: true},
	graph.InAlbum:     {name: "inAlbum", fb: true},
	graph.NextInAlbum: {name: "nextInAlbum", fb: true},
	graph.AnnouncedBy: {name: "announcedBy", fb: true},
	graph.Suggests:    {name: "suggests", fb: true},
}

// Triple is a subject, predicate and object, already in IRI form.
//
// Derived is the URL that asserted the claim, and it is empty only when the
// caller asked for the smaller file. It is not folded into the triple because
// provenance about a triple is a statement about a statement, and each writer
// has its own way of saying that.
type Triple struct {
	Subject   string
	Predicate string
	Object    string
	// ObjectIsLiteral marks an object that is a string rather than a node.
	ObjectIsLiteral bool
	Derived         string
}

// Options are what a caller gets to decide.
type Options struct {
	// NoProvenance drops the wasDerivedFrom statements. They roughly double the
	// size of a dump, and somebody who trusts their own store and wants the
	// smaller file is not doing anything wrong.
	NoProvenance bool
	// Types emits an rdf:type for every node whose kind is known from its URI.
	// On by default at the caller's discretion, because a dump of bare
	// relationships is much less use than one that says what the things are.
	Types bool
}

// Triples turns claims into triples, in the order the claims were asserted.
//
// Ordering matters more than it looks. A dump that reorders itself between two
// runs of the same read cannot be diffed, and a diff is how somebody notices
// that a page started saying something different.
func Triples(edges []graph.Edge, opt Options) []Triple {
	var out []Triple
	typed := map[string]bool{}
	for _, e := range edges {
		t, ok := terms[e.Predicate]
		if !ok {
			// An unmapped predicate is written in the fb namespace under its own
			// name rather than dropped. A claim fb knows how to make and this
			// package has not been taught to translate is still a claim, and
			// losing it here would be losing it silently.
			t = term{name: camel(e.Predicate), fb: true}
		}
		s, o := e.From, e.To
		if t.invert {
			s, o = o, s
		}
		out = append(out, Triple{Subject: s, Predicate: iri(t), Object: o, Derived: derived(e, opt)})
		if !opt.Types {
			continue
		}
		for _, n := range [2]string{e.From, e.To} {
			if typed[n] {
				continue
			}
			typed[n] = true
			kind, _, ok := graph.Split(n)
			if !ok {
				continue
			}
			// A type triple carries no provenance, and that is deliberate.
			// Facebook did not say the node is a schema:Person; fb worked
			// that out from the kind on the URI. Attributing it to the page
			// would be putting words in the page's mouth.
			if class, ok := TypeOf(kind); ok {
				out = append(out, Triple{Subject: n, Predicate: NSRDF + "type", Object: NSSchema + class})
			}
		}
	}
	return out
}

func derived(e graph.Edge, opt Options) string {
	if opt.NoProvenance {
		return ""
	}
	return e.Source
}

func iri(t term) string {
	if t.fb {
		return NSFB + t.name
	}
	return NSSchema + t.name
}

// camel turns next_in_album into nextInAlbum, so an unmapped predicate lands in
// the fb namespace looking like the ones that were mapped by hand.
func camel(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// Prefixes is what a Turtle or JSON-LD writer declares, sorted so two runs
// produce the same header.
func Prefixes(ts []Triple) map[string]string {
	used := map[string]string{}
	for _, t := range ts {
		for _, iri := range []string{t.Predicate, t.Object} {
			switch {
			case strings.HasPrefix(iri, NSSchema):
				used["schema"] = NSSchema
			case strings.HasPrefix(iri, NSFB):
				used["fb"] = NSFB
			case strings.HasPrefix(iri, NSRDF):
				used["rdf"] = NSRDF
			}
		}
		if t.Derived != "" {
			used["prov"] = NSProv
		}
	}
	return used
}

// prefixNames is Prefixes' keys in a fixed order.
func prefixNames(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
