---
title: "Quick start"
description: "From install to a Page's feed as structured data, in a few commands."
weight: 30
---

This walks from a fresh install to a Page's feed on your terminal as clean
records. It takes a minute.

## 1. Confirm the binary runs

```sh
fb version
fb id nasa -o json
```

`fb id` classifies any handle or URL with no network and no login, so it always
works and is the fastest sanity check.

```json
{"input":"nasa","kind":"page","page_id":"nasa","slug":"nasa","canonical_url":"https://www.facebook.com/nasa","mbasic_url":"https://mbasic.facebook.com/nasa"}
```

## 2. Add your session

Facebook shows almost nothing to a logged-out visitor, so set a cookie from a
browser where you are logged in:

```sh
export FACEBOOK_COOKIE="c_user=...; xs=...; datr=...; fr=..."
fb whoami
```

```
AUTHENTICATED  USER
true           4
```

If `fb whoami` reports `false`, the cookie did not load; see
[authentication](/guides/authentication/) for the accepted formats. Without a
session, the read commands below exit `4` with a login-wall hint instead of
returning data.

## 3. Resolve a Page

```sh
fb page nasa
```

```
NAME  CATEGORY            LIKES   FOLLOWERS  VERIFIED  URL
NASA  Aerospace company   24.1M   25.0M      true      https://www.facebook.com/nasa
```

## 4. Stream its feed

```sh
fb page nasa --posts --limit 10 -o jsonl
```

Each line is one full post record. Pipe it into `jq`, a file, or another tool:

```sh
fb page nasa --posts --limit 50 -o jsonl > nasa-posts.jsonl
```

## 5. Go deep on a post

Take a permalink from the feed and pull its comment thread and reactions:

```sh
fb post <url> --comments --reactions -o jsonl
```

## 6. Build a small dataset

Expand the feed into URLs and crawl each into SQLite, then query it:

```sh
fb seed page nasa --limit 100 | fb crawl --db nasa.db --comments
fb db --db nasa.db query "select owner_name, count(*) from posts group by 1"
```

## Where to go next

- [Authentication](/guides/authentication/): cookie formats and what each
  session unlocks.
- [Pages and profiles](/guides/pages-and-profiles/): the entity commands.
- [Posts and comments](/guides/posts-and-comments/): going deep on a story.
- [Output formats](/reference/output/): tables, JSON, CSV, fields, templates.
