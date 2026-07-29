package fb

// surfaces.go is the routing table, as a value.
//
// Spec 3004 doc 01 lists eight surfaces and which one answers which question at
// each tier. That table is here rather than in prose only, because a routing
// rule that lives in a document drifts from the code that implements it. `fb
// routes` prints this, and the invariant test checks that every question fb
// answers has a row and every row names a surface fb knows.

// The eight surfaces, by the short names that appear on every record.
const (
	surfaceComet     = "s1" // the Comet page: HTML carrying Relay results
	surfaceGraphQL   = "s2" // /api/graphql/, operations the page shipped
	surfaceOpenGraph = "s3" // the <meta> head of the same HTML
	surfaceEmbed     = "s4" // /plugins/*.php
	surfacePicture   = "s5" // graph.facebook.com/{id}/picture
	surfaceCDN       = "s6" // scontent.*.fbcdn.net
	surfaceDirectory = "s7" // /directory/pages/
	surfaceSession   = "s8" // s1 and s2 with your cookies
)

// Surface describes one route fb reads.
type Surface struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Host   string `json:"host"`
	Tier   int    `json:"tier"`
	Format string `json:"format"`
}

// Surfaces is the table from spec 3004 doc 01 section 0.
var Surfaces = []Surface{
	{surfaceComet, "Comet page", "www.facebook.com", 0, "HTML carrying Relay query results as JSON"},
	{surfaceGraphQL, "Comet GraphQL", "/api/graphql/", 0, "JSON, one object per line"},
	{surfaceOpenGraph, "OpenGraph head", "the same HTML", 0, "meta tags"},
	{surfaceEmbed, "Embed plugins", "/plugins/*.php", 0, "HTML"},
	{surfacePicture, "Picture redirect", "graph.facebook.com", 0, "302 to the CDN"},
	{surfaceCDN, "Media CDN", "scontent.*.fbcdn.net", 0, "bytes"},
	{surfaceDirectory, "Pages directory", "/directory/pages/", 0, "HTML, anchors only"},
	{surfaceSession, "Session Comet", "www.facebook.com", 1, "as surfaces 1 and 2"},
}

// SurfaceByID finds a surface, or false. Used by the invariant test, so that a
// record claiming a surface fb does not have is a failing test.
func SurfaceByID(id string) (Surface, bool) {
	for _, s := range Surfaces {
		if s.ID == id {
			return s, true
		}
	}
	return Surface{}, false
}

// Operation is one command's route: the surface it reads, the Comet operation
// it replays there, the tier that reaches it, and how deep it gets.
//
// `fb routes` prints this, and the invariant test checks it both ways: every
// row names a command fb has and an operation a parser reads, and every command
// that goes to the network has a row. A table that drifts from the code fails
// the build.
type Operation struct {
	Command string `json:"command"`
	Surface string `json:"surface"`
	Op      string `json:"operation,omitempty"`
	Tier    int    `json:"tier"`
	Depth   string `json:"depth"`
}

// Operations is the command-to-operation table from spec 3004 doc 05 section 3.
var Operations = []Operation{
	{"page", surfaceComet, "ProfileCometHeaderQuery", 0, "whole"},
	{"page --about", surfaceComet, "ProfileCometAboutAppSectionQuery", 0, "whole"},
	{"feed", surfaceComet, "ProfileCometTimelineFeedQuery", 0, "1 post"},
	{"feed", surfaceGraphQL, "ProfileCometTimelineFeedQuery", 1, "paged"},
	{"post", surfaceComet, "CometSinglePostDialogContentQuery", 0, "whole"},
	{"comments", surfaceComet, "CometSinglePostDialogContentQuery", 0, "20 shipped"},
	{"reactions", surfaceComet, "CometSinglePostDialogContentQuery", 0, "all seven types"},
	{"photo", surfaceComet, "CometPhotoRootContentQuery", 0, "whole"},
	{"photos", surfaceGraphQL, "ProfileCometTopAppSectionQuery", 0, "8 per page"},
	{"photos --album", surfaceComet, "CometPhotoRootContentQuery", 0, "one request per photo"},
	{"photo --download", surfaceCDN, "", 0, "the bytes and a sidecar"},
	{"video", surfaceComet, "FBReelsRootWithEntrypointQuery", 0, "whole"},
	{"video --transcript", surfaceComet, "CometVideoHomeNewPermalinkHeroUnitQuery", 0, "plus the transcript"},
	{"video --download", surfaceCDN, "", 0, "HD if there is one, else SD"},
	{"videos", surfaceGraphQL, "CometProfilePlusVideosRootQuery", 0, "21 per page"},
	{"events", surfaceGraphQL, "ProfileCometTopAppSectionQuery", 0, "8 per page"},
	{"event", surfaceComet, "EventCometPermalinkHeaderQuery", 0, "whole"},
	{"group", surfaceComet, "CometGroupRootQuery", 0, "whole"},
	{"group feed", surfaceGraphQL, "CometGroupDiscussionRootSuccessQuery", 0, "paged"},
	{"id --resolve", surfaceComet, "ProfileCometHeaderQuery", 0, "one request"},
	{"discover", surfaceDirectory, "", 0, "the index only"},
	{"search", surfaceSession, "", 1, "not implemented yet"},
}

