---
title: "CLI"
description: "Every command, what it emits, the flags that are its own, and the global flags they all share."
weight: 10
---

```
fb <command> [args] [flags]
```

`fb <command> --help` is authoritative, because it is generated from the same operation definitions the commands run on.
This page is the map.

Every read command takes exactly one reference.
There is no multi-argument form: `fb page nasa zuck` is a usage error, and a shell loop is the answer.

A reference is a handle, a numeric id, a permalink, a share link, a story key, or any Facebook URL fb can classify.
`fb id <thing>` says which of those it is without making a request.

## Reads

| Command | What it emits |
|---|---|
| `page <handle\|id>` | A Page or profile whole: identity, counts, contact, tabs, the timeline's first post |
| `profile <handle\|id>` | The same command under the other name |
| `feed <handle\|id>` | The timeline. One post signed out, paged with a session |
| `post <url\|id>` | A post, with its message entities, counts, attachments and the comments the permalink ships |
| `comments <url\|id>` | Just the comments from that read |
| `reactions <url\|id>` | Just the reaction breakdown, all seven types |
| `photos <handle\|id>` | The photo tab, or an album with `--album` |
| `photo <fbid\|url>` | One photo: image, alt, owner, album, caption, counts, comments, next |
| `videos <handle\|id>` | The video tab, grid and shows stitched together |
| `video <id\|url>` | One video: media URLs, dimensions, captions, music |
| `reel <id\|url>` | The same command under the other name |
| `events <handle\|id>` | The events tab, past and upcoming |
| `event <id\|url>` | One event: host, time, place, description, RSVP counts |
| `group <id\|slug>` | A group's public shell |
| `group feed <id\|slug>` | A group's discussion |
| `discover` | The Pages directory, as a seed source |
| `search <query>` | Search. Needs a session, exit 4 without one |

### Flags of their own

```
page      --about        also fetch the About page
          --no-posts     skip the timeline parse
          --tab          read one tab instead of the main page
post      --author       the profile a bare post id or a pfbid belongs to
comments  --author       the same
photos    --album        walk the album this photo is in, not the photo tab
          --download     write every image and its sidecar into this directory
photo     --download     the same, for one photo
video     --transcript   also fetch the watch route, which carries the transcript
          --download     write the video and its sidecar, HD if there is one
videos    --playlists    print the shows rather than the videos
event     --suggested    also print the events Facebook suggests alongside
group     --resolve-opaque   spend a request each on the pfbid authors in a feed
```

## Claims

| Command | What it emits |
|---|---|
| `edges <ref>` | The claims one read already makes. No request of its own beyond that read |
| `graph <ref>` | `edges` plus a walk. `--depth` hops, `--budget` requests |
| `rdf <ref>` | The claims as RDF. `--format nt\|turtle\|jsonld`, `--depth`, `--budget`, `--no-provenance` |

## Stores

| Command | What it emits |
|---|---|
| `crawl <seed>...` | Walks from the seeds into a store, then a manifest line |
| `db stats` | What is in the store, counted as nodes, claims and reads |
| `query <sql>` | Plain SQLite over the store, opened read-only |
| `export` | The whole store as RDF. `--format`, `--no-provenance` |

`crawl` flags: `--depth`, `--budget`, `--store`, `--seed-directory`, `--letter`, `--resolve-opaque`.

`--store` is the store these four commands read and write.
`--db` is the global tee flag, which writes records into a store as a side effect of any read.
They are separate on purpose.

## Captures and surfaces

| Command | What it emits |
|---|---|
| `archive <ref>` | One read written down whole into `--dir`. Never from the cache |
| `serve` | Every read operation over HTTP as NDJSON. `--addr`, `--allow-writes` |
| `mcp` | The same operations as MCP tools over stdio |

## Questions about fb

None of these five make a request.

| Command | What it emits |
|---|---|
| `id <anything>` | What a string is, which command handles it, and what is still uncertain. `--resolve` spends one request |
| `explain <url>` | `id` plus the routing: surface, GraphQL operation, tier, depth |
| `surfaces` | The eight surfaces and what each answers |
| `tiers` | The two tiers and what each needs |
| `routes` | Command to surface to operation, at each tier. `--questions` keys it by what you want to know instead |
| `fields [kind]` | Every field on a record kind, and how often the captured pages filled it |

## Session

| Command | What it does |
|---|---|
| `auth import --c-user <id> --xs <token>` | Store the two cookies |
| `auth status` | Whether a session is stored, and whose. No request |
| `auth logout` | Forget it |
| `whoami` | Prove the stored session against Facebook. One request |

## Housekeeping

| Command | What it does |
|---|---|
| `cache` | What is in the page cache |
| `cache clear` | Delete it |
| `config show` | The resolved configuration |
| `config path` | The data directory |
| `completion <shell>` | Shell completion |

## Global flags

Every command takes all of these.

```
-o --output      auto|table|markdown|list|json|jsonl|csv|tsv|url|raw
   --fields      which columns to show
   --template    a Go template applied per record
   --no-header   omit the header row
   --color       auto|always|never
-n --limit       stop after N records (0 = no limit)
   --tier        cap the tier (0|1) or pin one surface
   --rate        minimum delay between requests
   --retries     retry attempts on rate limit or 5xx
   --timeout     per-request timeout
   --proxy       an HTTP proxy for every request
   --no-cache    bypass the on-disk cache
   --cache-ttl   how long a cached page counts as fresh (default 15m)
   --data-dir    override the data directory
   --db          tee every record into a store
   --profile     load a named profile
   --dry-run     print what would happen
-q --quiet       suppress progress output
-v --verbose     more detail, repeatable
   --version     print the version
```

`--tier` takes a number or a surface name.
`--tier 0` refuses to use a session you have imported, which is how you check what an anonymous reader sees.
`--tier comet`, `graphql`, `og`, `embed` or `session` pins one surface and fails rather than falling back, which is how you find out which surface a field actually came from.
