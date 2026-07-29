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
	{"a photo", []string{surfaceComet}, []string{surfaceSession}, "/photo/?fbid=, 12 comments"},
	{"a video", []string{surfaceComet}, []string{surfaceSession}, "/watch/?v="},
	{"a reel", []string{surfaceComet}, []string{surfaceSession}, "/reel/{id}"},
	{"a profile's photos", []string{surfaceGraphQL}, []string{surfaceSession}, "8 per section fetch"},
	{"a profile's videos", []string{surfaceComet, surfaceGraphQL}, []string{surfaceSession}, "56 at a time"},
	{"a profile's events", []string{surfaceGraphQL}, []string{surfaceSession}, "8 per section fetch"},
	{"a profile's About", []string{surfaceComet}, []string{surfaceSession, surfaceGraphQL}, "the /about page only logged out: the About query is refused"},
	{"a public group", []string{surfaceComet}, []string{surfaceSession}, "by numeric id logged out, by slug too at tier 1"},
	{"a group's feed", []string{surfaceGraphQL}, []string{surfaceSession}, "as deep as asked"},
	{"a public event", []string{surfaceComet}, []string{surfaceSession}, ""},
	{"a profile picture", []string{surfacePicture}, []string{surfacePicture}, "by handle or id"},
	{"media bytes", []string{surfaceCDN}, []string{surfaceCDN}, ""},
	{"discovery", []string{surfaceDirectory}, []string{surfaceDirectory}, "pages and places"},
}
