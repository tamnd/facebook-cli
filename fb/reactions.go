package fb

// reactions.go names reactions and reads the feedback block.
//
// Facebook identifies a reaction by a numeric id, not by a name. The top-level
// top_reactions carry a localized_name alongside; a comment's do not. So the
// table below is the authority and localized_name is used to check it, not the
// other way round.

// reactionNames maps Facebook's reaction ids to names.
//
// Every entry here was checked against the localized_name Facebook sent
// alongside it in the captures, and TestReactionTableMatchesFixtures keeps it
// that way. That check is not ceremony: the first draft of this table had Haha
// and Wow swapped and the wrong id for Angry, and nothing about the output
// looked wrong, because a reaction breakdown with plausible numbers under the
// wrong labels reads exactly like a correct one.
var reactionNames = map[string]string{
	"1635855486666999": "Like",
	"1678524932434102": "Love",
	"115940658764963":  "Haha",
	"478547315650144":  "Wow",
	"908563459236466":  "Sad",
	"444813342392137":  "Angry",
	"613557422527858":  "Care",
}

// reactionOrder is the order a breakdown is printed in: most common first,
// which is also the order Facebook's own picker uses.
var reactionOrder = []string{"Like", "Love", "Care", "Haha", "Wow", "Sad", "Angry"}

// reactionName resolves an id, falling back to the localized name Facebook sent
// and then to the id itself. An id we do not know is reported by the live test
// rather than silently dropped, so a new reaction shows up as a failing test
// and not as a blank column.
func reactionName(id, localized string) string {
	if n, ok := reactionNames[id]; ok {
		return n
	}
	if localized != "" {
		return localized
	}
	return id
}

// feedbackCounts reads a feedback node and the UFI renderer nested inside it,
// and merges the two.
//
// Neither one is complete on its own. On a photo the reaction breakdown is on
// the outer feedback and the comment total is only inside the renderer; on a
// feed story it is the other way round. Reading one and not the other is how a
// photo ends up reported with 12,369 reactions and no breakdown.
func feedbackCounts(fb map[string]any) Counts {
	c := parseFeedback(fb)
	if r := findKey(fb, "comet_ufi_summary_and_actions_renderer"); r != nil {
		c = c.fill(parseFeedback(digMap(r, "comet_ufi_summary_and_actions_renderer", "feedback")))
	}
	return c
}

// fill takes each zero field from another Counts, leaving the set ones alone.
func (c Counts) fill(o Counts) Counts {
	if c.Reactions == 0 {
		c.Reactions = o.Reactions
	}
	if c.ReactionsText == "" {
		c.ReactionsText = o.ReactionsText
	}
	if c.Comments == 0 {
		c.Comments = o.Comments
	}
	if c.Shares == 0 {
		c.Shares = o.Shares
	}
	if c.SharesText == "" {
		c.SharesText = o.SharesText
	}
	if c.Views == 0 {
		c.Views = o.Views
	}
	if c.Plays == 0 {
		c.Plays = o.Plays
	}
	if len(c.ByType) == 0 {
		c.ByType = o.ByType
	}
	return c
}

// parseFeedback reads a feedback node into Counts.
//
// The node is reached by a different path on every object, so the callers dig
// and this reads what they found.
func parseFeedback(fb map[string]any) Counts {
	if fb == nil {
		return Counts{}
	}
	c := Counts{
		Reactions:     digInt(fb, "reaction_count", "count"),
		ReactionsText: digStr(fb, "i18n_reaction_count"),
		Shares:        digInt(fb, "share_count", "count"),
		SharesText:    digStr(fb, "i18n_share_count"),
		Views:         digInt(fb, "video_view_count"),
	}
	if c.Reactions == 0 {
		// A photo's feedback counts reactors instead of reactions.
		c.Reactions = digInt(fb, "unified_reactors", "count")
	}
	if c.Reactions == 0 {
		c.Reactions = digInt(fb, "reactors", "count")
	}
	if n := digInt(fb, "total_comment_count"); n > 0 {
		c.Comments = n
	}
	for _, p := range [][]string{
		{"comment_rendering_instance", "comments", "total_count"},
		{"comment_rendering_instance_for_feed_location", "comments", "total_count"},
		{"comment_count", "total_count"},
		{"comments", "total_count"},
	} {
		if n := digInt(fb, p...); n > 0 {
			c.Comments = n
			break
		}
	}
	for _, e := range digMaps(fb, "top_reactions", "edges") {
		id := digStr(e, "node", "id")
		n := digInt(e, "reaction_count")
		if n == 0 {
			continue
		}
		if c.ByType == nil {
			c.ByType = map[string]int{}
		}
		// Facebook sends only the reactions with a non-zero count, so an
		// absent key means zero and fb does not invent the missing ones.
		c.ByType[reactionName(id, digStr(e, "node", "localized_name"))] = n
	}
	return c
}
