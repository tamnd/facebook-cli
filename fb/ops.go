package fb

import (
	"context"
	"sort"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/facebook-cli/pkg/graph"
)

// ops.go is the read surface as operations: one registration each, served over
// HTTP by `fb serve` and to an agent by `fb mcp` (spec 3004 doc 05 section 5,
// doc 06 section 3).
//
// The command line is not built from these. It is hand-written in cli/, because
// a curated table beats a reflected dump of forty fields, and OpMeta.NoCLI is
// what lets both exist: the op serves, the hand-written command is what a
// person types.
//
// What is deliberately not here is the other half of that split. auth import
// handles session cookies and cache clear deletes files, and neither is a thing
// to put behind a network port, so they are commands and nothing else. An
// invariant test checks the route table has no write op in it.
//
// The handlers are thin on purpose. Every decision about what a surface says
// lives in the engine next to the data it decides about; what is left here is
// resolving a reference and making one call.

// OpOptions says how a caller wants the operations registered.
type OpOptions struct {
	// NoCLI keeps the operations off the command line, for a binary that ships
	// its own commands for the same reads and only wants the serve and MCP
	// surfaces filled in.
	NoCLI bool
}

// RegisterOps installs every read operation onto app.
func RegisterOps(app *kit.App, o OpOptions) {
	registerProfileOps(app, o)
	registerPostOps(app, o)
	registerMediaOps(app, o)
	registerGroupOps(app, o)
	registerEventOps(app, o)
	registerDirectoryOps(app, o)
	registerGraphOps(app, o)
}

// handle registers one operation, applying the caller's surface choice, so no
// registration below has to remember it.
func handle[In, Out any](app *kit.App, o OpOptions, m kit.OpMeta, fn func(context.Context, In, func(Out) error) error) {
	m.NoCLI = o.NoCLI
	kit.Handle(app, m, fn)
}

// --- inputs ---
//
// One struct per argument shape rather than one per operation. A dozen reads
// take a profile and nothing else, and a dozen near-identical structs would be
// a dozen places to get the help text wrong.

type profileRef struct {
	Ref    string  `kit:"arg" help:"handle, numeric id, or profile URL"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Engine *Engine `kit:"inject"`
}

type profileDetail struct {
	Ref     string  `kit:"arg" help:"handle, numeric id, or profile URL"`
	About   bool    `kit:"flag" help:"fetch the About page too"`
	NoPosts bool    `kit:"flag" help:"skip the timeline parse"`
	Tab     string  `kit:"flag" help:"read one tab instead of the main page"`
	Engine  *Engine `kit:"inject"`
}

// postRef carries the author because Facebook has no route that takes a post id
// alone: a bare id or a pfbid needs a profile to build a permalink from.
type postRef struct {
	Ref    string  `kit:"arg" help:"permalink, story key, pfbid, or post id with --author"`
	Author string  `kit:"flag" help:"the profile the post belongs to, for a bare post id or a pfbid"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Engine *Engine `kit:"inject"`
}

type videoRef struct {
	Ref        string  `kit:"arg" help:"video id, watch URL, or reel URL"`
	Transcript bool    `kit:"flag" help:"fetch the watch route for the transcript"`
	Engine     *Engine `kit:"inject"`
}

type groupRef struct {
	Ref    string  `kit:"arg" help:"group id, or slug with a session"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Engine *Engine `kit:"inject"`
}

type eventRef struct {
	Ref    string  `kit:"arg" help:"event id or permalink"`
	Engine *Engine `kit:"inject"`
}

type letterRef struct {
	Letter string  `kit:"arg" help:"one letter page of the directory (the index with none)"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Engine *Engine `kit:"inject"`
}

// --- profiles ---

func registerProfileOps(app *kit.App, o OpOptions) {
	handle(app, o, kit.OpMeta{Name: "page", Group: "read", Single: true,
		Summary: "Read a Page or profile whole", URIType: "profile", Resolver: true,
		Args: []kit.Arg{{Name: "ref", Help: "handle, numeric id, or profile URL"}}}, getPage)

	handle(app, o, kit.OpMeta{Name: "feed", Group: "read", List: true,
		Summary: "Read a profile's timeline (one post signed out)", URIType: "post",
		Args: []kit.Arg{{Name: "ref", Help: "handle, numeric id, or profile URL"}}}, listFeed)

	handle(app, o, kit.OpMeta{Name: "photos", Group: "read",
		Summary: "Read a profile's photo tab", URIType: "photo",
		Args: []kit.Arg{{Name: "ref", Help: "handle, numeric id, or profile URL"}}}, listPhotos)

	handle(app, o, kit.OpMeta{Name: "videos", Group: "read",
		Summary: "Read a profile's video tab, grid and shows together", URIType: "video",
		Args: []kit.Arg{{Name: "ref", Help: "handle, numeric id, or profile URL"}}}, listVideos)

	handle(app, o, kit.OpMeta{Name: "playlists", Group: "read",
		Summary: "Read the shows a profile groups its videos into", URIType: "playlist",
		Args: []kit.Arg{{Name: "ref", Help: "handle, numeric id, or profile URL"}}}, listPlaylists)

	handle(app, o, kit.OpMeta{Name: "events", Group: "read",
		Summary: "Read a profile's events tab", URIType: "event",
		Args: []kit.Arg{{Name: "ref", Help: "handle, numeric id, or profile URL"}}}, listProfileEvents)
}

