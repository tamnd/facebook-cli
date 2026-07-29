package cli

import (
	"context"
	"sort"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/facebook-cli/fb"
)

// cmd_read.go is the read surface: one command per question in doc 05 section
// 2, each one a thin wrapper over an engine method.
//
// Every one of them ends the same way, in a.done(...), which writes the failure
// record on jsonl and maps the error to its exit code. Nothing here formats
// anything: that is rows.go and the shared renderer.

func readCommands() []kit.Command {
	return []kit.Command{
		newPageCmd("page"),
		newPageCmd("profile"),
		newFeedCmd(),
		newPostCmd(),
		newCommentsCmd(),
		newReactionsCmd(),
		newPhotoCmd(),
		newPhotosCmd(),
		newVideoCmd("video"),
		newVideoCmd("reel"),
		newVideosCmd(),
		newGroupCmd(),
		newEventCmd(),
		newEventsCmd(),
		newDiscoverCmd(),
		newSearchCmd(),
		newEdgesCmd(),
		newGraphCmd(),
	}
}

// newPageCmd builds `fb page` and `fb profile`, which are the same read under
// the two words people use for it. Neither is an alias that prints a
// deprecation notice: the words mean different things to a reader and the
// record is the same either way.
func newPageCmd(name string) kit.Command {
	var about, noPosts bool
	var tab string
	short := "Show a Page or profile, whole"
	if name == "profile" {
		short = "Show a profile or Page, whole"
	}
	return kit.Command{
		Use:   name + " <handle|id>",
		Short: short,
		Long: "page reads the profile record: the name, the id and the delegate page id, the category, " +
			"the counts, the bio, the contact block, the cover and avatar, the tabs the page says it " +
			"has, and the newest post. --about fetches the About page as well, which is where the " +
			"category list, the address and the long contact block live.",
		Args: kit.ExactArgs(1),
		Flags: func(f *kit.FlagSet) {
			f.BoolVar(&about, "about", false, "fetch the About page too")
			f.BoolVar(&noPosts, "no-posts", false, "skip the timeline parse")
			f.StringVar(&tab, "tab", "", "read one tab instead of the main page")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			p, err := eng.Profile(a.ctx(), args[0], fb.ProfileOptions{About: about, NoPosts: noPosts, Tab: tab})
			if err != nil {
				return a.done(err)
			}
			a.warnMissed(p.Envelope)
			return a.done(a.emitOne(profileRow(p)))
		},
	}
}

// newFeedCmd is the timeline. Signed out it is one post, and the command says
// so on stderr rather than in a footnote, because one post from a page that
// posts daily is the kind of answer somebody builds a wrong conclusion on.
func newFeedCmd() kit.Command {
	return kit.Command{
		Use:   "feed <handle|id>",
		Short: "A profile's timeline",
		Long: "feed walks a profile's posts. Signed out Facebook serves the first post and refuses to " +
			"page, so fb prints what it got and says why there is no more. That is not an error and " +
			"the exit code is 0, because one post is a real answer.",
		Args: kit.ExactArgs(1),
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			p, err := eng.Feed(a.ctx(), args[0], a.limit)
			if err != nil {
				return a.done(err)
			}
			if len(p.Posts) == 0 {
				return a.done(fb.NoResults("%s shows no posts, or none it shows signed out", args[0]))
			}
			a.warnMissed(p.Envelope)
			if eng.Tier() == 0 && len(p.Posts) == 1 && p.PostsCursor != "" {
				a.logf("note: Tier 0 serves one post per profile; the timeline query is refused signed out.")
				a.logf("      Run fb auth import to read further, or use fb group feed which is not walled.")
			}
			rows := make([]Row, 0, len(p.Posts))
			for _, post := range p.Posts {
				rows = append(rows, postRow(post))
			}
			return a.done(a.emit(rows))
		},
	}
}

// authorFlag is on every post command, because Facebook has no route that takes
// a post id alone: a bare id needs the author to build a permalink from.
func authorFlag(p *string) func(*kit.FlagSet) {
	return func(f *kit.FlagSet) {
		f.StringVar(p, "author", "", "the profile the post belongs to, for a bare post id or a pfbid")
	}
}

func newPostCmd() kit.Command {
	var author string
	return kit.Command{
		Use:   "post <url|id>",
		Short: "Show a post, with the comments the permalink ships",
		Long: "post takes a permalink, a story_fbid URL, a pfbid token with --author, a story key, or a " +
			"bare post id with --author. A bare post id on its own is a usage error naming the two " +
			"ways to fix it, because Facebook has no route that takes one.",
		Args:  kit.ExactArgs(1),
		Flags: authorFlag(&author),
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			p, err := eng.Post(a.ctx(), args[0], author)
			if err != nil {
				return a.done(err)
			}
			a.warnMissed(p.Envelope)
			return a.done(a.emitOne(postRow(p)))
		},
	}
}

