package fb

import (
	"encoding/json"
	"testing"
)

// text_test.go is the offset arithmetic, which is the one place in the tool
// where being off by one produces a record that looks right.
//
// Facebook counts offsets in UTF-16 code units, because the renderer is
// JavaScript. Go counts bytes. The two agree on ASCII and on nothing else, and
// a post that opens with an emoji is the cheapest way to prove which one the
// code is doing.

// textJSON builds a TextWithEntities the way a payload carries it.
func textJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("fixture does not parse: %v", err)
	}
	return v
}

// TestAnEmojiBeforeAMentionDoesNotMoveIt. A rocket is two UTF-16 units and four
// bytes, so slicing this text by byte offset takes the wrong span and takes it
// silently: the record still has a link, and the words on it are wrong.
func TestAnEmojiBeforeAMentionDoesNotMoveIt(t *testing.T) {
	// "🚀 Watch NASA live" with the link on "NASA": in UTF-16 that starts at 9.
	v := textJSON(t, `{
		"text": "🚀 Watch NASA live",
		"ranges": [{
			"offset": 9, "length": 4,
			"entity": {"__typename": "ExternalUrl", "url": "https://www.nasa.gov/live"}
		}]
	}`)
	got := parseText(v)
	if len(got.Links) != 1 {
		t.Fatalf("got %d links, want 1", len(got.Links))
	}
	if got.Links[0].Display != "NASA" {
		t.Errorf("the link reads %q, want NASA: the offsets are being counted in bytes", got.Links[0].Display)
	}
	if got.Links[0].URL != "https://www.nasa.gov/live" {
		t.Errorf("link url = %q", got.Links[0].URL)
	}
}

// TestAMentionIsARefAndALinkIsNot. A mention names a node fb can key on and a
// link names somebody else's site, and putting both in one list would mean
// every claim about a mention had to filter the list first.
func TestAMentionIsARefAndALinkIsNot(t *testing.T) {
	v := textJSON(t, `{
		"text": "with NASA and nasa.gov",
		"ranges": [
			{"offset": 5, "length": 4, "entity": {"__typename": "Page", "id": "100044561550831", "name": "NASA"}},
			{"offset": 14, "length": 8, "entity": {"__typename": "ExternalUrl", "url": "https://www.nasa.gov/"}}
		]
	}`)
	got := parseText(v)
	if len(got.Mentions) != 1 || got.Mentions[0].ID != "100044561550831" {
		t.Fatalf("mentions = %+v", got.Mentions)
	}
	if len(got.Links) != 1 || got.Links[0].URL != "https://www.nasa.gov/" {
		t.Fatalf("links = %+v", got.Links)
	}
	if len(got.Ranges) != 2 {
		t.Errorf("kept %d ranges, want both: the ranges are what says where in the text each one was", len(got.Ranges))
	}
}

// TestTheRedirectShimIsNeverStored. The signature after h= is per-render and
// per-viewer, so a stored shim makes two reads of one link look like two links,
// and it makes the store impossible to join against anything.
func TestTheRedirectShimIsNeverStored(t *testing.T) {
	v := textJSON(t, `{
		"text": "nasa.gov",
		"ranges": [{
			"offset": 0, "length": 8,
			"entity": {
				"__typename": "ExternalUrl",
				"external_url": "https://l.facebook.com/l.php?u=https%3A%2F%2Fwww.nasa.gov%2F&h=AT1abc",
				"url": "https://l.facebook.com/l.php?u=https%3A%2F%2Fwww.nasa.gov%2F&h=AT2def"
			}
		}]
	}`)
	got := parseText(v)
	if len(got.Links) != 1 {
		t.Fatalf("got %d links, want 1", len(got.Links))
	}
	if got.Links[0].URL != "https://www.nasa.gov/" {
		t.Errorf("link url = %q, want the destination with no shim", got.Links[0].URL)
	}
	if got.Ranges[0].Entity == nil || got.Ranges[0].Entity.URL != "https://www.nasa.gov/" {
		t.Errorf("the range's entity kept the shim: %+v", got.Ranges[0].Entity)
	}
}

// TestTextWithNothingInItIsEmpty keeps the empty object out of the record.
// Every post has a message field and plenty of posts have no message, so this
// is the difference between a photo record and a photo record with `"message":
// {"text": ""}` on it.
func TestTextWithNothingInItIsEmpty(t *testing.T) {
	if !parseText(nil).Empty() {
		t.Error("a missing TextWithEntities came back as something")
	}
	if !parseText(textJSON(t, `{"text": ""}`)).Empty() {
		t.Error("an empty TextWithEntities came back as something")
	}
	if parseText(textJSON(t, `{"text": "hi"}`)).Empty() {
		t.Error("a text with words in it reported empty")
	}
}
