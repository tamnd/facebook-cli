package fb

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// engine.go is the thing the commands and the ops both call.
//
// Every read here is the same four steps: work out the URL from the reference,
// fetch the page, classify what came back, and hand the documents to a parser.
// The steps that vary are which operations the read needs and how it pages, and
// those are the only things the methods below say.
//
// Two rules the engine keeps so the parsers do not have to:
//
// Nothing is fetched twice for one record. A read that needs two pages, like a
// video with its transcript, merges the documents and parses once, so a field
// that appears on both surfaces is resolved in the parser where the preference
// is written down rather than by whichever fetch ran last.
//
// A surface that was tried and did not answer goes on the record as a miss. A
// thin record from a refused operation and a thin record from an empty page are
// different facts, and a reader who cannot tell them apart will believe the
// wrong one.

// Engine reads Facebook.
type Engine struct {
	cfg     Config
	c       *Client
	tierCap int
	pin     string
}

// NewEngine builds an engine from a config.
func NewEngine(cfg Config) (*Engine, error) {
	capTier, pin, err := parseTier(cfg.Tier, cfg.Cookies != "")
	if err != nil {
		return nil, err
	}
	if capTier == 0 {
		// --tier 0 with cookies in the file means read as if there were none,
		// so the cookies are dropped rather than merely not mentioned.
		cfg.Cookies = ""
	}
	c, err := cfg.client()
	if err != nil {
		return nil, err
	}
	return &Engine{cfg: cfg, c: c, tierCap: capTier, pin: pin}, nil
}

// Client exposes the HTTP client for the commands that need it directly, such
// as a media download.
func (e *Engine) Client() *Client { return e.c }

// Tier is the tier this engine is reading at.
func (e *Engine) Tier() int { return e.c.tier() }

// allow reports whether a surface may be used under --tier.
func (e *Engine) allow(surface string) bool {
	if e.pin != "" && surface != e.pin {
		return false
	}
	s, ok := SurfaceByID(surface)
	return ok && s.Tier <= e.tierCap
}

// limit resolves a per-command limit against the global one.
func (e *Engine) limit(n, fallback int) int {
	switch {
	case n > 0:
		return n
	case e.cfg.Limit > 0:
		return e.cfg.Limit
	default:
		return fallback
	}
}

// get fetches a page and classifies it.
func (e *Engine) get(ctx context.Context, url, what string, wanted ...string) (*Page, error) {
	p, err := e.c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := p.classify(what, wanted...); err != nil {
		return nil, err
	}
	return p, nil
}

// stamp records the tier, the surface and the time on a record.
func (e *Engine) stamp(env *Envelope, pages ...*Page) {
	env.Tier = e.c.tier()
	for _, p := range pages {
		if p == nil {
			continue
		}
		env.addSource(p.URL)
		if p.FinalURL != "" && p.FinalURL != p.URL {
			env.addSource(p.FinalURL)
		}
	}
	env.FetchedAt = time.Now().UTC()
}

// merge folds one page's documents into a set, first writer winning, so the
// route a read prefers is fetched first and stays preferred.
func merge(into map[string]*Document, from *Page) {
	if from == nil {
		return
	}
	for op, d := range from.Docs {
		if _, seen := into[op]; !seen {
			into[op] = d
		}
	}
}

// preloadOf finds an operation descriptor on a page.
func preloadOf(p *Page, op string) (Preload, bool) {
	for _, pre := range p.Preloads {
		if pre.Op == op {
			return pre, true
		}
	}
	return Preload{}, false
}

// cursorKeys and countKeys are the variables a paged operation spells its
// position with. Which pair an operation uses is a property of that operation,
// so both spellings are looked for and only the ones the descriptor already had
// are changed.
var (
	cursorKeys = []string{"cursor", "after"}
	countKeys  = []string{"count", "first"}
)

func hasAny(pre Preload, keys []string) bool {
	for _, k := range keys {
		if _, ok := pre.Variables[k]; ok {
			return true
		}
	}
	return false
}

