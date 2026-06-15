package fb

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// NodeKind is the kind of object a walk node represents. Pages, profiles, and
// groups are the actors; a post is a story; a comment is a leaf under a post.
type NodeKind string

const (
	NodePage    NodeKind = "page"
	NodeProfile NodeKind = "profile"
	NodeGroup   NodeKind = "group"
	NodePost    NodeKind = "post"
	NodeComment NodeKind = "comment"
)

// isActor reports whether a node kind is an actor: a page, profile, or group,
// the things that own a feed of posts.
func (k NodeKind) isActor() bool {
	return k == NodePage || k == NodeProfile || k == NodeGroup
}

// Edge is one relationship the walker can follow.
type Edge string

const (
	// EdgePosts is the feed edge: an actor (page/profile/group) to its recent
	// posts. actor -> post.
	EdgePosts Edge = "posts"
	// EdgeAuthor is the authorship edge: a post to the actor that posted it.
	// post -> actor. It is the hop that, from a post seed, reaches the poster
	// and then (one hop further) the rest of their feed.
	EdgeAuthor Edge = "author"
	// EdgeComments is the conversation edge: a post to its preview comments.
	// post -> comment. Comments are leaves; the walk does not expand them,
	// because the anonymous surface exposes a commenter's name but no id to
	// follow.
	EdgeComments Edge = "comments"
)

var allEdges = []Edge{EdgePosts, EdgeAuthor, EdgeComments}

var knownEdges = map[Edge]bool{
	EdgePosts:    true,
	EdgeAuthor:   true,
	EdgeComments: true,
}

// follows reports whether a node of kind k may depart along this edge. The
// posts edge departs from actors; author and comments depart from posts.
func (e Edge) follows(k NodeKind) bool {
	switch e {
	case EdgePosts:
		return k.isActor()
	case EdgeAuthor, EdgeComments:
		return k == NodePost
	}
	return false
}

// EdgeSet is a set of edges to follow.
type EdgeSet map[Edge]bool

func newEdgeSet(es ...Edge) EdgeSet {
	s := make(EdgeSet, len(es))
	for _, e := range es {
		s[e] = true
	}
	return s
}

// Has reports whether the set contains an edge.
func (s EdgeSet) Has(e Edge) bool { return s[e] }

// List returns the edges in the set in canonical order.
func (s EdgeSet) List() []Edge {
	var out []Edge
	for _, e := range allEdges {
		if s[e] {
			out = append(out, e)
		}
	}
	return out
}

// String renders the set as a comma-separated list in canonical order.
func (s EdgeSet) String() string { return joinEdges(s.List()) }

func (s EdgeSet) clone() EdgeSet {
	out := make(EdgeSet, len(s))
	for e := range s {
		out[e] = true
	}
	return out
}

// edgePresets are named edge sets. Preset names and edge names are deliberately
// disjoint so a preset can never shadow a same-named edge.
var edgePresets = map[string]EdgeSet{
	"content": newEdgeSet(EdgePosts, EdgeAuthor),
	"threads": newEdgeSet(EdgePosts, EdgeComments),
	"all":     newEdgeSet(allEdges...),
}

var presetNames = []string{"content", "threads", "all"}

// DefaultEdges is the edge set used when --follow is empty: the content preset.
func DefaultEdges() EdgeSet { return edgePresets["content"].clone() }

// EdgeHelp is a one-line catalogue of presets and edges for flag help and error
// messages.
func EdgeHelp() string {
	return "presets " + strings.Join(presetNames, "|") + "; edges " + strings.Join(edgeNames(), "|")
}

func edgeNames() []string {
	out := make([]string, len(allEdges))
	for i, e := range allEdges {
		out[i] = string(e)
	}
	return out
}

// ParseEdges turns a --follow spec into an edge set. The spec is empty (the
// default), a preset name, or a comma-separated list mixing presets and edge
// names. Each token resolves as a preset first and an edge name second; an
// unrecognized token is an error that names the catalogue.
func ParseEdges(spec string) (EdgeSet, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return DefaultEdges(), nil
	}
	set := newEdgeSet()
	for _, tok := range strings.Split(spec, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if p, ok := edgePresets[tok]; ok {
			for e := range p {
				set[e] = true
			}
			continue
		}
		if e := Edge(tok); knownEdges[e] {
			set[e] = true
			continue
		}
		return nil, fmt.Errorf("unknown edge or preset %q; %s", tok, EdgeHelp())
	}
	if len(set) == 0 {
		return DefaultEdges(), nil
	}
	return set, nil
}

