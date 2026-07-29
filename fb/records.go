package fb

import (
	"strings"
	"time"
)

// records.go is every type fb emits, and the envelope each one carries.
//
// Spec 3004 doc 03 is this file in prose. The rule both follow: if the value is
// on the rendered page it is in the Relay store, so it belongs on the record. A
// field Facebook ships and fb drops is a defect, and `fb fields` prints the
// census so it is visible rather than assumed.

// Envelope says where a record came from. Every record carries one, in every
// output format.
type Envelope struct {
	Tier      int               `json:"tier"`
	Surfaces  []string          `json:"surfaces,omitempty"`
	Sources   []string          `json:"sources,omitempty"`
	Via       map[string]string `json:"via,omitempty"`
	Missed    []Missed          `json:"missed,omitempty"`
	FetchedAt time.Time         `json:"fetched_at,omitzero"`
}

// Missed is a surface that was tried and did not answer. A thin record from a
// refused operation and a thin record from a page with nothing on it are
// different facts, and a record that renders them the same is lying by
// omission.
type Missed struct {
	Surface string `json:"surface"`
	Reason  string `json:"reason"`
}

func (e *Envelope) addSurface(s string) {
	for _, got := range e.Surfaces {
		if got == s {
			return
		}
	}
	e.Surfaces = append(e.Surfaces, s)
}

func (e *Envelope) addSource(u string) {
	for _, got := range e.Sources {
		if got == u {
			return
		}
	}
	e.Sources = append(e.Sources, u)
}

func (e *Envelope) miss(surface, reason string) {
	e.Missed = append(e.Missed, Missed{Surface: surface, Reason: reason})
}

func (e *Envelope) setVia(field, surface string) {
	if e.Via == nil {
		e.Via = map[string]string{}
	}
	e.Via[field] = surface
}

// Ref points at another node. It is what makes the graph plane possible: a post
// read carries refs to profiles, photos and groups nobody fetched.
type Ref struct {
	Kind     string `json:"kind"`
	ID       string `json:"id,omitempty"`
	Handle   string `json:"handle,omitempty"`
	Name     string `json:"name,omitempty"`
	URL      string `json:"url,omitempty"`
	Verified bool   `json:"verified,omitempty"`
	Opaque   bool   `json:"id_opaque,omitempty"`
}

// Empty reports whether the ref names nothing.
func (r Ref) Empty() bool { return r.ID == "" && r.URL == "" && r.Name == "" }

// kindOf maps a Comet __typename to the kind used in refs and in fb:// URIs.
// An unmapped typename keeps its raw string: a type we have not met is not an
// error, and `fb fields` reports it so it becomes work rather than a silence.
func kindOf(typename string) string {
	switch typename {
	case "User":
		return "profile"
	case "Page":
		return "page"
	case "Group":
		return "group"
	case "Event":
		return "event"
	case "Photo":
		return "photo"
	case "Video":
		return "video"
	case "Story":
		return "post"
	case "Comment":
		return "comment"
	case "Album":
		return "album"
	case "ExternalUrl", "ExternalWebLink":
		return "external"
	case "FreeformPlace":
		return "place"
	case "":
		return ""
	default:
		return typename
	}
}

// parseRef reads any actor-shaped or entity-shaped node.
func parseRef(m map[string]any) Ref {
	if m == nil {
		return Ref{}
	}
	id := digStr(m, "id")
	if id == "" {
		id = digStr(m, "user_id")
	}
	url := canonURL(firstStr(m, "url", "profile_url", "permalink_url"))
	r := Ref{
		Kind:     kindOf(digStr(m, "__typename")),
		ID:       id,
		Name:     firstStr(m, "name", "short_name", "title"),
		URL:      url,
		Verified: digBool(m, "is_verified") || digBool(m, "show_verified_badge_on_profile"),
		Handle:   handleOf(url),
	}
	if r.Kind == "external" {
		// An ExternalUrl's id is base64 of the redirect shim, signature and
		// all, so it is a different id on every render of the same link. The
		// destination URL is the only stable key an external node has.
		r.ID = ""
		r.URL = unshim(r.URL)
	}
	if strings.HasPrefix(r.ID, "pfbid") {
		// A pfbid is per-render and per-viewer. It is kept because it is what
		// the surface said, and it is never a graph key.
		r.Opaque = true
	}
	return r
}

func firstStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := digStr(m, k); s != "" {
			return s
		}
	}
	return ""
}