// advance replays an operation with the cursor moved on.
//
// This is the whole paging surface, and it holds to the rule from doc 01
// section 3: only an operation the page itself shipped, with the variables the
// page itself sent, changing nothing but the cursor and the count.
func (e *Engine) advance(ctx context.Context, p *Page, pre Preload, cursor string, count int) (*Document, error) {
	if !e.allow(surfaceGraphQL) && !e.allow(surfaceSession) {
		return nil, unsupported(pre.Op, "paging needs the GraphQL surface, which this --tier does not allow")
	}
	vars := map[string]any{}
	for _, k := range cursorKeys {
		if _, ok := pre.Variables[k]; ok {
			vars[k] = cursor
		}
	}
	if len(vars) == 0 {
		return nil, unsupported(pre.Op, "its variables carry no cursor, so there is no next page to ask for")
	}
	if count > 0 {
		for _, k := range countKeys {
			if _, ok := pre.Variables[k]; ok {
				vars[k] = count
			}
		}
	}
	return e.c.Replay(ctx, p, pre, vars)
}

// ProfileOptions are the flags on `fb page`.
type ProfileOptions struct {
	About   bool
	NoPosts bool
	Tab     string
}

// Profile reads a Page or a person.
func (e *Engine) Profile(ctx context.Context, ref string, opt ProfileOptions) (Profile, error) {
	url := profileURL(ref)
	if opt.Tab != "" {
		url = tabURL(ref, opt.Tab)
	}
	p, err := e.get(ctx, url, "profile "+ref, "ProfileCometHeaderQuery", "ProfilePlusCometLoggedOutRootQuery")
	if err != nil {
		return Profile{}, err
	}
	docs := map[string]*Document{}
	if e.allow(surfaceComet) || e.allow(surfaceSession) {
		merge(docs, p)
	}
	pages := []*Page{p}

	// The About page is a second fetch and it is the only place the category
	// list, the address and the long contact block are complete, so --about is
	// a real read rather than a rendering flag.
	aboutMissed := ""
	if opt.About {
		a, err := e.get(ctx, tabURL(ref, "about"), "the About page of "+ref, "ProfileCometAboutAppSectionQuery")
		if err != nil {
			aboutMissed = err.Error()
		} else {
			merge(docs, a)
			pages = append(pages, a)
		}
	}
	out := parseProfile(docs)
	if opt.NoPosts {
		out.Posts, out.PostsCursor, out.PostsTruncated = nil, "", false
	}
	if e.allow(surfaceOpenGraph) {
		applyProfileHead(&out, p.Head)
	}
	if out.ID == "" && out.Name == "" {
		return out, notFound("profile "+ref, "the page shipped "+strings.Join(p.ops(), ", ")+" and none of them named a profile")
	}
	if aboutMissed != "" {
		out.miss(surfaceComet, "the About page did not answer, so the category list and the contact block may be short: "+aboutMissed)
	}
	e.stamp(&out.Envelope, pages...)
	return out, nil
}

// Feed reads a profile's timeline as deep as it is allowed to go.
//
// Signed out this is one post and a cursor that cannot be spent: replaying
// ProfileCometTimelineFeedQuery answers 1675004, which is a refusal and not a
// throttle. The record says so through a miss rather than looking like a
// profile that has posted once.
func (e *Engine) Feed(ctx context.Context, ref string, limit int) (Profile, error) {
	limit = e.limit(limit, 25)
	p, err := e.get(ctx, profileURL(ref), "the feed of "+ref, "ProfileCometTimelineFeedQuery", "ProfileCometHeaderQuery")
	if err != nil {
		return Profile{}, err
	}
	out := parseProfile(p.Docs)
	applyProfileHead(&out, p.Head)
	e.stamp(&out.Envelope, p)

	pre, ok := preloadOf(p, "ProfileCometTimelineFeedQuery")
	for len(out.Posts) < limit && out.PostsCursor != "" && ok {
		d, err := e.advance(ctx, p, pre, out.PostsCursor, limit-len(out.Posts))
		if err != nil {
			out.miss(surfaceGraphQL, "the timeline query is not replayable here: "+err.Error())
			break
		}
		before := len(out.Posts)
		appendFeed(&out, d.Data)
		if len(out.Posts) == before {
			break
		}
	}
	if len(out.Posts) > limit {
		out.Posts = out.Posts[:limit]
	}
	return out, nil
}

