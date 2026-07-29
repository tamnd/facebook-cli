package fb

import (
	"strings"
	"testing"
)

// graphql_test.go covers the reply side of a replay: what comes back over the
// wire and how it turns into one document.
//
// The bodies here are cut down from real responses. A replayed operation with
// deferred fragments answers as several JSON objects, one per line, and the
// first line is the only one that looks like the answer. Reading it with a
// plain json.Unmarshal gets a document that parses, is missing half its
// fields, and reports no error at all.

// TestAStreamedResponseIsStitchedIntoOneDocument. Line one is the skeleton,
// line two fills in a fragment it left a hole for.
func TestAStreamedResponseIsStitchedIntoOneDocument(t *testing.T) {
	body := strings.Join([]string{
		`{"data":{"node":{"id":"1","name":"NASA"}}}`,
		`{"data":{"cover_photo":{"photo":{"image":{"uri":"https://scontent.xx.fbcdn.net/cover.jpg"}}}},"label":"CoverPhoto$defer$","path":["node"]}`,
		``,
	}, "\n")
	doc, err := readStream("ProfileCometHeaderQuery", []byte(body))
	if err != nil {
		t.Fatalf("readStream: %v", err)
	}
	if got := dig(doc.Data, "node", "name"); got != "NASA" {
		t.Errorf("the root payload is missing: name = %v", got)
	}
	uri := dig(doc.Data, "node", "cover_photo", "photo", "image", "uri")
	if s, _ := uri.(string); s == "" {
		t.Errorf("the deferred line was not stitched in at its path: %v", doc.Data)
	}
}

// TestTheHijackingGuardIsNotAParseError. Facebook prefixes some responses with
// `for (;;);`, which is not JSON and is not optional to handle.
func TestTheHijackingGuardIsNotAParseError(t *testing.T) {
	doc, err := readStream("Op", []byte(`for (;;);{"data":{"node":{"id":"1"}}}`))
	if err != nil {
		t.Fatalf("readStream: %v", err)
	}
	if got := dig(doc.Data, "node", "id"); got != "1" {
		t.Errorf("guarded body read as %v", doc.Data)
	}
}

// TestALineThatIsNotJSONDoesNotEndTheRead. A response can carry a trailing
// blank line or a chunk marker, and throwing away the payloads that did parse
// because of one that did not is losing the whole read to a stray byte.
func TestALineThatIsNotJSONDoesNotEndTheRead(t *testing.T) {
	body := "not json\n" + `{"data":{"node":{"id":"1"}}}` + "\n\n"
	doc, err := readStream("Op", []byte(body))
	if err != nil {
		t.Fatalf("readStream: %v", err)
	}
	if got := dig(doc.Data, "node", "id"); got != "1" {
		t.Errorf("data = %v", doc.Data)
	}
}

// TestTheAllowlistRefusalIsNotARateLimit pins the classification the message
// string argues against. It arrives at HTTP 200 saying "Rate limit exceeded",
// it comes back instantly on the first request of a cold run, and retrying it
// is the wrong thing to do about it.
func TestTheAllowlistRefusalIsNotARateLimit(t *testing.T) {
	body := `{"errors":[{"message":"Rate limit exceeded","code":1675004,"severity":"CRITICAL"}]}`
	_, err := readStream("CometPhotoRootContentQuery", []byte(body))
	if err == nil {
		t.Fatal("a refusal came back as a document")
	}
	if got := ExitCode(err); got != 4 {
		t.Errorf("exit code %d, want 4: it is a refusal to answer without a session, not a rate limit", got)
	}
}

// TestAnEmptyResponseSaysSo rather than handing back a document with nothing in
// it, which reads downstream as a page that exists and holds nothing.
func TestAnEmptyResponseSaysSo(t *testing.T) {
	if _, err := readStream("Op", []byte("\n\n")); err == nil {
		t.Fatal("an empty body came back as a document")
	} else if ExitCode(err) != 3 {
		t.Errorf("exit code %d, want 3", ExitCode(err))
	}
}