// Image is any picture, with the alt text Facebook wrote for it.
type Image struct {
	URI     string `json:"uri"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	Alt     string `json:"alt,omitempty"`
	Blurred string `json:"blurred,omitempty"`
	Focus   *Focus `json:"focus,omitempty"`
	Signed  bool   `json:"signed,omitempty"`
}

// Focus is the crop point, both 0..1, so a cover can be cropped the way
// Facebook crops it.
type Focus struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Empty reports whether there is no image here.
func (i Image) Empty() bool { return i.URI == "" }

// parseImage reads an image node, taking the first of the several shapes
// Facebook uses for one picture.
func parseImage(m map[string]any) Image {
	if m == nil {
		return Image{}
	}
	img := Image{
		URI:    firstStr(m, "uri", "url"),
		Width:  digInt(m, "width"),
		Height: digInt(m, "height"),
	}
	if img.URI != "" {
		// A CDN URI is signed and expires. Saying so on the record is the
		// difference between a confusing 403 later and an expected one.
		img.Signed = strings.Contains(img.URI, "fbcdn.net")
	}
	return img
}

// Counts is the engagement on any object that has feedback.
type Counts struct {
	Reactions     int            `json:"reactions"`
	ReactionsText string         `json:"reactions_text,omitempty"`
	ByType        map[string]int `json:"by_type,omitempty"`
	Comments      int            `json:"comments,omitempty"`
	Shares        int            `json:"shares,omitempty"`
	SharesText    string         `json:"shares_text,omitempty"`
	Views         int            `json:"views,omitempty"`
	Plays         int            `json:"plays,omitempty"`
}

// ContextItem is an intro tile row fb did not classify. Keeping it is the point:
// dropping it would make the record wrong, and keeping it makes the next
// release's work visible.
type ContextItem struct {
	Kind     string `json:"kind"`
	Text     string `json:"text"`
	URL      string `json:"url,omitempty"`
	Icon     string `json:"icon,omitempty"`
	Renderer string `json:"renderer,omitempty"`
}

// Tab is a section a profile or group says it has, which is also the list of
// second fetches worth making.
type Tab struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	ID   string `json:"id,omitempty"`
}

// Profile is a Page or a person. One type, because Facebook uses one __typename
// for both and the difference is a field rather than a schema.
type Profile struct {
	Envelope
	ID             string        `json:"id"`
	Handle         string        `json:"handle,omitempty"`
	Name           string        `json:"name"`
	URL            string        `json:"url,omitempty"`
	Kind           string        `json:"kind,omitempty"`
	DelegatePage   string        `json:"delegate_page_id,omitempty"`
	Verified       bool          `json:"verified,omitempty"`
	Bio            Text          `json:"bio,omitempty"`
	Category       []string      `json:"category,omitempty"`
	CategoryRaw    string        `json:"category_raw,omitempty"`
	Likes          int           `json:"likes,omitempty"`
	TalkingAbout   int           `json:"talking_about,omitempty"`
	Followers      int           `json:"followers,omitempty"`
	Websites       []string      `json:"websites,omitempty"`
	Email          string        `json:"email,omitempty"`
	Phone          string        `json:"phone,omitempty"`
	Address        string        `json:"address,omitempty"`
	ContactOther   []ContextItem `json:"contact_other,omitempty"`
	Avatar         Image         `json:"avatar,omitempty"`
	Cover          Image         `json:"cover,omitempty"`
	Memorialized   bool          `json:"memorialized,omitempty"`
	LiveNow        bool          `json:"live_now,omitempty"`
	Tabs           []Tab         `json:"tabs,omitempty"`
	Posts          []Post        `json:"posts,omitempty"`
	PostsCursor    string        `json:"posts_cursor,omitempty"`
	PostsTruncated bool          `json:"posts_truncated,omitempty"`
}

// Post is a story: the type most of this tool exists to produce.
type Post struct {
	Envelope
	ID             string       `json:"id"`
	StoryID        string       `json:"story_id,omitempty"`
	URL            string       `json:"url,omitempty"`
	Author         Ref          `json:"author,omitempty"`
	DelegatePage   string       `json:"delegate_page_id,omitempty"`
	CreatedAt      time.Time    `json:"created_at,omitzero"`
	Message        Text         `json:"message,omitempty"`
	SEOTitle       string       `json:"seo_title,omitempty"`
	Counts         Counts       `json:"counts"`
	Attachments    []Attachment `json:"attachments,omitempty"`
	Comments       []Comment    `json:"comments,omitempty"`
	CommentsCursor string       `json:"comments_cursor,omitempty"`
	Group          *Ref         `json:"group,omitempty"`
	SharedPost     *Post        `json:"shared_post,omitempty"`
	// Pinned is set on a group's featured units. Nothing else says a post is
	// pinned, so without this the featured post reads as an ordinary one that
	// happens to be first.
	Pinned bool `json:"pinned,omitempty"`
}

// Attachment is a photo, video, album, link card, event or shared story hanging
// off a post.
type Attachment struct {
	Kind           string       `json:"kind"`
	Title          string       `json:"title,omitempty"`
	Description    string       `json:"description,omitempty"`
	URL            string       `json:"url,omitempty"`
	Source         string       `json:"source,omitempty"`
	Media          *Ref         `json:"media,omitempty"`
	Image          *Image       `json:"image,omitempty"`
	Subattachments []Attachment `json:"subattachments,omitempty"`
}

// Comment is one comment, with the count of its own replies.
type Comment struct {
	Envelope
	ID        string    `json:"id"`
	URL       string    `json:"url,omitempty"`
	Author    Ref       `json:"author,omitempty"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	Body      Text      `json:"body,omitempty"`
	Counts    Counts    `json:"counts"`
	Replies   int       `json:"replies,omitempty"`
	IsAuthor  bool      `json:"is_author,omitempty"`
}

