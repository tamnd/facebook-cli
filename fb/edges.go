package fb

import (
	"encoding/base64"
	"strings"

	"github.com/tamnd/facebook-cli/pkg/graph"
)

// edges.go turns a record into claims.
//
// This is the point of the whole graph plane, and it is worth being precise
// about what it is not. It walks nothing and fetches nothing. Every claim here
// comes out of bytes that were already fetched for another reason, which is why
// one photo permalink is worth thirty-odd claims: the permalink ships the
// containing story, the album neighbours, the tag layer and the comments
// Facebook chose to include, each with its author.
//
// A profile that was mentioned in a post is a node with a name, a URL and a
// verified flag, and nobody spent a request on it. That is the difference
// between a crawler and a reader with a good memory.

// edgeCtx is what every claim off one read shares: where it came from.
type edgeCtx struct {
	source  string
	surface string
	tier    int
}

func ctxOf(env Envelope) edgeCtx {
	c := edgeCtx{tier: env.Tier}
	if len(env.Sources) > 0 {
		c.source = env.Sources[0]
	}
	if len(env.Surfaces) > 0 {
		c.surface = env.Surfaces[0]
	}
	return c
}

// add is the one place a claim is built, so every claim carries its provenance
// and none of them can forget to.
func (c edgeCtx) add(s *graph.Set, from, predicate, to, note string) {
	s.Add(graph.Edge{
		From: from, Predicate: predicate, To: to,
		Source: c.source, Surface: c.surface, Tier: c.tier, Note: note,
	})
}

// refURI is the URI for a ref, and "" for one that cannot have a stable name.
//
// A ref's Kind is the mapped __typename, so it is already the URI kind. The
// fallbacks matter more than they look: plenty of refs arrive with a URL and no
// id, and the numeric id is sitting in the URL.
func refURI(r Ref) string {
	if r.Opaque {
		return ""
	}
	if r.Kind == graph.KindExternal {
		return graph.External(r.URL)
	}
	if u := graph.URI(refKind(r), r.ID); u != "" {
		return u
	}
	// A ref with a permalink and no id: the id is in the URL often enough to be
	// worth the parse, and a claim recovered is a node nobody has to fetch.
	if r.URL != "" {
		if id := idFromURL(r.URL); id != "" {
			return graph.URI(refKind(r), id)
		}
	}
	return ""
}

// refKind falls back to profile for a ref that named no type. Facebook's actor
// nodes are overwhelmingly profiles, and a claim filed under the wrong kind is
// still recoverable where a claim not made at all is not.
func refKind(r Ref) string {
	if r.Kind != "" {
		return r.Kind
	}
	return graph.KindProfile
}

// note is the human-readable half of a claim: the name on the thing at the far
// end, so an edge to a profile nobody fetched still says who it is.
func note(r Ref) string {
	if r.Name != "" {
		return r.Name
	}
	return r.Handle
}

// ProfileEdges is what one profile read already knows.
func ProfileEdges(p Profile) []graph.Edge {
	var s graph.Set
	c := ctxOf(p.Envelope)
	self := graph.URI(graph.KindProfile, p.ID)
	if self == "" {
		self = graph.URI(graph.KindPage, p.ID)
	}
	if self != "" {
		// The delegate page is a second node and not the same node. Merging
		// them would lose the fact that the videos root query takes one of them
		// and the timeline query takes the other.
		c.add(&s, self, graph.DelegatesTo, graph.URI(graph.KindPage, p.DelegatePage), p.Name)
		coverEdges(&s, c, self, p.Cover, p.Avatar)
		for _, w := range p.Websites {
			c.add(&s, self, graph.LinksTo, graph.External(w), w)
		}
		for _, l := range p.Bio.Links {
			c.add(&s, self, graph.LinksTo, graph.External(l.URL), l.Display)
		}
		for _, m := range p.Bio.Mentions {
			c.add(&s, self, graph.Mentions, refURI(m), note(m))
		}
	}
	for _, post := range p.Posts {
		appendPostEdges(&s, c, post)
	}
	return s.Edges()
}

