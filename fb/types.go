package fb

import "time"

// Page is a Facebook Page (organization, brand, public figure).
type Page struct {
	PageID            string    `json:"page_id"`
	Slug              string    `json:"slug"`
	Name              string    `json:"name"`
	Category          string    `json:"category"`
	About             string    `json:"about"`
	Description       string    `json:"description,omitempty"`
	LikesCount        int64     `json:"likes_count"`
	FollowersCount    int64     `json:"followers_count"`
	TalkingAboutCount int64     `json:"talking_about_count,omitempty"`
	Verified          bool      `json:"verified"`
	Website           string    `json:"website,omitempty"`
	Phone             string    `json:"phone,omitempty"`
	Email             string    `json:"email,omitempty"`
	Address           string    `json:"address,omitempty"`
	Rating            float64   `json:"rating,omitempty"`
	RatingCount       int64     `json:"rating_count,omitempty"`
	CoverURL          string    `json:"cover_url,omitempty"`
	AvatarURL         string    `json:"avatar_url,omitempty"`
	URL               string    `json:"url"`
	FetchedAt         time.Time `json:"fetched_at"`
}

// Profile is a person's public profile.
type Profile struct {
	ProfileID      string    `json:"profile_id"`
	Username       string    `json:"username,omitempty"`
	Name           string    `json:"name"`
	Intro          string    `json:"intro,omitempty"`
	Bio            string    `json:"bio,omitempty"`
	FollowersCount int64     `json:"followers_count,omitempty"`
	FriendsCount   int64     `json:"friends_count,omitempty"`
	Verified       bool      `json:"verified"`
	Work           string    `json:"work,omitempty"`
	Education      string    `json:"education,omitempty"`
	Hometown       string    `json:"hometown,omitempty"`
	CurrentCity    string    `json:"current_city,omitempty"`
	Relationship   string    `json:"relationship,omitempty"`
	Websites       []string  `json:"websites,omitempty"`
	AvatarURL      string    `json:"avatar_url,omitempty"`
	CoverURL       string    `json:"cover_url,omitempty"`
	URL            string    `json:"url"`
	FetchedAt      time.Time `json:"fetched_at"`
}

// Group is a Facebook group.
type Group struct {
	GroupID      string    `json:"group_id"`
	Slug         string    `json:"slug,omitempty"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Privacy      string    `json:"privacy,omitempty"`
	MembersCount int64     `json:"members_count,omitempty"`
	Category     string    `json:"category,omitempty"`
	CreatedText  string    `json:"created_text,omitempty"`
	CoverURL     string    `json:"cover_url,omitempty"`
	URL          string    `json:"url"`
	FetchedAt    time.Time `json:"fetched_at"`
}

// Post is a single story by a page, profile, or group.
type Post struct {
	PostID        string    `json:"post_id"`
	OwnerID       string    `json:"owner_id,omitempty"`
	OwnerName     string    `json:"owner_name,omitempty"`
	OwnerType     string    `json:"owner_type,omitempty"`
	Text          string    `json:"text"`
	CreatedAtText string    `json:"created_at_text,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	LikeCount     int64     `json:"like_count"`
	ReactionCount int64     `json:"reaction_count,omitempty"`
	CommentCount  int64     `json:"comment_count"`
	ShareCount    int64     `json:"share_count"`
	ViewCount     int64     `json:"view_count,omitempty"`
	Permalink     string    `json:"permalink"`
	MediaURLs     []string  `json:"media_urls,omitempty"`
	ExternalLinks []string  `json:"external_links,omitempty"`
	IsPinned      bool      `json:"is_pinned,omitempty"`
	FetchedAt     time.Time `json:"fetched_at"`
}

