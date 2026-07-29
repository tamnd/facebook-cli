---
title: "Posts and comments"
description: "Read a post whole, with its message entities, its comments, and its reactions broken out by type."
weight: 30
---

A post permalink is the richest single fetch fb makes.
One request carries the message, the media, all three counts, the reaction breakdown by type, and the first page of comments, so `fb post`, `fb comments` and `fb reactions` are three views of the same read.

## Naming a post

Facebook has four ways of writing down the same post, and fb takes all of them:

```sh
fb post 'https://www.facebook.com/NASA/posts/1587860636042640/'
fb post 'https://www.facebook.com/permalink.php?id=100044561550831&story_fbid=1587860636042640'
fb post 'https://www.facebook.com/NASA/posts/pfbid0ikEX4Jf8...' 
fb post 1587860636042640 --author nasa
```

A bare post id on its own is a usage error rather than a guess, because Facebook has no route that takes one: a post id means nothing without the profile it belongs to.
The error says so and names the two ways to fix it.

A `pfbid` is an opaque per-viewer token rather than an id.
`fb id <pfbid url>` will tell you that, and `--resolve` spends a request to turn it into the real story id.

## The record

```sh
fb post 'https://www.facebook.com/NASA/posts/1587860636042640/' -o json
```

```
id                          1587860636042640
story_id                    the base64 story key, which is what the comment ids hang off
url                         the canonical permalink
author                      a Ref: kind, id, handle, name, url
created_at                  a real timestamp, not "2 days ago"
message                     text, plus ranges, mentions, links and styles
seo_title                   Facebook's own truncation, kept because it is sometimes all there is
counts                      reactions, by_type, comments, shares, and the rounded text forms
attachments                 photos, videos, shared links, shared posts
comments, comments_cursor   the comments the permalink shipped, and where to continue
```

`message` is worth a second look.
It is not just a string:

```json
{"text": "…", "ranges": [{"offset": 12, "length": 8, "entity": {"kind":"profile","id":"…","name":"…"}}],
 "links": [{"url":"https://go.nasa.gov/3RWg5e8","display":"go.nasa.gov/3RWg5e8","offset":215,"length":19}],
 "styles": [{"style":"BOLD","offset":0,"length":6}]}
```

A mention resolves to the node it points at, so a post that says "with @NASA" gives you NASA's id rather than the letters.
A link is unwrapped from the `l.facebook.com` redirect shim before it is stored, because that shim carries a per-render signature and keeping it would make two reads of one link look like two links.

Offsets are the ones Facebook sent, which means UTF-16 code units rather than bytes or runes.
fb does not renumber them, because a number that has been quietly converted is worse than one that needs converting.
It does slice the span for you: `display` on a link is the text that range covers, so the common case needs no arithmetic at all.

`counts` keeps both forms of a number: `reactions: 13054` and `reactions_text: "13K"`.
The exact number is the useful one and the text is what the page displayed, and neither is a rounding of the other you should try to reconstruct.

## Comments

```sh
fb comments 'https://www.facebook.com/NASA/posts/1587860636042640/' -n 3 \
  --fields author,replies,body
```

```
╭───────────────────────┬─────────┬───────────────────────────────────────────────────────────────────────────────────────╮
│ AUTHOR                │ REPLIES │ BODY                                                                                  │
├───────────────────────┼─────────┼───────────────────────────────────────────────────────────────────────────────────────┤
│ Edward Charles Haas V │ 1       │ I've been watching too much I, Claudius. I read Roman Telescope and you can assume lol │
│ James Fullerton       │         │ The September issue of Sky & Telescope has a great article title: 'The Start of the …' │
│ Oliver Walker         │ 1       │ How do I hitch a ride, you can drop me off anyplace outside of earth.                  │
╰───────────────────────┴─────────┴───────────────────────────────────────────────────────────────────────────────────────╯
3 of 532 shown
```

That last line is on stderr, and it is deliberate.
The permalink ships about twenty comments and the post has 532, so a read that printed three and said nothing would read as a post with three comments.
The record's `missed` field says the same thing for anything consuming the JSON.

`comments_cursor` is where to continue, and continuing is what a session buys.

A comment body has the same shape as a post message: entities resolved, offsets in bytes.
A reply carries the id of the comment it answers, so the tree is rebuildable downstream without fb having to nest it.

## Reactions

```sh
fb reactions 'https://www.facebook.com/NASA/posts/1587860636042640/'
```

```
╭───────┬───────┬───────╮
│ TYPE  │ COUNT │ SHARE │
├───────┼───────┼───────┤
│ Like  │ 10997 │ 0.842 │
│ Love  │ 1722  │ 0.132 │
│ Care  │ 153   │ 0.012 │
│ Haha  │ 97    │ 0.007 │
│ Wow   │ 73    │ 0.006 │
│ Angry │ 8     │ 0.001 │
│ Sad   │ 4     │ 0.000 │
╰───────┴───────┴───────╯
```

All seven types, signed out, from the same fetch as the post.
The breakdown arrives keyed by numeric reaction id rather than by name, so fb carries a table of the seven ids.
Those are platform constants: Like has been `1635855486666999` since reactions shipped.
A live test watches for an eighth turning up, because the day Facebook adds one it would otherwise be reported silently under its raw id.

Who reacted is not on this surface and fb does not pretend otherwise.

## Chaining

Every command writes JSON Lines with `-o jsonl`, and URLs with `-o url`, so the shell is the pipeline:

```sh
fb photos nasa -n 20 -o url | while read u; do fb photo "$u" -o jsonl; done > photos.jsonl
fb post "$URL" -o json | jq '.counts.by_type'
fb comments "$URL" -n 0 -o jsonl | jq -r '.author.name' | sort | uniq -c | sort -rn
```