// appendFeed adds a replayed page of timeline units to a profile.
func appendFeed(p *Profile, data any) {
	units := digMap(data, "user", "timeline_list_feed_units")
	if units == nil {
		if m := findKey(data, "timeline_list_feed_units"); m != nil {
			units = digMap(m, "timeline_list_feed_units")
		}
	}
	if units == nil {
		p.PostsCursor = ""
		return
	}
	seen := map[string]bool{}
	for _, post := range p.Posts {
		seen[post.ID] = true
	}
	for _, n := range edges(units) {
		post := parseStory(n)
		if post.ID == "" || seen[post.ID] {
			continue
		}
		seen[post.ID] = true
		p.Posts = append(p.Posts, post)
	}
	p.PostsCursor = digStr(units, "page_info", "end_cursor")
	p.PostsTruncated = digBool(units, "page_info", "has_next_page")
}

// Resolve spends one request to fill in what reading the string cannot.
//
// Three references are worth a request, and they are the three where taking the
// string at face value stores something that will be wrong later.
//
// A handle is an alias. It names a profile until its owner renames it, and then
// it names somebody else or nothing, so a graph keyed on a handle rots without
// saying so. The numeric id behind it is the key, and one page fetch has it.
//
// A pfbid is per-render. The permalink you were given works, and the token in it
// is not the post's identity and will not match the token in the next render of
// the same post. The numeric story id arrives with the fetch.
//
// A share link is a redirect and nothing else. There is no way to know what
// fb.watch/abc123 points at except to follow it.
//
// Everything else already carries a numeric id, so Resolve hands it straight
// back rather than spending a request to confirm what parsing established.
func (e *Engine) Resolve(ctx context.Context, ref string) (fbid.Ref, error) {
	r := fbid.Parse(ref)
	switch {
	case r.Kind == fbid.KindHandle, (r.Kind == fbid.KindProfile || r.Kind == fbid.KindPage) && !numeric(r.ID):
		p, err := e.get(ctx, profileURL(ref), "profile "+ref, "ProfileCometHeaderQuery", "ProfilePlusCometLoggedOutRootQuery")
		if err != nil {
			return r, err
		}
		out := parseProfile(p.Docs)
		if out.ID == "" {
			applyProfileHead(&out, p.Head)
		}
		if out.ID == "" {
			return r, notFound("profile "+ref, "the page named no numeric id, so there is nothing to resolve to")
		}
		r.ID, r.Kind = out.ID, fbid.KindProfile
		if out.Handle != "" {
			r.Handle = out.Handle
		}
		if out.URL != "" {
			r.URL = out.URL
		}
		r.Opaque = false
		r.Note = "resolved from the handle: this numeric id is the one to key on"
		return r, nil

	case r.Kind == fbid.KindShare:
		// The redirect is the answer, so the cheapest correct thing is to follow
		// it and re-read where it landed.
		p, err := e.c.Get(ctx, r.URL)
		if err != nil {
			return r, err
		}
		if p.FinalURL == "" || p.FinalURL == r.URL {
			return r, notFound("share link "+ref, "it did not redirect anywhere")
		}
		out := fbid.Parse(canonURL(p.FinalURL))
		out.Input = ref
		out.Note = "resolved by following the redirect from " + r.URL
		return out, nil

	case r.Opaque || r.Kind == fbid.KindPost && !numeric(r.PostID):
		if r.URL == "" {
			return r, usage("%s is a pfbid on its own, and Facebook has no route that takes one without its author: pass the permalink, or the id with --author", ref)
		}
		p, err := e.get(ctx, r.URL, "post "+r.ID)
		if err != nil {
			return r, err
		}
		out := parsePost(p.Docs)
		if out.ID == "" {
			return r, notFound("post "+r.ID, "the permalink carried no story")
		}
		r.ID, r.PostID, r.Opaque = out.ID, out.ID, false
		if out.StoryID != "" {
			r.Note = "resolved from the pfbid: story key " + out.StoryID
		} else {
			r.Note = "resolved from the pfbid: this numeric id is the one to key on"
		}
		if !out.Author.Empty() && out.Author.ID != "" && numeric(out.Author.ID) {
			r.AuthorID = out.Author.ID
		}
		return r, nil
	}
	if r.Kind == fbid.KindUnknown {
		return r, noResults("fb cannot tell what %s is, so there is nothing to resolve", ref)
	}
	r.Note = "already a numeric id: nothing to resolve, and no request was made"
	return r, nil
}

