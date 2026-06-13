package cli

import (
	"strconv"
	"strings"

	"github.com/tamnd/facebook-cli/fb"
	"github.com/tamnd/facebook-cli/pkg/fbid"
)

func i64(v int64) string { return strconv.FormatInt(v, 10) }

func pageRow(p *fb.Page) Row {
	return Row{
		Cols:  []string{"page_id", "name", "category", "likes", "followers", "verified", "website", "url"},
		Vals:  []string{p.PageID, p.Name, p.Category, i64(p.LikesCount), i64(p.FollowersCount), strconv.FormatBool(p.Verified), p.Website, p.URL},
		Value: p,
	}
}

func profileRow(p *fb.Profile) Row {
	return Row{
		Cols:  []string{"profile_id", "username", "name", "followers", "work", "education", "url"},
		Vals:  []string{p.ProfileID, p.Username, p.Name, i64(p.FollowersCount), p.Work, p.Education, p.URL},
		Value: p,
	}
}

func groupRow(g *fb.Group) Row {
	return Row{
		Cols:  []string{"group_id", "name", "privacy", "members", "url"},
		Vals:  []string{g.GroupID, g.Name, g.Privacy, i64(g.MembersCount), g.URL},
		Value: g,
	}
}

func postRow(p *fb.Post) Row {
	return Row{
		Cols:  []string{"post_id", "owner_name", "created", "likes", "comments", "shares", "text", "permalink"},
		Vals:  []string{p.PostID, p.OwnerName, p.CreatedAtText, i64(p.LikeCount), i64(p.CommentCount), i64(p.ShareCount), truncate(p.Text, 80), p.Permalink},
		Value: p,
	}
}

func commentRow(c *fb.Comment) Row {
	return Row{
		Cols:  []string{"comment_id", "author_name", "created", "likes", "text", "parent_id"},
		Vals:  []string{c.CommentID, c.AuthorName, c.CreatedAtText, i64(c.LikeCount), truncate(c.Text, 100), c.ParentID},
		Value: c,
	}
}

func reactionRow(r *fb.Reaction) Row {
	return Row{
		Cols:  []string{"type", "reactor_name", "reactor_url"},
		Vals:  []string{r.Type, r.ReactorName, r.ReactorURL},
		Value: r,
	}
}

func reactionSummaryRow(s *fb.ReactionSummary) Row {
	return Row{
		Cols:  []string{"like", "love", "care", "haha", "wow", "sad", "angry", "total", "source"},
		Vals:  []string{i64(s.Like), i64(s.Love), i64(s.Care), i64(s.Haha), i64(s.Wow), i64(s.Sad), i64(s.Angry), i64(s.Total), s.Source},
		Value: s,
	}
}

func photoRow(p *fb.Photo) Row {
	return Row{
		Cols:  []string{"photo_id", "caption", "full_url", "thumb_url"},
		Vals:  []string{p.PhotoID, truncate(p.Caption, 60), p.FullURL, p.ThumbURL},
		Value: p,
	}
}

func videoRow(v *fb.Video) Row {
	return Row{
		Cols:  []string{"video_id", "title", "is_reel", "permalink"},
		Vals:  []string{v.VideoID, truncate(v.Title, 60), strconv.FormatBool(v.IsReel), v.Permalink},
		Value: v,
	}
}

func streamRow(s *fb.Stream) Row {
	return Row{
		Cols:  []string{"quality", "mime", "url"},
		Vals:  []string{s.Quality, s.MIME, s.URL},
		Value: s,
	}
}

func eventRow(e *fb.Event) Row {
	return Row{
		Cols:  []string{"event_id", "name", "start", "going", "interested", "url"},
		Vals:  []string{e.EventID, e.Name, e.StartText, i64(e.GoingCount), i64(e.InterestedCount), e.URL},
		Value: e,
	}
}

func searchRow(r *fb.SearchResult) Row {
	return Row{
		Cols:  []string{"result_type", "title", "url"},
		Vals:  []string{r.ResultType, truncate(r.Title, 70), r.URL},
		Value: r,
	}
}

func identityRow(id fbid.Identity) Row {
	return Row{
		Cols:  []string{"kind", "id", "slug", "canonical_url"},
		Vals:  []string{string(id.Kind), firstNonEmpty(id.PageID, id.ProfileID, id.GroupID, id.PostID, id.PhotoID, id.VideoID, id.EventID), id.Slug, id.CanonicalURL},
		Value: id,
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
