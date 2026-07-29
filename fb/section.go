package fb

import "github.com/tamnd/facebook-cli/pkg/fbid"

// section.go parses the three profile tabs that answer as a list: photos,
// events and videos.
//
// They are one file because two of them are one operation. /nasa/photos and
// /nasa/events both ship ProfileCometTopAppSectionQuery, and the only difference
// is section_type and which renderer the collection carries. The videos tab is
// its own operation, CometProfilePlusVideosRootQuery, and it is by far the most
// generous of the three: twenty-one videos with their media URLs, titles,
// messages and reaction breakdowns in one page fetch, where the photo tab gives
// eight thumbnails and an id.
//
// All three page the same way, through the cursor on the connection, which is
// why the engine can walk any of them with one loop.

// Section is one tab of a profile read as a list.
//
// One type for the three, with one slice filled, rather than three types that
// would differ only in that slice. The envelope and the cursor are the same
// question in each case.
//
// There is no total on it because no tab gives one honestly. The videos tab
// ships a count field and it reads zero signed out, and the photo and event tabs
// ship none at all, so the only truthful answer to "how many" is to walk the
// cursor to the end.
type Section struct {
	Envelope
	Kind      string      `json:"kind"`
	Name      string      `json:"name,omitempty"`
	URL       string      `json:"url,omitempty"`
	Owner     Ref         `json:"owner,omitempty"`
	Photos    []Photo     `json:"photos,omitempty"`
	Events    []EventCard `json:"events,omitempty"`
	Videos    []Video     `json:"videos,omitempty"`
	Playlists []Playlist  `json:"playlists,omitempty"`
	Cursor    string      `json:"cursor,omitempty"`
	More      bool        `json:"more,omitempty"`
}

