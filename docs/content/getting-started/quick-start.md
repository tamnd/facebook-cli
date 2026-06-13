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

## 2. Confirm how fb reads Facebook

fb reads anonymously, as a web crawler, with no login and no cookie:

```sh
fb whoami
```

```
MODE       USER_AGENT
anonymous  Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)
```

The read commands below work against public Pages, profiles, groups, and posts.
When a target is private, fb exits `4` with a login-wall hint instead of guessing.
See [how fb reads Facebook](/guides/authentication/) for what the crawler surface
exposes.

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

## 5. Read a post in full

Take a permalink from the feed and pull its text, counts, and preview comments:

```sh
fb post <url> --comments -o jsonl
```

## 6. Build a small dataset

Expand the feed into URLs and crawl each into SQLite, then query it:

```sh
fb seed page nasa --limit 100 | fb crawl --db nasa.db --comments
fb db --db nasa.db query "select owner_name, count(*) from posts group by 1"
```

## Where to go next

- [How fb reads Facebook](/guides/authentication/): the anonymous crawler model
  and its limits.
- [Pages and profiles](/guides/pages-and-profiles/): the entity commands.
- [Posts and comments](/guides/posts-and-comments/): going deep on a story.
- [Output formats](/reference/output/): tables, JSON, CSV, fields, templates.
