package fbid

import (
	"net/url"
	"strings"
)

// url.go routes a facebook.com URL to the thing it names.
//
// This is the table `fb explain` prints, and it is also how every command that
// takes "a URL or an id" gets from one to the other. The routes were read off
// the captures in spec 3004 doc 01 rather than off Facebook's documentation,
// which does not describe them.
//
// Facebook has several routes per object, mostly for historical reasons: a post
// is /{handle}/posts/{id}, /permalink.php?story_fbid={id}&id={author} and
// /story.php with the same parameters, and all three answer. So the router maps
// many routes onto one Ref rather than trying to keep them apart.

// hosts that mean facebook.com.
var fbHosts = map[string]bool{
	"facebook.com":          true,
	"www.facebook.com":      true,
	"web.facebook.com":      true,
	"m.facebook.com":        true,
	"mbasic.facebook.com":   true,
	"free.facebook.com":     true,
	"d.facebook.com":        true,
	"business.facebook.com": true,
}

// tabs are the profile sections that follow a handle, so /nasa/photos is a tab
// on a profile and not a profile called photos.
var tabs = map[string]string{
	"about": "about", "photos": "photos", "videos": "videos", "reels": "reels",
	"events": "events", "posts": "posts", "groups": "groups", "live": "live",
	"community": "community", "reviews": "reviews", "shop": "shop", "app": "app",
	"followers": "followers", "friends": "friends", "map": "map", "notes": "notes",
	"music": "music", "sports": "sports", "likes": "likes", "info": "info",
}

