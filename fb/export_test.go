package fb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSlugifyVietnamese(t *testing.T) {
	cases := map[string]string{
		"Khóa học AI miễn phí cho người Việt": "khoa-hoc-ai-mien-phi-cho-nguoi-viet",
		"Hello, World!": "hello-world",
		"   ":           "",
		"Đăng ký ngay":  "dang-ky-ngay",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPostRelPath(t *testing.T) {
	p := Post{PostID: "123", Text: "Một bài viết", CreatedAt: time.Date(2025, 11, 3, 9, 0, 0, 0, time.UTC)}
	if got, want := postRelPath(p), "2025/11/2025-11-03_mot-bai-viet.md"; got != want {
		t.Errorf("postRelPath = %q, want %q", got, want)
	}
	und := Post{PostID: "999", Text: "x"}
	if got := postRelPath(und); !strings.HasPrefix(got, "unknown/post-999") {
		t.Errorf("undated postRelPath = %q, want unknown/post-999...", got)
	}
}

func TestArchiveWriteAndIncremental(t *testing.T) {
	dir := t.TempDir()
	page := &Page{PageID: "demo", Name: "Demo Page", Category: "Education", URL: "https://www.facebook.com/demo"}
	ar, err := OpenArchive(dir, page)
	if err != nil {
		t.Fatal(err)
	}
	p := Post{
		PostID:        "p1",
		Text:          "First post about AI\nsecond line",
		CreatedAt:     time.Date(2025, 11, 3, 9, 0, 0, 0, time.UTC),
		LikeCount:     12,
		CommentCount:  2,
		ShareCount:    1,
		Permalink:     "https://www.facebook.com/demo/posts/p1",
		MediaURLs:     []string{"https://scontent.x/a.jpg?oh=1", "https://scontent.y/a.jpg?oh=2", "https://z/b.jpg"},
		ExternalLinks: []string{"https://example.com"},
	}
	comments := []Comment{
		{CommentID: "c1", Text: "great", AuthorName: "Alice", CreatedAtText: "2h"},
		{CommentID: "c2", Text: "thanks", AuthorName: "Bob", ParentID: "c1"},
	}
	rel, err := ar.WritePost(p, comments)
	if err != nil {
		t.Fatal(err)
	}
	if rel != "2025/11/2025-11-03_first-post-about-ai.md" {
		t.Fatalf("rel = %q", rel)
	}
	if err := ar.Flush(); err != nil {
		t.Fatal(err)
	}

	md, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	body := string(md)
	for _, want := range []string{
		"# First post about AI",
		"12 likes, 2 comments, 1 shares",
		"[View on Facebook](https://www.facebook.com/demo/posts/p1)",
		"## Images",
		"## Links",
		"## Comments (2)",
		"- **Alice (2h)**: great",
		"  - **Bob**: thanks", // reply indented under parent
		"[Back to index](../../README.md)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("post md missing %q\n--- got ---\n%s", want, body)
		}
	}
	// images deduped by base name: a.jpg appears once, b.jpg once
	if n := strings.Count(body, "![]("); n != 2 {
		t.Errorf("want 2 deduped images, got %d", n)
	}

	readme, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	rd := string(readme)
	for _, want := range []string{"# Demo Page", "*Education*", "## Stats", "| Posts | 1 |", "### 2025", "#### November 2025", "first-post-about-ai.md"} {
		if !strings.Contains(rd, want) {
			t.Errorf("README missing %q\n--- got ---\n%s", want, rd)
		}
	}

	// Incremental: re-open and confirm the post is now skipped.
	ar2, err := OpenArchive(dir, page)
	if err != nil {
		t.Fatal(err)
	}
	if !ar2.Has("p1") {
		t.Error("Has(p1) = false after archiving; incremental skip broken")
	}
	if ar2.Has("p2") {
		t.Error("Has(p2) = true for a post never archived")
	}
	// If the file is deleted, Has must report false so it is re-fetched.
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
	if ar2.Has("p1") {
		t.Error("Has(p1) = true after its file was deleted; should re-fetch")
	}
}
