package fb

import (
	"strings"
)

// post.go parses a story.
//
// A story is the same object wherever it turns up: a profile timeline, a group
// feed, a post permalink, the container of a photo. What changes is how deep it
// is buried and which comet_sections the route filled in, so the parser digs
// rather than following one path.

// parseStory reads one feed unit or story node.
func parseStory(n map[string]any) Post {
	if n == nil {
		return Post{}
	}
	p := Post{
		ID:           digStr(n, "post_id"),
		StoryID:      digStr(n, "id"),
		URL:          canonURL(digStr(n, "permalink_url")),
		CreatedAt:    digTime(n, "creation_time"),
		DelegatePage: digStr(n, "delegate_page_id"),
	}
	if p.ID == "" {
		p.ID = digStr(n, "legacy_story_hideable_id")
	}
	if p.ID == "" {
		p.ID = digStr(n, "comet_sections", "content", "story", "post_id")
	}
	if p.URL == "" {
		p.URL = canonURL(digStr(n, "url"))
	}

	// The message lives at four paths in the same story and the first one that
	// exists is not always the one with the text in it. A group's featured unit
	// ships {"__typename":"TextWithEntities"} at the shallow path and the real
	// message seven levels down, because Relay put the text under the fragment
	// that asked for it. So the paths are tried for text, not for presence.
	for _, path := range [][]string{
		{"comet_sections", "content", "story", "message"},
		{"comet_sections", "content", "story", "comet_sections", "message", "story", "message"},
		{"comet_sections", "content", "story", "comet_sections", "message_container", "story", "message"},
		{"message"},
	} {
		if t := parseText(digMap(n, path...)); !t.Empty() {
			p.Message = t
			break
		}
	}
	p.SEOTitle = firstStr(n, "seo_title")
	if p.SEOTitle == "" {
		if s := findKey(n, "seo_title"); s != nil {
			p.SEOTitle = digStr(s, "seo_title")
		}
	}

	// Actors are on the story itself on a permalink and under the context
	// layout on a feed unit. Several copies of the same actor turn up in one
	// story and they are not equally filled in: the copy under the content
	// strategy is an id and an avatar with no name, because the renderer that
	// asked for it only draws the picture. So an actor with a name beats one
	// without, and the last resort is a walk.
	for _, path := range [][]string{
		{"actors"},
		{"comet_sections", "context_layout", "story", "actors"},
		{"comet_sections", "context_layout", "story", "comet_sections", "actor_photo", "story", "actors"},
		{"comet_sections", "content", "story", "actors"},
	} {
		actors := digMaps(n, path...)
		if len(actors) == 0 {
			continue
		}
		ref := parseRef(actors[0])
		if ref.Empty() {
			continue
		}
		if p.Author.Empty() {
			p.Author = ref
		}
		if ref.Name != "" {
			p.Author = ref
			break
		}
	}
	if p.Author.Empty() {
		if a := findKey(n, "actors"); a != nil {
			if actors := digMaps(a, "actors"); len(actors) > 0 {
				p.Author = parseRef(actors[0])
			}
		}
	}
	if p.URL == "" {
		// A feed unit hides the permalink in the timestamp row.
		if md := digMaps(n, "comet_sections", "context_layout", "story", "comet_sections", "metadata"); len(md) > 0 {
			p.URL = canonURL(digStr(md[0], "story", "url"))
		}
	}

	p.Counts = storyCounts(n)
	p.Attachments = parseAttachments(digMaps(n, "attachments"))
	if len(p.Attachments) == 0 {
		p.Attachments = parseAttachments(digMaps(n, "comet_sections", "content", "story", "attachments"))
	}
	for _, a := range p.Attachments {
		if a.Kind == "post" && a.Media != nil && p.SharedPost == nil {
			p.SharedPost = &Post{ID: a.Media.ID, URL: a.Media.URL, Author: *a.Media}
		}
	}
	if g := digMap(n, "feedback", "associated_group"); g != nil {
		ref := parseRef(g)
		ref.Kind = "group"
		p.Group = &ref
	}
	p.Comments, p.CommentsCursor = storyComments(n, p.Author.ID)
	return p
}

// storyCounts finds the UFI renderer wherever this route put it.
//
// Its path is nine levels deep on a feed unit and shorter on a permalink, and
// it moves between routes. The renderer's own name is stable, so that is what
// we look for.
func storyCounts(n map[string]any) Counts {
	var c Counts
	if r := findKey(n, "comet_ufi_summary_and_actions_renderer"); r != nil {
		c = parseFeedback(digMap(r, "comet_ufi_summary_and_actions_renderer", "feedback"))
	}
	if f := digMap(n, "feedback"); f != nil {
		c = c.fill(parseFeedback(f))
	}
	if c.Comments == 0 {
		if cr := findKey(n, "comment_rendering_instance"); cr != nil {
			c.Comments = digInt(cr, "comment_rendering_instance", "comments", "total_count")
		}
	}
	return c
}

