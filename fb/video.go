package fb

// video.go parses a video and a reel.
//
// One type for both, because they are one object: the same media id serves
// /watch/?v=1380134307388381 and /reel/1380134307388381, and the difference is
// which operation the route runs.
//
// The two routes are not equally generous, and this is the most useful thing in
// this file. /watch signed out ships CometVideoHomeNewPermalinkHeroUnitQuery,
// which has the id, the title, the owner and the counts and nothing else: no
// duration, no dimensions, no playable URL. /reel ships
// FBReelsRootWithEntrypointQuery, which has all of that plus the dash manifest,
// the captions track and the music. So `fb video` reads the reel route first
// and falls back to watch, and the record says which one answered.
//
// The one thing only /watch has is the transcript, in the SEO payload.

// parseVideo builds a video from whichever of the three operations came back.
func parseVideo(docs map[string]*Document) Video {
	var v Video
	if d := docs["FBReelsRootWithEntrypointQuery"]; d != nil {
		v.addSurface(surfaceComet)
		applyReel(&v, d.Data)
	}
	if d := docs["CometVideoHomeNewPermalinkHeroUnitQuery"]; d != nil {
		v.addSurface(surfaceComet)
		applyWatch(&v, d.Data)
	}
	if d := docs["CometVideoHomeLOEVideoPermalinkAuxiliaryRootQuery"]; d != nil {
		v.Transcript = digStr(d.Data, "video_home_www_loe_video_permalink_seo_info", "transcript")
	}
	return v
}

// applyReel reads the reel route, which is the one that carries the media.
func applyReel(v *Video, data any) {
	story := digMap(data, "video", "creation_story")
	if story == nil {
		return
	}
	v.Kind = "reel"
	v.ID = digStr(data, "video", "id")
	v.PostID = digStr(story, "post_id")
	v.CreatedAt = digTime(story, "creation_time")
	v.Message = parseText(digMap(story, "message"))
	v.Privacy = digStr(story, "privacy_scope", "label")

	ctx := digMap(story, "short_form_video_context")
	if o := digMap(ctx, "video_owner"); o != nil {
		v.Owner = parseRef(o)
		v.Owner.Kind = "profile"
	}
	if v.Owner.Empty() {
		v.Owner = parseRef(digMap(ctx, "video", "owner"))
	}
	if art := digStr(ctx, "music_album_art_uri"); art != "" {
		v.Music = &Music{AlbumArt: art}
	}
	v.URL = canonURL(firstStr(ctx, "shareable_url"))

	pb := digMap(ctx, "playback_video")
	if pb != nil {
		v.Width = digInt(pb, "width")
		v.Height = digInt(pb, "height")
		v.Duration = digFloat(pb, "length_in_second")
		v.LoopCount = digInt(pb, "loop_count")
		v.Thumbnail = parseImage(digMap(pb, "thumbnailImage"))
		if v.Thumbnail.Empty() {
			v.Thumbnail = parseImage(digMap(pb, "preferred_thumbnail", "image"))
		}
		if v.URL == "" {
			v.URL = canonURL(digStr(pb, "permalink_url"))
		}
		legacy := digMap(pb, "videoDeliveryLegacyFields")
		v.HDURL = digStr(legacy, "browser_native_hd_url")
		v.SDURL = digStr(legacy, "browser_native_sd_url")
		v.DashURL = digStr(legacy, "dash_manifest_url")
		for _, c := range digMaps(pb, "video_available_captions_locales") {
			v.Captions = append(v.Captions, Caption{
				Locale: digStr(c, "locale"),
				Name:   digStr(c, "localized_unambiguous_language"),
				URL:    digStr(c, "captions_url"),
			})
		}
	}
	if d := digInt(ctx, "video", "playable_duration_in_ms"); v.Duration == 0 && d > 0 {
		v.Duration = float64(d) / 1000
	}
	v.Counts = reelCounts(story)
}

// reelCounts reads the reel's feedback, which is spread over three nodes.
//
// The reaction total is on the react button's feedback as likers.count, the
// comment total is on the story's own feedback, and the share count arrives as
// a string in share_count_reduced. None of the three has the other two.
func reelCounts(story map[string]any) Counts {
	c := feedbackCounts(digMap(story, "feedback"))
	btn := digMap(story, "fb_reel_react_button", "story", "feedback")
	c = c.fill(parseFeedback(btn))
	if c.Reactions == 0 {
		c.Reactions = digInt(btn, "likers", "count")
	}
	if c.Shares == 0 {
		c.Shares = digInt(story, "feedback", "share_count_reduced")
	}
	return c
}

// applyWatch reads the /watch route, filling in what the reel route did not
// have and leaving what it did.
func applyWatch(v *Video, data any) {
	story := digMap(data, "video", "story")
	if story == nil {
		return
	}
	if v.ID == "" {
		v.ID = digStr(data, "video", "id")
	}
	var media map[string]any
	if atts := digMaps(story, "attachments"); len(atts) > 0 {
		media = digMap(atts[0], "media")
	}
	if media == nil {
		return
	}
	if v.Kind == "" {
		v.Kind = "video"
	}
	if v.Title == "" {
		v.Title = digStr(media, "title", "text")
	}
	if v.Owner.Empty() {
		v.Owner = parseRef(digMap(media, "owner"))
	}
	if v.PostID == "" {
		v.PostID = digStr(story, "post_id")
	}
	v.IsLive = digStr(media, "live_status") == "LIVE"
	v.Counts = v.Counts.fill(feedbackCounts(digMap(media, "feedback")))
}