func newCommentsCmd() kit.Command {
	var author string
	return kit.Command{
		Use:   "comments <url|id>",
		Short: "The comments on a post",
		Long: "comments reads the comment list the permalink ships, and pages further where the cursor " +
			"can be spent. The count of what was shown against what the post has is printed on " +
			"stderr, so a truncated read never reads as a complete one.",
		Args:  kit.ExactArgs(1),
		Flags: authorFlag(&author),
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			p, err := eng.Comments(a.ctx(), args[0], author, a.limit)
			if err != nil {
				return a.done(err)
			}
			if len(p.Comments) == 0 {
				return a.done(fb.NoResults("post %s has no comments on this surface", p.ID))
			}
			a.warnMissed(p.Envelope)
			rows := make([]Row, 0, len(p.Comments))
			for _, c := range p.Comments {
				rows = append(rows, commentRow(c))
			}
			if err := a.emit(rows); err != nil {
				return a.done(err)
			}
			a.logf("%d of %d shown", len(p.Comments), p.Counts.Comments)
			return nil
		},
	}
}

// newReactionsCmd prints the breakdown alone, one row per type. Signed out
// Facebook ships all seven, which is more than the post page shows a person.
func newReactionsCmd() kit.Command {
	var author string
	return kit.Command{
		Use:   "reactions <url|id>",
		Short: "The reaction breakdown on a post",
		Args:  kit.ExactArgs(1),
		Flags: authorFlag(&author),
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			p, err := eng.Post(a.ctx(), args[0], author)
			if err != nil {
				return a.done(err)
			}
			if len(p.Counts.ByType) == 0 {
				return a.done(fb.NoResults("post %s carries no reaction breakdown", p.ID))
			}
			a.warnMissed(p.Envelope)
			kinds := make([]string, 0, len(p.Counts.ByType))
			for k := range p.Counts.ByType {
				kinds = append(kinds, k)
			}
			// Biggest first, and by name where two types tie, so the same post
			// prints the same table twice running.
			sort.Slice(kinds, func(i, j int) bool {
				if p.Counts.ByType[kinds[i]] != p.Counts.ByType[kinds[j]] {
					return p.Counts.ByType[kinds[i]] > p.Counts.ByType[kinds[j]]
				}
				return kinds[i] < kinds[j]
			})
			total := p.Counts.Reactions
			if total == 0 {
				for _, k := range kinds {
					total += p.Counts.ByType[k]
				}
			}
			rows := make([]Row, 0, len(kinds))
			for _, k := range kinds {
				rows = append(rows, reactionRow(k, p.Counts.ByType[k], total))
			}
			return a.done(a.emit(rows))
		},
	}
}

func newPhotoCmd() kit.Command {
	var dir string
	return kit.Command{
		Use:   "photo <fbid|url>",
		Short: "Show a photo permalink",
		Long: "photo reads a photo whole: the image and its alt text, the owner, the album, the post it " +
			"belongs to, the neighbours in the album, the tagged profiles and the comments the " +
			"permalink ships. --download writes the file and a .json sidecar beside it.",
		Args: kit.ExactArgs(1),
		Flags: func(f *kit.FlagSet) {
			f.StringVar(&dir, "download", "", "write the image and its sidecar into this directory")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			p, err := eng.Photo(a.ctx(), args[0])
			if err != nil {
				return a.done(err)
			}
			a.warnMissed(p.Envelope)
			if dir != "" {
				d, err := eng.DownloadPhoto(a.ctx(), p, dir)
				if err != nil {
					return a.done(err)
				}
				return a.done(a.emitOne(downloadRow(d)))
			}
			return a.done(a.emitOne(photoRow(p)))
		},
	}
}

// emitPhotos turns a list of photos into rows, writing the files first when
// there is a directory to write them into.
//
// It stops at the first download that fails and hands back what it wrote along
// with the error, rather than swallowing it and printing a list that reads as
// though every file arrived. The caller prints those rows and then exits on the
// error, so a half-finished walk says so both ways.
func emitPhotos(a *App, eng *fb.Engine, photos []fb.Photo, dir string) ([]Row, error) {
	rows := make([]Row, 0, len(photos))
	for _, p := range photos {
		a.warnMissed(p.Envelope)
		if dir == "" {
			rows = append(rows, photoRow(p))
			continue
		}
		d, err := eng.DownloadPhoto(a.ctx(), p, dir)
		if err != nil {
			return rows, err
		}
		rows = append(rows, downloadRow(d))
	}
	return rows, nil
}

