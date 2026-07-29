package fb

// photo.go parses a photo permalink.
//
// The photo route is the richest single request in the tool. One fetch of
// /photo/?fbid= returns the image, its alt text, its owner, the whole post it
// belongs to with the post's own reactions and comments, the album it is in,
// and the next photo in that album. That last one is what makes an album walk
// possible without a paging query: follow nextMedia until it repeats.

// parsePhoto builds a photo from the operations a photo permalink ships.
func parsePhoto(docs map[string]*Document) Photo {
	var p Photo
	d := docs["CometPhotoRootContentQuery"]
	if d == nil {
		return p
	}
	p.addSurface(surfaceComet)
	m := digMap(d.Data, "currMedia")
	if m == nil {
		return p
	}
	p.ID = digStr(m, "id")
	p.URL = canonURL(digStr(m, "url"))
	p.Image = parseImage(digMap(m, "image"))
	p.Image.Alt = digStr(m, "accessibility_caption")
	if o := digMap(m, "owner"); o != nil {
		p.Owner = parseRef(o)
	}
	p.AlbumID = digStr(m, "default_mediaset", "id")
	p.AlbumKind = digStr(m, "default_mediaset", "__typename")

	// The story is the same object post.go parses, so the caption, the counts
	// and the comments all come from one place rather than being read twice in
	// two shapes.
	story := firstOf(m, []string{"creation_story"}, []string{"container_story"})
	if story != nil {
		post := parseStory(story)
		p.Caption = post.Message
		if post.ID != "" {
			ref := Ref{Kind: "post", ID: post.ID, URL: post.URL}
			p.Post = &ref
		}
		if p.Owner.Empty() {
			p.Owner = post.Author
		}
	}
	p.Counts = feedbackCounts(digMap(m, "feedback"))
	p.Comments, _ = storyComments(m, p.Owner.ID)

	// nextMedia is the album walk. There is no prevMedia in a signed-out
	// render, so a walk goes forward only, and it terminates when an id repeats
	// rather than when the list runs out: the last photo points back at the
	// first.
	for _, n := range edges(m, "default_mediaset", "nextMedia") {
		ref := parseRef(n)
		if ref.Empty() {
			continue
		}
		ref.Kind = "photo"
		p.Next = &ref
		break
	}
	for _, n := range edges(m, "default_mediaset", "prevMedia") {
		ref := parseRef(n)
		if ref.Empty() {
			continue
		}
		ref.Kind = "photo"
		p.Prev = &ref
		break
	}
	p.Tags = photoTags(docs["CometPhotoTagLayerQuery"])
	return p
}

// photoTags reads the tag layer, which is a separate operation on the same page
// and is empty for most photos.
func photoTags(d *Document) []Ref {
	if d == nil {
		return nil
	}
	var out []Ref
	for _, tag := range digMaps(d.Data, "photo", "photo_tags") {
		if r := parseRef(digMap(tag, "node")); !r.Empty() {
			out = append(out, r)
		}
	}
	return out
}