func joinEdges(es []Edge) string {
	parts := make([]string, len(es))
	for i, e := range es {
		parts[i] = string(e)
	}
	return strings.Join(parts, ",")
}

// Node is one object reached by a walk, tagged with how it was reached. Exactly
// one of the payload pointers is set, per Kind.
type Node struct {
	Kind    NodeKind `json:"kind"`
	Depth   int      `json:"depth"`
	Via     Edge     `json:"via,omitempty"`
	Parent  string   `json:"parent,omitempty"`
	Page    *Page    `json:"page,omitempty"`
	Profile *Profile `json:"profile,omitempty"`
	Group   *Group   `json:"group,omitempty"`
	Post    *Post    `json:"post,omitempty"`
	Comment *Comment `json:"comment,omitempty"`
}

// Endpoint returns the node's identity string: the actor id, the post id, or the
// post-scoped comment id.
func (n *Node) Endpoint() string {
	switch n.Kind {
	case NodePage:
		if n.Page != nil {
			return firstNonEmpty(n.Page.PageID, n.Page.Slug)
		}
	case NodeProfile:
		if n.Profile != nil {
			return firstNonEmpty(n.Profile.ProfileID, n.Profile.Username)
		}
	case NodeGroup:
		if n.Group != nil {
			return firstNonEmpty(n.Group.GroupID, n.Group.Slug)
		}
	case NodePost:
		if n.Post != nil {
			return firstNonEmpty(n.Post.PostID, n.Post.Permalink)
		}
	case NodeComment:
		if n.Comment != nil {
			return n.Comment.PostID + ":" + n.Comment.CommentID
		}
	}
	return ""
}

// nodeKey is the dedup key for a node: kind plus identity.
func nodeKey(kind NodeKind, ref string) string {
	return string(kind) + ":" + ref
}

// ownerKind maps a post's owner_type to the actor node kind it points at,
// defaulting to a page (the most common public owner).
func ownerKind(ownerType string) NodeKind {
	switch ownerType {
	case "group":
		return NodeGroup
	case "profile":
		return NodeProfile
	default:
		return NodePage
	}
}

// Seed is a starting point for a walk: any id or URL the client can resolve to a
// page, profile, group, or post.
type Seed struct {
	Raw string
}

// WalkOptions bounds a walk. Depth is the number of hops from each seed; Max
// caps the total nodes streamed; Fanout caps neighbors followed per edge. Note,
// when set, receives a one-line message for each non-fatal failure deeper in the
// walk.
type WalkOptions struct {
	Depth  int
	Max    int
	Fanout int
	Edges  EdgeSet
	Note   func(string)
}

// grapher is the subset of *Client the walker needs. It exists so the BFS logic
// is tested over an in-memory fake with no network.
type grapher interface {
	Resolve(ctx context.Context, input string) (Identity, error)
	Page(ctx context.Context, idOrURL string) (*Page, error)
	Profile(ctx context.Context, idOrURL string) (*Profile, error)
	Group(ctx context.Context, idOrURL string) (*Group, error)
	Post(ctx context.Context, idOrURL string, opt PostOptions) (*Post, error)
	PagePosts(ctx context.Context, idOrURL string, opt ListOptions) iter.Seq2[Post, error]
	ProfilePosts(ctx context.Context, idOrURL string, opt ListOptions) iter.Seq2[Post, error]
	GroupPosts(ctx context.Context, idOrURL string, opt ListOptions) iter.Seq2[Post, error]
	Comments(ctx context.Context, postURL string, opt CommentOptions) iter.Seq2[Comment, error]
}

var _ grapher = (*Client)(nil)

// Walker performs a breadth-first walk of the graph over a grapher.
type Walker struct {
	g grapher
}

// NewWalker returns a walker backed by g.
func NewWalker(g grapher) *Walker { return &Walker{g: g} }

// Walk runs a breadth-first walk from a client, streaming each node to emit.
func (c *Client) Walk(ctx context.Context, seeds []Seed, opts WalkOptions, emit func(*Node) error) error {
	return NewWalker(c).Walk(ctx, seeds, opts, emit)
}

