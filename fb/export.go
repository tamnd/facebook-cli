package fb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Archive writes a Page and its posts as a tree of incremental Markdown files.
//
// The layout mirrors a human-browsable archive:
//
//	<dir>/index.json              lightweight state (the incremental marker)
//	<dir>/README.md               generated index, grouped by year and month
//	<dir>/2025/11/2025-11-03_slug.md   one file per post, with its comments
//
// index.json is the only state: a post is re-fetched only when it is missing
// from the index or its Markdown file is gone from disk, so re-running the
// archive picks up just the new posts.
type Archive struct {
	Dir  string
	Page *Page
	idx  *archiveIndex
}

type archiveIndex struct {
	Page  *Page          `json:"page"`
	Posts []archiveEntry `json:"posts"`
	byID  map[string]int // post_id -> index in Posts
}

type archiveEntry struct {
	PostID    string    `json:"post_id"`
	Title     string    `json:"title"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
	Likes     int64     `json:"likes"`
	Comments  int64     `json:"comments"`
	Shares    int64     `json:"shares"`
	Images    int       `json:"images"`
	Permalink string    `json:"permalink"`
}

// OpenArchive loads (or creates) the archive rooted at dir. The page metadata is
// refreshed on every open so the README always reflects the latest profile.
func OpenArchive(dir string, page *Page) (*Archive, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	idx := &archiveIndex{byID: map[string]int{}}
	if b, err := os.ReadFile(filepath.Join(dir, "index.json")); err == nil {
		if jerr := json.Unmarshal(b, idx); jerr != nil {
			return nil, fmt.Errorf("read archive index: %w", jerr)
		}
	}
	idx.reindex()
	if page != nil {
		idx.Page = page
	}
	return &Archive{Dir: dir, Page: idx.Page, idx: idx}, nil
}

func (ix *archiveIndex) reindex() {
	ix.byID = make(map[string]int, len(ix.Posts))
	for i, e := range ix.Posts {
		ix.byID[e.PostID] = i
	}
}

// Has reports whether a post is already archived: present in the index and its
// Markdown file still exists on disk. This is the incremental skip check.
func (a *Archive) Has(postID string) bool {
	i, ok := a.idx.byID[postID]
	if !ok {
		return false
	}
	_, err := os.Stat(filepath.Join(a.Dir, filepath.FromSlash(a.idx.Posts[i].Path)))
	return err == nil
}

// WritePost renders one post (with its comments) to Markdown, writes it
// immediately, and records it in the index. It returns the file's path relative
// to the archive root.
func (a *Archive) WritePost(p Post, comments []Comment) (string, error) {
	rel := postRelPath(p)
	abs := filepath.Join(a.Dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, []byte(renderPost(p, comments, rel)), 0o644); err != nil {
		return "", err
	}
	e := archiveEntry{
		PostID:    p.PostID,
		Title:     leadLine(p.Text, 120),
		Path:      rel,
		CreatedAt: p.CreatedAt,
		Likes:     firstNonZero(p.ReactionCount, p.LikeCount),
		Comments:  p.CommentCount,
		Shares:    p.ShareCount,
		Images:    len(dedupImages(p.MediaURLs)),
		Permalink: p.Permalink,
	}
	if e.Title == "" {
		e.Title = "post-" + p.PostID
	}
	if i, ok := a.idx.byID[p.PostID]; ok {
		a.idx.Posts[i] = e
	} else {
		a.idx.byID[p.PostID] = len(a.idx.Posts)
		a.idx.Posts = append(a.idx.Posts, e)
	}
	return rel, nil
}

// Count returns the number of posts currently in the archive index.
func (a *Archive) Count() int { return len(a.idx.Posts) }

// Flush persists the index and regenerates README.md from every archived post.
func (a *Archive) Flush() error {
	a.idx.Page = a.Page
	b, err := json.MarshalIndent(a.idx, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(a.Dir, "index.json"), b, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.Dir, "README.md"), []byte(renderReadme(a.Page, a.idx.Posts)), 0o644)
}

// postRelPath is the post's file path relative to the archive root:
// "2025/11/2025-11-03_slug.md", or "unknown/post-<id>.md" when the date is zero.
func postRelPath(p Post) string {
	slug := slugify(p.Text)
	if p.CreatedAt.IsZero() {
		name := "post-" + p.PostID
		if slug != "" {
			name += "_" + slug
		}
		return "unknown/" + name + ".md"
	}
	dir := p.CreatedAt.Format("2006/01")
	name := p.CreatedAt.Format("2006-01-02")
	if slug != "" {
		name += "_" + slug
	}
	return dir + "/" + name + ".md"
}

func renderPost(p Post, comments []Comment, rel string) string {
	var b strings.Builder
	title := leadLine(p.Text, 120)
	if title == "" {
		title = "Post " + p.PostID
	}
	fmt.Fprintf(&b, "# %s\n\n", title)

	when := p.CreatedAtText
	if when == "" && !p.CreatedAt.IsZero() {
		when = p.CreatedAt.Format("Mon, 02 Jan 2006")
	}
	likes := firstNonZero(p.ReactionCount, p.LikeCount)
	meta := fmt.Sprintf("%d likes, %d comments, %d shares", likes, p.CommentCount, p.ShareCount)
	if when != "" {
		fmt.Fprintf(&b, "**%s** | %s\n\n", when, meta)
	} else {
		fmt.Fprintf(&b, "%s\n\n", meta)
	}
	if p.Permalink != "" {
		fmt.Fprintf(&b, "[View on Facebook](%s)\n\n", p.Permalink)
	}
	if t := strings.TrimSpace(p.Text); t != "" {
		fmt.Fprintf(&b, "%s\n\n", t)
	}

	if imgs := dedupImages(p.MediaURLs); len(imgs) > 0 {
		b.WriteString("## Images\n\n")
		for _, u := range imgs {
			fmt.Fprintf(&b, "![](%s)\n\n", u)
		}
	}
	if len(p.ExternalLinks) > 0 {
		b.WriteString("## Links\n\n")
		for _, u := range p.ExternalLinks {
			fmt.Fprintf(&b, "- <%s>\n", u)
		}
		b.WriteString("\n")
	}

	if len(comments) > 0 {
		fmt.Fprintf(&b, "## Comments (%d)\n\n", len(comments))
		for _, c := range comments {
			indent := ""
			if c.ParentID != "" {
				indent = "  "
			}
			author := c.AuthorName
			if author == "" {
				author = "Someone"
			}
			line := author
			if c.CreatedAtText != "" {
				line += " (" + c.CreatedAtText + ")"
			}
			fmt.Fprintf(&b, "%s- **%s**: %s\n", indent, line, strings.TrimSpace(c.Text))
		}
		b.WriteString("\n")
	}

	depth := strings.Count(rel, "/")
	fmt.Fprintf(&b, "[Back to index](%sREADME.md)\n", strings.Repeat("../", depth))
	return b.String()
}

func renderReadme(page *Page, entries []archiveEntry) string {
	var b strings.Builder
	name := "Facebook Archive"
	if page != nil && page.Name != "" {
		name = page.Name
	}
	fmt.Fprintf(&b, "# %s\n\n", name)
	if page != nil {
		if page.Category != "" {
			fmt.Fprintf(&b, "*%s*\n\n", page.Category)
		}
		if page.URL != "" {
			fmt.Fprintf(&b, "[View on Facebook](%s)\n\n", page.URL)
		}
		if about := strings.TrimSpace(page.About); about != "" {
			for ln := range strings.SplitSeq(leadLine(about, 600), "\n") {
				fmt.Fprintf(&b, "> %s\n", ln)
			}
			b.WriteString("\n")
		}
	}

	var comments, images int64
	for _, e := range entries {
		comments += e.Comments
		images += int64(e.Images)
	}
	b.WriteString("## Stats\n\n")
	b.WriteString("| Metric | Count |\n|---|---:|\n")
	fmt.Fprintf(&b, "| Posts | %d |\n", len(entries))
	fmt.Fprintf(&b, "| Comments | %d |\n", comments)
	fmt.Fprintf(&b, "| Images | %d |\n\n", images)

	// newest first, undated posts last
	sorted := make([]archiveEntry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, c := sorted[i].CreatedAt, sorted[j].CreatedAt
		if a.IsZero() != c.IsZero() {
			return c.IsZero() // dated before undated
		}
		return a.After(c)
	})

	b.WriteString("## Posts\n\n")
	var curYear int
	var curMonth time.Month
	undatedHeader := false
	for _, e := range sorted {
		if e.CreatedAt.IsZero() {
			if !undatedHeader {
				b.WriteString("### Undated\n\n")
				undatedHeader = true
			}
		} else {
			y, m := e.CreatedAt.Year(), e.CreatedAt.Month()
			if y != curYear {
				fmt.Fprintf(&b, "### %d\n\n", y)
				curYear, curMonth = y, 0
			}
			if m != curMonth {
				fmt.Fprintf(&b, "#### %s %d\n\n", m.String(), y)
				curMonth = m
			}
		}
		date := "      "
		if !e.CreatedAt.IsZero() {
			date = e.CreatedAt.Format("Jan 02")
		}
		fmt.Fprintf(&b, "- %s  [%s](%s) (%d likes, %d comments)\n",
			date, mdEscape(e.Title), e.Path, e.Likes, e.Comments)
	}
	b.WriteString("\n")
	if page != nil && !page.FetchedAt.IsZero() {
		fmt.Fprintf(&b, "_Last updated %s_\n", page.FetchedAt.Format("2006-01-02 15:04"))
	}
	return b.String()
}

// dedupImages removes duplicate image URLs that differ only by CDN query string
// or path prefix, keying on the base file name.
func dedupImages(urls []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, u := range urls {
		key := imageBaseName(u)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, u)
	}
	return out
}

func imageBaseName(u string) string {
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	if i := strings.LastIndex(u, "/"); i >= 0 {
		u = u[i+1:]
	}
	return u
}

func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "]", "\\]")
	s = strings.ReplaceAll(s, "[", "\\[")
	return s
}

// firstLine returns the first non-empty line of text, trimmed to maxLen at a
// word boundary.
func leadLine(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = strings.TrimSpace(text[:i])
	}
	return truncateWords(text, maxLen)
}

func truncateWords(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	cut := s[:maxLen]
	if i := strings.LastIndexByte(cut, ' '); i > maxLen/2 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut) + "..."
}

// slugify turns post text into a filesystem-safe, ASCII slug, transliterating
// Vietnamese diacritics so the archive reads well for Vietnamese pages.
func slugify(text string) string {
	s := leadLine(text, 80)
	s = viReplacer.Replace(strings.ToLower(s))
	var b strings.Builder
	lastDash := true // avoid a leading dash
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	return truncateSlug(out, 60)
}

func truncateSlug(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	cut := s[:maxLen]
	if i := strings.LastIndexByte(cut, '-'); i > maxLen/2 {
		cut = cut[:i]
	}
	return strings.Trim(cut, "-")
}

// viReplacer maps Vietnamese letters (and a few common symbols) to ASCII.
var viReplacer = strings.NewReplacer(
	"à", "a", "á", "a", "ả", "a", "ã", "a", "ạ", "a",
	"ă", "a", "ằ", "a", "ắ", "a", "ẳ", "a", "ẵ", "a", "ặ", "a",
	"â", "a", "ầ", "a", "ấ", "a", "ẩ", "a", "ẫ", "a", "ậ", "a",
	"è", "e", "é", "e", "ẻ", "e", "ẽ", "e", "ẹ", "e",
	"ê", "e", "ề", "e", "ế", "e", "ể", "e", "ễ", "e", "ệ", "e",
	"ì", "i", "í", "i", "ỉ", "i", "ĩ", "i", "ị", "i",
	"ò", "o", "ó", "o", "ỏ", "o", "õ", "o", "ọ", "o",
	"ô", "o", "ồ", "o", "ố", "o", "ổ", "o", "ỗ", "o", "ộ", "o",
	"ơ", "o", "ờ", "o", "ớ", "o", "ở", "o", "ỡ", "o", "ợ", "o",
	"ù", "u", "ú", "u", "ủ", "u", "ũ", "u", "ụ", "u",
	"ư", "u", "ừ", "u", "ứ", "u", "ử", "u", "ữ", "u", "ự", "u",
	"ỳ", "y", "ý", "y", "ỷ", "y", "ỹ", "y", "ỵ", "y",
	"đ", "d",
)