func newPhotosCmd() kit.Command {
	var album, dir string
	return kit.Command{
		Use:   "photos <handle|id>",
		Short: "A profile's photo tab",
		Long: "photos reads a profile's photo tab, which pages on a cursor. --album walks a set instead, " +
			"one request per photo, following the neighbour chain the viewer's arrows use. Give it any " +
			"photo in the set: the album's own page carries nothing to a signed-out reader, and `fb photo " +
			"<id>` prints the album id so you can tell which set you are in.",
		Args: kit.MaximumNArgs(1),
		Flags: func(f *kit.FlagSet) {
			f.StringVar(&album, "album", "", "walk the set this photo belongs to, rather than the photo tab")
			f.StringVar(&dir, "download", "", "write every image and its sidecar into this directory")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			if album == "" && len(args) == 0 {
				return mapErr(fb.Usage("give photos a profile to read the photo tab, or --album with any photo from the set to walk it"))
			}
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			if album != "" {
				a.target = album
				photos, err := eng.Album(a.ctx(), album, a.limit)
				if err != nil {
					return a.done(err)
				}
				rows, err := emitPhotos(a, eng, photos, dir)
				if e := a.emit(rows); e != nil {
					return a.done(e)
				}
				return a.done(err)
			}
			a.target = args[0]
			s, err := eng.Photos(a.ctx(), args[0], a.limit)
			if err != nil {
				return a.done(err)
			}
			a.warnMissed(s.Envelope)
			if dir != "" {
				rows, err := emitPhotos(a, eng, s.Photos, dir)
				if e := a.emit(rows); e != nil {
					return a.done(e)
				}
				return a.done(err)
			}
			return a.done(a.emit(sectionRows(s)))
		},
	}
}

// newVideoCmd builds `fb video` and `fb reel`. They are one read: a reel is a
// video with a vertical aspect and a music track, and the same media id serves
// both routes.
func newVideoCmd(name string) kit.Command {
	var transcript bool
	var dir string
	short := "Show a video or a reel"
	if name == "reel" {
		short = "Show a reel"
	}
	return kit.Command{
		Use:   name + " <id|url>",
		Short: short,
		Long: "video reads the reel route first, which carries the media URL, the dimensions, the " +
			"captions and the music. --transcript fetches the watch route as well, which is where " +
			"Facebook publishes its own machine transcript and the only long-form text a video has.",
		Args: kit.ExactArgs(1),
		Flags: func(f *kit.FlagSet) {
			f.BoolVar(&transcript, "transcript", false, "fetch the watch route for the transcript")
			f.StringVar(&dir, "download", "", "write the video and its sidecar into this directory, HD if there is one")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			v, err := eng.Video(a.ctx(), args[0], transcript)
			if err != nil {
				return a.done(err)
			}
			a.warnMissed(v.Envelope)
			if dir != "" {
				d, err := eng.DownloadVideo(a.ctx(), v, dir)
				if err != nil {
					return a.done(err)
				}
				return a.done(a.emitOne(downloadRow(d)))
			}
			return a.done(a.emitOne(videoRow(v)))
		},
	}
}

// newVideosCmd reads the video tab, which answers with two things and not one:
// the grid, and the show playlists beside it. Both go in the list, and
// --playlists prints the shows instead so it is visible which episode came
// from where.
func newVideosCmd() kit.Command {
	var playlists bool
	return kit.Command{
		Use:   "videos <handle|id>",
		Short: "A profile's video tab, grid and shows together",
		Args:  kit.ExactArgs(1),
		Flags: func(f *kit.FlagSet) {
			f.BoolVar(&playlists, "playlists", false, "print the shows rather than the videos")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			s, err := eng.Videos(a.ctx(), args[0], a.limit)
			if err != nil {
				return a.done(err)
			}
			a.warnMissed(s.Envelope)
			if playlists {
				if len(s.Playlists) == 0 {
					return a.done(fb.NoResults("%s has no shows beside its video grid", args[0]))
				}
				rows := make([]Row, 0, len(s.Playlists))
				for _, p := range s.Playlists {
					rows = append(rows, playlistRow(p))
				}
				return a.done(a.emit(rows))
			}
			return a.done(a.emit(sectionRows(s)))
		},
	}
}