// PostEdges is what one post read already knows.
func PostEdges(p Post) []graph.Edge {
	var s graph.Set
	appendPostEdges(&s, ctxOf(p.Envelope), p)
	return s.Edges()
}

// appendPostEdges is the shared body, because a post arrives on its own, inside
// a profile, inside a group feed and inside a photo permalink, and the claims
// are the same claims every time.
func appendPostEdges(s *graph.Set, c edgeCtx, p Post) {
	self := graph.URI(graph.KindPost, postID(p))
	if self == "" {
		return
	}
	author := refURI(p.Author)
	c.add(s, author, graph.Authored, self, note(p.Author))
	c.add(s, author, graph.DelegatesTo, graph.URI(graph.KindPage, p.DelegatePage), note(p.Author))
	if p.Group != nil {
		c.add(s, self, graph.PostedIn, refURI(*p.Group), note(*p.Group))
	}
	for _, m := range p.Message.Mentions {
		c.add(s, self, graph.Mentions, refURI(m), note(m))
	}
	for _, l := range p.Message.Links {
		c.add(s, self, graph.LinksTo, graph.External(l.URL), l.Display)
	}
	attachmentEdges(s, c, self, p.Attachments)
	if p.SharedPost != nil {
		if shared := graph.URI(graph.KindPost, postID(*p.SharedPost)); shared != "" {
			c.add(s, self, graph.Shares, shared, note(p.SharedPost.Author))
		}
		appendPostEdges(s, c, *p.SharedPost)
	}
	for _, cm := range p.Comments {
		commentEdges(s, c, self, cm)
	}
}

// attachmentEdges walks a post's media, following the subattachment nesting an
// album post uses.
func attachmentEdges(s *graph.Set, c edgeCtx, post string, list []Attachment) {
	for _, a := range list {
		if a.Media != nil {
			c.add(s, post, graph.Attaches, refURI(*a.Media), note(*a.Media))
		}
		// A link card's destination is the claim, and it is on the attachment
		// rather than on the media node.
		if a.Kind == "link" || a.Kind == "share" {
			if u := firstNonEmpty(a.URL, a.Source); u != "" {
				c.add(s, post, graph.LinksTo, graph.External(u), a.Title)
			}
		}
		attachmentEdges(s, c, post, a.Subattachments)
	}
}

func commentEdges(s *graph.Set, c edgeCtx, post string, cm Comment) {
	self := graph.URI(graph.KindComment, commentID(cm.ID))
	if self == "" {
		return
	}
	c.add(s, self, graph.CommentsOn, post, "")
	c.add(s, refURI(cm.Author), graph.Commented, self, note(cm.Author))
	for _, m := range cm.Body.Mentions {
		c.add(s, self, graph.Mentions, refURI(m), note(m))
	}
	for _, l := range cm.Body.Links {
		c.add(s, self, graph.LinksTo, graph.External(l.URL), l.Display)
	}
}

// PhotoEdges is what one photo permalink already knows, which is more than any
// other single read in this tool.
func PhotoEdges(p Photo) []graph.Edge {
	var s graph.Set
	c := ctxOf(p.Envelope)
	self := graph.URI(graph.KindPhoto, p.ID)
	if self == "" {
		return nil
	}
	c.add(&s, refURI(p.Owner), graph.Owns, self, note(p.Owner))
	c.add(&s, self, graph.InAlbum, graph.URI(graph.KindAlbum, p.AlbumID), p.AlbumKind)
	if p.Post != nil {
		c.add(&s, refURI(*p.Post), graph.Attaches, self, note(*p.Post))
	}
	if p.Next != nil {
		c.add(&s, self, graph.NextInAlbum, refURI(*p.Next), "")
	}
	if p.Prev != nil {
		// Recorded forwards from the previous photo, so the chain reads one way
		// in the store however it was walked.
		c.add(&s, refURI(*p.Prev), graph.NextInAlbum, self, "")
	}
	for _, t := range p.Tags {
		c.add(&s, refURI(t), graph.TaggedIn, self, note(t))
	}
	for _, m := range p.Caption.Mentions {
		c.add(&s, self, graph.Mentions, refURI(m), note(m))
	}
	for _, l := range p.Caption.Links {
		c.add(&s, self, graph.LinksTo, graph.External(l.URL), l.Display)
	}
	for _, cm := range p.Comments {
		commentEdges(&s, c, self, cm)
	}
	return s.Edges()
}

