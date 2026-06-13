package fb

import (
	"testing"

	"github.com/tamnd/facebook-cli/pkg/fbid"
)

func TestScanJSONString(t *testing.T) {
	cases := []struct {
		in   string
		i    int
		want string
		end  int
		ok   bool
	}{
		{`"hello"`, 0, "hello", 7, true},
		{`"a\"b"`, 0, `a"b`, 6, true},
		{`"line\nbreak"`, 0, "line\nbreak", 13, true},
		{`"A"`, 0, "A", 3, true},
		{`not a string`, 0, "", 0, false},
		{`"unterminated`, 0, "", 13, false},
	}
	for _, c := range cases {
		got, end, ok := scanJSONString(c.in, c.i)
		if ok != c.ok || got != c.want {
			t.Errorf("scanJSONString(%q) = (%q, %d, %v), want (%q, %d, %v)", c.in, got, end, ok, c.want, c.end, c.ok)
		}
		if ok && end != c.end {
			t.Errorf("scanJSONString(%q) end = %d, want %d", c.in, end, c.end)
		}
	}
}

func TestJSONStringsForKey(t *testing.T) {
	doc := `{"message":{"text":"first"},"x":1,"message":{"text":"second longer"}}`
	got := jsonStringsForKey(doc, `"message":{"text":`)
	if len(got) != 2 {
		t.Fatalf("got %d values, want 2", len(got))
	}
	if got[0].val != "first" || got[1].val != "second longer" {
		t.Errorf("values = %q, %q", got[0].val, got[1].val)
	}
	if got[0].pos >= got[1].pos {
		t.Errorf("positions not ordered: %d, %d", got[0].pos, got[1].pos)
	}
	if longestString(got) != "second longer" {
		t.Errorf("longestString = %q", longestString(got))
	}
}

func TestIntsForKey(t *testing.T) {
	doc := `"reaction_count":{"count":131},"reaction_count":{"count":9}`
	got := intsForKey(doc, `"reaction_count":{"count":`)
	if len(got) != 2 || got[0] != 131 || got[1] != 9 {
		t.Fatalf("intsForKey = %v", got)
	}
	if maxInt(got) != 131 {
		t.Errorf("maxInt = %d, want 131", maxInt(got))
	}
}

func TestOGMeta(t *testing.T) {
	doc := `<meta property="og:title" content="AI VIET NAM" />` +
		`<meta property="og:description" content="416,254 likes &amp; 2,815 talking about this" />` +
		`<meta property="og:title" content="ignored duplicate" />`
	og := ogMeta(doc)
	if og["title"] != "AI VIET NAM" {
		t.Errorf("og:title = %q", og["title"])
	}
	if og["description"] != "416,254 likes & 2,815 talking about this" {
		t.Errorf("og:description = %q", og["description"])
	}
}

func TestPageCountsFromDescription(t *testing.T) {
	likes, talking := pageCountsFromDescription("AI VIET NAM. 416,254 likes . 2,815 talking about this")
	if likes != 416254 {
		t.Errorf("likes = %d, want 416254", likes)
	}
	if talking != 2815 {
		t.Errorf("talking = %d, want 2815", talking)
	}
}

func TestFindPostPermalinks(t *testing.T) {
	doc := `text https:\/\/www.facebook.com\/nasa\/posts\/pfbid02abc more ` +
		`https://www.facebook.com/nasa/posts/123 and a dupe ` +
		`https://www.facebook.com/nasa/posts/pfbid02abc end ` +
		`https://www.facebook.com/groups/456/posts/789`
	got := findPostPermalinks(doc)
	if len(got) != 3 {
		t.Fatalf("got %d permalinks, want 3: %v", len(got), got)
	}
	if got[0] != "https://www.facebook.com/nasa/posts/pfbid02abc" {
		t.Errorf("first permalink = %q", got[0])
	}
	if got[2] != "https://www.facebook.com/groups/456/posts/789" {
		t.Errorf("group permalink = %q", got[2])
	}
}

