package fb

import (
	"context"

	"github.com/tamnd/facebook-cli/pkg/fbid"
	"github.com/tamnd/facebook-cli/pkg/graph"
)

// claims.go is the one entry point that turns a reference into claims: read
// whatever it names, then hand the record to edges.go.
//
// It exists so that `fb edges`, `fb graph`, `fb rdf` and eventually `fb crawl`
// all route the same way, and so that adding a kind means adding one case here
// rather than four.

// Claims is one read and everything it asserted.
type Claims struct {
	Envelope
	URI    string       `json:"uri"`
	Kind   string       `json:"kind"`
	Record any          `json:"record,omitempty"`
	Edges  []graph.Edge `json:"edges"`
}

// Edges reads whatever ref names and returns the claims, without walking
// anywhere. One request, or the handful the underlying read already makes.
//
// The record comes back with the claims because the two answer different halves
// of the same question, and a caller that wants only the edges drops the field
// for free.
func (e *Engine) Edges(ctx context.Context, ref string) (Claims, error) {
	r := fbid.Parse(ref)
	switch r.Command {
	case "post", "comments", "reactions":
		p, err := e.Post(ctx, ref, "")
		if err != nil {
			return Claims{}, err
		}
		return claimsOf(p.Envelope, graph.KindPost, postID(p), p, PostEdges(p)), nil
	case "photo":
		p, err := e.Photo(ctx, ref)
		if err != nil {
			return Claims{}, err
		}
		return claimsOf(p.Envelope, graph.KindPhoto, p.ID, p, PhotoEdges(p)), nil
	case "video", "reel":
		v, err := e.Video(ctx, ref, false)
		if err != nil {
			return Claims{}, err
		}
		return claimsOf(v.Envelope, graph.KindVideo, v.ID, v, VideoEdges(v)), nil
	case "group":
		g, err := e.Group(ctx, ref)
		if err != nil {
			return Claims{}, err
		}
		return claimsOf(g.Envelope, graph.KindGroup, g.ID, g, GroupEdges(g)), nil
	case "event":
		ev, err := e.Event(ctx, ref)
		if err != nil {
			return Claims{}, err
		}
		return claimsOf(ev.Envelope, graph.KindEvent, ev.ID, ev, EventEdges(ev)), nil
	case "page", "profile", "feed", "":
		p, err := e.Profile(ctx, ref, ProfileOptions{})
		if err != nil {
			return Claims{}, err
		}
		// The URI says profile even for a Page, which is what ProfileEdges
		// files its claims under. Facebook gives both one __typename and one id
		// space, and two spellings of one node is how a store grows a duplicate
		// for every Page in it. The kind is on the record for anyone who cares.
		c := claimsOf(p.Envelope, graph.KindProfile, p.ID, p, ProfileEdges(p))
		if p.Kind != "" {
			c.Kind = p.Kind
		}
		return c, nil
	}
	return Claims{}, unsupported(ref, "fb reads it as `"+r.Command+"`, which asserts no claims: `fb explain` says what it is")
}

func claimsOf(env Envelope, kind, id string, record any, edges []graph.Edge) Claims {
	return Claims{
		Envelope: env,
		URI:      graph.URI(kind, id),
		Kind:     kind,
		Record:   record,
		Edges:    edges,
	}
}