// frontier is a queued walk item. A seed carries only seedRaw and is resolved
// when popped; a neighbor arrives pre-hydrated with its kind, ref, and payload.
type frontier struct {
	seedRaw string
	kind    NodeKind
	ref     string
	depth   int
	via     Edge
	parent  string
	page    *Page
	profile *Profile
	group   *Group
	post    *Post
	comment *Comment
}

// toNode wraps a pre-hydrated neighbor frontier as a node.
func (f frontier) toNode() *Node {
	return &Node{
		Kind: f.kind, Depth: f.depth, Via: f.via, Parent: f.parent,
		Page: f.page, Profile: f.profile, Group: f.group, Post: f.post, Comment: f.comment,
	}
}

// Walk runs a breadth-first walk from the seeds, streaming each node to emit as
// it is reached. A seed that cannot be fetched is fatal; a failure deeper in the
// walk is reported through opts.Note and the walk continues.
func (w *Walker) Walk(ctx context.Context, seeds []Seed, opts WalkOptions, emit func(*Node) error) error {
	if opts.Edges == nil {
		opts.Edges = DefaultEdges()
	}
	visited := make(map[string]bool)
	queue := make([]frontier, 0, len(seeds))
	for _, s := range seeds {
		queue = append(queue, frontier{seedRaw: s.Raw, depth: 0})
	}
	emitted := 0
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		f := queue[0]
		queue = queue[1:]

		node, key, err := w.hydrate(ctx, f)
		if err != nil {
			if f.depth == 0 {
				// A seed that cannot be fetched fails the walk, like a single read.
				return err
			}
			note(opts, err)
			continue
		}
		if visited[key] {
			continue
		}
		visited[key] = true

		if err := emit(node); err != nil {
			return err
		}
		emitted++
		if opts.Max > 0 && emitted >= opts.Max {
			return nil
		}
		if f.depth >= opts.Depth {
			continue
		}
		queue = append(queue, w.neighbors(ctx, node, f.depth, opts)...)
	}
	return nil
}

// hydrate turns a frontier item into a node and its dedup key. A pre-hydrated
// neighbor is wrapped as-is; a seed is resolved and fetched by identity.
func (w *Walker) hydrate(ctx context.Context, f frontier) (*Node, string, error) {
	if f.seedRaw == "" {
		return f.toNode(), nodeKey(f.kind, f.ref), nil
	}
	id, err := w.g.Resolve(ctx, f.seedRaw)
	if err != nil {
		return nil, "", err
	}
	ref := firstNonEmpty(id.CanonicalURL, f.seedRaw)
	switch id.Kind {
	case fbid.KindPage:
		p, err := w.g.Page(ctx, ref)
		if err != nil {
			return nil, "", err
		}
		return &Node{Kind: NodePage, Page: p}, nodeKey(NodePage, firstNonEmpty(p.PageID, p.Slug, id.Slug)), nil
	case fbid.KindProfile:
		p, err := w.g.Profile(ctx, ref)
		if err != nil {
			return nil, "", err
		}
		return &Node{Kind: NodeProfile, Profile: p}, nodeKey(NodeProfile, firstNonEmpty(p.ProfileID, p.Username, id.ProfileID)), nil
	case fbid.KindGroup:
		g, err := w.g.Group(ctx, ref)
		if err != nil {
			return nil, "", err
		}
		return &Node{Kind: NodeGroup, Group: g}, nodeKey(NodeGroup, firstNonEmpty(g.GroupID, g.Slug, id.GroupID)), nil
	case fbid.KindPost:
		p, err := w.g.Post(ctx, ref, PostOptions{})
		if err != nil {
			return nil, "", err
		}
		return &Node{Kind: NodePost, Post: p}, nodeKey(NodePost, firstNonEmpty(p.PostID, p.Permalink, id.PostID)), nil
	default:
		return nil, "", fmt.Errorf("discover walks pages, profiles, groups, and posts; %q resolves to a %s", f.seedRaw, id.Kind)
	}
}