// numeric reports whether a string is all digits, which is what a real Facebook
// id looks like and what a pfbid and a handle do not.
func numeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Post reads a post permalink.
//
// author is the --author flag, and it is not optional for a bare post id:
// Facebook has no route that takes one alone.
func (e *Engine) Post(ctx context.Context, ref, author string) (Post, error) {
	r, err := fbid.Post(ref, author)
	if err != nil {
		return Post{}, usage("%s", err.Error())
	}
	p, err := e.get(ctx, r.URL, "post "+r.ID)
	if err != nil {
		return Post{}, err
	}
	out := parsePost(p.Docs)
	if out.ID == "" {
		return Post{}, notFound("post "+r.ID, "the permalink carried no story, which is what a deleted or private post looks like")
	}
	e.stamp(&out.Envelope, p)
	return out, nil
}

// Comments reads a post's comments, paging where a cursor can be spent.
func (e *Engine) Comments(ctx context.Context, ref, author string, limit int) (Post, error) {
	limit = e.limit(limit, 20)
	r, err := fbid.Post(ref, author)
	if err != nil {
		return Post{}, usage("%s", err.Error())
	}
	p, err := e.get(ctx, r.URL, "post "+r.ID)
	if err != nil {
		return Post{}, err
	}
	out := parsePost(p.Docs)
	if out.ID == "" {
		return Post{}, notFound("post "+r.ID, "the permalink carried no story")
	}
	e.stamp(&out.Envelope, p)

	pre, ok := commentPreload(p)
	for len(out.Comments) < limit && out.CommentsCursor != "" && ok {
		d, err := e.advance(ctx, p, pre, out.CommentsCursor, limit-len(out.Comments))
		if err != nil {
			out.miss(surfaceGraphQL, "the comment query is not replayable here: "+err.Error())
			break
		}
		before := len(out.Comments)
		appendComments(&out, d.Data)
		if len(out.Comments) == before {
			break
		}
	}
	if len(out.Comments) > limit {
		out.Comments = out.Comments[:limit]
	}
	return out, nil
}

// commentPreload finds the operation a permalink pages its comments with.
//
// It is found on the page rather than named here because the operation differs
// by route and by build, and a hardcoded name would be a doc id in disguise.
func commentPreload(p *Page) (Preload, bool) {
	for _, pre := range p.Preloads {
		if !strings.Contains(pre.Op, "Comment") && !strings.Contains(pre.Op, "UFI") {
			continue
		}
		if hasAny(pre, cursorKeys) {
			return pre, true
		}
	}
	return Preload{}, false
}

// appendComments adds a replayed page of comments to a post.
func appendComments(p *Post, data any) {
	node := findKey(data, "comment_rendering_instance_for_feed_location")
	if node == nil {
		node = digMap(data, "feedback")
	}
	if node == nil {
		node = digMap(data)
	}
	more, cursor := storyComments(node, p.Author.ID)
	seen := map[string]bool{}
	for _, c := range p.Comments {
		seen[c.ID] = true
	}
	for _, c := range more {
		if c.ID == "" || seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		p.Comments = append(p.Comments, c)
	}
	p.CommentsCursor = cursor
}

// Photo reads a photo permalink.
func (e *Engine) Photo(ctx context.Context, ref string) (Photo, error) {
	r, err := fbid.Photo(ref)
	if err != nil {
		return Photo{}, usage("%s", err.Error())
	}
	p, err := e.get(ctx, r.URL, "photo "+r.ID, "CometPhotoRootContentQuery")
	if err != nil {
		return Photo{}, err
	}
	out := parsePhoto(p.Docs)
	if out.ID == "" {
		return Photo{}, notFound("photo "+r.ID, "the permalink carried no media")
	}
	e.stamp(&out.Envelope, p)
	return out, nil
}

// Photos reads a profile's photo tab.
func (e *Engine) Photos(ctx context.Context, ref string, limit int) (Section, error) {
	return e.section(ctx, ref, "photos", "ProfileCometTopAppSectionQuery", e.limit(limit, 8))
}

