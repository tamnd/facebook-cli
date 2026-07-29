package rdf

import (
	"strings"
	"testing"

	"github.com/tamnd/facebook-cli/pkg/graph"
)

// sample is one claim of each interesting shape: a mapped predicate that
// inverts, a mapped one that does not, and one that only exists in the fb
// namespace.
var sample = []graph.Edge{
	{From: "fb://profile/100044561550831", Predicate: graph.Authored, To: "fb://post/1587860636042640", Source: "https://www.facebook.com/NASA"},
	{From: "fb://post/1587860636042640", Predicate: graph.Attaches, To: "fb://photo/1587860609375976", Source: "https://www.facebook.com/NASA"},
	{From: "fb://photo/1587860609375976", Predicate: graph.InAlbum, To: "fb://album/416661013162614", Source: "https://www.facebook.com/NASA"},
}

func TestAuthoredTurnsRoundBecauseSchemaDefinesItTheOtherWay(t *testing.T) {
	// fb writes `profile authored post` because that is how a page reads.
	// schema:author runs from the work to its author. One of the two has to
	// turn round, and getting it backwards produces a file that loads fine and
	// says NASA was written by a photograph.
	ts := Triples(sample[:1], Options{})
	if len(ts) != 1 {
		t.Fatalf("one claim gave %d triples", len(ts))
	}
	if ts[0].Subject != "fb://post/1587860636042640" {
		t.Errorf("schema:author runs from the post, got subject %s", ts[0].Subject)
	}
	if ts[0].Object != "fb://profile/100044561550831" {
		t.Errorf("schema:author points at the author, got object %s", ts[0].Object)
	}
	if ts[0].Predicate != NSSchema+"author" {
		t.Errorf("predicate = %s", ts[0].Predicate)
	}
}

func TestAPredicateSchemaDoesNotHaveGoesInOurOwnNamespace(t *testing.T) {
	// Borrowing a near-miss from schema.org would be worse than declaring a
	// term: somebody joining two stores would get a wrong answer rather than no
	// answer.
	ts := Triples(sample[2:], Options{})
	if ts[0].Predicate != NSFB+"inAlbum" {
		t.Errorf("in_album = %s, want the fb namespace", ts[0].Predicate)
	}
	if !strings.Contains(NSFB, "://") {
		t.Error("the fb namespace has to be dereferenceable enough to look up")
	}
}

func TestAnUnmappedPredicateIsWrittenDownRatherThanDropped(t *testing.T) {
	// A claim fb knows how to make and this package has not been taught to
	// translate is still a claim. Dropping it here would drop it silently,
	// which is the failure mode where the file looks complete.
	ts := Triples([]graph.Edge{{From: "fb://post/1", Predicate: "some_new_thing", To: "fb://post/2"}}, Options{})
	if len(ts) != 1 {
		t.Fatalf("an unmapped predicate produced %d triples", len(ts))
	}
	if ts[0].Predicate != NSFB+"someNewThing" {
		t.Errorf("predicate = %s", ts[0].Predicate)
	}
}

func TestEveryPredicateHasATerm(t *testing.T) {
	// The fallback above exists so nothing is lost, not so the mapping can go
	// unfinished. Anything fb asserts should have been thought about here.
	for _, p := range graph.Predicates {
		if _, ok := terms[p]; !ok {
			t.Errorf("%s has no RDF term and fell through to the fallback", p)
		}
	}
}

func TestATypeIsEmittedOncePerNode(t *testing.T) {
	ts := Triples(sample, Options{Types: true})
	types := map[string]int{}
	for _, tr := range ts {
		if tr.Predicate == NSRDF+"type" {
			types[tr.Subject]++
		}
	}
	if types["fb://photo/1587860609375976"] != 1 {
		t.Errorf("the photo got %d type triples across two claims that both name it", types["fb://photo/1587860609375976"])
	}
	// An external URL is somebody else's page and guessing what kind of page it
	// is would be inventing a fact.
	if _, ok := TypeOf(graph.KindExternal); ok {
		t.Error("an external link was given a class")
	}
}

func TestProvenanceSurvivesEveryFormat(t *testing.T) {
	ts := Triples(sample, Options{Types: true})
	for _, f := range Formats {
		var b strings.Builder
		if err := Write(&b, ts, f); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if !strings.Contains(b.String(), "https://www.facebook.com/NASA") {
			t.Errorf("%s dropped the source URL, so the dump cannot be audited", f)
		}
		if !strings.Contains(b.String(), "wasDerivedFrom") {
			t.Errorf("%s wrote no provenance", f)
		}
	}
}

func TestNoProvenanceMeansNone(t *testing.T) {
	ts := Triples(sample, Options{NoProvenance: true})
	for _, tr := range ts {
		if tr.Derived != "" {
			t.Errorf("--no-provenance left a source on %+v", tr)
		}
	}
	var b strings.Builder
	if err := Write(&b, ts, "nt"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "wasDerivedFrom") {
		t.Error("--no-provenance still wrote provenance")
	}
}

func TestTurtleDeclaresEveryNamespaceItUses(t *testing.T) {
	// A prefixed name with no @prefix line is a file no parser accepts, and it
	// is the easy thing to get wrong when a new term lands.
	var b strings.Builder
	if err := Write(&b, Triples(sample, Options{Types: true}), "turtle"); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, line := range strings.Split(out, "\n") {
		for _, tok := range strings.Fields(line) {
			name, _, ok := strings.Cut(tok, ":")
			if !ok || strings.HasPrefix(tok, "<") || strings.HasPrefix(tok, "@") || strings.HasPrefix(tok, `"`) {
				continue
			}
			if !strings.Contains(out, "@prefix "+name+":") {
				t.Errorf("%s is used and never declared", name)
			}
		}
	}
}

func TestTheJSONLDContextIsInline(t *testing.T) {
	// A context that has to be dereferenced is a context that stops working the
	// day the host does.
	var b strings.Builder
	if err := Write(&b, Triples(sample, Options{Types: true}), "jsonld"); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, `"@context"`) || !strings.Contains(out, NSSchema) {
		t.Error("the context is not in the file")
	}
	if strings.Contains(out, `"@context": "http`) {
		t.Error("the context is a URL somebody has to fetch")
	}
}

func TestTheSameClaimsWriteTheSameBytesTwice(t *testing.T) {
	// A dump that reorders itself between runs cannot be diffed, and a diff is
	// how somebody notices a page started saying something different.
	for _, f := range Formats {
		var a, b strings.Builder
		if err := Write(&a, Triples(sample, Options{Types: true}), f); err != nil {
			t.Fatal(err)
		}
		if err := Write(&b, Triples(sample, Options{Types: true}), f); err != nil {
			t.Fatal(err)
		}
		if a.String() != b.String() {
			t.Errorf("%s is not stable between two runs of the same input", f)
		}
	}
}

func TestAnIRIWithASpaceInItDoesNotBreakTheFile(t *testing.T) {
	// Facebook's own URLs carry spaces and quotes often enough that leaving
	// them would produce a file no parser accepts.
	ts := Triples([]graph.Edge{{
		From: "fb://post/1", Predicate: graph.LinksTo, To: "https://example.com/a b\"c",
		Source: "https://www.facebook.com/x?q=a b",
	}}, Options{})
	var b strings.Builder
	if err := Write(&b, ts, "nt"); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(b.String()), "\n") {
		if strings.Count(line, "<") != strings.Count(line, ">") {
			t.Errorf("unbalanced brackets, so an escape leaked through: %s", line)
		}
		if strings.Contains(line, `"`) {
			t.Errorf("a quote survived into an IRI: %s", line)
		}
	}
}