func TestLastNumericSegment(t *testing.T) {
	cases := map[string]string{
		"https://www.facebook.com/nasa/posts/123":      "123",
		"https://www.facebook.com/nasa/posts/123/?x=1": "123",
		"https://www.facebook.com/nasa/posts/pfbid0":   "",
	}
	for in, want := range cases {
		if got := lastNumericSegment(in); got != want {
			t.Errorf("lastNumericSegment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPostImagesExcludesAvatars(t *testing.T) {
	doc := `"photo_image":{"uri":"https://scontent.test/t39.30808-6/content.jpg"}` +
		`"image":{"uri":"https://scontent.test/t1.30497-1/avatar.jpg"}`
	og := "https://scontent.test/t1.30497-1/page-avatar.jpg"
	got := postImages(doc, og)
	if len(got) != 1 {
		t.Fatalf("got %v, want exactly the content image", got)
	}
	if got[0] != "https://scontent.test/t39.30808-6/content.jpg" {
		t.Errorf("image = %q", got[0])
	}
}

func TestParsePostSSR(t *testing.T) {
	doc := `<meta property="og:url" content="https://www.facebook.com/nasa/posts/123" />` +
		`<meta property="og:title" content="NASA" />` +
		`<meta property="og:image" content="https://scontent.test/t39.30808-6/p.jpg" />` +
		`"message":{"text":"Hello from orbit"}` +
		`"creation_time":1717372800,` +
		`"reaction_count":{"count":131}` +
		`"comments":{"total_count":48}` +
		`"share_count":{"count":4}`
	id := fbid.Classify("https://www.facebook.com/nasa/posts/123")
	post := parsePostSSR(doc, id, "https://www.facebook.com/nasa/posts/123")
	if post.PostID != "123" {
		t.Errorf("PostID = %q", post.PostID)
	}
	if post.Text != "Hello from orbit" {
		t.Errorf("Text = %q", post.Text)
	}
	if post.ReactionCount != 131 || post.CommentCount != 48 || post.ShareCount != 4 {
		t.Errorf("counts = %d/%d/%d", post.ReactionCount, post.CommentCount, post.ShareCount)
	}
	if post.CreatedAt.IsZero() || post.CreatedAtText != "2024-06-03 00:00" {
		t.Errorf("CreatedAt = %v (%q)", post.CreatedAt, post.CreatedAtText)
	}
	if len(post.MediaURLs) != 1 {
		t.Errorf("MediaURLs = %v", post.MediaURLs)
	}
}

func TestParseCommentsSSR(t *testing.T) {
	doc := `"author":{"__typename":"User","id":"1","name":"Alice"}` +
		`"body":{"text":"great post"}` +
		`"author":{"__typename":"User","name":"Bob"}` +
		`"body":{"text":"thanks"}` +
		`"body":{"text":"thanks"}` // duplicate of Bob's, should be dropped
	got := parseCommentsSSR(doc, "123")
	if len(got) != 2 {
		t.Fatalf("got %d comments, want 2: %+v", len(got), got)
	}
	if got[0].AuthorName != "Alice" || got[0].Text != "great post" {
		t.Errorf("comment 0 = %+v", got[0])
	}
	if got[1].AuthorName != "Bob" || got[1].Text != "thanks" {
		t.Errorf("comment 1 = %+v", got[1])
	}
	if got[0].PostID != "123" {
		t.Errorf("PostID = %q", got[0].PostID)
	}
}

func TestParsePageSSR(t *testing.T) {
	doc := `<meta property="og:title" content="AI VIET NAM" />` +
		`<meta property="og:description" content="416,254 likes . 2,815 talking about this" />` +
		`<meta property="og:image" content="https://scontent.test/avatar.jpg" />` +
		`<meta property="og:url" content="https://www.facebook.com/aivietnam.edu.vn" />`
	page := parsePageSSR(doc, "aivietnam.edu.vn")
	if page.Name != "AI VIET NAM" {
		t.Errorf("Name = %q", page.Name)
	}
	if page.LikesCount != 416254 || page.TalkingAboutCount != 2815 {
		t.Errorf("counts = %d / %d", page.LikesCount, page.TalkingAboutCount)
	}
	if page.AvatarURL != "https://scontent.test/avatar.jpg" {
		t.Errorf("AvatarURL = %q", page.AvatarURL)
	}
}

func TestWWWURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://mbasic.facebook.com/nasa/posts/123", "https://www.facebook.com/nasa/posts/123"},
		{"https://m.facebook.com/nasa", "https://www.facebook.com/nasa"},
		{"nasa", "https://www.facebook.com/nasa"},
		{"/nasa", "https://www.facebook.com/nasa"},
	}
	for _, c := range cases {
		id := fbid.Classify(c.in)
		if got := wwwURL(c.in, id); got != c.want {
			t.Errorf("wwwURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