// Comment is a comment or reply on a post or photo.
type Comment struct {
	CommentID     string    `json:"comment_id"`
	PostID        string    `json:"post_id"`
	ParentID      string    `json:"parent_id,omitempty"`
	AuthorID      string    `json:"author_id,omitempty"`
	AuthorName    string    `json:"author_name,omitempty"`
	AuthorURL     string    `json:"author_url,omitempty"`
	Text          string    `json:"text"`
	CreatedAtText string    `json:"created_at_text,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	LikeCount     int64     `json:"like_count"`
	ReplyCount    int64     `json:"reply_count,omitempty"`
	MediaURLs     []string  `json:"media_urls,omitempty"`
	Permalink     string    `json:"permalink,omitempty"`
	FetchedAt     time.Time `json:"fetched_at"`
}

// Reaction is one reactor's reaction to a post or comment.
type Reaction struct {
	PostID      string    `json:"post_id,omitempty"`
	CommentID   string    `json:"comment_id,omitempty"`
	Type        string    `json:"type"`
	ReactorID   string    `json:"reactor_id,omitempty"`
	ReactorName string    `json:"reactor_name,omitempty"`
	ReactorURL  string    `json:"reactor_url,omitempty"`
	FetchedAt   time.Time `json:"fetched_at"`
}

// ReactionSummary aggregates reaction counts by type.
type ReactionSummary struct {
	PostID    string    `json:"post_id,omitempty"`
	CommentID string    `json:"comment_id,omitempty"`
	Like      int64     `json:"like"`
	Love      int64     `json:"love"`
	Care      int64     `json:"care"`
	Haha      int64     `json:"haha"`
	Wow       int64     `json:"wow"`
	Sad       int64     `json:"sad"`
	Angry     int64     `json:"angry"`
	Total     int64     `json:"total"`
	Source    string    `json:"source,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

// Photo is one image.
type Photo struct {
	PhotoID       string    `json:"photo_id"`
	OwnerID       string    `json:"owner_id,omitempty"`
	PostID        string    `json:"post_id,omitempty"`
	Caption       string    `json:"caption,omitempty"`
	FullURL       string    `json:"full_url,omitempty"`
	ThumbURL      string    `json:"thumb_url,omitempty"`
	Width         int       `json:"width,omitempty"`
	Height        int       `json:"height,omitempty"`
	AlbumID       string    `json:"album_id,omitempty"`
	AlbumName     string    `json:"album_name,omitempty"`
	CreatedAtText string    `json:"created_at_text,omitempty"`
	LikeCount     int64     `json:"like_count,omitempty"`
	CommentCount  int64     `json:"comment_count,omitempty"`
	FetchedAt     time.Time `json:"fetched_at"`
}

// Video covers videos and reels.
type Video struct {
	VideoID         string    `json:"video_id"`
	OwnerID         string    `json:"owner_id,omitempty"`
	OwnerName       string    `json:"owner_name,omitempty"`
	Title           string    `json:"title,omitempty"`
	Description     string    `json:"description,omitempty"`
	CreatedAtText   string    `json:"created_at_text,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	DurationSeconds int       `json:"duration_seconds,omitempty"`
	ViewCount       int64     `json:"view_count,omitempty"`
	LikeCount       int64     `json:"like_count,omitempty"`
	CommentCount    int64     `json:"comment_count,omitempty"`
	ShareCount      int64     `json:"share_count,omitempty"`
	ThumbURL        string    `json:"thumb_url,omitempty"`
	Permalink       string    `json:"permalink"`
	IsReel          bool      `json:"is_reel,omitempty"`
	Streams         []Stream  `json:"streams,omitempty"`
	FetchedAt       time.Time `json:"fetched_at"`
}

// Stream is one playable media source.
type Stream struct {
	Quality string `json:"quality,omitempty"`
	MIME    string `json:"mime,omitempty"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	URL     string `json:"url"`
	IsAudio bool   `json:"is_audio,omitempty"`
}

// Event is a public event.
type Event struct {
	EventID         string    `json:"event_id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	StartText       string    `json:"start_text,omitempty"`
	StartAt         time.Time `json:"start_at,omitempty"`
	EndAt           time.Time `json:"end_at,omitempty"`
	LocationName    string    `json:"location_name,omitempty"`
	LocationAddress string    `json:"location_address,omitempty"`
	HostName        string    `json:"host_name,omitempty"`
	HostID          string    `json:"host_id,omitempty"`
	GoingCount      int64     `json:"going_count,omitempty"`
	InterestedCount int64     `json:"interested_count,omitempty"`
	Online          bool      `json:"online,omitempty"`
	CoverURL        string    `json:"cover_url,omitempty"`
	URL             string    `json:"url"`
	FetchedAt       time.Time `json:"fetched_at"`
}

// SearchResult is one discriminated search hit.
type SearchResult struct {
	Query      string    `json:"query"`
	ResultType string    `json:"result_type"`
	EntityID   string    `json:"entity_id,omitempty"`
	Title      string    `json:"title"`
	Snippet    string    `json:"snippet,omitempty"`
	URL        string    `json:"url"`
	FetchedAt  time.Time `json:"fetched_at"`
}

// ListOptions tunes the streaming list methods.
type ListOptions struct {
	Limit int
	Since time.Time
	Until time.Time
}

// PostOptions tunes Post detail fetching.
type PostOptions struct {
	Comments  bool
	Replies   bool
	Reactions bool
	NoDetail  bool
}

// CommentOptions tunes comment-thread walking.
type CommentOptions struct {
	Replies bool
	Order   string // chrono|ranked
	Limit   int
}

// SearchOptions tunes search.
type SearchOptions struct {
	Type  string // page|profile|group|post|photo|video|event|all
	Limit int
	Since time.Time
	Until time.Time
}
