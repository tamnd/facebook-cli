package fb

import (
	"slices"
	"testing"
)

func TestDataSJSFindsBlocks(t *testing.T) {
	blocks := dataSJS(fixture(t, "page_nasa"))
	if len(blocks) < 50 {
		t.Fatalf("got %d data-sjs blocks, want the page's full set", len(blocks))
	}
	for i, b := range blocks {
		if b.truncated() {
			t.Errorf("block %d: data-content-len says %d, got %d bytes", i, b.Claimed, len(b.Body))
		}
	}
}

func TestModuleNameStripsHash(t *testing.T) {
	// An event page names the module with a hash suffix. Matching on equality
	// finds the profile payloads and silently misses events and groups.
	cases := map[string]string{
		"RelayPrefetchedStreamCache":                                  "RelayPrefetchedStreamCache",
		"RelayPrefetchedStreamCache@4a1fb7054d45192b147ad69ff77e2f17": "RelayPrefetchedStreamCache",
		"ScheduledServerJS":                                           "ScheduledServerJS",
	}
	for in, want := range cases {
		if got := moduleName(in); got != want {
			t.Errorf("moduleName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOpFromPreloaderID(t *testing.T) {
	cases := map[string]string{
		"adp_ProfileCometHeaderQueryRelayPreloader_6a697058bcbae7f25800971": "ProfileCometHeaderQuery",
		"adp_CometGroupDiscussionRootSuccessQueryRelayPreloader_abc":        "CometGroupDiscussionRootSuccessQuery",
	}
	for in, want := range cases {
		if got := opFromPreloaderID(in); got != want {
			t.Errorf("opFromPreloaderID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPageShipsItsOperations(t *testing.T) {
	cases := []struct {
		fixture string
		want    []string
	}{
		{"page_nasa", []string{"ProfileCometHeaderQuery", "ProfileCometTimelineFeedQuery"}},
		{"post_perm", []string{"CometSinglePostDialogContentQuery"}},
		{"photo", []string{"CometPhotoRootContentQuery"}},
		{"group_big", []string{"CometGroupRootQuery", "CometGroupDiscussionRootSuccessQuery"}},
		{"event", []string{"EventCometPermalinkHeaderQuery", "PublicEventCometAboutRootQuery"}},
		{"page_videos", []string{"CometProfilePlusVideosRootQuery"}},
		{"reel", []string{"FBReelsRootWithEntrypointQuery"}},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			docs := fixtureDocs(t, c.fixture)
			var got []string
			for op := range docs {
				got = append(got, op)
			}
			slices.Sort(got)
			for _, want := range c.want {
				if _, ok := docs[want]; !ok {
					t.Errorf("%s missing, page shipped %v", want, got)
				}
			}
		})
	}
}

func TestStitchAppliesDeferredFragments(t *testing.T) {
	// The header's cover photo arrives in a deferred fragment, not in the root
	// payload, so finding it proves the stitch ran.
	docs := fixtureDocs(t, "page_nasa")
	doc := docs["ProfileCometHeaderQuery"]
	if doc == nil {
		t.Fatal("no ProfileCometHeaderQuery")
	}
	uri := dig(doc.Data, "user", "profile_header_renderer", "user", "cover_photo", "photo", "image", "uri")
	if s, _ := uri.(string); s == "" {
		t.Fatalf("cover photo uri not stitched in, got %v", uri)
	}
}

func TestWalkPathHandlesArrayIndices(t *testing.T) {
	root := map[string]any{"user": map[string]any{"items": []any{map[string]any{"id": "1"}}}}
	node := walkPath(root, []any{"user", "items", float64(0)})
	if node == nil || node["id"] != "1" {
		t.Fatalf("walkPath through an array index got %v", node)
	}
	if walkPath(root, []any{"user", "items", float64(9)}) != nil {
		t.Error("an index past the end should not resolve")
	}
}