func parseURL(raw string) Ref {
	r := Ref{Input: raw, Kind: KindUnknown}
	s := raw
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		r.Note = "this is not a URL fb can read: " + err.Error()
		return r
	}
	host := strings.ToLower(u.Host)
	q := u.Query()
	segs := split(u.Path)

	switch {
	case host == "fb.watch":
		// fb.watch is a redirect service and the token in it is signed, so the
		// only way to know what it names is to follow it.
		r.Kind = KindShare
		r.ID = first(segs)
		r.URL = "https://fb.watch/" + r.ID
		r.Command = "video"
		r.Note = "a share link resolves only by following the redirect, which costs one request"
		return r
	case host == "fb.me":
		r.Kind = KindShare
		r.ID = first(segs)
		r.URL = "https://fb.me/" + r.ID
		r.Note = "a share link resolves only by following the redirect, which costs one request"
		return r
	case !fbHosts[host]:
		r.Note = host + " is not a Facebook host"
		return r
	}

	if len(segs) == 0 {
		r.Note = "this is the Facebook home page and it names nothing in particular"
		return r
	}

	switch segs[0] {
	case "share":
		// /share/p/{token}, and v, r, g for a video, a reel and a group.
		r.Kind = KindShare
		r.URL = Host + u.Path
		r.Note = "a share link resolves only by following the redirect, which costs one request"
		if len(segs) > 1 {
			r.ID = last(segs)
			switch segs[1] {
			case "p":
				r.Command = "post"
			case "v":
				r.Command = "video"
			case "r":
				r.Command = "reel"
			case "g":
				r.Command = "group"
			}
		}
		return r

	case "profile.php":
		r.Kind = KindProfile
		r.ID = q.Get("id")
		r.Tab = q.Get("sk")
		r.URL = Host + "/profile.php?id=" + r.ID
		r.Command = "page"
		if strings.HasPrefix(r.ID, "pfbid") {
			r.Opaque = true
			r.Note = "a pfbid profile id is per-render: it fetches, but it is never a graph key"
		}
		return r

	case "permalink.php", "story.php":
		r.Kind = KindPost
		r.ID = q.Get("story_fbid")
		r.PostID = r.ID
		r.AuthorID = q.Get("id")
		r.URL = Host + "/permalink.php?story_fbid=" + r.ID + "&id=" + r.AuthorID
		r.Command = "post"
		if strings.HasPrefix(r.ID, "pfbid") {
			r.Opaque = true
		}
		return r

	case "photo", "photo.php":
		r.Kind = KindPhoto
		r.ID = q.Get("fbid")
		r.PhotoID = r.ID
		r.Set = q.Get("set")
		r.URL = Host + "/photo/?fbid=" + r.ID
		if r.Set != "" {
			r.URL += "&set=" + url.QueryEscape(r.Set)
		}
		r.Command = "photo"
		return r

	case "watch", "watch.php", "video.php":
		if v := q.Get("v"); v != "" {
			r.Kind = KindVideo
			r.ID = v
			r.VideoID = v
			r.URL = Host + "/watch/?v=" + v
			r.Command = "video"
			return r
		}
		r.Note = "the watch feed names nothing on its own: /watch/?v={id} does"
		return r

	case "reel":
		r.Kind = KindReel
		r.ID = first(segs[1:])
		r.VideoID = r.ID
		r.URL = Host + "/reel/" + r.ID
		r.Command = "reel"
		return r

	case "groups":
		if len(segs) == 1 {
			r.Note = "this is the groups home page and it names no group"
			return r
		}
		gid := segs[1]
		// /groups/{id}/posts/{post} and /groups/{id}/permalink/{post} both name a
		// post in a group, and the group id is the useful half of either.
		if len(segs) >= 4 && (segs[2] == "posts" || segs[2] == "permalink") {
			r.Kind = KindPost
			r.ID = segs[3]
			r.PostID = segs[3]
			r.URL = Host + "/groups/" + gid + "/posts/" + segs[3]
			r.Command = "post"
			r.Note = "a post in group " + gid
			if strings.HasPrefix(r.ID, "pfbid") {
				r.Opaque = true
			}
			return r
		}
		r.Kind = KindGroup
		r.ID = gid
		r.URL = Host + "/groups/" + gid
		r.Command = "group"
		return r

	case "events":
		if len(segs) == 1 {
			r.Note = "this is the events home page and it names no event"
			return r
		}
		r.Kind = KindEvent
		r.ID = segs[1]
		r.URL = Host + "/events/" + segs[1]
		r.Command = "event"
		return r

	case "directory":
		r.Kind = KindDirectory
		r.URL = Host + u.Path
		r.Command = "discover"
		if len(segs) > 2 {
			r.Letter = segs[2]
		}
		return r

	case "search":
		r.Kind = KindSearch
		r.Query = q.Get("q")
		r.URL = Host + u.Path + "?q=" + url.QueryEscape(r.Query)
		r.Command = "search"
		r.Note = "search is Tier 1 only: Facebook answers a signed-out caller with 404"
		return r

	case "media":
		// /media/set/?set=a.416661013162614
		if set := q.Get("set"); set != "" {
			r.Kind = KindAlbum
			r.Set = set
			r.ID = strings.TrimPrefix(set, "a.")
			if i := strings.Index(r.ID, "."); i > 0 {
				r.ID = r.ID[:i]
			}
			r.URL = Host + "/media/set/?set=" + url.QueryEscape(set)
			r.Command = "photos --album"
			return r
		}

	case "pages":
		// The old /pages/{name}/{id} route, which still answers and still turns
		// up in the directory.
		if id := last(segs); allDigits(id) {
			r.Kind = KindPage
			r.ID = id
			r.URL = Host + "/profile.php?id=" + id
			r.Command = "page"
			return r
		}

	case "people":
		if id := last(segs); allDigits(id) {
			r.Kind = KindProfile
			r.ID = id
			r.URL = Host + "/profile.php?id=" + id
			r.Command = "page"
			return r
		}
	}

	// What is left starts with a handle, and the segment after it says whether
	// this is the profile, one of its tabs, or one object on it.
	handle := segs[0]
	if !isHandle(handle) {
		r.Note = "fb does not have a route for " + u.Path
		return r
	}
	if len(segs) == 1 {
		r = handleRef(raw, handle)
		r.Input = raw
		if v := q.Get("sk"); v != "" {
			r.Tab = v
		}
		return r
	}

	switch segs[1] {
	case "posts":
		r.Kind = KindPost
		r.Handle = handle
		r.ID = last(segs)
		r.PostID = r.ID
		r.URL = Host + "/" + handle + "/posts/" + r.ID
		r.Command = "post"
		if strings.HasPrefix(r.ID, "pfbid") {
			r.Opaque = true
			r.Note = "the pfbid in this permalink is per-render: the numeric post id comes back with the fetch"
		}
		return r
	case "videos":
		if id := last(segs); allDigits(id) {
			r.Kind = KindVideo
			r.Handle = handle
			r.ID = id
			r.VideoID = id
			r.URL = Host + "/watch/?v=" + id
			r.Command = "video"
			return r
		}
	case "reel", "reels":
		if id := last(segs); allDigits(id) {
			r.Kind = KindReel
			r.Handle = handle
			r.ID = id
			r.VideoID = id
			r.URL = Host + "/reel/" + id
			r.Command = "reel"
			return r
		}
	case "photos":
		if id := last(segs); allDigits(id) {
			r.Kind = KindPhoto
			r.Handle = handle
			r.ID = id
			r.PhotoID = id
			r.URL = Host + "/photo/?fbid=" + id
			r.Command = "photo"
			return r
		}
	}
	if tab, ok := tabs[segs[1]]; ok {
		r = handleRef(raw, handle)
		r.Input = raw
		r.Tab = tab
		r.URL = Host + "/" + handle + "/" + segs[1]
		r.Note = "the " + tab + " tab of " + handle
		return r
	}
	// An unknown second segment is more likely a route Facebook added than a
	// mistake, so the profile is still the answer and the note says what was
	// dropped.
	r = handleRef(raw, handle)
	r.Input = raw
	r.Note = "fb reads this as the profile " + handle + " and ignores /" + strings.Join(segs[1:], "/")
	return r
}

func split(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func first(segs []string) string {
	if len(segs) == 0 {
		return ""
	}
	return segs[0]
}

func last(segs []string) string {
	for i := len(segs) - 1; i >= 0; i-- {
		if segs[i] != "" {
			return segs[i]
		}
	}
	return ""
}
