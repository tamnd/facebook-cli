package fb

import (
	"encoding/json"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// Facebook serves a server-rendered page to web crawlers: the same URL a person
// sees, but with the post text, counts, media, and a few preview comments baked
// into the HTML as Open Graph tags and embedded Relay JSON. This file reads that
// surface, so fb works with no login and no browser. The trade-off is depth: the
// crawler page exposes the most recent posts and a handful of preview comments
// per post, not the full history or every comment.

// scanJSONString decodes the JSON string token that begins at s[i] (which must
// be the opening quote) and returns the decoded value plus the index just past
// the closing quote.
func scanJSONString(s string, i int) (string, int, bool) {
	if i >= len(s) || s[i] != '"' {
		return "", i, false
	}
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++ // skip the escaped character
		case '"':
			var out string
			if err := json.Unmarshal([]byte(s[i:j+1]), &out); err != nil {
				return "", j + 1, false
			}
			return out, j + 1, true
		}
	}
	return "", len(s), false
}

// indexedString is a decoded JSON string paired with the byte offset at which it
// was found, so values from different keys can be ordered against each other.
type indexedString struct {
	pos int
	val string
}

// jsonStringsForKey returns every JSON-string value that immediately follows key
// in s. key must end right before the opening quote, e.g. `"message":{"text":`.
func jsonStringsForKey(s, key string) []indexedString {
	var out []indexedString
	for i := 0; ; {
		k := strings.Index(s[i:], key)
		if k < 0 {
			break
		}
		pos := i + k + len(key)
		if v, _, ok := scanJSONString(s, pos); ok {
			out = append(out, indexedString{pos: pos, val: v})
		}
		i = pos
	}
	return out
}

// intsForKey returns every integer literal that immediately follows key in s.
func intsForKey(s, key string) []int64 {
	var out []int64
	for i := 0; ; {
		k := strings.Index(s[i:], key)
		if k < 0 {
			break
		}
		pos := i + k + len(key)
		j := pos
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j > pos {
			if n, err := strconv.ParseInt(s[pos:j], 10, 64); err == nil {
				out = append(out, n)
			}
		}
		i = pos + 1
	}
	return out
}

func maxInt(vals []int64) int64 {
	var m int64
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}

func longestString(vals []indexedString) string {
	out := ""
	for _, v := range vals {
		if len(v.val) > len(out) {
			out = v.val
		}
	}
	return out
}

var ogMetaRe = regexp.MustCompile(`<meta property="og:([a-z_:]+)" content="([^"]*)"`)

// ogMeta returns the Open Graph tags keyed by the part after "og:".
func ogMeta(doc string) map[string]string {
	out := map[string]string{}
	for _, m := range ogMetaRe.FindAllStringSubmatch(doc, -1) {
		if _, ok := out[m[1]]; !ok {
			out[m[1]] = html.UnescapeString(m[2])
		}
	}
	return out
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

func htmlTitle(doc string) string {
	m := titleRe.FindStringSubmatch(doc)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(m[1]))
}

// permalinkRe matches the canonical post permalinks Facebook renders into a feed
// page. It covers Page/profile posts (slug/posts/id) and group posts
// (groups/gid/posts|permalink/id).
var permalinkRe = regexp.MustCompile(`https://www\.facebook\.com/(?:groups/\d+/(?:posts|permalink)|[A-Za-z0-9.\-]+/posts)/(?:pfbid[0-9A-Za-z]+|\d+)`)

// storyRe matches the older story.php / permalink.php post forms.
var storyRe = regexp.MustCompile(`https://www\.facebook\.com/(?:permalink|story)\.php\?story_fbid=[0-9A-Za-z]+&id=\d+`)

// findPostPermalinks extracts the post permalinks embedded in a feed page, in
// document order, de-duplicated by post id.
func findPostPermalinks(doc string) []string {
	unesc := strings.ReplaceAll(doc, `\/`, "/")
	var links []string
	seen := map[string]bool{}
	add := func(matches []string) {
		for _, m := range matches {
			pid := fbid.Classify(m).PostID
			if pid == "" || seen[pid] {
				continue
			}
			seen[pid] = true
			links = append(links, m)
		}
	}
	add(permalinkRe.FindAllString(unesc, -1))
	add(storyRe.FindAllString(unesc, -1))
	return links
}

var lastNumRe = regexp.MustCompile(`/(\d+)/?$`)