// Photo is a photo permalink, which also carries the whole story it belongs to.
type Photo struct {
	Envelope
	ID        string    `json:"id"`
	URL       string    `json:"url,omitempty"`
	Image     Image     `json:"image"`
	Owner     Ref       `json:"owner,omitempty"`
	AlbumID   string    `json:"album_id,omitempty"`
	AlbumKind string    `json:"album_kind,omitempty"`
	Post      *Ref      `json:"post,omitempty"`
	Caption   Text      `json:"caption,omitempty"`
	Counts    Counts    `json:"counts"`
	Comments  []Comment `json:"comments,omitempty"`
	Next      *Ref      `json:"next,omitempty"`
	Prev      *Ref      `json:"prev,omitempty"`
	Tags      []Ref     `json:"tags,omitempty"`
}

// Caption is one subtitle track.
type Caption struct {
	Locale string `json:"locale,omitempty"`
	Name   string `json:"name,omitempty"`
	URL    string `json:"url"`
}

// Music is the audio track on a reel.
type Music struct {
	Title    string `json:"title,omitempty"`
	Artist   string `json:"artist,omitempty"`
	AlbumArt string `json:"album_art,omitempty"`
}

// Video is a video or a reel. One type, because a reel is a video with a
// vertical aspect and a short_form_video_context, and the same media id serves
// /watch/?v= and /reel/.
type Video struct {
	Envelope
	ID        string    `json:"id"`
	URL       string    `json:"url,omitempty"`
	Kind      string    `json:"kind,omitempty"`
	Owner     Ref       `json:"owner,omitempty"`
	Title     string    `json:"title,omitempty"`
	Message   Text      `json:"message,omitempty"`
	PostID    string    `json:"post_id,omitempty"`
	Width     int       `json:"width,omitempty"`
	Height    int       `json:"height,omitempty"`
	Duration  float64   `json:"duration_seconds,omitempty"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	Thumbnail Image     `json:"thumbnail,omitempty"`
	SDURL     string    `json:"sd_url,omitempty"`
	HDURL     string    `json:"hd_url,omitempty"`
	DashURL   string    `json:"dash_url,omitempty"`
	Captions  []Caption `json:"captions,omitempty"`
	IsLive    bool      `json:"is_live,omitempty"`
	LoopCount int       `json:"loop_count,omitempty"`
	Music     *Music    `json:"music,omitempty"`
	Privacy   string    `json:"privacy,omitempty"`
	// Transcript is Facebook's own machine transcript, which it publishes for
	// SEO on the /watch route and nowhere else. It is the only long-form text
	// on a video record and it is worth the extra fetch.
	Transcript string `json:"transcript,omitempty"`
	Counts     Counts `json:"counts"`
}

// Group is a group and, when the feed was read, its posts.
type Group struct {
	Envelope
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url,omitempty"`
	Privacy     string `json:"privacy,omitempty"`
	Visible     bool   `json:"visible,omitempty"`
	MembersText string `json:"members_text,omitempty"`
	Members     int    `json:"members,omitempty"`
	Description Text   `json:"description,omitempty"`
	Cover       Image  `json:"cover,omitempty"`
	Avatar      Image  `json:"avatar,omitempty"`
	Address     string `json:"address,omitempty"`
	ThemeColor  string `json:"theme_color,omitempty"`
	Tabs        []Tab  `json:"tabs,omitempty"`
	JoinState   string `json:"join_state,omitempty"`
	Posts       []Post `json:"posts,omitempty"`
	PostsCursor string `json:"posts_cursor,omitempty"`
}

