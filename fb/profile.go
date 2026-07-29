package fb

import (
	"regexp"
	"strings"
)

// profile.go parses a Page or a person.
//
// Three operations contribute. ProfileCometHeaderQuery has the name, ids,
// cover, avatar and tabs. ProfilePlusCometLoggedOutRootQuery has the intro
// tiles: bio, category, websites, contact. ProfileCometTimelineFeedQuery has
// the newest post and the cursor. Any of the three can be missing, and the
// record says which were.

// parseProfile builds a profile from whatever operations the page shipped.
func parseProfile(docs map[string]*Document) Profile {
	var p Profile
	if d := docs["ProfileCometHeaderQuery"]; d != nil {
		p.addSurface(surfaceComet)
		applyHeader(&p, d.Data)
	}
	if d := docs["ProfilePlusCometLoggedOutRootQuery"]; d != nil {
		applyTiles(&p, d.Data)
	}
	if d := docs["ProfileCometTimelineFeedQuery"]; d != nil {
		applyTimeline(&p, d.Data)
	}
	return p
}

// applyProfileHead takes the two counts that exist only in the meta head.
//
// The like count and the talking-about count are rendered into og:description
// and are in no Relay payload the signed-out page ships, so this is not a
// fallback: it is the only source. `via` records that, because a reader who
// sees likes on the record deserves to know it came off a meta tag.
func applyProfileHead(p *Profile, h Head) {
	if h.Description == "" {
		return
	}
	p.addSurface(surfaceOpenGraph)
	if n, _ := headCount(h.Description, reLikes); n > 0 {
		p.Likes = n
		p.setVia("likes", surfaceOpenGraph)
	}
	if n, _ := headCount(h.Description, reTalkingAbout); n > 0 {
		p.TalkingAbout = n
		p.setVia("talking_about", surfaceOpenGraph)
	}
	if p.ID == "" && h.AppID != "" {
		p.ID = h.AppID
		p.setVia("id", surfaceOpenGraph)
	}
	if p.URL == "" {
		p.URL = h.URL
		p.Handle = handleOf(h.URL)
	}
	if p.Name == "" {
		p.Name = h.Title
	}
}

// applyHeader reads profile_header_renderer.
func applyHeader(p *Profile, data any) {
	u := digMap(data, "user", "profile_header_renderer", "user")
	if u == nil {
		// A person's header nests one level differently on some routes.
		if r := findKey(data, "profile_header_renderer"); r != nil {
			u = digMap(r, "profile_header_renderer", "user")
		}
	}
	if u == nil {
		return
	}
	p.ID = digStr(u, "id")
	p.Name = digStr(u, "name")
	p.URL = canonURL(digStr(u, "url"))
	p.Handle = handleOf(p.URL)
	p.Verified = digBool(u, "show_verified_badge_on_profile") || digBool(u, "is_verified")
	p.Memorialized = digBool(u, "is_visibly_memorialized")
	p.LiveNow = digBool(u, "is_currently_live")
	p.DelegatePage = digStr(u, "delegate_page", "id")

	if c := digMap(u, "cover_photo", "photo"); c != nil {
		p.Cover = parseImage(digMap(c, "image"))
		p.Cover.Alt = digStr(c, "accessibility_caption")
		if f := digMap(u, "cover_photo", "focus"); f != nil {
			p.Cover.Focus = &Focus{X: digFloat(f, "x"), Y: digFloat(f, "y")}
		}
	}
	if a := digMap(u, "profile_picture_for_sticky_bar"); a != nil {
		p.Avatar = parseImage(a)
	}
	if p.Avatar.Empty() {
		p.Avatar = parseImage(digMap(u, "profile_picture"))
	}
	for _, n := range edges(u, "profile_tabs", "profile_user", "timeline_nav_app_sections") {
		p.Tabs = append(p.Tabs, Tab{
			Name: digStr(n, "name"),
			URL:  canonURL(digStr(n, "url")),
			ID:   digStr(n, "id"),
		})
	}
}

