---
title: "Groups and events"
description: "The public shell of a group and its discussion, and events from the tab or from a permalink."
weight: 50
---

## Groups

```sh
fb group 1443890352589739
fb group feed 1443890352589739 -n 50
```

Note the order: the subcommand comes before the id.
`fb group feed <id>`, not `fb group <id> feed`.

Signed out, a public group's shell reads fine:

```
id, name, url
privacy               "Public group"
visible               whether a signed-out reader can see the discussion
members, members_text 1600, and "1.6K members"
description           the About text, with entities
cover, avatar, theme_color
tabs                  About, Discussion, Featured, and the rest
join_state            CAN_JOIN
posts, posts_cursor   the discussion, when it is served
```

A numeric id always works.
A slug needs a session, because the slug-to-id lookup is behind the wall.

### The group route gets walled early

This is the one place where signed-out reading is genuinely fragile, and it is worth being blunt about it.

Facebook cuts a signed-out reader off from `/groups/` after a few dozen requests, whatever the group, and it stays cut off for a good while.
fb says exactly that rather than blaming your input:

```
Facebook answered every request for group 1443890352589739 with the log-in page.
It cuts a signed-out reader off from the group route after a few dozen requests, whatever the
group, and it does not lift for a while: wait, or import a session with `fb auth import`.
```

Exit 4.
Waiting works.
A session works better, and it is the one read where the two cookies really earn their keep.

`fb group feed` has one flag worth knowing: `--resolve-opaque`.
A group discussion often gives post authors as `pfbid` tokens rather than ids, and that flag spends a request each to turn them into real profiles so the posts have an author you can key on.
It is off by default because it multiplies the request count.

## Events

```sh
fb events nasa -n 3 --fields id,name,start
```

The events tab, eight per page, cursor-paged.
It carries past events as well as upcoming ones, so a page with nothing coming up is not an empty read.

```
╭──────────────────┬───────────────────────────────────────────────────────────────────┬──────────────────╮
│ ID               │ NAME                                                              │ START            │
├──────────────────┼───────────────────────────────────────────────────────────────────┼──────────────────┤
│ 1526795995494848 │ NASA's Artemis III Announcement                                   │ 2026-06-09 15:30 │
│ 1435272301191667 │ NASA's Artemis II Crew Comes Home                                 │ 2026-04-10 22:30 │
│ 1288387823256240 │ NASA's Artemis II Crew Flies Around the Moon (Official Broadcast) │ 2026-04-06 17:00 │
╰──────────────────┴───────────────────────────────────────────────────────────────────┴──────────────────╯
```

### One event

```sh
fb event 1526795995494848
fb event 1526795995494848 --suggested
```

```
╭──────────────────┬─────────────────────────────────┬──────────────────┬──────────────┬────────────┬───────╮
│ ID               │ NAME                            │ START            │ PLACE        │ INTERESTED │ GOING │
├──────────────────┼─────────────────────────────────┼──────────────────┼──────────────┼────────────┼───────┤
│ 1526795995494848 │ NASA's Artemis III Announcement │ 2026-06-09 15:30 │ Online event │ 2,300      │ 324   │
╰──────────────────┴─────────────────────────────────┴──────────────────┴──────────────┴────────────┴───────╯
```

The record has the host and every co-host, the host's page id, the start with its timezone and Facebook's own rendered sentence for it, the place, whether it is online and what kind, whether it has happened, the description with entities, the cover, the discovery flags, and the frequency.

`--suggested` adds the events Facebook recommends alongside it, which is a decent discovery hop.

### The RSVP counts come from somewhere else

`interested` and `going` are not in the event permalink's Relay data at all.
Facebook puts them in the meta description instead, in a sentence:

```
Event by NASA - National Aeronautics and Space Administration on Tuesday, June 9 2026
with 2.3K people interested and 324 people going.
```

fb parses that, and the `via` field records where each number came from:

```json
"via": {"going": "s3", "interested": "s3"}
```

Which means `interested` is 2,300 rather than 2,347: the meta sentence rounds and there is no unrounded copy on this surface.
When the sentence is not there, fb prints `n/a` rather than a blank, because a blank in a GOING column reads as nobody going.