// storyComments reads the comment list Facebook shipped with the story.
//
// A permalink ships two lists: three comments Facebook picked to show under the
// post, and twenty with a cursor. We read the second and ignore the first,
// because three curated comments read as "these are the comments".
func storyComments(n map[string]any, authorID string) ([]Comment, string) {
	var node map[string]any
	for _, key := range []string{
		"comment_rendering_instance_for_feed_location",
		"comment_rendering_instance",
	} {
		if r := findKey(n, key); r != nil {
			node = digMap(r, key, "comments")
			if node != nil {
				break
			}
		}
	}
	if node == nil {
		return nil, ""
	}
	var out []Comment
	for _, e := range edges(node) {
		c := parseComment(e)
		if c.ID == "" {
			continue
		}
		c.IsAuthor = authorID != "" && c.Author.ID == authorID
		out = append(out, c)
	}
	return out, digStr(node, "page_info", "end_cursor")
}

// parseComment reads one comment node.
func parseComment(n map[string]any) Comment {
	if n == nil {
		return Comment{}
	}
	c := Comment{
		ID:        digStr(n, "id"),
		URL:       canonURL(digStr(n, "feedback", "url")),
		CreatedAt: digTime(n, "created_time"),
		Body:      parseText(digMap(n, "body")),
		Counts:    parseFeedback(digMap(n, "feedback")),
		Replies:   digInt(n, "feedback", "replies_fields", "total_count"),
	}
	if c.URL == "" {
		c.URL = canonURL(digStr(n, "url"))
	}
	if a := digMap(n, "author"); a != nil {
		c.Author = parseRef(a)
	}
	if c.Body.Empty() {
		if b := findKey(n, "body"); b != nil {
			c.Body = parseText(digMap(b, "body"))
		}
	}
	return c
}

// parseAttachments reads a story's attachment list, including album
// subattachments.
//
// An album post is one attachment with subattachments, each a photo with its
// own id, so one post read yields every photo id in the album with no second
// request. That is where most of the graph's photo nodes come from.
func parseAttachments(list []map[string]any) []Attachment {
	var out []Attachment
	for _, a := range list {
		att := parseAttachment(a)
		if att.Kind == "" && att.URL == "" && att.Media == nil {
			continue
		}
		out = append(out, att)
	}
	return out
}

func parseAttachment(a map[string]any) Attachment {
	att := Attachment{
		Title:       firstStr(a, "title_with_entities.text"),
		Description: digStr(a, "description", "text"),
		URL:         canonURL(unshim(digStr(a, "url"))),
		Source:      digStr(a, "source", "text"),
	}
	if t := digMap(a, "title_with_entities"); t != nil {
		att.Title = digStr(t, "text")
	}
	media := digMap(a, "media")
	if media == nil {
		media = digMap(a, "target")
	}
	if media != nil {
		ref := parseRef(media)
		if !ref.Empty() {
			att.Media = &ref
		}
		if img := parseImage(firstOf(media,
			[]string{"image"},
			[]string{"photo_image"},
			[]string{"preferred_thumbnail", "image"},
		)); !img.Empty() {
			img.Alt = digStr(media, "accessibility_caption")
			att.Image = &img
		}
	}
	att.Kind = attachmentKind(a, media)
	if u := digStr(a, "web_link", "url"); u != "" {
		att.URL = u
	}
	for _, sub := range digMaps(a, "all_subattachments", "nodes") {
		att.Subattachments = append(att.Subattachments, parseAttachment(sub))
	}
	for _, sub := range digMaps(a, "subattachments") {
		att.Subattachments = append(att.Subattachments, parseAttachment(sub))
	}
	return att
}

// attachmentKind names an attachment from its style list, falling back to the
// media typename. style_list is Facebook's own classification and it is more
// reliable than guessing from which fields are set.
func attachmentKind(a, media map[string]any) string {
	for _, s := range digSlice(a, "style_list") {
		switch str, _ := s.(string); str {
		case "photo":
			return "photo"
		case "video", "video_inline", "video_autoplay":
			return "video"
		case "album":
			return "album"
		case "share", "external_share":
			return "link"
		case "event":
			return "event"
		case "share_from_feed", "attached_story":
			return "post"
		}
	}
	if media != nil {
		if k := kindOf(digStr(media, "__typename")); k != "" {
			return k
		}
	}
	if strings.Contains(digStr(a, "url"), "/events/") {
		return "event"
	}
	if digStr(a, "url") != "" {
		return "link"
	}
	return ""
}