func lastNumericSegment(rawURL string) string {
	u := rawURL
	if i := strings.IndexByte(u, '?'); i >= 0 {
		u = u[:i]
	}
	m := lastNumRe.FindStringSubmatch(u)
	if m == nil {
		return ""
	}
	return m[1]
}

// avatarMarkers flag profile photos and small chrome images that are attached
// to a story's chrome rather than being the post's own media.
var avatarMarkers = []string{"/t1.30497", "s32x32", "s40x40", "s48x48", "s60x60", "p50x50", "p64x64"}

// postImages collects a post's own attached media. It reads the dedicated image
// keys in the embedded JSON (photo_image and image), which point at the content
// photos, rather than scraping every scontent URL on the page (which would pull
// in avatars and page chrome). The Open Graph image is included as a fallback.
func postImages(doc, ogImage string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(u string) {
		if u == "" || seen[u] {
			return
		}
		for _, mk := range avatarMarkers {
			if strings.Contains(u, mk) {
				return
			}
		}
		seen[u] = true
		out = append(out, u)
	}
	if ogImage != "" {
		add(ogImage)
	}
	for _, v := range jsonStringsForKey(doc, `"photo_image":{"uri":`) {
		add(v.val)
	}
	for _, v := range jsonStringsForKey(doc, `"image":{"uri":`) {
		add(v.val)
	}
	return out
}

var lphpRe = regexp.MustCompile(`https://l\.facebook\.com/l\.php\?u=([^"&\\]+)`)

