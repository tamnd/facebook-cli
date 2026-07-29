package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/facebook-cli/fb"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

// rows.go turns a record into a row: a curated column set for the eye, the
// whole typed object for everything else.
//
// The columns are chosen the way the examples in doc 05 are written, which is
// to say by asking what somebody typing the command wanted to know. Nothing is
// dropped by choosing them: -o json prints the record whole, and --fields picks
// any of it.

func profileRow(p fb.Profile) Row {
	cols := []string{"id", "handle", "name", "kind", "verified", "likes", "followers", "url"}
	vals := []string{
		p.ID, p.Handle, oneline(p.Name), p.Kind, yn(p.Verified),
		count(p.Likes), count(p.Followers), p.URL,
	}
	if len(p.Category) > 0 {
		cols = append(cols, "category")
		vals = append(vals, strings.Join(p.Category, ", "))
	}
	if len(p.Posts) > 0 {
		cols = append(cols, "posts")
		vals = append(vals, strconv.Itoa(len(p.Posts)))
	}
	return Row{Cols: cols, Vals: vals, Value: p}
}

func postRow(p fb.Post) Row {
	return Row{
		Cols: []string{"id", "created", "author", "reactions", "comments", "shares", "message", "url"},
		Vals: []string{
			p.ID, stamp(p.CreatedAt), oneline(p.Author.Name),
			count(p.Counts.Reactions), count(p.Counts.Comments), count(p.Counts.Shares),
			oneline(p.Message.Text), p.URL,
		},
		Value: p,
	}
}

func commentRow(c fb.Comment) Row {
	return Row{
		Cols: []string{"id", "created", "author", "reactions", "replies", "body", "url"},
		Vals: []string{
			c.ID, stamp(c.CreatedAt), oneline(c.Author.Name),
			count(c.Counts.Reactions), count(c.Replies), oneline(c.Body.Text), c.URL,
		},
		Value: c,
	}
}

// reactionRow is the breakdown as a table, which is the shape people want in a
// spreadsheet: one row per type, with its share of the total.
func reactionRow(kind string, n, total int) Row {
	share := ""
	if total > 0 {
		share = fmt.Sprintf("%.3f", float64(n)/float64(total))
	}
	return Row{
		Cols:  []string{"type", "count", "share"},
		Vals:  []string{kind, strconv.Itoa(n), share},
		Value: map[string]any{"type": kind, "count": n, "share": share},
	}
}

func photoRow(p fb.Photo) Row {
	size := ""
	if p.Image.Width > 0 {
		size = fmt.Sprintf("%dx%d", p.Image.Width, p.Image.Height)
	}
	return Row{
		Cols: []string{"id", "owner", "size", "reactions", "comments", "alt", "url"},
		Vals: []string{
			p.ID, oneline(p.Owner.Name), size,
			count(p.Counts.Reactions), count(p.Counts.Comments),
			oneline(p.Image.Alt), p.URL,
		},
		Value: p,
	}
}

// downloadRow is what a --download prints instead of the record: where the file
// landed and how big it is. The record itself is not lost, it is in the sidecar
// beside the file.
//
// The source URL is in the JSON and out of the columns. It is five hundred
// characters of signature and it stops working within the hour, so in a table it
// is a wall that hides the two things somebody is actually reading.
func downloadRow(d fb.Downloaded) Row {
	return Row{
		Cols:  []string{"id", "path", "bytes", "quality"},
		Vals:  []string{d.ID, d.Path, strconv.FormatInt(d.Bytes, 10), d.Quality},
		Value: d,
	}
}

func videoRow(v fb.Video) Row {
	size := ""
	if v.Width > 0 {
		size = fmt.Sprintf("%dx%d", v.Width, v.Height)
	}
	return Row{
		Cols: []string{"id", "created", "owner", "duration", "size", "plays", "reactions", "title", "url"},
		Vals: []string{
			v.ID, stamp(v.CreatedAt), oneline(v.Owner.Name), secs(v.Duration), size,
			count(v.Counts.Plays), count(v.Counts.Reactions),
			oneline(videoTitle(v)), v.URL,
		},
		Value: v,
	}
}

// videoTitle is the title if the video has one and the first line of the
// message if it does not, because a tab item usually has only the message and a
// column of blanks says nothing.
func videoTitle(v fb.Video) string {
	if v.Title != "" {
		return v.Title
	}
	return v.Message.Text
}

func groupRow(g fb.Group) Row {
	cols := []string{"id", "name", "privacy", "members", "url"}
	vals := []string{g.ID, oneline(g.Name), g.Privacy, count(g.Members), g.URL}
	if g.Members == 0 && g.MembersText != "" {
		vals[3] = g.MembersText
	}
	if len(g.Posts) > 0 {
		cols = append(cols, "posts")
		vals = append(vals, strconv.Itoa(len(g.Posts)))
	}
	return Row{Cols: cols, Vals: vals, Value: g}
}

