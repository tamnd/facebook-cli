package fb

import (
	"regexp"
	"strings"
)

// group.go parses a group.
//
// A group page ships three operations and each one holds a third of the record.
// CometGroupRootQuery has the header: name, cover, avatar, privacy, member
// count, tabs. GroupsCometDiscussionLayoutRootQuery has the about card, which
// is where the description lives. CometGroupDiscussionRootSuccessQuery has the
// feed.
//
// The feed on a page load is not the feed. What arrives is one
// GroupsSectionHeaderUnit reading "Most relevant" and, separately, the pinned
// highlight units. The actual posts come from replaying
// CometGroupDiscussionRootSuccessQuery with a count, which is what `fb group
// feed` does. So this parser reads whatever stories are present, pinned ones
// included, and the caller decides whether to go back for more.

// reMembers pulls the number out of "1.6K members" and out of the plain
// "12,043 members" a smaller group sends.
var reMembers = regexp.MustCompile(`^([\d.,KMB]+)`)

// parseGroup builds a group from the operations a group page ships.
func parseGroup(docs map[string]*Document) Group {
	var g Group
	if d := docs["CometGroupRootQuery"]; d != nil {
		g.addSurface(surfaceComet)
		applyGroupHeader(&g, d.Data)
	}
	if d := docs["GroupsCometDiscussionLayoutRootQuery"]; d != nil {
		g.addSurface(surfaceComet)
		applyGroupCards(&g, d.Data)
	}
	if d := docs["CometGroupDiscussionRootSuccessQuery"]; d != nil {
		g.addSurface(surfaceComet)
		applyGroupFeed(&g, d.Data)
	}
	return g
}

// applyGroupHeader reads the header renderer.
func applyGroupHeader(g *Group, data any) {
	grp := digMap(data, "group", "profile_header_renderer", "group")
	if grp == nil {
		grp = digMap(data, "group")
	}
	if grp == nil {
		return
	}
	if g.ID == "" {
		g.ID = digStr(grp, "id")
	}
	g.Name = digStr(grp, "name")
	if g.Name == "" {
		g.Name = digStr(grp, "featurable_title", "text")
	}
	g.URL = canonURL(digStr(grp, "url"))
	g.Privacy = digStr(grp, "privacy_info", "title", "text")
	g.Visible = strings.HasPrefix(strings.ToLower(g.Privacy), "public")
	g.Address = digStr(grp, "group_address")
	g.JoinState = digStr(grp, "viewer_join_state")
	g.ThemeColor = digStr(grp, "if_viewer_can_see_expanded_color", "group_theme_color", "hexcolor")

	// The count is a sentence, not a number. Both halves are kept: 1.6K is what
	// Facebook said and 1600 is what fb inferred, and only one of them is a
	// measurement.
	g.MembersText = digStr(grp, "group_member_profiles", "formatted_count_text")
	if m := reMembers.FindStringSubmatch(g.MembersText); m != nil {
		g.Members, _ = approxCount(m[1])
	}

	if c := digMap(grp, "cover_renderer", "cover_photo_content"); c != nil {
		g.Cover = parseImage(digMap(c, "photo", "image"))
		if g.Cover.Empty() {
			g.Cover = parseImage(digMap(c, "photo"))
		}
		if f := digMap(c, "focus"); f != nil {
			g.Cover.Focus = &Focus{X: digFloat(f, "x"), Y: digFloat(f, "y")}
		}
	}
	g.Avatar = parseImage(digMap(grp, "profile_picture_for_sticky_bar"))
	if g.Avatar.Empty() {
		g.Avatar = parseImage(digMap(grp, "if_viewer_cannot_change_cover_photo", "profile_picture_120"))
	}

	// The Discussion tab has a null URI because it is the page you are already
	// on, so it gets the group's own URL rather than being dropped.
	for _, n := range edges(grp, "group_content_views") {
		name := digStr(n, "content_view_title")
		if name == "" {
			continue
		}
		url := canonURL(digStr(n, "content_view_uri"))
		if url == "" {
			url = g.URL
		}
		g.Tabs = append(g.Tabs, Tab{Name: name, URL: url, ID: digStr(n, "content_view_type")})
	}
}

// applyGroupCards reads the discussion tab cards, which is where the
// description is.
func applyGroupCards(g *Group, data any) {
	if g.ID == "" {
		g.ID = digStr(data, "group", "id")
	}
	for _, card := range digMaps(data, "group", "comet_discussion_tab_cards") {
		grp := digMap(card, "group")
		if grp == nil {
			continue
		}
		if d := digMap(grp, "description_with_entities"); d != nil && g.Description.Empty() {
			g.Description = parseText(d)
		}
		if g.Privacy == "" {
			g.Privacy = digStr(grp, "privacy_info", "title", "text")
		}
	}
}

// applyGroupFeed reads the discussion feed and the pinned highlight units.
//
// The feed carries units that are not posts: a section header naming the sort
// order, invitation cards, join prompts. A unit with no post id is one of
// those, so the filter is "did a post come out of it", not a typename list that
// would need editing every time Facebook adds a card.
func applyGroupFeed(g *Group, data any) {
	if g.ID == "" {
		g.ID = digStr(data, "group", "id")
	}
	if g.Name == "" {
		g.Name = digStr(data, "group", "name")
	}
	seen := map[string]bool{}
	add := func(p Post) {
		// A unit with no post id is not a post: the section header naming the
		// sort order, the end-of-feed marker, a join prompt. Testing for the id
		// rather than listing the typenames means a card Facebook adds next
		// month falls out on its own instead of showing up as a blank post.
		if p.ID == "" || seen[p.ID] {
			return
		}
		seen[p.ID] = true
		if p.Group == nil && g.ID != "" {
			ref := Ref{Kind: "group", ID: g.ID, Name: g.Name, URL: g.URL}
			p.Group = &ref
		}
		g.Posts = append(g.Posts, p)
	}

	hl := digMap(data, "group", "if_viewer_can_see_highlight_units", "highlight_units")
	for _, n := range edges(hl) {
		s := digMap(n, "story")
		if s == nil {
			continue
		}
		p := parseStory(s)
		p.Pinned = digBool(n, "is_unit_pinned")
		add(p)
	}

	feed := digMap(data, "group", "group_feed")
	for _, n := range edges(feed) {
		if s := digMap(n, "story"); s != nil {
			add(parseStory(s))
			continue
		}
		add(parseStory(n))
	}
	g.PostsCursor = digStr(feed, "page_info", "end_cursor")
}
