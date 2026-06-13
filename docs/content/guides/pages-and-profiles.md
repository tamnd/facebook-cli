---
title: "Pages and profiles"
description: "Resolve Pages, profiles, and groups to rich records, and stream their feeds."
weight: 20
---

The entity commands turn a handle into a full record and stream its feed.
`page`, `profile`, and `group` share the same shape: bare for metadata, with a
flag to stream.

## Pages

A bare `page` resolves the Page's metadata:

```sh
fb page nasa -o json
```

The record carries the name, category, about text, like and follower counts,
verification, website, and avatar. Stream the feed with `--posts`:

```sh
fb page nasa --posts --limit 20 -o jsonl
```

`page` also has shortcuts into a Page's other tabs, so you do not need the
separate media commands when you start from a Page:

```sh
fb page nasa --photos --limit 50 -o jsonl
fb page nasa --videos -o jsonl
fb page nasa --events
fb page nasa --about -o json
```

You can pass several handles at once, or read them from stdin with `-`:

```sh
fb page nasa spacex -o jsonl
echo -e "nasa\nspacex" | fb page - -o jsonl
```

## Profiles

A profile is a person rather than a Page, but works the same way:

```sh
fb profile zuck -o json
fb profile zuck --posts --limit 20 -o jsonl
```

Profiles accept a username (`zuck`) or a numeric id
(`profile.php?id=100000000000000`). What is visible depends on the person's
privacy settings; a profile that is not public to anonymous visitors exits `4`.

## Groups

A group resolves by id or slug, and streams its feed with `--posts`:

```sh
fb group 123456789 -o json
fb group 123456789 --posts --limit 50 -o jsonl
```

## feed: any handle, one command

When you do not care whether something is a Page, profile, or group, `feed`
classifies the handle and streams whatever feed it has:

```sh
fb feed nasa zuck --limit 20 -o jsonl
```

## Walking deep, and stopping by date

`--limit` walks as many "see more" pages as it needs to reach the count, then
stops. To bound a feed by time instead, use `--since` and `--until`:

```sh
fb page nasa --posts --since 2026-01-01 -o jsonl
fb page nasa --posts --until 2026-06-01 --since 2026-05-01 -o jsonl
```

`--since` stops walking once posts are older than the date; `--until` skips
posts newer than the date.
