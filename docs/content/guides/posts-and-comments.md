---
title: "Posts and comments"
description: "Resolve one post in full, walk its comment thread, and read the reaction breakdown."
weight: 30
---

A post is where Facebook gets deep: text and media, a comment thread with nested
replies, and a reaction breakdown. fb pulls each on demand so you only fetch what
you ask for.

## One post

Pass any post URL or id. fb classifies it first, whether it is a `pfbid`
permalink, a `story_fbid`, or a numeric id:

```sh
fb post "https://www.facebook.com/nasa/posts/pfbid0xyz" -o json
```

The record carries the author, text, timestamp, media URLs, external links, and
the reaction, comment, and share counts. Several posts at once, or from stdin:

```sh
fb post <url1> <url2> -o jsonl
fb seed page nasa --limit 20 | fb post - -o jsonl
```

Use `--no-detail` when you only want the counters and want it fast; it skips the
extra detail fetch.

## The comment thread

Add `--comments` to stream the thread after the post, or use the dedicated
`comments` command:

```sh
fb post <url> --comments -o jsonl
fb comments <url> --limit 200 -o jsonl
```

Nested replies are not expanded by default, since that is an extra fetch per
comment. Add `--replies` to pull them. Each reply carries a `parent_id` pointing
at the comment it answers, so you can rebuild the tree downstream:

```sh
fb comments <url> --replies --limit 500 -o jsonl
```

`--order chrono|ranked` picks Facebook's chronological or ranked ordering.

### Downloading everything

Both the comment walk and the reply walk follow Facebook's pagination to the end
on their own; the only bound is `--limit`. Pass `--limit 0` (or `-n 0`) to take
the whole thread, replies and all:

```sh
fb comments <url> --replies -n 0 -o jsonl > thread.jsonl
```

The same holds for feeds: `fb page <slug> --posts -n 0` walks the timeline until
the next-page cursor runs out, so you capture every post the surface exposes, not
a fixed first page. Pair it with `--since YYYY-MM-DD` to stop at a date instead
of the very end.

## Reactions

`--reactions` on `post` emits the breakdown (how many of each type), and the
`reactions` command does the same on its own:

```sh
fb reactions <url> -o json
```

```json
{"total":1240,"like":900,"love":210,"haha":40,"wow":60,"sad":20,"angry":10}
```

To list every reactor and how they reacted, add `--list`, and narrow to one type
with `--type`:

```sh
fb reactions <url> --list -o jsonl
fb reactions <url> --list --type love -o jsonl
```

## Composing

Because every command streams JSON Lines, you can chain them. Pull a feed, keep
the high-engagement posts, and fetch their threads:

```sh
fb page nasa --posts --limit 100 -o jsonl \
  | jq -r 'select(.comments_count > 100) | .permalink' \
  | fb post - --comments -o jsonl
```
