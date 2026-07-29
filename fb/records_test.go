package fb

import (
	"testing"

	"github.com/tamnd/facebook-cli/pkg/graph"
)

// records_test.go covers parseRef, which every record leans on and which has
// two shapes on it that are not ids at all.

// TestAPfbidOwnerIsKeptAndNeverKeyedOn. A group feed gives every author as a
// pfbid, and a pfbid is per-render and per-viewer: the same person on two reads
// is two of them. It stays on the record because it is what the surface said,
// it is flagged opaque, and nothing in the graph is allowed to key on it.
func TestAPfbidOwnerIsKeptAndNeverKeyedOn(t *testing.T) {
	r := parseRef(map[string]any{
		"__typename": "User",
		"id":         "pfbid02Xv9pWc5Q3f4a6b8c",
		"name":       "A Person",
		"url":        "https://www.facebook.com/people/A-Person/pfbid02Xv9pWc5Q3f4a6b8c/",
	})
	if !r.Opaque {
		t.Fatal("a pfbid was not flagged opaque, so it is about to become a node")
	}
	if r.ID != "pfbid02Xv9pWc5Q3f4a6b8c" {
		t.Errorf("the id was rewritten to %q, and the record should say what the page said", r.ID)
	}
	if uri := refURI(r); uri != "" {
		t.Errorf("a pfbid produced the URI %q", uri)
	}
	// The URL is still worth keeping: it is the one thing --resolve-opaque has
	// to work from.
	if r.URL == "" {
		t.Error("the profile URL was dropped, so --resolve-opaque has nothing to fetch")
	}
}

// TestANumericOwnerIsANode is the other half, so the test above is not just
// asserting that refURI returns nothing for everything.
func TestANumericOwnerIsANode(t *testing.T) {
	r := parseRef(map[string]any{"__typename": "Page", "id": "100044561550831", "name": "NASA"})
	if r.Opaque {
		t.Error("a numeric id was flagged opaque")
	}
	if got, want := refURI(r), "fb://page/100044561550831"; got != want {
		t.Errorf("uri = %q, want %q", got, want)
	}
}

// TestAnExternalLinkIsKeyedOnWhereItGoes. An ExternalUrl's id is base64 of the
// redirect shim, signature and all, so it is a different id on every render of
// the same link, and keying on it would put one link in the store once per read.
func TestAnExternalLinkIsKeyedOnWhereItGoes(t *testing.T) {
	r := parseRef(map[string]any{
		"__typename": "ExternalUrl",
		"id":         "aHR0cHM6Ly9sLmZhY2Vib29rLmNvbS9sLnBocD91PWh0dHBzJTNBJTJGJTJG",
		"url":        "https://l.facebook.com/l.php?u=https%3A%2F%2Fwww.nasa.gov%2F&h=AT1abc",
	})
	if r.ID != "" {
		t.Errorf("an external link kept the id %q, which changes on every render", r.ID)
	}
	if r.URL != "https://www.nasa.gov/" {
		t.Errorf("url = %q, want the destination with the shim off", r.URL)
	}
	if got := refURI(r); got != graph.External("https://www.nasa.gov/") {
		t.Errorf("uri = %q, want the hash of the destination", got)
	}
}

// TestARefWithNothingOnItIsEmpty. Facebook sends the field with nulls in it
// often enough that without this every record has an author with no name.
func TestARefWithNothingOnItIsEmpty(t *testing.T) {
	if !parseRef(nil).Empty() {
		t.Error("a missing ref came back as something")
	}
	if !parseRef(map[string]any{"__typename": "User"}).Empty() {
		t.Error("a ref with only a typename came back as something")
	}
}