// Events reads a profile's events tab.
func (e *Engine) Events(ctx context.Context, ref string, limit int) (Section, error) {
	return e.section(ctx, ref, "events", "ProfileCometTopAppSectionQuery", e.limit(limit, 8))
}

// Videos reads a profile's video tab.
func (e *Engine) Videos(ctx context.Context, ref string, limit int) (Section, error) {
	return e.section(ctx, ref, "videos", "CometProfilePlusVideosRootQuery", e.limit(limit, 21))
}

// section reads one profile tab and walks its cursor.
func (e *Engine) section(ctx context.Context, ref, tab, op string, limit int) (Section, error) {
	p, err := e.get(ctx, tabURL(ref, tab), "the "+tab+" of "+ref, op)
	if err != nil {
		return Section{}, err
	}
	out := parseSection(p.Docs, Ref{Kind: "profile", Handle: handleOfRef(ref), ID: numericRef(ref)})
	if out.Kind == "" {
		out.Kind = tab
	}
	e.stamp(&out.Envelope, p)
	if out.Len() == 0 {
		return out, noResults("%s has nothing on its %s tab, or nothing it shows signed out", ref, tab)
	}

	pre, ok := preloadOf(p, op)
	for out.Len() < limit && out.More && out.Cursor != "" && ok {
		d, err := e.advance(ctx, p, pre, out.Cursor, limit-out.Len())
		if err != nil {
			out.miss(surfaceGraphQL, "the "+tab+" query is not replayable here: "+err.Error())
			break
		}
		before := out.Len()
		appendSection(&out, d.Data)
		if out.Len() == before {
			break
		}
	}
	trimSection(&out, limit)
	return out, nil
}

// appendSection folds a replayed page of a tab into the section.
func appendSection(s *Section, data any) {
	var next Section
	if s.Kind == "videos" {
		next = parseVideoSection(data, s.Owner)
	} else {
		next = parseAppSection(data, s.Owner)
		next.Kind = s.Kind
	}
	seen := map[string]bool{}
	for _, v := range s.Videos {
		seen[v.ID] = true
	}
	for _, v := range next.Videos {
		if v.ID != "" && !seen[v.ID] {
			seen[v.ID] = true
			s.Videos = append(s.Videos, v)
		}
	}
	for _, ph := range next.Photos {
		if ph.ID != "" && !seen[ph.ID] {
			seen[ph.ID] = true
			s.Photos = append(s.Photos, ph)
		}
	}
	for _, ev := range next.Events {
		if ev.ID != "" && !seen[ev.ID] {
			seen[ev.ID] = true
			s.Events = append(s.Events, ev)
		}
	}
	s.Playlists = append(s.Playlists, next.Playlists...)
	s.Cursor, s.More = next.Cursor, next.More
}

// trimSection cuts a section to the limit asked for, whichever list it filled.
func trimSection(s *Section, limit int) {
	if limit <= 0 || s.Len() <= limit {
		return
	}
	switch {
	case len(s.Photos) > limit:
		s.Photos = s.Photos[:limit]
	case len(s.Events) > limit:
		s.Events = s.Events[:limit]
	case len(s.Videos) > limit:
		s.Videos = s.Videos[:limit]
	}
	s.More = true
}

// Video reads a video or a reel.
//
// The reel route is read first because it is the generous one: it carries the
// media URL, the dimensions, the captions and the music, where /watch signed
// out carries the id, the title, the owner and the counts and nothing else.
// /watch is still fetched when the transcript is wanted, because that is the
// only surface that has it.
func (e *Engine) Video(ctx context.Context, ref string, transcript bool) (Video, error) {
	r, err := fbid.Video(ref)
	if err != nil {
		return Video{}, usage("%s", err.Error())
	}
	docs := map[string]*Document{}
	var pages []*Page
	var firstErr error

	reel, err := e.get(ctx, pageURL("/reel/"+r.ID), "video "+r.ID, "FBReelsRootWithEntrypointQuery")
	if err == nil {
		merge(docs, reel)
		pages = append(pages, reel)
	} else {
		firstErr = err
	}
	if len(docs) == 0 || transcript {
		watch, err := e.get(ctx, pageURL("/watch/", "v", r.ID), "video "+r.ID,
			"CometVideoHomeNewPermalinkHeroUnitQuery", "CometVideoHomeLOEVideoPermalinkAuxiliaryRootQuery")
		if err == nil {
			merge(docs, watch)
			pages = append(pages, watch)
		} else if firstErr == nil {
			firstErr = err
		}
	}
	if len(docs) == 0 {
		if firstErr != nil {
			return Video{}, firstErr
		}
		return Video{}, notFound("video "+r.ID, "neither the reel route nor the watch route carried it")
	}
	out := parseVideo(docs)
	if out.ID == "" {
		out.ID = r.ID
	}
	if transcript && out.Transcript == "" {
		out.miss(surfaceComet, "the watch route shipped no transcript for this video")
	}
	e.stamp(&out.Envelope, pages...)
	return out, nil
}