// Graph is Edges plus one hop: every node a claim named gets read too.
//
// depth 0 is Edges. depth 1 spends one request per node the first read named,
// which on a photo permalink is a dozen and on a videos tab is twenty, so the
// budget is in requests and the caller says how many.
func (e *Engine) Graph(ctx context.Context, ref string, depth, budget int) ([]Claims, error) {
	first, err := e.Edges(ctx, ref)
	if err != nil {
		return nil, err
	}
	out := []Claims{first}
	if depth <= 0 {
		return out, nil
	}
	seen := map[string]bool{first.URI: true}
	frontier := frontierOf(first, seen)
	for d := 0; d < depth && len(frontier) > 0 && budget > 0; d++ {
		var next []node
		for _, n := range frontier {
			if budget <= 0 {
				break
			}
			if n.ref == "" {
				continue
			}
			budget--
			c, err := e.Edges(ctx, n.ref)
			if err != nil {
				// A node that will not read is a fact about the graph rather
				// than the end of the walk: a person's profile behind the
				// log-in page, a photo taken down, a group behind the wall. It
				// is recorded on the first read's envelope and the walk carries
				// on.
				out[0].miss(surfaceComet, "could not read "+n.uri+": "+err.Error())
				continue
			}
			out = append(out, c)
			next = append(next, frontierOf(c, seen)...)
		}
		frontier = next
	}
	return out, nil
}

// node is one thing on the frontier: its identity, the reference to fetch it
// with, which is empty when there is nothing worth fetching, and the predicate
// that named it, which is what the frontier's order uses to tell a photo in a
// story from a profile picture.
type node struct{ uri, ref, via string }

// frontierOf is the nodes a read named that have not been read yet, in the order
// they were asserted.
func frontierOf(c Claims, seen map[string]bool) []node {
	// A post's permalink needs its author's id as well as its own, and the only
	// place the author is written down is the authored claim. Collect those
	// first so the posts in the same batch can be fetched at all.
	authors := map[string]string{}
	// The delegate page is a second node and a first-class claim, but it is not
	// a second read: Facebook answers `profile.php?id=<delegate>` with the page
	// that delegated to it, so fetching one spends a request to be handed a
	// record the walk already has. Collect them here and leave them off.
	aliases := map[string]bool{}
	for _, e := range c.Edges {
		switch e.Predicate {
		case graph.Authored:
			if _, id, ok := graph.Split(e.From); ok {
				authors[e.To] = id
			}
		case graph.DelegatesTo:
			aliases[e.To] = true
		}
	}
	var out []node
	for _, e := range c.Edges {
		for _, uri := range [2]string{e.From, e.To} {
			if uri == "" || seen[uri] || aliases[uri] {
				continue
			}
			seen[uri] = true
			kind, id, ok := graph.Split(uri)
			if !ok || !worthFetching(kind) {
				continue
			}
			out = append(out, node{uri, refFor(kind, id, authors[uri]), e.Predicate})
		}
	}
	return out
}

// worthFetching says which kinds have a page behind them.
//
// A comment has no permalink of its own that fb can read, an album's page ships
// nothing to a signed-out reader, and an external URL is somebody else's site
// and none of this tool's business. Fetching any of the three spends a request
// to learn nothing, so the walk skips them and keeps the nodes.
func worthFetching(kind string) bool {
	switch kind {
	case graph.KindProfile, graph.KindPage, graph.KindPost,
		graph.KindPhoto, graph.KindVideo, graph.KindGroup, graph.KindEvent:
		return true
	}
	return false
}

// refFor turns a URI back into something the readers take, and returns nothing
// when it cannot build one that will work.
//
// A post is the awkward one. Its permalink is `permalink.php?story_fbid=X&id=Y`
// where Y is the author, and without the author Facebook answers 404. So a post
// whose author nobody claimed is left off the frontier rather than spending a
// request to be told it does not exist.
func refFor(kind, id, owner string) string {
	switch kind {
	case graph.KindPhoto:
		return fbid.Host + "/photo/?fbid=" + id
	case graph.KindVideo:
		return fbid.Host + "/reel/" + id
	case graph.KindGroup:
		return fbid.Host + "/groups/" + id
	case graph.KindEvent:
		return fbid.Host + "/events/" + id
	case graph.KindProfile, graph.KindPage:
		return id // a bare numeric id is what the profile reader wants
	case graph.KindPost:
		if owner == "" {
			return ""
		}
		return fbid.Host + "/permalink.php?story_fbid=" + id + "&id=" + owner
	}
	return ""
}
