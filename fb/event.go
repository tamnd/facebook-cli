package fb

import (
	"regexp"
	"strings"
)

// event.go parses an event permalink.
//
// The permalink ships three operations. EventCometPermalinkHeaderQuery has the
// name, the cover, the times and the place. PublicEventCometAboutRootQuery has
// the description, the hosts, the flags and the suggestions.
// PublicEventCometRootQuery has only the discovery category list, which belongs
// to the events browser and not to this event, so it is read for nothing here.
//
// The counts are the interesting part. Neither Relay operation for the event
// you asked for carries an interested or a going number: they arrive on the
// *suggested* events beside it, and on directory cards. What does carry them
// for the target is og:description in the same page's head, reading "with 2.3K
// people interested and 324 people going". So the counts come off surface 3,
// via records that they came from there, and the sentence Facebook wrote is
// kept beside the parsed number because 2.3K is a measurement and 2300 is an
// inference.

var (
	// reSocialInterested and reSocialGoing read a card's social_context, which
	// reads "256 interested · 5 going" and drops either half when it is zero.
	reSocialInterested = regexp.MustCompile(`([\d.,KMB]+) interested`)
	reSocialGoing      = regexp.MustCompile(`([\d.,KMB]+) going`)
)

// parseEvent builds an event from the operations an event permalink ships.
func parseEvent(docs map[string]*Document, head Head) Event {
	var e Event
	if d := docs["EventCometPermalinkHeaderQuery"]; d != nil {
		e.addSurface(surfaceComet)
		applyEventCore(&e, digMap(d.Data, "event"))
		applyEventHeader(&e, digMap(d.Data, "event"))
	}
	if d := docs["PublicEventCometAboutRootQuery"]; d != nil {
		e.addSurface(surfaceComet)
		applyEventCore(&e, digMap(d.Data, "event"))
		applyEventAbout(&e, digMap(d.Data, "event"))
	}
	applyEventRSVP(&e, head)
	return e
}

// applyEventCore reads the fields both operations carry, so that either one
// alone is enough to name the event.
func applyEventCore(e *Event, ev map[string]any) {
	if ev == nil {
		return
	}
	if e.ID == "" {
		e.ID = digStr(ev, "id")
	}
	if e.Name == "" {
		e.Name = digStr(ev, "name")
	}
	if e.URL == "" {
		e.URL = canonURL(firstStr(ev, "url", "eventUrl"))
	}
	if e.Kind == "" {
		e.Kind = digStr(ev, "event_kind")
	}
	if e.Start.IsZero() {
		e.Start = digTime(ev, "start_timestamp")
	}
	// An end_timestamp of 0 means the event did not say when it ends, which is
	// not the same as ending at the epoch.
	if e.End.IsZero() {
		if n := digInt(ev, "end_timestamp"); n > 0 {
			e.End = digTime(ev, "end_timestamp")
		}
	}
	if !e.IsOnline {
		e.IsOnline = digBool(ev, "is_online")
	}
	if !e.IsPast {
		e.IsPast = digBool(ev, "is_past")
	}
	if !e.Canceled {
		e.Canceled = digBool(ev, "is_canceled")
	}
	if e.Place == nil {
		e.Place = parsePlace(digMap(ev, "event_place"))
	}

	parent := digMap(ev, "parent_if_exists_or_self")
	if e.OnlineKind == "" {
		e.OnlineKind = firstStr(parent, "online_event_setup.type")
		if e.OnlineKind == "" {
			e.OnlineKind = digStr(parent, "online_event_setup", "type")
		}
	}
	if e.OnlineURL == "" {
		e.OnlineURL = digStr(parent, "online_event_setup", "third_party_url")
	}
	if e.Frequency == "" {
		e.Frequency = digStr(parent, "event_frequency")
	}
	// parent_if_exists_or_self points at itself for a one-off, and a parent id
	// equal to the event id says nothing, so it is not recorded.
	if id := digStr(parent, "id"); e.ParentID == "" && id != "" && id != e.ID {
		e.ParentID = id
	}
	if e.ChildCount == 0 {
		e.ChildCount = digInt(parent, "child_events", "count")
	}

	if e.Transparency == nil {
		name, dateChanged := digBool(ev, "has_name_changed"), digBool(ev, "has_date_changed")
		if name || dateChanged {
			e.Transparency = &Transparency{NameChanged: name, DateChanged: dateChanged}
		}
	}
}

// applyEventHeader reads what only the header query has: the cover, the
// formatted time sentence and the owning page.
func applyEventHeader(e *Event, ev map[string]any) {
	if ev == nil {
		return
	}
	e.DayTimeSentence = digStr(ev, "day_time_sentence")
	e.RSVPStyle = digStr(ev, "rsvp_style")
	e.HostPageID = digStr(ev, "page_as_owner", "id")

	if c := digMap(ev, "cover_media_renderer", "cover_photo"); c != nil {
		photo := digMap(c, "photo")
		e.Cover = parseImage(digMap(photo, "full_image"))
		if e.Cover.Empty() {
			e.Cover = parseImage(digMap(photo, "viewer_image"))
		}
		e.Cover.Alt = digStr(photo, "accessibility_caption")
		e.Cover.Blurred = digStr(photo, "blurred_image", "uri")
		if f := digMap(c, "focus"); f != nil {
			e.Cover.Focus = &Focus{X: digFloat(f, "x"), Y: digFloat(f, "y")}
		}
	}
	if e.Host.Empty() {
		e.Host = parseRef(digMap(ev, "online_cta_renderer", "event", "event_creator"))
	}
}