// VideoEdges is what one video or reel read already knows.
func VideoEdges(v Video) []graph.Edge {
	var s graph.Set
	c := ctxOf(v.Envelope)
	self := graph.URI(graph.KindVideo, v.ID)
	if self == "" {
		return nil
	}
	c.add(&s, refURI(v.Owner), graph.Owns, self, note(v.Owner))
	if post := graph.URI(graph.KindPost, v.PostID); post != "" {
		c.add(&s, post, graph.Attaches, self, v.Title)
	}
	for _, m := range v.Message.Mentions {
		c.add(&s, self, graph.Mentions, refURI(m), note(m))
	}
	for _, l := range v.Message.Links {
		c.add(&s, self, graph.LinksTo, graph.External(l.URL), l.Display)
	}
	return s.Edges()
}

// GroupEdges is what one group read already knows, feed included when the feed
// was read.
func GroupEdges(g Group) []graph.Edge {
	var s graph.Set
	c := ctxOf(g.Envelope)
	self := graph.URI(graph.KindGroup, g.ID)
	if self == "" {
		return nil
	}
	coverEdges(&s, c, self, g.Cover, g.Avatar)
	for _, m := range g.Description.Mentions {
		c.add(&s, self, graph.Mentions, refURI(m), note(m))
	}
	for _, l := range g.Description.Links {
		c.add(&s, self, graph.LinksTo, graph.External(l.URL), l.Display)
	}
	for _, p := range g.Posts {
		appendPostEdges(&s, c, p)
		if post := graph.URI(graph.KindPost, postID(p)); post != "" {
			c.add(&s, post, graph.PostedIn, self, g.Name)
		}
	}
	return s.Edges()
}

// EventEdges is what one event read already knows.
func EventEdges(e Event) []graph.Edge {
	var s graph.Set
	c := ctxOf(e.Envelope)
	self := graph.URI(graph.KindEvent, e.ID)
	if self == "" {
		return nil
	}
	for _, h := range append([]Ref{e.Host}, e.Hosts...) {
		c.add(&s, refURI(h), graph.Hosts, self, note(h))
	}
	c.add(&s, graph.URI(graph.KindPage, e.HostPageID), graph.Hosts, self, "")
	c.add(&s, self, graph.AnnouncedBy, graph.URI(graph.KindPost, e.AnnouncedBy), "")
	if e.Place != nil {
		place := graph.URI(graph.KindPlace, e.Place.ID)
		c.add(&s, self, graph.LocatedAt, place, e.Place.Name)
		if e.Place.City != nil && place != "" {
			c.add(&s, place, graph.InCity, refURI(*e.Place.City), note(*e.Place.City))
		}
	}
	if e.OnlineURL != "" {
		c.add(&s, self, graph.LinksTo, graph.External(e.OnlineURL), e.OnlineKind)
	}
	for _, sg := range e.Suggested {
		c.add(&s, self, graph.Suggests, graph.URI(graph.KindEvent, sg.ID), sg.Name)
	}
	for _, m := range e.Description.Mentions {
		c.add(&s, self, graph.Mentions, refURI(m), note(m))
	}
	for _, l := range e.Description.Links {
		c.add(&s, self, graph.LinksTo, graph.External(l.URL), l.Display)
	}
	coverEdges(&s, c, self, e.Cover, Image{})
	return s.Edges()
}