func postExternalLinks(doc string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range lphpRe.FindAllStringSubmatch(doc, -1) {
		if u := extractRedirectTarget("l.php?u=" + m[1]); u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// parsePageSSR builds a Page record from the crawler page's Open Graph tags.
func parsePageSSR(doc, slug string) *Page {
	og := ogMeta(doc)
	name := firstNonEmpty(og["title"], stripTitleSuffix(htmlTitle(doc)))
	desc := og["description"]
	likes, talking := pageCountsFromDescription(desc)
	return &Page{
		PageID:            slug,
		Slug:              slug,
		Name:              name,
		About:             truncate(desc, 1000),
		LikesCount:        likes,
		FollowersCount:    likes,
		TalkingAboutCount: talking,
		Verified:          strings.Contains(strings.ToLower(doc), `"is_verified":true`),
		AvatarURL:         og["image"],
		URL:               firstNonEmpty(og["url"], "https://www.facebook.com/"+slug),
		FetchedAt:         time.Now(),
	}
}

var descNumRe = regexp.MustCompile(`[0-9][0-9.,]*`)

// pageCountsFromDescription pulls the like and talking-about counts out of the
// Open Graph description, whose number positions are stable across locales:
// "<name>. <likes> likes . <talking> talking about this".
func pageCountsFromDescription(desc string) (likes, talking int64) {
	nums := descNumRe.FindAllString(desc, -1)
	parse := func(s string) int64 {
		s = strings.NewReplacer(".", "", ",", "", " ", "").Replace(s)
		n, _ := strconv.ParseInt(s, 10, 64)
		return n
	}
	if len(nums) > 0 {
		likes = parse(nums[0])
	}
	if len(nums) > 1 {
		talking = parse(nums[1])
	}
	return likes, talking
}

// parseGroupSSR builds a Group record from a group's crawler page. The crawler
// surface carries the group name, description, cover image, and visibility, but
// not the member count, so MembersCount is only set when the description happens
// to spell it out.
func parseGroupSSR(doc, gid string) *Group {
	og := ogMeta(doc)
	name := firstNonEmpty(stripTitleSuffix(og["title"]), stripTitleSuffix(htmlTitle(doc)))
	desc := og["description"]
	return &Group{
		GroupID:      gid,
		Slug:         gid,
		Name:         name,
		Description:  truncate(desc, 1000),
		Privacy:      groupVisibility(doc),
		MembersCount: membersFromDescription(desc),
		CoverURL:     og["image"],
		URL:          firstNonEmpty(og["url"], "https://www.facebook.com/groups/"+gid),
		FetchedAt:    time.Now(),
	}
}

// groupVisibility maps the embedded group visibility enum, or a body-text
// fallback, to fb's public/private vocabulary.
func groupVisibility(doc string) string {
	switch {
	case strings.Contains(doc, `"visibility":"OPEN"`):
		return "public"
	case strings.Contains(doc, `"visibility":"CLOSED"`), strings.Contains(doc, `"visibility":"SECRET"`):
		return "private"
	}
	low := strings.ToLower(doc)
	switch {
	case strings.Contains(low, "private group"):
		return "private"
	case strings.Contains(low, "public group"):
		return "public"
	}
	return ""
}

var membersCountRe = regexp.MustCompile(`([0-9][0-9.,]*)\s*members`)

// membersFromDescription pulls a "<n> members" count out of a group's Open Graph
// description when one is present, and returns 0 otherwise.
func membersFromDescription(desc string) int64 {
	m := membersCountRe.FindStringSubmatch(strings.ToLower(desc))
	if m == nil {
		return 0
	}
	s := strings.NewReplacer(".", "", ",", "").Replace(m[1])
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// parsePostSSR builds a Post from a single post's crawler page.
func parsePostSSR(doc string, id fbid.Identity, target string) *Post {
	og := ogMeta(doc)
	permalink := firstNonEmpty(og["url"], target)
	postID := firstNonEmpty(lastNumericSegment(permalink), id.PostID)

	text := longestString(jsonStringsForKey(doc, `"message":{"text":`))
	if text == "" {
		// Fall back to the page-name-prefixed document title.
		text = strings.TrimSpace(strings.TrimPrefix(htmlTitle(doc), firstNonEmpty(og["title"], "")))
		text = strings.TrimPrefix(text, "-")
		text = strings.TrimSpace(text)
	}

	reactions := maxInt(intsForKey(doc, `"reaction_count":{"count":`))
	comments := maxInt(intsForKey(doc, `"comments":{"total_count":`))
	shares := maxInt(intsForKey(doc, `"share_count":{"count":`))

	post := &Post{
		PostID:        postID,
		OwnerID:       id.OwnerID,
		OwnerName:     og["title"],
		Text:          text,
		LikeCount:     reactions,
		ReactionCount: reactions,
		CommentCount:  comments,
		ShareCount:    shares,
		Permalink:     permalink,
		MediaURLs:     postImages(doc, og["image"]),
		ExternalLinks: postExternalLinks(doc),
		FetchedAt:     time.Now(),
	}
	if ct := intsForKey(doc, `"creation_time":`); len(ct) > 0 {
		post.CreatedAt = time.Unix(ct[0], 0).UTC()
		post.CreatedAtText = post.CreatedAt.Format("2006-01-02 15:04")
	}
	return post
}

// authorNameRe locates the start of the name string inside a comment author
// block, in both the id-bearing and id-less forms Facebook emits.
var authorNameRe = regexp.MustCompile(`"author":\{"__typename":"User"(?:,"id":"[^"]*")?,"name":`)

// authorNames returns each comment author's name paired with its position.
func authorNames(doc string) []indexedString {
	var out []indexedString
	for _, loc := range authorNameRe.FindAllStringIndex(doc, -1) {
		if v, _, ok := scanJSONString(doc, loc[1]); ok {
			out = append(out, indexedString{pos: loc[1], val: v})
		}
	}
	return out
}

// parseCommentsSSR extracts the preview comments Facebook renders into a post's
// crawler page. Each comment's body is attributed to the author name that most
// closely precedes it in the document.
func parseCommentsSSR(doc, postID string) []Comment {
	names := authorNames(doc)
	sort.Slice(names, func(i, j int) bool { return names[i].pos < names[j].pos })

	bodies := jsonStringsForKey(doc, `"body":{"text":`)
	var out []Comment
	seen := map[string]bool{}
	for _, b := range bodies {
		if strings.TrimSpace(b.val) == "" {
			continue
		}
		author := ""
		for _, n := range names {
			if n.pos < b.pos {
				author = n.val
			} else {
				break
			}
		}
		key := author + "\x00" + b.val
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Comment{
			CommentID:  strconv.Itoa(len(out) + 1),
			PostID:     postID,
			AuthorName: author,
			Text:       b.val,
			FetchedAt:  time.Now(),
		})
	}
	return out
}

// wwwURL normalizes any post input to its www.facebook.com form, the surface the
// crawler HTML is served on.
func wwwURL(input string, id fbid.Identity) string {
	if strings.HasPrefix(input, "http") {
		u := strings.Replace(input, "://mbasic.facebook.com", "://www.facebook.com", 1)
		return strings.Replace(u, "://m.facebook.com", "://www.facebook.com", 1)
	}
	if id.CanonicalURL != "" {
		return strings.Replace(id.CanonicalURL, "://mbasic.facebook.com", "://www.facebook.com", 1)
	}
	return "https://www.facebook.com/" + strings.TrimPrefix(input, "/")
}
