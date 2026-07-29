package fbid

import "testing"

// The references in this file are real. Every id, key and token was read out of
// the captures in fb/testdata, so a change here that breaks a shape breaks a
// shape Facebook actually serves.

func TestParseKinds(t *testing.T) {
	cases := []struct {
		in   string
		kind string
		id   string
	}{
		{"nasa", KindHandle, ""},
		{"@NASA", KindHandle, ""},
		{"100044561550831", KindNumeric, "100044561550831"},
		{"54971236771", KindNumeric, "54971236771"},
		{"a.416661013162614", KindAlbum, "416661013162614"},
		{"pfbid02nHshyUHVP3EabcdefGH", KindOpaque, "pfbid02nHshyUHVP3EabcdefGH"},
		{"ZmVlZGJhY2s6MTU4Nzg2MDYzNjA0MjY0MA==", KindFeedback, "1587860636042640"},
		{"UzpfSTEwMDA0NDU2MTU1MDgzMToxNTg3ODYwNjM2MDQyNjQwOjE1ODc4NjA2MDkzNzU5NzY=", KindStory, "1587860636042640"},
		{"https://www.facebook.com/NASA", KindHandle, ""},
		{"https://www.facebook.com/profile.php?id=100044561550831", KindProfile, "100044561550831"},
		{"https://www.facebook.com/NASA/posts/pfbid02nHshy", KindPost, "pfbid02nHshy"},
		{"https://www.facebook.com/permalink.php?story_fbid=1587860636042640&id=100044561550831", KindPost, "1587860636042640"},
		{"https://www.facebook.com/photo/?fbid=1587860609375976&set=a.416661013162614", KindPhoto, "1587860609375976"},
		{"https://www.facebook.com/watch/?v=1380134307388381", KindVideo, "1380134307388381"},
		{"https://www.facebook.com/reel/1380134307388381", KindReel, "1380134307388381"},
		{"https://www.facebook.com/groups/1443890352589739", KindGroup, "1443890352589739"},
		{"https://www.facebook.com/groups/1443890352589739/posts/1373114781480892", KindPost, "1373114781480892"},
		{"https://www.facebook.com/events/1526795995494848", KindEvent, "1526795995494848"},
		{"https://www.facebook.com/directory/pages/A", KindDirectory, ""},
		{"https://www.facebook.com/search/top/?q=nasa", KindSearch, ""},
		{"https://fb.watch/abc123", KindShare, "abc123"},
		{"https://www.facebook.com/share/p/abc123/", KindShare, "abc123"},
		{"https://example.com/nasa", KindUnknown, ""},
		{"", KindUnknown, ""},
	}
	for _, c := range cases {
		got := Parse(c.in)
		if got.Kind != c.kind {
			t.Errorf("Parse(%q).Kind = %q, want %q", c.in, got.Kind, c.kind)
		}
		if c.id != "" && got.ID != c.id {
			t.Errorf("Parse(%q).ID = %q, want %q", c.in, got.ID, c.id)
		}
	}
}

func TestStoryKeyCarriesTheAuthor(t *testing.T) {
	// The point of the story key: it names a profile nobody fetched.
	r := Parse("UzpfSTEwMDA0NDU2MTU1MDgzMToxNTg3ODYwNjM2MDQyNjQwOjE1ODc4NjA2MDkzNzU5NzY=")
	if r.AuthorID != "100044561550831" {
		t.Fatalf("author = %q, want 100044561550831", r.AuthorID)
	}
	if r.PostID != "1587860636042640" || r.PhotoID != "1587860609375976" {
		t.Fatalf("post = %q, photo = %q", r.PostID, r.PhotoID)
	}
	if r.Decoded != "S:_I100044561550831:1587860636042640:1587860609375976" {
		t.Fatalf("decoded = %q", r.Decoded)
	}
}