// Playlist is one of the shows a page groups its videos into.
//
// It is on the videos tab beside the grid, and the videos in it are not in the
// grid, so a reader that took the grid alone would miss twelve of NASA's
// twenty-one videos and never know it. The videos themselves go in Videos with
// the rest, and this records which show they came from.
type Playlist struct {
	ID          string   `json:"id"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Count       int      `json:"count,omitempty"`
	VideoIDs    []string `json:"video_ids,omitempty"`
}

// Len is how many items came back, whichever kind they are.
func (s Section) Len() int { return len(s.Photos) + len(s.Events) + len(s.Videos) }

// parseSection reads whichever section operation the page shipped.
func parseSection(docs map[string]*Document, owner Ref) Section {
	if d := docs["CometProfilePlusVideosRootQuery"]; d != nil {
		return parseVideoSection(d.Data, owner)
	}
	if d := docs["ProfileCometTopAppSectionQuery"]; d != nil {
		return parseAppSection(d.Data, owner)
	}
	return Section{}
}

// parseAppSection reads the photo and event tabs.
func parseAppSection(data any, owner Ref) Section {
	s := Section{Owner: owner}
	s.addSurface(surfaceComet)
	node := digMap(data, "node")
	s.Name = digStr(node, "name")
	s.URL = canonURL(digStr(node, "url"))
	switch digStr(node, "section_type") {
	case "PHOTOS":
		s.Kind = "photos"
	case "EVENTS":
		s.Kind = "events"
	default:
		s.Kind = "section"
	}
	for _, items := range sectionCollections(data) {
		for _, e := range digMaps(items, "edges") {
			n := digMap(e, "node")
			if n == nil {
				continue
			}
			switch s.Kind {
			case "events":
				if c := parseEventItem(n); c.ID != "" {
					s.Events = append(s.Events, c)
				}
			default:
				if p := parsePhotoItem(n, owner); p.ID != "" {
					s.Photos = append(s.Photos, p)
				}
			}
		}
		// Several collections can answer in one section: an events tab has
		// Upcoming and Past. The cursor of the last one that has a next page is
		// the one worth spending, and the rest are exhausted.
		if digBool(items, "page_info", "has_next_page") {
			s.Cursor = digStr(items, "page_info", "end_cursor")
			s.More = true
		}
	}
	return s
}

// sectionCollections finds the item connections in a section document.
//
// A page ships them under the section it rendered. A replayed page of the same
// section arrives rooted at the collection instead, so the fallback looks the
// connection up by name rather than assuming either shape.
func sectionCollections(data any) []map[string]any {
	var out []map[string]any
	for _, coll := range digMaps(data, "node", "all_collections", "nodes") {
		if items := digMap(coll, "style_renderer", "collection", "pageItems"); items != nil {
			out = append(out, items)
		}
	}
	if len(out) == 0 {
		if m := findKey(data, "pageItems"); m != nil {
			if items := digMap(m, "pageItems"); items != nil {
				out = append(out, items)
			}
		}
	}
	return out
}

// parsePhotoItem reads one thumbnail off the photo tab.
//
// It is a thin Photo on purpose. The tab carries the id, the alt text and two
// sizes of the image and nothing else: no caption, no counts, no album. `fb
// photo` on the id is what fills the rest in, at one request each, and a record
// that pretended otherwise would read as a photo with no engagement rather than
// a photo nobody asked about yet.
func parsePhotoItem(n map[string]any, owner Ref) Photo {
	node := digMap(n, "node")
	p := Photo{
		ID:    digStr(node, "id"),
		URL:   canonURL(firstStr(n, "url")),
		Owner: owner,
	}
	p.Image = parseImage(digMap(node, "viewer_image"))
	if p.Image.Empty() {
		p.Image = parseImage(digMap(n, "image"))
	}
	p.Image.Alt = digStr(node, "accessibility_caption")
	p.addSurface(surfaceComet)
	return p
}

// parseEventItem reads one card off the events tab.
//
// The card is split across two nodes and neither half is enough: the event node
// has the name, the place and the host, and the actions renderer beside it has
// the start timestamp and the online and past flags. Checked July 2026, and
// worth writing down: these cards do not carry the RSVP counts. The suggestion
// cards on an event permalink do, which is what doc 03 section 8 describes, and
// the two are different surfaces wearing the same shape.
func parseEventItem(n map[string]any) EventCard {
	c := parseEventCard(digMap(n, "node"))
	ev := digMap(n, "actions_renderer", "event")
	if c.ID == "" {
		c.ID = digStr(ev, "id")
	}
	if c.Start.IsZero() {
		c.Start = digTime(ev, "start_timestamp")
	}
	if !c.IsOnline {
		c.IsOnline = digBool(ev, "is_online_or_detected_online")
	}
	if !c.IsPast {
		c.IsPast = digBool(ev, "is_past")
	}
	if c.Kind == "" {
		c.Kind = digStr(ev, "event_kind")
	}
	if c.URL == "" {
		c.URL = canonURL(firstStr(n, "url"))
	}
	if c.Image.Empty() {
		c.Image = parseImage(digMap(n, "image"))
	}
	return c
}

// parseVideoSection reads the videos tab: the grid, and the shows beside it.
func parseVideoSection(data any, owner Ref) Section {
	s := Section{Kind: "videos", Owner: owner}
	s.addSurface(surfaceComet)
	page := digMap(data, "page")
	if page == nil {
		page = digMap(data, "node")
	}
	conn := digMap(page, "all_videos")
	if conn == nil {
		if m := findKey(data, "all_videos"); m != nil {
			conn = digMap(m, "all_videos")
		}
	}
	seen := map[string]bool{}
	add := func(n map[string]any) string {
		v := parseVideoItem(n)
		if v.ID == "" || seen[v.ID] {
			return v.ID
		}
		seen[v.ID] = true
		s.Videos = append(s.Videos, v)
		return v.ID
	}
	for _, e := range digMaps(conn, "edges") {
		add(digMap(e, "node"))
	}
	s.Cursor = digStr(conn, "page_info", "end_cursor")
	s.More = digBool(conn, "page_info", "has_next_page")
	for _, e := range digMaps(page, "page_video", "shows", "edges") {
		vl := digMap(e, "node", "channel_tab_series_card_renderer", "videolist")
		p := Playlist{
			ID:          digStr(vl, "id"),
			Title:       digStr(vl, "video_list_title", "text"),
			Description: digStr(vl, "video_list_description", "text"),
			Count:       digInt(vl, "series_videos_count"),
		}
		// A show is either a flat series or split into seasons, and the two
		// spell the same thing differently.
		lists := []map[string]any{digMap(vl, "series_videos")}
		for _, season := range digMaps(vl, "show_seasons", "nodes") {
			lists = append(lists, digMap(season, "video_list_view_model", "video_list_videos"))
		}
		for _, l := range lists {
			for _, ve := range digMaps(l, "edges") {
				if id := add(digMap(ve, "node")); id != "" {
					p.VideoIDs = append(p.VideoIDs, id)
				}
			}
		}
		if p.Count == 0 {
			p.Count = len(p.VideoIDs)
		}
		if p.ID != "" {
			s.Playlists = append(s.Playlists, p)
		}
	}
	return s
}

// parseVideoItem reads one video off the videos tab.
//
// This is the richest list item Facebook serves signed out. It has the media
// URL, the dimensions, the title, the whole message with its entity ranges, both
// play counts and the reactions broken down by type, so `fb videos nasa`
// produces nine near-complete video records from one page fetch.
//
// What it does not have is the comment and share counts: the feedback on a tab
// item carries reactions and nothing else. `fb video` on the id fills those in.
// Old show episodes come back with playable_url null too, so a video with no
// media URL here is Facebook declining rather than the parser missing.
func parseVideoItem(n map[string]any) Video {
	vid := digMap(n, "channel_tab_thumbnail_renderer", "video")
	if vid == nil {
		vid = n
	}
	v := Video{
		Kind:      "video",
		ID:        firstStr(n, "id"),
		URL:       canonURL(digStr(vid, "canonical_uri_with_fallback")),
		CreatedAt: digTime(vid, "publish_time"),
		Title:     digStr(vid, "savable_title", "text"),
		Message:   parseText(digMap(vid, "creation_story", "message")),
		Thumbnail: parseImage(digMap(vid, "image")),
		Owner:     parseRef(digMap(vid, "owner")),
	}
	if v.ID == "" {
		v.ID = digStr(vid, "id")
	}
	v.PostID = digStr(vid, "container_story", "post_id")
	if v.PostID == "" {
		// The tab ships the container story as its base64 key and nothing else,
		// and the key has the post id in it. This is the one place in the parsers
		// where decoding a key saves a request.
		v.PostID = fbid.Parse(digStr(vid, "container_story", "id")).PostID
	}
	if target := digMap(vid, "creation_story", "attachments", "0", "target"); target != nil {
		v.Width = digInt(target, "width")
		v.Height = digInt(target, "height")
		v.SDURL = digStr(target, "playable_url")
		if v.Title == "" {
			v.Title = digStr(target, "video_title", "text")
		}
		if v.Thumbnail.Empty() {
			v.Thumbnail = parseImage(digMap(target, "preferred_thumbnail", "image"))
		}
	}
	v.Counts = feedbackCounts(digMap(vid, "feedback"))
	// Facebook counts these separately and means different things by them:
	// play_count is every play of the video anywhere, post_play_count is the
	// plays on this post. Folding them together would lose that.
	v.Counts.Plays = digInt(vid, "play_count")
	if n := digInt(vid, "post_play_count"); n > 0 {
		v.Counts.Views = n
	}
	v.addSurface(surfaceComet)
	return v
}
