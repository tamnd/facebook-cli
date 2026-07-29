package fb

import "testing"

// section_test.go runs the three tab parsers over the three captures.
//
// The numbers below were counted by hand out of the fixtures, so a change that
// silently drops half a page fails here rather than looking like a quiet day on
// the profile.

func TestPhotoTab(t *testing.T) {
	s := parseSection(fixtureDocs(t, "page_photos"), Ref{ID: "100044561550831", Name: "NASA"})
	if s.Kind != "photos" {
		t.Fatalf("kind = %q, want photos", s.Kind)
	}
	if len(s.Photos) != 8 {
		t.Fatalf("got %d photos, want the 8 the tab ships", len(s.Photos))
	}
	if !s.More || s.Cursor == "" {
		t.Error("the photo tab has more pages and has to hand back a cursor to reach them")
	}
	for _, p := range s.Photos {
		if p.ID == "" || p.URL == "" || p.Image.URI == "" {
			t.Fatalf("thin photo: %+v", p)
		}
		if p.Owner.ID != "100044561550831" {
			t.Errorf("photo %s lost its owner", p.ID)
		}
	}
	// The alt text is the one piece of description the tab carries, and it is
	// the reason the thumbnail is worth keeping at all.
	var alt int
	for _, p := range s.Photos {
		if p.Image.Alt != "" {
			alt++
		}
	}
	if alt == 0 {
		t.Error("no photo kept its accessibility caption")
	}
}

func TestEventTab(t *testing.T) {
	s := parseSection(fixtureDocs(t, "page_events"), Ref{ID: "100044561550831", Name: "NASA"})
	if s.Kind != "events" {
		t.Fatalf("kind = %q, want events", s.Kind)
	}
	if len(s.Events) == 0 {
		t.Fatal("the events tab parsed to nothing")
	}
	for _, e := range s.Events {
		if e.ID == "" || e.Name == "" {
			t.Fatalf("nameless event card: %+v", e)
		}
		if e.Start.IsZero() {
			t.Errorf("event %s has no start time, and the actions renderer beside it does", e.ID)
		}
		if e.URL == "" {
			t.Errorf("event %s has no permalink", e.ID)
		}
	}
	// Documented in the parser and asserted here so nobody adds a counts column
	// to the events table and wonders why it is empty: the tab does not carry
	// interested or going.
	for _, e := range s.Events {
		if e.Interested > 0 || e.Going > 0 {
			t.Fatalf("event %s came back with RSVP counts: the tab shape changed and doc 03 needs a revisit", e.ID)
		}
	}
}

func TestVideoTab(t *testing.T) {
	s := parseSection(fixtureDocs(t, "page_videos"), Ref{ID: "100044561550831", Name: "NASA"})
	if s.Kind != "videos" {
		t.Fatalf("kind = %q, want videos", s.Kind)
	}
	if len(s.Videos) != 9 {
		t.Fatalf("got %d videos, want the 9 the tab ships", len(s.Videos))
	}
	for _, v := range s.Videos {
		if v.ID == "" || v.URL == "" || v.Title == "" {
			t.Fatalf("thin video: %+v", v)
		}
		if v.CreatedAt.IsZero() {
			t.Errorf("video %s has no publish time", v.ID)
		}
		if v.SDURL == "" {
			t.Errorf("video %s came back without its media URL", v.ID)
		}
		if v.Width == 0 || v.Height == 0 {
			t.Errorf("video %s has no dimensions", v.ID)
		}
		if v.Owner.ID == "" {
			t.Errorf("video %s has no owner", v.ID)
		}
	}
	// The whole point of the videos tab over the photos tab: the items arrive
	// nearly complete, with the reactions on them, so nine videos cost one
	// request. Comments and shares are the exception, and the parser says so.
	first := s.Videos[0]
	if first.Counts.Reactions == 0 || len(first.Counts.ByType) == 0 {
		t.Errorf("video %s has no reactions: %+v", first.ID, first.Counts)
	}
	if first.Counts.Plays == 0 || first.Counts.Views == 0 {
		t.Errorf("video %s lost a play count: plays %d, views %d", first.ID, first.Counts.Plays, first.Counts.Views)
	}
	if first.PostID == "" {
		t.Error("the container story key did not give up its post id")
	}
	if first.Message.Text == "" {
		t.Error("the creation story message went missing")
	}
}

func TestSectionIsEmptyWithoutTheOperation(t *testing.T) {
	// A profile page ships neither section query, and asking it for a section
	// has to come back empty rather than half-parsed off the timeline.
	if s := parseSection(fixtureDocs(t, "page_nasa"), Ref{}); s.Len() != 0 || s.Kind != "" {
		t.Fatalf("got %+v", s)
	}
}
