package fb

import "testing"

// event_test.go covers event_place, which is three different things wearing one
// field name, and the reason Place carries a kind rather than leaving a caller
// to guess from which fields happen to be set.

// TestAnOnlineEventHasAPlaceThatIsNotAPlace. Facebook gives an online event a
// FreeformPlace with an empty id and the literal name "Online event". Filing
// that as a venue puts a node called Online event in the store with every
// online event on Earth pointing at it.
func TestAnOnlineEventHasAPlaceThatIsNotAPlace(t *testing.T) {
	p := parsePlace(map[string]any{
		"__typename": "FreeformPlace",
		"id":         "",
		"name":       "Online event",
	})
	if p == nil {
		t.Fatal("the place was dropped, so an online event reads as an event with nowhere")
	}
	if p.Kind != "freeform" {
		t.Errorf("kind = %q, want freeform", p.Kind)
	}
	if p.ID != "" {
		t.Errorf("a freeform place claimed the id %q", p.ID)
	}
	if p.City != nil || p.Lat != nil {
		t.Error("a freeform place came back with a city or coordinates")
	}
}

// TestAVenueIsAPageWithACityBehindIt. The venue is a node fb can read, and the
// city is a second node, so an event in Houston is two claims rather than a
// string that says Houston.
func TestAVenueIsAPageWithACityBehindIt(t *testing.T) {
	p := parsePlace(map[string]any{
		"__typename":      "Page",
		"id":              "112172952138162",
		"contextual_name": "Space Center Houston",
		"city":            map[string]any{"id": "111751778855715", "contextual_name": "Houston, Texas"},
		"location":        map[string]any{"latitude": 29.5518, "longitude": -95.0981},
		"address":         map[string]any{"street": "1601 E NASA Pkwy"},
	})
	if p == nil {
		t.Fatal("the venue was dropped")
	}
	if p.Kind != "page" || p.ID != "112172952138162" {
		t.Errorf("kind %q id %q", p.Kind, p.ID)
	}
	if p.Name != "Space Center Houston" {
		t.Errorf("name = %q, want the contextual name the page renders", p.Name)
	}
	if p.City == nil || p.City.ID != "111751778855715" {
		t.Fatalf("city = %+v, want a ref to the city's own node", p.City)
	}
	if p.Lat == nil || p.Lng == nil {
		t.Fatal("coordinates were dropped")
	}
	if p.Address != "1601 E NASA Pkwy" {
		t.Errorf("address = %q", p.Address)
	}
}

// TestAPlaceWithNoIdAndNoNameIsNothing. An event_place that is present and
// empty is a field Facebook sent, not a location, and keeping it would put a
// blank place on the record for every event that has none.
func TestAPlaceWithNoIdAndNoNameIsNothing(t *testing.T) {
	if p := parsePlace(map[string]any{"__typename": "FreeformPlace"}); p != nil {
		t.Errorf("an empty place came back as %+v", p)
	}
	if p := parsePlace(nil); p != nil {
		t.Errorf("a missing place came back as %+v", p)
	}
}

// TestTheEventFixtureCarriesTheWholeHeader is the same three shapes read off a
// page Facebook really served, rather than off a map written here.
func TestTheEventFixtureCarriesTheWholeHeader(t *testing.T) {
	e := parseEvent(fixtureDocs(t, "event"), parseHead(fixture(t, "event")))
	if e.ID == "" || e.Name == "" {
		t.Fatalf("event fixture parsed to %+v", e)
	}
	if e.Start.IsZero() {
		t.Error("no start time, and the timezone is the whole reason this record has one")
	}
	if e.Host.Empty() {
		t.Error("no host, so nobody is claimed to be putting it on")
	}
	if e.Place == nil {
		t.Fatal("no place")
	}
	if e.Place.Kind == "" {
		t.Error("the place has no kind, so a caller has to guess which of the three shapes it is")
	}
}