func eventRow(e fb.Event) Row {
	return Row{
		Cols: []string{"id", "name", "start", "place", "host", "interested", "going", "url"},
		Vals: []string{
			e.ID, oneline(e.Name), stamp(e.Start), placeName(e.Place), oneline(e.Host.Name),
			maybeCount(e.Interested), maybeCount(e.Going), e.URL,
		},
		Value: e,
	}
}

func eventCardRow(c fb.EventCard) Row {
	when := stamp(c.Start)
	if when == "" {
		when = c.WhenText
	}
	return Row{
		Cols: []string{"id", "name", "start", "place", "interested", "going", "url"},
		Vals: []string{
			c.ID, oneline(c.Name), when, placeName(c.Place),
			count(c.Interested), count(c.Going), c.URL,
		},
		Value: c,
	}
}

func playlistRow(p fb.Playlist) Row {
	return Row{
		Cols:  []string{"id", "title", "videos", "description"},
		Vals:  []string{p.ID, oneline(p.Title), strconv.Itoa(len(p.VideoIDs)), oneline(p.Description)},
		Value: p,
	}
}

func directoryRow(e fb.DirectoryEntry) Row {
	return Row{
		Cols:  []string{"name", "letter", "url"},
		Vals:  []string{oneline(e.Name), e.Letter, e.URL},
		Value: e,
	}
}

func tabRow(t fb.Tab) Row {
	return Row{
		Cols:  []string{"name", "url"},
		Vals:  []string{oneline(t.Name), t.URL},
		Value: t,
	}
}

// sectionRows renders whichever list a profile tab filled. A section carries
// one kind of thing, so this is a switch and not a merge.
func sectionRows(s fb.Section) []Row {
	var rows []Row
	for _, p := range s.Photos {
		rows = append(rows, photoRow(p))
	}
	for _, e := range s.Events {
		rows = append(rows, eventCardRow(e))
	}
	for _, v := range s.Videos {
		rows = append(rows, videoRow(v))
	}
	return rows
}

func surfaceRow(s fb.Surface) Row {
	return Row{
		Cols:  []string{"id", "name", "host", "tier", "format"},
		Vals:  []string{s.ID, s.Name, s.Host, strconv.Itoa(s.Tier), s.Format},
		Value: s,
	}
}

func routeRow(r fb.Route) Row {
	return Row{
		Cols: []string{"question", "tier0", "tier1", "note"},
		Vals: []string{
			r.Question, strings.Join(r.Tier0, " "), strings.Join(r.Tier1, " "), r.Note,
		},
		Value: r,
	}
}

func statusRow(s fb.Status) Row {
	return Row{
		Cols:  []string{"present", "account", "imported", "path"},
		Vals:  []string{yn(s.Present), s.Account, stamp(s.ImportedAt), s.Path},
		Value: s,
	}
}

// count formats a number with thousands separators, and an empty string for
// zero. A zero engagement count is nearly always a count the surface did not
// carry, and printing 0 claims otherwise.
func count(n int) string {
	if n == 0 {
		return ""
	}
	s := strconv.Itoa(n)
	if n < 0 {
		return s
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// maybeCount renders a count that may not exist on this surface. nil is not
// zero: an event permalink does not carry the RSVP numbers at all, and a blank
// column would read as nobody going.
func maybeCount(n *int) string {
	if n == nil {
		return "n/a"
	}
	return count(*n)
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04")
}

func secs(d float64) string {
	if d <= 0 {
		return ""
	}
	total := int(d + 0.5)
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

func yn(b bool) string {
	if b {
		return "true"
	}
	return ""
}

// yesno is yn for a settings list rather than a table column. A blank cell in a
// table of profiles reads as "not verified", but a blank value next to a name
// reads as "fb did not fill this in", so a settings row says the word.
func yesno(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func placeName(p *fb.Place) string {
	if p == nil {
		return ""
	}
	if p.Name != "" {
		return p.Name
	}
	return p.Address
}

// oneline flattens a message for a table cell. The record keeps the newlines.
func oneline(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

func operationRow(o fb.Operation) Row {
	return Row{
		Cols:  []string{"command", "surface", "operation", "tier", "depth"},
		Vals:  []string{o.Command, o.Surface, o.Op, strconv.Itoa(o.Tier), o.Depth},
		Value: o,
	}
}

// explainRow is one thing fb would do with a URL: the command that handles it,
// the page it would fetch, and the operation it would read there.
func explainRow(r fbid.Ref, o fb.Operation) Row {
	return Row{
		Cols: []string{"command", "kind", "surface", "operation", "tier", "depth", "url"},
		Vals: []string{
			o.Command, r.Kind, o.Surface, o.Op, strconv.Itoa(o.Tier), o.Depth, r.URL,
		},
		Value: map[string]any{"ref": r, "route": o},
	}
}
