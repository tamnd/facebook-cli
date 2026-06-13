# fb

A delightful, scriptable command line for [Facebook](https://facebook.com). One
binary that resolves a Page, profile, or group to a rich record, streams its
recent feed, and pulls posts, comments, photos, videos, and events into clean
structured data you can pipe anywhere, with no login and no browser.

```
fb page nasa
```

```
NAME  CATEGORY                 LIKES      FOLLOWERS  VERIFIED  URL
NASA  Aerospace company        24.1M      25.0M      true      https://www.facebook.com/nasa
```

Full documentation: [facebook-cli.tamnd.com](https://facebook-cli.tamnd.com).

## Why

Pulling data out of Facebook usually means a headless browser, a brittle pile of
selectors, or the Graph API with its app review and tokens. `fb` takes a
different route: it reads the public pages Facebook serves to search engines,
parses the server-rendered HTML into typed records, and renders them in the
output format you ask for. One static binary, no login, no browser, no API key.

Because it reads the public crawler surface, `fb` sees what a search engine
sees: full public Pages, profiles, groups, and posts, the most recent feed
rather than the entire history, and a post's preview comments rather than its
whole thread. Private content stays private, and `fb` is explicit about the wall
rather than silently returning nothing.

## Install

```sh
go install github.com/tamnd/facebook-cli/cmd/fb@latest
```

Or grab a prebuilt binary from the [releases page](https://github.com/tamnd/facebook-cli/releases).
The binary is pure Go with no runtime dependencies.

Build from source:

```sh
git clone https://github.com/tamnd/facebook-cli
cd facebook-cli
make build      # produces ./bin/fb
```

## Quick start

```sh
fb page nasa                       # a Page's full profile
fb page nasa --posts --limit 20    # its twenty most recent posts
fb post <url> --comments           # a post and its comment thread
fb id <anything>                   # classify any Facebook id or URL
```

## How it reads Facebook

`fb` reads anonymously, as a web crawler, with no login and no cookie. It asks
Facebook for the same server-rendered pages a search engine gets and parses what
comes back.

```sh
fb whoami        # reports the access mode and user agent
```

This works on any public Page, profile, group, or post. When a target is
private, or behind a login wall, `fb` exits `4` with a one-line hint so scripts
can tell that apart from a real error. The trade-off is depth: a feed exposes the
most recent posts rather than the full history, and a post carries a few preview
comments rather than its whole thread.

## How it works

`fb` resolves any handle, id, or URL to a typed identity first (`fb id` shows
exactly what it sees), then fetches the matching no-JavaScript page from
`mbasic.facebook.com` and parses the HTML into records with
[goquery](https://github.com/PuerkitoBio/goquery). Feeds and comment threads
page through Facebook's "see more" links, so a `--limit` walks as many pages as
it needs and stops cleanly. Responses are cached on disk by URL, so re-running a
command is instant and polite to Facebook.

Every record is a plain struct with JSON tags, so `-o json` gives you the full
shape and `--fields` narrows it. Nothing is invented: a field that Facebook does
not surface anonymously simply stays empty rather than being guessed.

## Commands

| Command | What it does |
| --- | --- |
| `page` | A Page, fully resolved; `--posts/--about/--photos/--videos/--events` |
| `profile` | A person's public profile; `--posts`, `--about`, `--photos` |
| `group` | A group and its feed; `--posts` |
| `post` | One or more posts; `--comments`, `--replies`, `--reactions` |
| `comments` | Every comment and reply on a post |
| `reactions` | The reaction breakdown, and `--list` every reactor |
| `photos` / `photo` | A handle's photos, or one photo with full metadata |
| `videos` / `video` | A Page's videos and reels, or one with `--streams` |
| `events` / `event` | A Page's public events, or one event in full |
| `search` | Search pages, profiles, groups, posts, photos, videos, events |
| `feed` | Stream the feed of any handle (page/profile/group) |
| `id` | Classify any Facebook id or URL, no network needed |
| `seed` | Expand a root into a stream of URLs for crawling |
| `crawl` | Fetch a stream of URLs into full records (and optionally a DB) |
| `db` | Query the local SQLite store |
| `whoami` | Report how `fb` is accessing Facebook |
| `config` | Show resolved configuration and paths |
| `cache` | Inspect and clear the on-disk cache |
| `completion` | Generate a shell completion script |
| `version` | Print version, commit, and build date |

Run `fb <command> --help` for the full flag list on any command.

## Recipes

Pull a Page's last 50 posts as JSON Lines:

```sh
fb page nasa --posts --limit 50 -o jsonl
```

Get a post and its whole comment thread, replies expanded:

```sh
fb post <url> --comments --replies -o jsonl
```

See who loved a post:

```sh
fb reactions <url> --list --type love -o table
```

Collect every photo URL on a Page:

```sh
fb photos nasa --limit 200 -o url
```

Resolve a short link to a canonical id:

```sh
fb id "https://fb.watch/xxxxx" -o json
```

Build a dataset: expand a Page feed into URLs, then crawl each into SQLite:

```sh
fb seed page nasa --limit 100 | fb crawl --db nasa.db --comments
fb db --db nasa.db query "select owner_name, count(*) from posts group by 1"
```

## Output formats

Every command renders through the same formatter. Pick a format with `-o`, or
let `fb` choose: a table when writing to a terminal, JSON Lines when piped.

```sh
fb page nasa --posts -o table   # aligned columns for reading
fb page nasa --posts -o jsonl   # one JSON object per line, for piping
fb page nasa --posts -o json    # a single JSON array
fb page nasa --posts -o csv     # spreadsheet friendly (tsv too)
fb page nasa --posts -o yaml    # YAML documents
fb page nasa --posts -o url     # just the permalink
```

Narrow the columns with `--fields`, or template each row:

```sh
fb page nasa --posts --fields permalink,reactions_count,comments_count
fb page nasa --posts --template '{{.Permalink}} {{.ReactionsCount}}'
```

`--raw` prints the upstream HTML untouched, for when you want to parse it
yourself.

## Configuration

`fb` keeps its cache and data under the standard XDG paths
(`~/.cache/fb` and `~/.local/share/fb` by default; honor `XDG_CACHE_HOME` and
`XDG_DATA_HOME` to move them). See the resolved paths and settings any time:

```sh
fb config show
fb config path
```

Useful global flags (all have sensible defaults):

| Flag | Meaning |
| --- | --- |
| `-o, --output` | Output format (default auto) |
| `-n, --limit` | Maximum records (`0` means unlimited) |
| `--since` / `--until` | Stop or skip feed items by date |
| `--rate` | Minimum delay between requests, to stay polite (default 2s) |
| `--surface` | `mbasic`, `mobile`, or `auto` |
| `-j, --workers` | Concurrency for fan-out commands |
| `--no-cache` | Bypass the on-disk cache |

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Generic error |
| `2` | Usage error |
| `3` | Content not found or unavailable |
| `4` | Login wall: the content is not public |
| `5` | Rate limited |
| `6` | Network error |

## Development

```sh
make test    # run the test suite
make vet     # go vet
make build   # build ./bin/fb
make smoke   # run every command and assert it works or walls cleanly
```

The code is layered. `cli/` is the command tree built on Cobra. `fb/` is the
library it sits on: the HTTP client, cache, the server-rendered HTML parsers for
each record type, and the SQLite store. `pkg/fbid/` is a standalone URL/id
classifier with no other dependencies, importable on its own.

## License

Apache-2.0. See [LICENSE](LICENSE).
