---
title: "Archiving"
description: "Save a Page's whole feed as a browsable, incremental tree of Markdown files: one file per post with its comments, indexed by month."
weight: 70
---

`archive` turns a Page into a folder of Markdown you can read, grep, or commit to
git. Each post becomes its own file, comments are embedded inline, and a
generated `README.md` indexes everything by month. The archive is incremental:
re-running only fetches posts that are not already on disk, so you can keep a
Page mirrored over time with the same command.

## A first archive

```sh
fb archive aivietnam.edu.vn --comments
```

By default the tree is written under `~/data/<page>`. Point `--out` somewhere
else to choose the root:

```sh
fb archive nasa --out ~/archives -n 100 --comments
```

The layout is browsable on disk and on any git host:

```
~/data/aivietnam.edu.vn/
  README.md                              index, grouped by year and month
  index.json                             state used for incremental runs
  2025/
    11/
      2025-11-03_khoa-hoc-ai-mien-phi.md one file per post, with its comments
      2025-11-01_thong-bao-tuyen-sinh.md
    10/
      ...
```

Each post file carries its title, date and engagement counts, a link back to
Facebook, the post text, images, external links, and the full comment thread.
Post slugs are transliterated to ASCII, so Vietnamese (and other accented)
titles produce clean file names.

## Comments and replies

`--comments` is on by default and embeds each post's comment thread under a
`## Comments` heading. Add `--replies` to walk the reply threads too; replies are
indented under the comment they answer:

```sh
fb archive aivietnam.edu.vn --replies
```

Turn comments off for a faster, text-only archive:

```sh
fb archive aivietnam.edu.vn --comments=false
```

## Incremental runs

The archive remembers what it has already saved in `index.json`. On the next
run, any post whose Markdown file is still on disk is skipped, and only new posts
are fetched and written:

```sh
# first run: pulls the recent feed
fb archive aivietnam.edu.vn

# a week later: only the new posts are fetched, README is regenerated
fb archive aivietnam.edu.vn
```

The index file and each post are written as the crawl proceeds, so an
interrupted run loses nothing: re-running picks up exactly where it stopped.

Pass `--force` to re-fetch and overwrite posts that are already archived, for
example to refresh engagement counts or pull newly added comments:

```sh
fb archive aivietnam.edu.vn --force
```

## Bounding the crawl

The same global flags that bound a feed apply here:

```sh
fb archive nasa -n 50                 # at most 50 posts
fb archive nasa --since 2025-01-01    # stop once posts get older than this
```

## Sessions

Most Pages gate their feed behind a login wall for anonymous visitors, so a full
archive needs your session cookie. Pass it with `--cookie`, point at a file with
`--cookie-file`, or set `FACEBOOK_COOKIE` in the environment:

```sh
export FACEBOOK_COOKIE="c_user=...; xs=..."
fb archive aivietnam.edu.vn --comments
```

See [Authentication]({{< relref "authentication.md" >}}) for how to obtain and
store the cookie.