// applyTiles reads the intro section: bio, category, websites, contact.
//
// None of these are fields. They are a list of renderers, and the switch below
// has a default arm on purpose: a context item fb does not recognise still
// lands on the record, because dropping it would make the record wrong and
// keeping it makes the next release's work visible.
func applyTiles(p *Profile, data any) {
	// The same tile list arrives under two roots: the column the page renders
	// and the user it belongs to. They were identical in every capture, so the
	// first one that has sections wins.
	sections := edges(data, "user", "profile_tile_sections")
	if len(sections) == 0 {
		sections = edges(data, "mainColumnTiles", "profile_tile_sections")
	}
	for _, sec := range sections {
		for _, view := range digMaps(sec, "profile_tile_views", "nodes") {
			r := digMap(view, "view_style_renderer")
			switch digStr(r, "__typename") {
			case "ProfileTileViewIntroBioRenderer":
				for _, item := range digMaps(r, "view", "profile_tile_items", "nodes") {
					if t := digMap(item, "node", "profile_status_text"); t != nil {
						p.Bio = parseText(t)
					}
				}
			case "ProfileTileViewContextListRenderer":
				for _, item := range digMaps(r, "view", "profile_tile_items", "nodes") {
					applyContextItem(p, item)
				}
			}
		}
	}
}

func applyContextItem(p *Profile, item map[string]any) {
	rend := digMap(item, "node", "timeline_context_item", "renderer")
	if rend == nil {
		return
	}
	ci := digMap(rend, "context_item")
	title := parseText(digMap(ci, "title"))
	icon := digStr(item, "icon_image", "uri")
	switch digStr(rend, "__typename") {
	case "InfluencerCategoryContextItemRenderer", "PageCategoryContextItemRenderer":
		p.CategoryRaw = title.Text
		p.Kind, p.Category = splitCategory(title.Text)
		if p.DelegatePage == "" {
			p.DelegatePage = digStr(rend, "associated_page_id")
		}
	case "WebsiteContextItemRenderer":
		// The rendered text is the shortened host; the destination is in the
		// range entity, already unwrapped from the redirect shim.
		if len(title.Links) > 0 {
			p.Websites = append(p.Websites, title.Links[0].URL)
		} else if u := unshim(digStr(rend, "url")); u != "" {
			p.Websites = append(p.Websites, u)
		}
	default:
		text := title.Text
		switch {
		case reEmail.MatchString(text):
			p.Email = text
		case rePhone.MatchString(text) && p.Phone == "":
			p.Phone = text
		default:
			p.ContactOther = append(p.ContactOther, ContextItem{
				Kind:     "unknown",
				Text:     text,
				Icon:     icon,
				Renderer: digStr(rend, "__typename"),
			})
		}
	}
}

var (
	reEmail = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[A-Za-z]{2,}$`)
	rePhone = regexp.MustCompile(`^[+(]?[\d][\d\s().+-]{6,}$`)
)

// splitCategory turns "Page · Government organisation" into a kind and a
// category list.
//
// There is no separate category field anywhere in the signed-out payload, so
// this is a split on a rendered separator, which is a guess. CategoryRaw keeps
// the string it guessed from, because the raw string is the evidence.
func splitCategory(raw string) (kind string, cats []string) {
	if raw == "" {
		return "", nil
	}
	parts := strings.Split(raw, " · ")
	for i, s := range parts {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if i == 0 {
			switch strings.ToLower(s) {
			case "page":
				kind = "page"
				continue
			case "digital creator", "public figure":
				kind = "page"
			}
		}
		cats = append(cats, s)
	}
	if kind == "" {
		kind = "person"
	}
	return kind, cats
}

// applyTimeline reads the newest posts and the cursor, and fills in the fields
// the timeline query carries that the header does not.
func applyTimeline(p *Profile, data any) {
	u := digMap(data, "user")
	if u == nil {
		return
	}
	if p.ID == "" {
		p.ID = digStr(u, "id")
	}
	if dp := digMap(u, "delegate_page"); dp != nil {
		if p.DelegatePage == "" {
			p.DelegatePage = digStr(dp, "id")
		}
		// The delegate page carries a clean category and a long description,
		// which the intro tile only has as a rendered sentence.
		if c := digStr(dp, "category_name"); c != "" && len(p.Category) == 0 {
			p.Category = []string{c}
		}
		if p.Bio.Empty() {
			p.Bio = parseText(digMap(dp, "best_description"))
		}
		if p.Cover.Empty() {
			p.Cover = parseImage(digMap(dp, "cover_photo", "photo", "image"))
		}
		if p.Avatar.Empty() {
			p.Avatar = Image{URI: digStr(dp, "profile_picture_uri")}
		}
	}
	units := digMap(u, "timeline_list_feed_units")
	if units == nil {
		return
	}
	for _, n := range edges(units) {
		post := parseStory(n)
		if post.ID == "" {
			continue
		}
		post.Envelope = Envelope{}
		p.Posts = append(p.Posts, post)
	}
	p.PostsCursor = digStr(units, "page_info", "end_cursor")
	p.PostsTruncated = digBool(units, "page_info", "has_next_page")
}