func getPage(ctx context.Context, in profileDetail, emit func(*Profile) error) error {
	p, err := in.Engine.Profile(ctx, in.Ref, ProfileOptions{About: in.About, NoPosts: in.NoPosts, Tab: in.Tab})
	if err != nil {
		return MapErr(err)
	}
	return emit(&p)
}

// listFeed drops the note the CLI prints about Tier 0 serving one post. Over
// HTTP and MCP there is no stderr to say it on, and the envelope already
// carries the miss that explains the short answer.
func listFeed(ctx context.Context, in profileRef, emit func(*Post) error) error {
	p, err := in.Engine.Feed(ctx, in.Ref, in.Limit)
	if err != nil {
		return MapErr(err)
	}
	for i := range p.Posts {
		if err := emit(&p.Posts[i]); err != nil {
			return err
		}
	}
	return nil
}

func listPhotos(ctx context.Context, in profileRef, emit func(*Photo) error) error {
	s, err := in.Engine.Photos(ctx, in.Ref, in.Limit)
	if err != nil {
		return MapErr(err)
	}
	for i := range s.Photos {
		if err := emit(&s.Photos[i]); err != nil {
			return err
		}
	}
	return nil
}

func listVideos(ctx context.Context, in profileRef, emit func(*Video) error) error {
	s, err := in.Engine.Videos(ctx, in.Ref, in.Limit)
	if err != nil {
		return MapErr(err)
	}
	for i := range s.Videos {
		if err := emit(&s.Videos[i]); err != nil {
			return err
		}
	}
	return nil
}

func listPlaylists(ctx context.Context, in profileRef, emit func(*Playlist) error) error {
	s, err := in.Engine.Videos(ctx, in.Ref, in.Limit)
	if err != nil {
		return MapErr(err)
	}
	if len(s.Playlists) == 0 {
		return MapErr(noResults("%s has no shows beside its video grid", in.Ref))
	}
	for i := range s.Playlists {
		if err := emit(&s.Playlists[i]); err != nil {
			return err
		}
	}
	return nil
}