// groupWall adds what is worth knowing when a group read hits the log-in page.
//
// Spec 3004 captured groups at tier 0 and they do read in full: a fresh
// signed-out client gets the header, the description, the member count, the tabs
// and the pinned post. What the spec did not capture is that Facebook gives up
// on the group route quickly. Roughly thirty requests to /groups/ turned a route
// that had just answered into one that bounced to /login every time, on every
// group id, and it had not lifted nine minutes later, while profile reads over
// the same connection carried on as normal.
//
// So the group route is honest at tier 0 and it is not dependable there, and
// somebody who gets this error deserves to know which of the two they are
// looking at rather than assuming they picked a private group.
func groupWall(what string, err error) error {
	var na *NeedAuthError
	if !errors.As(err, &na) {
		return err
	}
	return needAuth("Facebook answered every request for %s with the log-in page. It cuts a signed-out reader off from the group route after a few dozen requests, whatever the group, and it does not lift for a while: wait, or import a session with `fb auth import`", what)
}

// Group reads a group.
func (e *Engine) Group(ctx context.Context, ref string) (Group, error) {
	r, err := fbid.Group(ref)
	if err != nil {
		return Group{}, usage("%s", err.Error())
	}
	p, err := e.get(ctx, r.URL, "group "+r.ID, "CometGroupRootQuery", "GroupsCometDiscussionLayoutRootQuery")
	if err != nil {
		return Group{}, groupWall("group "+r.ID, err)
	}
	out := parseGroup(p.Docs)
	if out.ID == "" {
		out.ID = r.ID
	}
	if out.Name == "" {
		return out, notFound("group "+r.ID, "the page carried no group header, which is what a private group looks like signed out")
	}
	e.stamp(&out.Envelope, p)
	return out, nil
}

// GroupFeed reads a group's discussion as deep as asked.
//
// This is the deepest free read in the tool: the discussion query is not on the
// refused list, so the cursor can actually be spent signed out.
func (e *Engine) GroupFeed(ctx context.Context, ref string, limit int) (Group, error) {
	limit = e.limit(limit, 20)
	r, err := fbid.Group(ref)
	if err != nil {
		return Group{}, usage("%s", err.Error())
	}
	p, err := e.get(ctx, r.URL, "the feed of group "+r.ID, "CometGroupDiscussionRootSuccessQuery", "CometGroupRootQuery")
	if err != nil {
		return Group{}, groupWall("the feed of group "+r.ID, err)
	}
	out := parseGroup(p.Docs)
	if out.ID == "" {
		out.ID = r.ID
	}
	e.stamp(&out.Envelope, p)

	pre, ok := preloadOf(p, "CometGroupDiscussionRootSuccessQuery")
	if !ok {
		pre, ok = feedPreload(p)
	}
	for len(out.Posts) < limit && out.PostsCursor != "" && ok {
		d, err := e.advance(ctx, p, pre, out.PostsCursor, limit-len(out.Posts))
		if err != nil {
			out.miss(surfaceGraphQL, "the group feed query is not replayable here: "+err.Error())
			break
		}
		before := len(out.Posts)
		appendGroupFeed(&out, d.Data)
		if len(out.Posts) == before {
			break
		}
	}
	if len(out.Posts) > limit {
		out.Posts = out.Posts[:limit]
	}
	if len(out.Posts) == 0 {
		return out, noResults("group %s shows no posts signed out", r.ID)
	}
	return out, nil
}