// neighbors expands one node into its frontier items, following only the edges
// whose source matches the node's kind. Each neighbor is pre-built from the list
// it came in on (or, for an author, from the post's owner fields), so it needs
// no refetch when popped.
func (w *Walker) neighbors(ctx context.Context, n *Node, depth int, opts WalkOptions) []frontier {
	limit := opts.Fanout
	if limit <= 0 {
		limit = opts.Max
	}
	var out []frontier
	switch {
	case n.Kind.isActor():
		if opts.Edges.Has(EdgePosts) {
			ref := actorRef(n)
			posts, err := collectPosts(w.postsFor(ctx, n.Kind, ref, ListOptions{Limit: limit}), limit)
			for i := range posts {
				p := posts[i]
				out = append(out, frontier{kind: NodePost, ref: postRef(&p), depth: depth + 1, via: EdgePosts, parent: ref, post: &p})
			}
			note(opts, err)
		}
	case n.Kind == NodePost:
		p := n.Post
		if opts.Edges.Has(EdgeAuthor) && p.OwnerID != "" {
			out = append(out, actorStub(ownerKind(p.OwnerType), p.OwnerID, p.OwnerName, depth+1, EdgeAuthor, p.PostID))
		}
		if opts.Edges.Has(EdgeComments) {
			cs, err := collectComments(w.g.Comments(ctx, p.Permalink, CommentOptions{Limit: limit}), limit)
			for i := range cs {
				c := cs[i]
				out = append(out, frontier{kind: NodeComment, ref: c.PostID + ":" + c.CommentID, depth: depth + 1, via: EdgeComments, parent: p.PostID, comment: &c})
			}
			note(opts, err)
		}
	}
	return out
}

// postsFor dispatches to the right feed method for an actor node kind.
func (w *Walker) postsFor(ctx context.Context, kind NodeKind, ref string, opt ListOptions) iter.Seq2[Post, error] {
	switch kind {
	case NodeGroup:
		return w.g.GroupPosts(ctx, ref, opt)
	case NodeProfile:
		return w.g.ProfilePosts(ctx, ref, opt)
	default:
		return w.g.PagePosts(ctx, ref, opt)
	}
}

// actorRef returns the id used to fetch an actor node's feed and to dedup it.
func actorRef(n *Node) string {
	switch n.Kind {
	case NodePage:
		if n.Page != nil {
			return firstNonEmpty(n.Page.PageID, n.Page.Slug)
		}
	case NodeProfile:
		if n.Profile != nil {
			return firstNonEmpty(n.Profile.ProfileID, n.Profile.Username)
		}
	case NodeGroup:
		if n.Group != nil {
			return firstNonEmpty(n.Group.GroupID, n.Group.Slug)
		}
	}
	return ""
}

// postRef returns a post's dedup id.
func postRef(p *Post) string { return firstNonEmpty(p.PostID, p.Permalink) }

// actorStub builds a pre-hydrated actor frontier from a post's owner fields. It
// carries only the id, name, and a canonical URL; full metadata is left to a
// later `fb page`/`profile`/`group` if the caller wants it.
func actorStub(kind NodeKind, id, name string, depth int, via Edge, parent string) frontier {
	f := frontier{kind: kind, ref: id, depth: depth, via: via, parent: parent}
	switch kind {
	case NodeGroup:
		f.group = &Group{GroupID: id, Slug: id, Name: name, URL: "https://www.facebook.com/groups/" + id + "/"}
	case NodeProfile:
		f.profile = &Profile{ProfileID: id, Name: name, URL: "https://www.facebook.com/" + id}
	default:
		f.page = &Page{PageID: id, Slug: id, Name: name, URL: "https://www.facebook.com/" + id}
	}
	return f
}

// collectPosts drains a post iterator up to limit, returning whatever it
// gathered plus the first error. A mid-stream error keeps the posts already read
// so the walk can still follow them.
func collectPosts(seq iter.Seq2[Post, error], limit int) ([]Post, error) {
	var out []Post
	for p, err := range seq {
		if err != nil {
			return out, err
		}
		out = append(out, p)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// collectComments drains a comment iterator up to limit, returning whatever it
// gathered plus the first error.
func collectComments(seq iter.Seq2[Comment, error], limit int) ([]Comment, error) {
	var out []Comment
	for c, err := range seq {
		if err != nil {
			return out, err
		}
		out = append(out, c)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func note(opts WalkOptions, err error) {
	if err == nil || opts.Note == nil {
		return
	}
	opts.Note(err.Error())
}