func TestVideoStoryKey(t *testing.T) {
	// VK means the story is about a video or an event, so the last id is not a
	// post id and saying it was would put a video in the store as a post. This
	// key is the reel capture's.
	r := Parse("UzpfSTEwMDA0NDU2MTU1MDgzMTpWSzoxMzgwMTM0MzA3Mzg4Mzgx")
	if r.Kind != KindStory || r.PostID != "" || r.VideoID != "1380134307388381" {
		t.Fatalf("got %+v", r)
	}
	if _, err := Post("UzpfSTEwMDA0NDU2MTU1MDgzMTpWSzoxMzgwMTM0MzA3Mzg4Mzgx", ""); err == nil {
		t.Fatal("a VK story key is not a post and Post should say so")
	}
	v, err := Video("UzpfSTEwMDA0NDU2MTU1MDgzMTpWSzoxMzgwMTM0MzA3Mzg4Mzgx")
	if err != nil || v.ID != "1380134307388381" {
		t.Fatalf("Video: %+v, %v", v, err)
	}
}

func TestPfbidIsNeverAKey(t *testing.T) {
	r := Parse("pfbid031Q5ygEzfmJxQQQ")
	if !r.Opaque {
		t.Fatal("a pfbid has to come back opaque, or it ends up as a graph key")
	}
	if _, err := Profile("pfbid031Q5ygEzfmJxQQQ"); err == nil {
		t.Fatal("a pfbid is not a profile id")
	}
}

func TestProfileRoutes(t *testing.T) {
	// A number goes to /profile.php?id= and a name goes to /{name}. Getting that
	// backwards is a 404 rather than a redirect.
	num, err := Profile("100044561550831")
	if err != nil || num.URL != Host+"/profile.php?id=100044561550831" {
		t.Fatalf("numeric: %+v, %v", num, err)
	}
	name, err := Profile("nasa")
	if err != nil || name.URL != Host+"/nasa" {
		t.Fatalf("handle: %+v, %v", name, err)
	}
}

func TestPostNeedsAnAuthor(t *testing.T) {
	if _, err := Post("1587860636042640", ""); err == nil {
		t.Fatal("a bare post id has no route and Post should say so")
	}
	byID, err := Post("1587860636042640", "100044561550831")
	if err != nil || byID.URL != Host+"/permalink.php?story_fbid=1587860636042640&id=100044561550831" {
		t.Fatalf("with author id: %+v, %v", byID, err)
	}
	byName, err := Post("1587860636042640", "nasa")
	if err != nil || byName.URL != Host+"/nasa/posts/1587860636042640" {
		t.Fatalf("with handle: %+v, %v", byName, err)
	}
	// A feedback key carries the post id, so it needs no author beyond one.
	fromKey, err := Post("ZmVlZGJhY2s6MTU4Nzg2MDYzNjA0MjY0MA==", "nasa")
	if err != nil || fromKey.ID != "1587860636042640" {
		t.Fatalf("from feedback key: %+v, %v", fromKey, err)
	}
	// A story key carries both halves, so it needs nothing.
	whole, err := Post("UzpfSTEwMDA0NDU2MTU1MDgzMToxNTg3ODYwNjM2MDQyNjQwOjE1ODc4NjA2MDkzNzU5NzY=", "")
	if err != nil || whole.URL != Host+"/permalink.php?story_fbid=1587860636042640&id=100044561550831" {
		t.Fatalf("from story key: %+v, %v", whole, err)
	}
}

func TestTabsAreNotProfiles(t *testing.T) {
	r := Parse("https://www.facebook.com/NASA/photos")
	if r.Kind != KindHandle || r.Handle != "NASA" || r.Tab != "photos" {
		t.Fatalf("got %+v", r)
	}
}

func TestHostsNormalise(t *testing.T) {
	// Facebook writes web.facebook.com into every URL inside a payload, so a
	// reference copied out of one has to route the same as one copied out of the
	// address bar.
	for _, u := range []string{
		"https://web.facebook.com/groups/1443890352589739",
		"https://m.facebook.com/groups/1443890352589739",
		"facebook.com/groups/1443890352589739",
	} {
		r := Parse(u)
		if r.Kind != KindGroup || r.URL != Host+"/groups/1443890352589739" {
			t.Errorf("Parse(%q) = %+v", u, r)
		}
	}
}

func TestProfileShaped(t *testing.T) {
	if !ProfileShaped("100044561550831") {
		t.Error("15 digits starting 1000 is the profile shape")
	}
	if ProfileShaped("54971236771") {
		t.Error("a page id is not profile-shaped")
	}
}
