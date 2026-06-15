---
title: "CLI"
description: "Every command and subcommand, with the flags that matter and a runnable example for each."
weight: 10
---

```
fb <command> [args] [flags]
```

Run `fb <command> --help` for the exact flag list on any command. This page is
the complete map: what each command emits, the flags that change its behaviour,
and one example you can paste.

Every command shares the [global flags](#global-flags) (output format, limits,
rate, cache). Reads are anonymous: fb crawls the public pages Facebook serves to
search engines, with no login. See [how fb reads Facebook](/guides/authentication/).

## Command index

| Command | What it does |
|---|---|
| [`page`](#page) | A Page, fully resolved; stream its posts, photos, videos, events |
| [`profile`](#profile) | A person's public profile and timeline |
| [`group`](#group) | A group and its feed |
| [`post`](#post) | One or more posts, with comments and reactions |
| [`comments`](#comments) | Every comment and reply on a post |
| [`reactions`](#reactions) | The reaction breakdown, or every reactor |
| [`photos`](#photos) / [`photo`](#photo) | A handle's photos; one photo's full metadata |
| [`videos`](#videos) / [`video`](#video) | A Page's videos and reels; one video with playable streams |
| [`events`](#events) / [`event`](#event) | A Page's events; one event, full |
| [`search`](#search) | Search across pages, profiles, groups, posts, photos, videos, events |
| [`feed`](#feed) | Stream the feed of any handle, whatever its type |
| [`discover`](#discover) | Breadth-first walk of the graph linked from a seed |
| [`id`](#id) | Classify any Facebook id or URL without a network call |
| [`seed`](#seed) / [`crawl`](#crawl) | Expand a root into URLs, then fetch them into records and a DB |
| [`archive`](#archive) | Mirror a Page's feed to incremental Markdown, indexed by month |
| [`db`](#db) | Query the local SQLite store |
| [`whoami`](#whoami) | Report how fb is accessing Facebook |
| [`config`](#config) / [`cache`](#cache) | Inspect configuration, paths, and the cache |
| [`completion`](#completion) / [`version`](#version) | Shell completion; build info |

## Entity commands

### page

```
fb page <slug|id|url>... [flags]
```

Resolves a Page to a full record (name, category, about, like and follower
counts, verified flag, website, phone, address, rating, cover and avatar). With
a stream flag it walks the corresponding surface to exhaustion, bounded only by
`--limit`.

| Flag | Meaning |
|---|---|
| `--about` | Metadata only (the default) |
| `--posts` | Stream the Page's feed |
| `--photos` | Stream the Page's photos |
| `--videos` | Stream the Page's videos and reels |
| `--events` | Stream the Page's public events |

```bash
fb page nasa                       # the Page record
fb page nasa --posts -n 50         # its 50 most recent posts
fb page nasa --posts -n 0          # every post the feed exposes
fb page nasa cocacola -o jsonl     # several Pages at once
```

### profile

```
fb profile <user|id|url> [flags]
```

A person's public profile: intro, bio, work, education, hometown, current city,
relationship, follower and friend counts. `--posts` streams the public timeline;
`--photos` streams their photos.

```bash
fb profile zuck --about
fb profile zuck --posts -n 20 -o jsonl
```

### group

```
fb group <id|slug|url> [flags]
```

A group record (name, description, privacy, member count, category) and, with
`--posts`, its feed.

```bash
fb group 123456789 --about
fb group 123456789 --posts -n 0 -o jsonl > group-feed.jsonl
```

## Post and engagement commands

### post

```
fb post <url|id>... [flags]
```

Resolves one or more posts to full records: text, author, timestamps, reaction /
comment / share / view counts, attached media, external links, and pinned state.
Add engagement with the flags below.

| Flag | Meaning |
|---|---|
| `--comments` | Also stream the comment thread |
| `--replies` | Expand nested replies (implies the comment walk) |
| `--reactions` | Emit the reaction breakdown |
| `--no-detail` | Counters only, skip the detail fetch (faster) |

```bash
fb post "https://www.facebook.com/nasa/posts/pfbid0xyz"
fb post "<url>" --comments --replies -o jsonl     # the whole thread, replies and all
fb post "<url>" --reactions -o json               # the reaction breakdown
```

### comments

```
fb comments <post-url|id> [flags]
```

Streams every comment on a post, following the "View more comments" pagination
to the end. `--replies` expands each comment's reply thread inline, with
`parent_id` set so you can reconstruct the tree.

| Flag | Meaning |
|---|---|
| `--replies` | Expand all nested replies, attributing each to its parent |
| `--order` | `chrono` (default) or `ranked` |

```bash
fb comments "<post-url>" -n 0 -o jsonl            # every top-level comment
fb comments "<post-url>" --replies -n 0 -o jsonl  # every comment and reply
```

### reactions

```
fb reactions <post-url|id> [flags]
```

By default emits the per-type breakdown (like, love, care, haha, wow, sad,
angry, total). `--list` emits one row per reactor instead.

```bash
fb reactions "<post-url>"                 # the breakdown
fb reactions "<post-url>" --list -o jsonl # every reactor as a row
```

## Media commands

### photos

```
fb photos <page|profile|url> [flags]
```

Streams a handle's photos, walking albums to exhaustion. Each photo carries its
full and thumbnail URLs, dimensions, album, caption, and counts.

```bash
fb photos nasa -n 0 -o jsonl
fb photos nasa -o url > photo-urls.txt    # just the full-resolution URLs
```

### photo

```
fb photo <fbid|url>
```

One photo with complete metadata.

```bash
fb photo "https://www.facebook.com/photo.php?fbid=123"
```

### videos

```
fb videos <page> [flags]
```

Streams a Page's videos and reels: title, description, duration, view / like /
comment / share counts, thumbnail, and reel flag.

```bash
fb videos nasa -n 0 -o jsonl
```

### video

```
fb video <id|url> [flags]
```

One video or reel. `--streams` adds the playable source URLs (quality, MIME,
dimensions) so you can download the media.

```bash
fb video "https://fb.watch/abc123/"
fb video "<url>" --streams -o jsonl
```

## Discovery commands

### events / event

```
fb events <page>
fb event  <id|url>
```

`events` streams a Page's public events; `event` resolves one to a full record
(name, description, start and end, location, host, going / interested counts).

```bash
fb events nasa -o jsonl
fb event 1234567890 -o json
```

### search

```
fb search <query> [flags]
```

Searches across Facebook's surfaces. `--type` narrows to one of `all` (default),
`page`, `profile`, `group`, `post`, `photo`, `video`, `event`.

```bash
fb search "open source" -n 20 -o jsonl
fb search nasa --type page -o table
```

### feed

```
fb feed <slug|id>... [flags]
```

Streams the feed of any handle without you having to know whether it is a page,
profile, or group; `fb` classifies it and walks the right surface.

```bash
fb feed nasa zuck -n 25 -o jsonl
```

### discover

```
fb discover <id|url>... [flags]
```

Walks the graph linked from one or more seeds, breadth first, streaming one node
per record. From an actor it follows `posts`; from a post it follows `author` and
`comments`. `--follow` takes a preset (`content`, `threads`, `all`) or a
comma-separated edge list. Aliases: `walk`, `graph`. See the
[Discovering](/guides/discovering/) guide.

| Flag | Meaning |
|---|---|
| `--depth` | Hops to follow from each seed (default `1`; `0` = seeds only) |
| `--fanout` | Max neighbors to follow per edge (default `25`; `0` = unlimited) |
| `--follow` | Edges to follow (default `content`): presets `content\|threads\|all`, edges `posts\|author\|comments` |

```bash
fb discover nasa --depth 2 -o jsonl > graph.jsonl
fb discover "https://www.facebook.com/nasa/posts/123" --depth 2
fb discover nasa --follow threads --depth 2
fb search "climate" -o url | fb discover - --depth 1
```

### id

```
fb id <thing>
```

Classifies any Facebook id or URL (slug, numeric id, `pfbid`, `story_fbid`,
group id, `fb.watch` / `fb.me` short link) into a typed record. Pure: no network
call, so it is instant and works offline.

```bash
fb id nasa
fb id "https://www.facebook.com/groups/123/permalink/456" -o json
```

## Bulk and storage

### seed

```
fb seed <page|profile|group|search> <arg> [flags]
```

Expands a root into a stream of URLs on stdout, ready to pipe into `crawl`.

```bash
fb seed page nasa | fb crawl --db fb.db
fb seed search "climate" --type page | fb crawl -o jsonl
```

### crawl

```
fb crawl [flags]
```

Reads URLs from stdin (or `--from <file>`), fetches each into a full record, and
optionally upserts into a SQLite store. `--comments` also pulls each post's
thread.

| Flag | Meaning |
|---|---|
| `--from` | Read URLs from a file instead of stdin |
| `--db` | Upsert records into this SQLite store |
| `--comments` | Also fetch each post's comments |

```bash
fb seed page nasa | fb crawl --db fb.db --comments
cat urls.txt | fb crawl -o jsonl > records.jsonl
```

### archive

```
fb archive <page>... [flags]
```

Walks a Page's feed and writes it as an incremental tree of Markdown: one file
per post (with its comments) under `<out>/<page>/YYYY/MM/`, plus a generated
`README.md` index. Re-running skips posts already on disk and fetches only what
is new. See the [Archiving]({{< relref "../guides/archiving.md" >}}) guide.

| Flag | Meaning |
|---|---|
| `--out` | Root directory for the archive (default `~/data`) |
| `--comments` | Fetch and embed each post's comments (default on) |
| `--replies` | Expand reply threads under comments |
| `--force` | Re-fetch and overwrite posts already on disk |

```bash
fb archive aivietnam.edu.vn --comments
fb archive nasa --out ~/archives -n 100
```

### db

```
fb db --db <file> query <sql> [flags]
```

Runs read-only SQL against the local store built by `crawl`.

```bash
fb db --db fb.db query "select name, followers_count from pages order by followers_count desc limit 10"
```

## Utility

### whoami

```
fb whoami
```

Reports how fb is accessing Facebook: the mode (always `anonymous`) and the user
agent requests are sent with.

### config

```
fb config show    # resolved configuration
fb config path    # config / cache / data directories
```

### cache

```
fb cache dir      # the cache directory
fb cache clear    # empty it
```

### completion

```
fb completion bash|zsh|fish|powershell
```

Generates a shell completion script. See your shell's docs for where to install
it.

### version

```
fb version
```

Prints the version, commit, and build date.

## Global flags

These apply to every command. See [configuration](/reference/configuration/) for
the network flags and [output](/reference/output/) for the formatting flags.

| Flag | Meaning |
|---|---|
| `-o, --output` | `table\|json\|jsonl\|csv\|tsv\|yaml\|url\|raw` (default auto) |
| `--fields` | Comma-separated columns to keep and order |
| `--template` | Go `text/template` applied per record |
| `--no-header` | Omit the header row (table / csv / tsv) |
| `-n, --limit` | Maximum records emitted (`0` = unlimited) |
| `--since` / `--until` | Bound a feed walk by date (`YYYY-MM-DD`) |
| `--surface` | `mbasic`, `mobile`, or `auto` |
| `--lang` | `Accept-Language` / locale (default `en-US`) |
| `--rate` | Minimum delay between requests (default `2s`) |
| `--retries` | Retry attempts on 429 / 5xx (default `4`) |
| `--timeout` | Per-request timeout (default `30s`) |
| `-j, --workers` | Concurrency for fan-out commands (default `2`) |
| `--proxy` | HTTP / SOCKS proxy URL |
| `--user-agent` | Override the default rotating UA set |
| `--no-cache` | Bypass the on-disk cache |
| `--cache-ttl` | Cache freshness window (default `1h`) |
| `--raw` | Print the upstream HTML / JSON untouched |
| `--dry-run` | Print the requests that would be made, do nothing |
| `-q, --quiet` | Suppress progress on stderr |
| `-v, --verbose` | Increase verbosity (repeatable) |