// Route is one question and the surfaces that answer it at each tier.
type Route struct {
	Question string   `json:"question"`
	Tier0    []string `json:"tier0"`
	Tier1    []string `json:"tier1"`
	Note     string   `json:"note,omitempty"`
}

// Routes is the routing table from spec 3004 doc 01 section 12.
var Routes = []Route{
	{"a Page or profile", []string{surfaceComet, surfaceOpenGraph}, []string{surfaceSession}, "surface 3 carries the like and talking-about counts"},
	{"a profile's newest post", []string{surfaceComet}, []string{surfaceSession, surfaceGraphQL}, "one post and a cursor logged out, paged with the cursor spent"},
	{"a post whole", []string{surfaceComet}, []string{surfaceSession}, "the permalink ships 20 comments"},
	{"a post's comments", []string{surfaceComet}, []string{surfaceSession, surfaceGraphQL}, "paged at tier 1"},
	{"a post's reactions by type", []string{surfaceComet}, []string{surfaceSession}, "all seven types logged out"},
	{"a photo", []string{surfaceComet}, []string{surfaceSession}, "/photo/?fbid=, the comments the permalink ships"},
	{"a video", []string{surfaceComet}, []string{surfaceSession}, "/reel/{id} first, /watch/?v= for the transcript"},
	{"a reel", []string{surfaceComet}, []string{surfaceSession}, "/reel/{id}"},
	{"a profile's photos", []string{surfaceGraphQL}, []string{surfaceSession}, "8 per section fetch"},
	{"an album's photos", []string{surfaceComet}, []string{surfaceSession}, "walked one request at a time through the neighbour chain: a set has no cursor and the album page ships nothing"},
	{"a profile's videos", []string{surfaceComet, surfaceGraphQL}, []string{surfaceSession}, "the grid and the show playlists beside it, 21 on NASA"},
	{"a profile's events", []string{surfaceGraphQL}, []string{surfaceSession}, "8 per section fetch"},
	{"a profile's About", []string{surfaceComet}, []string{surfaceSession, surfaceGraphQL}, "the /about page only logged out: the About query is refused"},
	{"a public group", []string{surfaceComet}, []string{surfaceSession}, "by numeric id logged out, by slug too at tier 1: tier 0 is best effort, Facebook cuts the group route off after a few dozen requests"},
	{"a group's feed", []string{surfaceGraphQL}, []string{surfaceSession}, "as deep as asked, on the same best-effort footing as the group page at tier 0"},
	{"a public event", []string{surfaceComet, surfaceOpenGraph}, []string{surfaceSession}, "surface 3 carries the RSVP counts, rounded"},
	{"a profile picture", []string{surfacePicture}, []string{surfacePicture}, "by handle or id"},
	{"media bytes", []string{surfaceCDN}, []string{surfaceCDN}, ""},
	{"the numeric id behind a handle, a pfbid or a share link", []string{surfaceComet}, []string{surfaceSession}, "one request, and only for the three references that parsing cannot settle"},
	{"discovery", []string{surfaceDirectory}, []string{surfaceSession}, "the index only, the letter pages are blocked"},
}
