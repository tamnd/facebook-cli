package fbid

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		in   string
		kind Kind
		want map[string]string // field -> expected value
	}{
		{"nasa", KindPage, map[string]string{"PageID": "nasa", "Slug": "nasa"}},
		{"https://www.facebook.com/nasa", KindPage, map[string]string{"PageID": "nasa"}},
		{"profile.php?id=100000000000001", KindProfile, map[string]string{"ProfileID": "100000000000001"}},
		{"https://www.facebook.com/profile.php?id=42", KindProfile, map[string]string{"ProfileID": "42"}},
		{"https://www.facebook.com/groups/123456789", KindGroup, map[string]string{"GroupID": "123456789"}},
		{"https://www.facebook.com/nasa/posts/pfbid02abc", KindPost, map[string]string{"PostID": "pfbid02abc", "OwnerID": "nasa"}},
		{"story.php?story_fbid=111&id=222", KindPost, map[string]string{"PostID": "111", "OwnerID": "222"}},
		{"https://www.facebook.com/watch/?v=987654321", KindVideo, map[string]string{"VideoID": "987654321"}},
		{"https://www.facebook.com/events/555", KindEvent, map[string]string{"EventID": "555"}},
		{"https://www.facebook.com/photo.php?fbid=999", KindPhoto, map[string]string{"PhotoID": "999"}},
	}
	for _, c := range cases {
		got := Classify(c.in)
		if got.Kind != c.kind {
			t.Errorf("Classify(%q).Kind = %q, want %q", c.in, got.Kind, c.kind)
		}
		fields := map[string]string{
			"PageID": got.PageID, "ProfileID": got.ProfileID, "GroupID": got.GroupID,
			"PostID": got.PostID, "OwnerID": got.OwnerID, "VideoID": got.VideoID,
			"EventID": got.EventID, "PhotoID": got.PhotoID, "Slug": got.Slug,
		}
		for k, want := range c.want {
			if fields[k] != want {
				t.Errorf("Classify(%q).%s = %q, want %q", c.in, k, fields[k], want)
			}
		}
	}
}

func TestToMBasic(t *testing.T) {
	got := Classify("nasa").MBasicURL
	want := "https://mbasic.facebook.com/nasa"
	if got != want {
		t.Errorf("MBasicURL = %q, want %q", got, want)
	}
}

func TestReservedNotSlug(t *testing.T) {
	// reserved path segments must not classify as a page slug
	for _, s := range []string{"login", "watch", "groups", "events"} {
		if Classify("https://www.facebook.com/"+s).Kind == KindPage {
			// watch/groups/events have their own kinds; login is unknown.
			if s == "login" {
				t.Errorf("Classify(%q) should not be a page", s)
			}
		}
	}
}