// applyEventAbout reads the about payload: description, hosts, flags, timezone,
// the announcing story and the suggestions.
func applyEventAbout(e *Event, ev map[string]any) {
	if ev == nil {
		return
	}
	e.Description = parseText(digMap(ev, "event_description"))
	e.Timezone = digStr(ev, "tz_display_name")
	e.AnnouncedBy = digStr(ev, "creation_story", "id")

	if h := parseRef(digMap(ev, "event_creator")); !h.Empty() {
		e.Host = h
	}
	for _, h := range digMaps(ev, "event_hosts_that_can_view_guestlist") {
		if ref := parseRef(h); !ref.Empty() {
			e.Hosts = append(e.Hosts, ref)
		}
	}
	if e.Host.Empty() && len(e.Hosts) > 0 {
		e.Host = e.Hosts[0]
	}
	for _, f := range digMaps(ev, "event_flags") {
		label := digStr(f, "label")
		if label == "" {
			continue
		}
		e.Flags = append(e.Flags, Flag{Label: label, URL: canonURL(digStr(f, "uri"))})
	}
	for _, n := range edges(ev, "event_suggested_events") {
		if c := parseEventCard(n); c.ID != "" {
			e.Suggested = append(e.Suggested, c)
		}
	}
}

// applyEventRSVP takes the counts off the meta head, which is the only surface
// that has them for the event you asked for.
func applyEventRSVP(e *Event, head Head) {
	desc := head.Description
	if desc == "" {
		return
	}
	if n, _ := headCount(desc, reInterested); n > 0 {
		e.Interested = &n
		e.setVia("interested", surfaceOpenGraph)
	}
	if n, _ := headCount(desc, reGoing); n > 0 {
		e.Going = &n
		e.setVia("going", surfaceOpenGraph)
	}
	if e.Interested != nil || e.Going != nil {
		// The sentence is kept because "2.3K" is what Facebook measured and
		// 2300 is what fb inferred from it.
		e.RSVPText = desc
		e.addSurface(surfaceOpenGraph)
	}
}

// parseEventCard reads an event as it appears in a list.
func parseEventCard(n map[string]any) EventCard {
	if n == nil {
		return EventCard{}
	}
	c := EventCard{
		ID:       digStr(n, "id"),
		Name:     digStr(n, "name"),
		URL:      canonURL(firstStr(n, "url", "eventUrl")),
		Start:    digTime(n, "start_timestamp"),
		WhenText: firstStr(n, "capitalized_day_time_sentence", "day_time_sentence"),
		Kind:     digStr(n, "event_kind"),
		IsPast:   digBool(n, "is_past"),
		IsOnline: digBool(n, "is_online") || digBool(n, "is_online_or_detected_online"),
		Place:    parsePlace(digMap(n, "event_place")),
	}
	c.Interested = digInt(n, "interested_users_count")
	c.Image = parseImage(digMap(n, "suggestedProfilePicture"))
	if c.Image.Empty() {
		c.Image = parseImage(digMap(n, "cover_photo", "photo", "image"))
	}

	// social_context reads "256 interested · 5 going" and drops either half
	// when it is zero. Its interested number and interested_users_count
	// disagreed by five in the captures, so the field wins and the sentence is
	// only mined for what the field does not have.
	if s := digStr(n, "social_context", "text"); s != "" {
		c.SocialText = s
		if c.Interested == 0 {
			c.Interested, _ = headCount(s, reSocialInterested)
		}
		c.Going, _ = headCount(s, reSocialGoing)
	}
	return c
}

// parsePlace reads an event_place, which is three different things wearing one
// field name.
//
// An online event has a FreeformPlace with an empty id and the literal name
// "Online event". A physical one is a Page with a contextual_name, a city that
// is itself a node, and coordinates. A third shape names only a city. kind says
// which one this is rather than leaving the caller to guess from which fields
// happen to be set.
func parsePlace(m map[string]any) *Place {
	if m == nil {
		return nil
	}
	p := Place{
		Kind:    strings.ToLower(strings.TrimSuffix(digStr(m, "__typename"), "Place")),
		ID:      digStr(m, "id"),
		Name:    firstStr(m, "contextual_name", "name"),
		Address: digStr(m, "address", "street"),
	}
	switch digStr(m, "__typename") {
	case "Page":
		p.Kind = "page"
	case "FreeformPlace":
		p.Kind = "freeform"
	case "City":
		p.Kind = "city"
	}
	if c := digMap(m, "city"); c != nil {
		ref := Ref{Kind: "page", ID: digStr(c, "id"), Name: firstStr(c, "contextual_name", "name")}
		if !ref.Empty() {
			p.City = &ref
		}
	}
	if loc := digMap(m, "location"); loc != nil {
		lat, lng := digFloat(loc, "latitude"), digFloat(loc, "longitude")
		if lat != 0 || lng != 0 {
			p.Lat, p.Lng = &lat, &lng
		}
	}
	if p.ID == "" && p.Name == "" {
		return nil
	}
	return &p
}