// feedPreload finds an operation that pages a feed, for the builds where the
// discussion query is not the one that does the paging.
func feedPreload(p *Page) (Preload, bool) {
	for _, pre := range p.Preloads {
		if !strings.Contains(pre.Op, "Feed") && !strings.Contains(pre.Op, "Discussion") {
			continue
		}
		if hasAny(pre, cursorKeys) {
			return pre, true
		}
	}
	return Preload{}, false
}

// appendGroupFeed adds a replayed page of group stories.
func appendGroupFeed(g *Group, data any) {
	units := findKey(data, "group_feed")
	var conn map[string]any
	if units != nil {
		conn = digMap(units, "group_feed")
	}
	if conn == nil {
		if m := findKey(data, "edges"); m != nil {
			conn = m
		}
	}
	seen := map[string]bool{}
	for _, post := range g.Posts {
		seen[post.ID] = true
	}
	for _, n := range edges(conn) {
		post := parseStory(n)
		if post.ID == "" || seen[post.ID] {
			continue
		}
		seen[post.ID] = true
		g.Posts = append(g.Posts, post)
	}
	g.PostsCursor = digStr(conn, "page_info", "end_cursor")
}

// Event reads an event permalink.
func (e *Engine) Event(ctx context.Context, ref string) (Event, error) {
	r, err := fbid.Event(ref)
	if err != nil {
		return Event{}, usage("%s", err.Error())
	}
	p, err := e.get(ctx, r.URL, "event "+r.ID, "EventCometPermalinkHeaderQuery", "PublicEventCometAboutRootQuery")
	if err != nil {
		return Event{}, err
	}
	out := parseEvent(p.Docs, p.Head)
	if out.ID == "" {
		out.ID = r.ID
	}
	if out.Name == "" {
		return out, notFound("event "+r.ID, "the permalink carried no event")
	}
	e.stamp(&out.Envelope, p)
	return out, nil
}

// Discover reads the Pages directory.
//
// The index is readable and the letter pages are not: every one of them answers
// the security interstitial signed out. That is on the record as a miss rather
// than as an empty list, because an empty list reads as "Facebook has no pages
// starting with A".
func (e *Engine) Discover(ctx context.Context, letter string) (Directory, error) {
	url := pageURL("/directory/pages/")
	if letter != "" {
		url = pageURL("/directory/pages/" + strings.ToUpper(letter))
	}
	p, err := e.c.Get(ctx, url)
	if err != nil {
		return Directory{}, err
	}
	out := parseDirectory(url, p.HTML)
	e.stamp(&out.Envelope, p)
	if out.Blocked {
		return out, needAuth("Facebook blocked the directory page for %s: the letter pages need a session", strings.ToUpper(letter))
	}
	if len(out.Index) == 0 && len(out.Entries) == 0 {
		return out, noResults("the directory page carried no links")
	}
	return out, nil
}

// Search is Tier 1 only. Facebook answers a signed-out search with 404, so this
// refuses rather than fetching something it knows will not answer.
func (e *Engine) Search(ctx context.Context, query string) error {
	if e.c.tier() < 1 {
		return needAuth("search needs a session: Facebook answers a signed-out search with 404, so run fb auth import first")
	}
	return unsupported("search", "the signed-in search surface is not implemented yet")
}

// tabURL is the page for one section of a profile. A numeric id takes the sk
// parameter and a handle takes a path segment, and swapping them is a 404.
func tabURL(ref, tab string) string {
	ref = strings.TrimPrefix(ref, "@")
	if allDigits(ref) {
		return pageURL("/profile.php", "id", ref, "sk", tab)
	}
	return pageURL("/" + ref + "/" + tab)
}

// handleOfRef and numericRef split a reference into the half that names the
// owner of a section, so a photo read from a tab knows whose it is.
func handleOfRef(ref string) string {
	ref = strings.TrimPrefix(ref, "@")
	if allDigits(ref) {
		return ""
	}
	return ref
}

func numericRef(ref string) string {
	ref = strings.TrimPrefix(ref, "@")
	if allDigits(ref) {
		return ref
	}
	return ""
}