// SectionEdges is what one tab read already knows: a profile owning everything
// on it, plus whatever each item carries.
func SectionEdges(sec Section) []graph.Edge {
	var s graph.Set
	c := ctxOf(sec.Envelope)
	owner := refURI(sec.Owner)
	for _, p := range sec.Photos {
		if u := graph.URI(graph.KindPhoto, p.ID); u != "" {
			c.add(&s, owner, graph.Owns, u, note(sec.Owner))
			c.add(&s, u, graph.InAlbum, graph.URI(graph.KindAlbum, p.AlbumID), p.AlbumKind)
		}
	}
	for _, v := range sec.Videos {
		if u := graph.URI(graph.KindVideo, v.ID); u != "" {
			c.add(&s, owner, graph.Owns, u, note(sec.Owner))
		}
	}
	for _, e := range sec.Events {
		if u := graph.URI(graph.KindEvent, e.ID); u != "" {
			c.add(&s, owner, graph.Hosts, u, e.Name)
			if e.Place != nil {
				c.add(&s, u, graph.LocatedAt, graph.URI(graph.KindPlace, e.Place.ID), e.Place.Name)
			}
		}
	}
	return s.Edges()
}

// coverEdges records a cover and an avatar as photos when they carry an id.
//
// Most of the time they do not: a cover arrives as a signed CDN URL with no
// photo id anywhere near it, and a URL is not a node. When the id is there it is
// in the URL, and a cover photo that can be named is a photo permalink worth
// reading.
func coverEdges(s *graph.Set, c edgeCtx, self string, cover, avatar Image) {
	for _, img := range [2]Image{cover, avatar} {
		if img.URI == "" {
			continue
		}
		if id := photoIDFromCDN(img.URI); id != "" {
			c.add(s, self, graph.Covers, graph.URI(graph.KindPhoto, id), img.Alt)
		}
	}
}

// commentID pulls the comment's own numeric id out of the node id Comet ships.
//
// A comment arrives as base64 of `comment:{post_id}_{comment_id}`, and the
// number after the underscore is the same one the permalink puts in its
// ?comment_id= query, so it is stable and it is a node. This is not the pfbid
// situation: a pfbid is signed and per-render, and this is a plain encoding of
// two ids that do not change.
func commentID(raw string) string {
	if isDigits(raw) {
		return raw
	}
	dec, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return ""
	}
	body, ok := strings.CutPrefix(string(dec), "comment:")
	if !ok {
		return ""
	}
	_, id, ok := strings.Cut(body, "_")
	if !ok || !isDigits(id) {
		return ""
	}
	return id
}

// postID prefers the numeric story id over whatever else is on the record.
func postID(p Post) string {
	if p.ID != "" {
		return p.ID
	}
	return p.StoryID
}

// idFromURL pulls a numeric id out of a facebook.com URL, which is where the id
// often is when the payload did not carry one separately.
func idFromURL(raw string) string {
	for _, key := range []string{"fbid=", "id=", "v=", "story_fbid="} {
		if i := strings.Index(raw, key); i >= 0 {
			rest := raw[i+len(key):]
			if j := strings.IndexAny(rest, "&#"); j >= 0 {
				rest = rest[:j]
			}
			if isDigits(rest) {
				return rest
			}
		}
	}
	// The last path segment, for /groups/123 and /events/123 and /reel/123.
	path := raw
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimSuffix(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		if seg := path[i+1:]; isDigits(seg) {
			return seg
		}
	}
	return ""
}

// photoIDFromCDN reads the photo id out of a scontent filename.
//
// A CDN filename is {something}_{photo_id}_{something}_n.jpg and the middle
// number is the photo's own fbid, which is how a cover photo gets a node at all.
// It is a guess with a shape rather than a documented format, so it only fires
// on the exact shape and stays quiet otherwise.
func photoIDFromCDN(raw string) string {
	path := raw
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		path = path[i+1:]
	}
	parts := strings.Split(strings.TrimSuffix(path, ".jpg"), "_")
	if len(parts) < 4 || !isDigits(parts[1]) || len(parts[1]) < 10 {
		return ""
	}
	return parts[1]
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