func listProfileEvents(ctx context.Context, in profileRef, emit func(*EventCard) error) error {
	s, err := in.Engine.Events(ctx, in.Ref, in.Limit)
	if err != nil {
		return MapErr(err)
	}
	for i := range s.Events {
		if err := emit(&s.Events[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- posts ---

func registerPostOps(app *kit.App, o OpOptions) {
	handle(app, o, kit.OpMeta{Name: "post", Group: "read", Single: true,
		Summary: "Read a post, with the comments the permalink ships", URIType: "post", Resolver: true,
		Args: []kit.Arg{{Name: "ref", Help: "permalink, story key, pfbid, or post id with --author"}}}, getPost)

	handle(app, o, kit.OpMeta{Name: "comments", Group: "read", List: true,
		Summary: "Read the comments on a post", URIType: "comment",
		Args: []kit.Arg{{Name: "ref", Help: "permalink, story key, pfbid, or post id with --author"}}}, listComments)

	handle(app, o, kit.OpMeta{Name: "reactions", Group: "read",
		Summary: "Read the reaction breakdown on a post", URIType: "reaction",
		Args: []kit.Arg{{Name: "ref", Help: "permalink, story key, pfbid, or post id with --author"}}}, listReactions)
}

func getPost(ctx context.Context, in postRef, emit func(*Post) error) error {
	p, err := in.Engine.Post(ctx, in.Ref, in.Author)
	if err != nil {
		return MapErr(err)
	}
	return emit(&p)
}

func listComments(ctx context.Context, in postRef, emit func(*Comment) error) error {
	p, err := in.Engine.Comments(ctx, in.Ref, in.Author, in.Limit)
	if err != nil {
		return MapErr(err)
	}
	if len(p.Comments) == 0 {
		return MapErr(noResults("post %s has no comments on this surface", p.ID))
	}
	for i := range p.Comments {
		if err := emit(&p.Comments[i]); err != nil {
			return err
		}
	}
	return nil
}

// Reaction is one row of a post's breakdown. It is its own type because the
// breakdown is a map on the post and "how did people react to this" is a list,
// and because a share is worth computing once here rather than in every caller.
type Reaction struct {
	Kind  string  `json:"kind"`
	Count int     `json:"count"`
	Share float64 `json:"share,omitempty"`
}

func listReactions(ctx context.Context, in postRef, emit func(*Reaction) error) error {
	p, err := in.Engine.Post(ctx, in.Ref, in.Author)
	if err != nil {
		return MapErr(err)
	}
	if len(p.Counts.ByType) == 0 {
		return MapErr(noResults("post %s carries no reaction breakdown", p.ID))
	}
	kinds := make([]string, 0, len(p.Counts.ByType))
	total := p.Counts.Reactions
	for k := range p.Counts.ByType {
		kinds = append(kinds, k)
	}
	if total == 0 {
		for _, k := range kinds {
			total += p.Counts.ByType[k]
		}
	}
	// Biggest first, and by name where two types tie, so the same post answers
	// in the same order twice running.
	sort.Slice(kinds, func(i, j int) bool {
		if p.Counts.ByType[kinds[i]] != p.Counts.ByType[kinds[j]] {
			return p.Counts.ByType[kinds[i]] > p.Counts.ByType[kinds[j]]
		}
		return kinds[i] < kinds[j]
	})
	for _, k := range kinds {
		r := Reaction{Kind: k, Count: p.Counts.ByType[k]}
		if total > 0 {
			r.Share = float64(r.Count) / float64(total)
		}
		if err := emit(&r); err != nil {
			return err
		}
	}
	return nil
}

// --- media ---

func registerMediaOps(app *kit.App, o OpOptions) {
	handle(app, o, kit.OpMeta{Name: "photo", Group: "read", Single: true,
		Summary: "Read a photo permalink", URIType: "photo", Resolver: true,
		Args: []kit.Arg{{Name: "ref", Help: "photo fbid or photo URL"}}}, getPhoto)

	handle(app, o, kit.OpMeta{Name: "video", Group: "read", Single: true,
		Summary: "Read a video or a reel", URIType: "video", Resolver: true,
		Args: []kit.Arg{{Name: "ref", Help: "video id, watch URL, or reel URL"}}}, getVideo)
}

func getPhoto(ctx context.Context, in eventRef, emit func(*Photo) error) error {
	p, err := in.Engine.Photo(ctx, in.Ref)
	if err != nil {
		return MapErr(err)
	}
	return emit(&p)
}

func getVideo(ctx context.Context, in videoRef, emit func(*Video) error) error {
	v, err := in.Engine.Video(ctx, in.Ref, in.Transcript)
	if err != nil {
		return MapErr(err)
	}
	return emit(&v)
}

// --- groups ---

func registerGroupOps(app *kit.App, o OpOptions) {
	handle(app, o, kit.OpMeta{Name: "group", Group: "read", Single: true,
		Summary: "Read a public group", URIType: "group", Resolver: true,
		Args: []kit.Arg{{Name: "ref", Help: "group id, or slug with a session"}}}, getGroup)

	handle(app, o, kit.OpMeta{Name: "feed", Parent: "group", Group: "read", List: true,
		Summary: "Read a group's discussion, as deep as asked", URIType: "post",
		Args: []kit.Arg{{Name: "ref", Help: "group id, or slug with a session"}}}, listGroupFeed)
}

func getGroup(ctx context.Context, in groupRef, emit func(*Group) error) error {
	g, err := in.Engine.Group(ctx, in.Ref)
	if err != nil {
		return MapErr(err)
	}
	return emit(&g)
}

func listGroupFeed(ctx context.Context, in groupRef, emit func(*Post) error) error {
	g, err := in.Engine.GroupFeed(ctx, in.Ref, in.Limit)
	if err != nil {
		return MapErr(err)
	}
	for i := range g.Posts {
		if err := emit(&g.Posts[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- events ---

func registerEventOps(app *kit.App, o OpOptions) {
	handle(app, o, kit.OpMeta{Name: "event", Group: "read", Single: true,
		Summary: "Read a public event", URIType: "event", Resolver: true,
		Args: []kit.Arg{{Name: "ref", Help: "event id or permalink"}}}, getEvent)

	handle(app, o, kit.OpMeta{Name: "suggested", Parent: "event", Group: "read",
		Summary: "Read the events Facebook suggests beside one", URIType: "event",
		Args: []kit.Arg{{Name: "ref", Help: "event id or permalink"}}}, listSuggested)
}

func getEvent(ctx context.Context, in eventRef, emit func(*Event) error) error {
	e, err := in.Engine.Event(ctx, in.Ref)
	if err != nil {
		return MapErr(err)
	}
	return emit(&e)
}

func listSuggested(ctx context.Context, in eventRef, emit func(*EventCard) error) error {
	e, err := in.Engine.Event(ctx, in.Ref)
	if err != nil {
		return MapErr(err)
	}
	if len(e.Suggested) == 0 {
		return MapErr(noResults("event %s has no suggestions beside it", e.ID))
	}
	for i := range e.Suggested {
		if err := emit(&e.Suggested[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- directory ---

func registerDirectoryOps(app *kit.App, o OpOptions) {
	handle(app, o, kit.OpMeta{Name: "discover", Group: "read",
		Summary: "Read the Pages directory, as a seed source", URIType: "profile",
		Args: []kit.Arg{{Name: "letter", Help: "one letter page of the directory (the index with none)", Optional: true}}},
		listDirectory)
}

// listDirectory answers with entries for a letter and with the index otherwise.
// The index rows are letters rather than pages, and they come back as entries
// with the letter set and no id, because a caller walking the directory wants
// one shape and the index is a list of places to go next.
func listDirectory(ctx context.Context, in letterRef, emit func(*DirectoryEntry) error) error {
	d, err := in.Engine.Discover(ctx, in.Letter)
	if err != nil {
		return MapErr(err)
	}
	if len(d.Entries) > 0 {
		for i := range d.Entries {
			if in.Limit > 0 && i >= in.Limit {
				break
			}
			if err := emit(&d.Entries[i]); err != nil {
				return err
			}
		}
		return nil
	}
	for _, t := range d.Index {
		e := DirectoryEntry{Name: t.Name, URL: t.URL, Letter: t.Name}
		e.Envelope = d.Envelope
		if err := emit(&e); err != nil {
			return err
		}
	}
	return nil
}

// --- claims ---

// graphRef is the graph plane's input. Depth and budget are flags rather than
// two operations, because a walk of depth zero is exactly `edges` and splitting
// them would be two names for one thing.
type graphRef struct {
	Ref    string  `kit:"arg" help:"anything fb can read: a profile, post, photo, video, group or event"`
	Depth  int     `kit:"flag" help:"how many hops out from the seed"`
	Budget int     `kit:"flag" help:"how many requests the walk may spend"`
	Engine *Engine `kit:"inject"`
}

func registerGraphOps(app *kit.App, o OpOptions) {
	handle(app, o, kit.OpMeta{Name: "edges", Group: "read",
		Summary: "Read the claims one page already makes about everything else",
		Args:    []kit.Arg{{Name: "ref", Help: "anything fb can read"}}}, listEdges)

	handle(app, o, kit.OpMeta{Name: "graph", Group: "read",
		Summary: "Read the claims, then the claims of everything they named",
		Args:    []kit.Arg{{Name: "ref", Help: "anything fb can read"}}}, listGraph)
}

func listEdges(ctx context.Context, in graphRef, emit func(*graph.Edge) error) error {
	c, err := in.Engine.Edges(ctx, in.Ref)
	if err != nil {
		return MapErr(err)
	}
	if len(c.Edges) == 0 {
		return MapErr(noResults("%s asserted nothing about anything else", in.Ref))
	}
	for i := range c.Edges {
		if err := emit(&c.Edges[i]); err != nil {
			return err
		}
	}
	return nil
}

// listGraph defaults the budget rather than trusting a caller who left it at
// zero to have meant zero. Over HTTP and MCP an unset field is indistinguishable
// from a deliberate zero, and a walk that silently does nothing is worse than
// one that spends a modest number of requests.
func listGraph(ctx context.Context, in graphRef, emit func(*graph.Edge) error) error {
	if in.Depth <= 0 {
		in.Depth = 1
	}
	if in.Budget <= 0 {
		in.Budget = 25
	}
	claims, err := in.Engine.Graph(ctx, in.Ref, in.Depth, in.Budget)
	if err != nil {
		return MapErr(err)
	}
	var seen graph.Set
	for _, c := range claims {
		for _, e := range c.Edges {
			seen.Add(e)
		}
	}
	if seen.Len() == 0 {
		return MapErr(noResults("%s asserted nothing about anything else", in.Ref))
	}
	edges := seen.Edges()
	for i := range edges {
		if err := emit(&edges[i]); err != nil {
			return err
		}
	}
	return nil
}
