---
title: "Datasets"
description: "Expand a root into URLs, crawl them into records, and store them in a local SQLite database you can query with SQL."
weight: 60
---

For bulk work, fb composes into a small pipeline: `seed` turns a root into a
stream of URLs, `crawl` fetches each into a full record, and `db` queries the
result. Each stage is a separate command so you can inspect or reshape the
stream in between.

## Seed: a root into URLs

`seed` expands a page, profile, group, or search into a stream of post URLs, one
per line:

```sh
fb seed page nasa --limit 100
fb seed search "climate" --type page --limit 50
```

Because it is just lines of URLs, you can filter, sort, or split the stream with
ordinary tools before crawling.

## Crawl: URLs into records

`crawl` reads URLs from stdin (or `--from <file>`), classifies each, fetches the
matching record, and emits it. Point it at a database with `--db` to store as it
goes:

```sh
fb seed page nasa --limit 100 | fb crawl --db nasa.db
```

Pull the comment thread and reactions for each post as you crawl:

```sh
fb seed page nasa --limit 100 | fb crawl --db nasa.db --comments --reactions
```

A URL that fails is logged to stderr and skipped, so one bad item does not stop
the run.

## The SQLite store

The store is an ordinary SQLite file with one table per record type: `pages`,
`profiles`, `groups`, `posts`, `comments`, `reactions`, `photos`, `videos`,
`events`. Records upsert by id, so re-crawling refreshes rather than duplicates.
Each row keeps the full record as JSON alongside its key columns.

Query it with `fb db query`, which renders results through the usual formatter:

```sh
fb db --db nasa.db query "select owner_name, count(*) n from posts group by 1 order by n desc"
fb db --db nasa.db query "select text from comments order by reactions_count desc limit 10" -o jsonl
```

Because it is plain SQLite, any other tool works too:

```sh
sqlite3 nasa.db "select count(*) from posts"
```

## A full example

Collect a Page's last 200 posts with their comments into a database, then ask
which posts drew the most discussion:

```sh
fb seed page nasa --limit 200 | fb crawl --db nasa.db --comments
fb db --db nasa.db query \
  "select permalink, comments_count from posts order by comments_count desc limit 10"
```