// Place is where an event is. It covers the three shapes Facebook uses: a Page
// with an address, a freeform string, and a city.
type Place struct {
	Kind    string   `json:"kind"`
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name,omitempty"`
	City    *Ref     `json:"city,omitempty"`
	Address string   `json:"address,omitempty"`
	Lat     *float64 `json:"lat,omitempty"`
	Lng     *float64 `json:"lng,omitempty"`
}

// Flag is a discovery label on an event, such as Online.
type Flag struct {
	Label string `json:"label"`
	URL   string `json:"url,omitempty"`
}

// Transparency is what an event used to say. It matters to anyone reading an
// event critically.
type Transparency struct {
	NameChanged bool `json:"name_changed"`
	DateChanged bool `json:"date_changed"`
}

// Event is an event permalink.
type Event struct {
	Envelope
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	URL             string        `json:"url,omitempty"`
	Host            Ref           `json:"host,omitempty"`
	Hosts           []Ref         `json:"hosts,omitempty"`
	HostPageID      string        `json:"host_page_id,omitempty"`
	Start           time.Time     `json:"start,omitzero"`
	End             time.Time     `json:"end,omitzero"`
	Timezone        string        `json:"timezone,omitempty"`
	DayTimeSentence string        `json:"day_time_sentence,omitempty"`
	Place           *Place        `json:"place,omitempty"`
	IsOnline        bool          `json:"is_online,omitempty"`
	OnlineKind      string        `json:"online_kind,omitempty"`
	OnlineURL       string        `json:"online_url,omitempty"`
	IsPast          bool          `json:"is_past,omitempty"`
	Canceled        bool          `json:"canceled,omitempty"`
	Kind            string        `json:"kind,omitempty"`
	RSVPStyle       string        `json:"rsvp_style,omitempty"`
	Description     Text          `json:"description,omitempty"`
	Cover           Image         `json:"cover,omitempty"`
	Flags           []Flag        `json:"flags,omitempty"`
	Frequency       string        `json:"frequency,omitempty"`
	ParentID        string        `json:"parent_id,omitempty"`
	ChildCount      int           `json:"child_count,omitempty"`
	Interested      *int          `json:"interested"`
	Going           *int          `json:"going"`
	RSVPText        string        `json:"rsvp_text,omitempty"`
	AnnouncedBy     string        `json:"announced_by,omitempty"`
	Suggested       []EventCard   `json:"suggested,omitempty"`
	Transparency    *Transparency `json:"transparency,omitempty"`
}

// EventCard is an event as it arrives in a list: the suggestions beside an
// event, a profile's events tab, a directory row.
//
// It is a separate type from Event because a card is not a thin event, it is a
// different set of fields. A card has the RSVP counts an event permalink does
// not, and it has none of the description, cover or host detail. Flattening the
// two would produce a record where half the columns are empty for reasons the
// reader cannot see.
type EventCard struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	URL        string    `json:"url,omitempty"`
	Start      time.Time `json:"start,omitzero"`
	WhenText   string    `json:"when_text,omitempty"`
	Place      *Place    `json:"place,omitempty"`
	IsOnline   bool      `json:"is_online,omitempty"`
	IsPast     bool      `json:"is_past,omitempty"`
	Kind       string    `json:"kind,omitempty"`
	Interested int       `json:"interested,omitempty"`
	Going      int       `json:"going,omitempty"`
	SocialText string    `json:"social_text,omitempty"`
	Image      Image     `json:"image,omitempty"`
}

// DirectoryEntry is a row off the Pages directory. It is thin on purpose: a name
// and a URL is not a profile, and storing it as one would fill the store with
// half-empty rows.
type DirectoryEntry struct {
	Envelope
	Name   string `json:"name"`
	URL    string `json:"url"`
	ID     string `json:"id,omitempty"`
	Letter string `json:"letter,omitempty"`
}