// newGroupCmd is `fb group <id>` with `fb group feed <id>` under it. The feed
// is the deepest free read in the tool: the discussion query is not on the
// refused list, so its cursor can actually be spent signed out.
func newGroupCmd() kit.Command {
	return kit.Command{
		Use:   "group <id|slug>",
		Short: "Show a public group",
		Long: "group reads a group by numeric id signed out, and by slug as well with a session. " +
			"`fb group feed <id>` reads the discussion, as deep as --limit asks for.",
		Args: kit.ExactArgs(1),
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			g, err := eng.Group(a.ctx(), args[0])
			if err != nil {
				return a.done(err)
			}
			a.warnMissed(g.Envelope)
			return a.done(a.emitOne(groupRow(g)))
		},
		Sub: []kit.Command{newGroupFeedCmd()},
	}
}

func newGroupFeedCmd() kit.Command {
	return kit.Command{
		Use:   "feed <id|slug>",
		Short: "A group's discussion, as deep as asked",
		Args:  kit.ExactArgs(1),
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			g, err := eng.GroupFeed(a.ctx(), args[0], a.limit)
			if err != nil {
				return a.done(err)
			}
			a.warnMissed(g.Envelope)
			rows := make([]Row, 0, len(g.Posts))
			for _, p := range g.Posts {
				rows = append(rows, postRow(p))
			}
			return a.done(a.emit(rows))
		},
	}
}

func newEventCmd() kit.Command {
	var suggested bool
	return kit.Command{
		Use:   "event <id|url>",
		Short: "Show a public event",
		Long: "event reads an event permalink. The RSVP counts are not on the permalink's Relay data at " +
			"all, so fb takes them from the meta description when it can and prints n/a when it " +
			"cannot, rather than a blank that would read as nobody going.",
		Args: kit.ExactArgs(1),
		Flags: func(f *kit.FlagSet) {
			f.BoolVar(&suggested, "suggested", false, "print the events Facebook suggests alongside")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			e, err := eng.Event(a.ctx(), args[0])
			if err != nil {
				return a.done(err)
			}
			a.warnMissed(e.Envelope)
			if suggested {
				if len(e.Suggested) == 0 {
					return a.done(fb.NoResults("event %s has no suggestions beside it", e.ID))
				}
				rows := make([]Row, 0, len(e.Suggested))
				for _, c := range e.Suggested {
					rows = append(rows, eventCardRow(c))
				}
				return a.done(a.emit(rows))
			}
			return a.done(a.emitOne(eventRow(e)))
		},
	}
}

func newEventsCmd() kit.Command {
	return kit.Command{
		Use:   "events <handle|id>",
		Short: "A profile's events tab",
		Args:  kit.ExactArgs(1),
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			s, err := eng.Events(a.ctx(), args[0], a.limit)
			if err != nil {
				return a.done(err)
			}
			a.warnMissed(s.Envelope)
			return a.done(a.emit(sectionRows(s)))
		},
	}
}

// newDiscoverCmd reads the Pages directory as a seed source. The index answers
// signed out and the letter pages do not, and the difference matters: a blocked
// letter exits 4, not 3, because there is no evidence that no page starts with
// N, only evidence that we were not allowed to look.
func newDiscoverCmd() kit.Command {
	var letter string
	return kit.Command{
		Use:   "discover",
		Short: "The Pages directory, as a seed source",
		Args:  kit.NoArgs,
		Flags: func(f *kit.FlagSet) {
			f.StringVar(&letter, "letter", "", "one letter page of the directory")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = "directory"
			if letter != "" {
				a.target = "directory/" + letter
			}
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			d, err := eng.Discover(a.ctx(), letter)
			if err != nil {
				return a.done(err)
			}
			a.warnMissed(d.Envelope)
			if len(d.Entries) > 0 {
				rows := make([]Row, 0, len(d.Entries))
				for _, e := range d.Entries {
					rows = append(rows, directoryRow(e))
				}
				return a.done(a.emit(rows))
			}
			rows := make([]Row, 0, len(d.Index))
			for _, t := range d.Index {
				rows = append(rows, tabRow(t))
			}
			return a.done(a.emit(rows))
		},
	}
}

func newSearchCmd() kit.Command {
	return kit.Command{
		Use:   "search <query>",
		Short: "Search Facebook (needs a session)",
		Long: "search needs a session. Facebook answers a signed-out search with 404, so this exits 4 " +
			"with the sentence that would fix it rather than pretending a partial answer exists.",
		Args: kit.MinimumNArgs(1),
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = args[0]
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			return a.done(eng.Search(a.ctx(), joinArgs(args)))
		},
	}
}

func joinArgs(args []string) string {
	out := ""
	for i, s := range args {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}
