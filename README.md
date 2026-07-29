# fb

Facebook as structured data, from one pure-Go binary.

There is no app id, no developer app and no API token anywhere in this tool.
`fb` reads the data the site already ships to a signed-out browser.

```sh
fb page nasa --fields id,handle,name,likes,followers,category
```

```
╭─────────────────┬────────┬──────────────────────────────────────────────────────┬────────────┬────────────┬─────────────────────────╮
│ ID              │ HANDLE │ NAME                                                 │ LIKES      │ FOLLOWERS  │ CATEGORY                │
├─────────────────┼────────┼──────────────────────────────────────────────────────┼────────────┼────────────┼─────────────────────────┤
│ 100044561550831 │ NASA   │ NASA - National Aeronautics and Space Administration │ 28,660,738 │ 28,000,000 │ Government organisation │
╰─────────────────┴────────┴──────────────────────────────────────────────────────┴────────────┴────────────┴─────────────────────────╯
```

Full documentation: [facebook-cli.tamnd.com](https://facebook-cli.tamnd.com).

## Why

There are three ways to get data out of Facebook.

The Graph API wants an app, a review and a token, and then does not have most of this.
A headless browser works and costs you a browser per read.

`fb` takes the third route.
Signed-out facebook.com renders through Relay, and the page ships the query results inline as `<script type="application/json" data-sjs>` blocks: the same objects the site's own JavaScript renders from.
`fb` extracts them, stitches them and parses them into typed records.

One static binary, nothing to sign up for.

## Install

```sh
go install github.com/tamnd/facebook-cli/cmd/fb@latest
brew install --cask tamnd/tap/fb
```

Prebuilt archives for Linux, macOS, Windows and FreeBSD, plus deb, rpm and apk packages, SBOMs and signed checksums, are on the [releases page](https://github.com/tamnd/facebook-cli/releases).

```sh
docker run --rm -v ~/data/fb:/data ghcr.io/tamnd/fb page nasa
```

From source:

```sh
git clone https://github.com/tamnd/facebook-cli
cd facebook-cli
make build      # produces ./bin/fb
```

## Quick start

```sh
fb id nasa                                  # what is this string? no request made
fb page nasa                                # a Page or profile, whole
fb post 'https://www.facebook.com/NASA/posts/1587860636042640/'
fb comments "$URL" -n 20                    # from the same read
fb reactions "$URL"                         # all seven types, signed out
fb photos nasa -n 20 --fields id,size,alt
fb video 1380134307388381 --transcript
fb edges nasa                               # what that read claims about everything else
fb crawl nasa --depth 2 --budget 50 --store ./nasa.db
fb query "select kind, count(*) from nodes group by kind" --store ./nasa.db
fb export --store ./nasa.db --format turtle
```

## Eight surfaces, two tiers

```sh
fb surfaces      # the eight places fb reads, and what each answers
fb tiers         # the two tiers, and what each needs
fb routes        # which surface and operation answers which command
```

Tier 0 needs nothing at all, and nearly everything lives there: pages, profiles, posts, comments, all seven reaction types, photos with their alt text, videos with captions and transcripts, events with RSVP counts, and a public group's shell.

Tier 1 is two cookies from a browser you are already signed into, and it buys exactly three things: the timeline past the first post, group discussions, and search.

```sh
fb auth import --c-user <id> --xs <token>
```

## Every record says what it did not see

```json
{"tier":0,"surfaces":["s1","s3"],
 "sources":["https://www.facebook.com/profile.php?id=100044561550831"],
 "via":{"followers":"s1","likes":"s3","talking_about":"s3"},
 "missed":["comments past the first 20"],
 "fetched_at":"2026-07-29T18:02:57Z"}
```

`via` is there because `likes` and `followers` are different numbers from different surfaces, and pretending they are one field is a bug.
`missed` is there because a post with 532 comments read from a permalink that ships twenty should not look like a post with twenty comments.

## Claims, not just records

A record is what one page said about itself.
A claim is what it said about something else, and that is the part that composes.

```sh
fb edges nasa --fields predicate,to,note
```

```
╭──────────────┬────────────────────────────────────────────────────────────────────────────────┬──────────────────────────────────────────────────────╮
│ PREDICATE    │ TO                                                                             │ NOTE                                                 │
├──────────────┼────────────────────────────────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ delegates_to │ fb://page/54971236771                                                          │ NASA - National Aeronautics and Space Administration │
│ covers       │ fb://photo/1496429661852405                                                    │                                                      │
│ covers       │ fb://photo/416661036495945                                                     │                                                      │
│ links_to     │ fb://external/5bc0ad51cb2d1c745e141f87a537df877be241e909da9e7059bd133937fc133a │ https://www.nasa.gov/nasa-app/                       │
│ links_to     │ fb://external/639a6da94698ad1a2274a8d490f69c4a97615e451e0f70ebbf0d35b4c43c7a5f │ https://www.nasa.gov/                                │
│ authored     │ fb://post/1589486402546730                                                     │ NASA - National Aeronautics and Space Administration │
│ attaches     │ fb://video/1629850552078859                                                    │                                                      │
╰──────────────┴────────────────────────────────────────────────────────────────────────────────┴──────────────────────────────────────────────────────╯
```

Nineteen predicates, stable `fb://kind/id` URIs, and every claim carrying the URL, the surface and the tier that asserted it.
`fb graph` walks them, `fb crawl` keeps them in SQLite, and `fb rdf` and `fb export` write them as schema.org with RDF-star provenance.

## Commands

| | |
| --- | --- |
| `page` `profile` `feed` | A Page or profile whole, and its timeline |
| `post` `comments` `reactions` | A permalink, its thread, its seven reaction types |
| `photos` `photo` `videos` `video` `reel` | Media, with alt text, captions, transcripts and the bytes |
| `events` `event` `group` | Events from the tab or a permalink, and public groups |
| `search` `discover` | Search (needs a session) and the Pages directory |
| `id` `explain` | What a string is, and what fb would do with it. No request |
| `surfaces` `tiers` `routes` `fields` | What fb reads and how. No request |
| `edges` `graph` `rdf` | The claim graph, walked and exported |
| `crawl` `db` `query` `export` | Into SQLite, out as SQL or RDF |
| `archive` | One read written down whole, for a bug report or a fixture |
| `serve` `mcp` | The same eighteen operations over HTTP or MCP |
| `auth` `whoami` `cache` `config` | Session and housekeeping |

`fb <command> --help` is generated from the operation definitions, so it cannot drift from the code.

## Read-only, and checked

`fb` never posts, likes, comments, follows or joins anything.

Every request is a GET, except one form post to the GraphQL endpoint that replays a query the page itself shipped.
That is enforced rather than asserted: a test parses the package as source and fails the build if a second non-GET appears anywhere, if that replay's URL stops being the GraphQL endpoint, if any string in the package ends in `Mutation`, or if any registered operation is anything but a read.

Another test asserts the archive format redacts the cookie header, because captures get pasted into issues.

## Configuration

Everything lives in one directory: the page cache, the session file, and the default store.

```sh
fb config show
fb config path
```

`FB_DATA_DIR` moves it, and it is the only environment variable `fb` reads.
One directory rather than three because splitting cache from data means a Docker user mounts one volume and silently loses the other.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Unclassified failure |
| `2` | Usage |
| `3` | The read worked and there was nothing in it |
| `4` | Needs a session, or Facebook refused |
| `5` | Rate limited |
| `6` | Not found |
| `7` | fb cannot do this here |
| `8` | Network |

Exit 4 is the one to handle.
It covers a genuinely private thing and a refusal that lifts on its own, and the message says which.
The throttle is per URL rather than per address, so when `/nasa` walls, `/nasa/photos` usually still answers.

## Development

```sh
make test    # go test ./...
make vet     # go vet ./...
make build   # ./bin/fb
make smoke   # run every command and assert it works or walls cleanly
```

`cli/` is the command tree.
`fb/` is the library under it: the client, the cache, the Relay extraction, the parsers and the store.
`pkg/fbid/` classifies ids and URLs with no other dependency and is importable on its own.
`pkg/graph/` and `pkg/rdf/` are the claim model and its vocabulary.

The parser tests read `fb archive` captures of real pages, so they exercise the bytes Facebook actually served rather than a hand-written approximation.

## License

Apache-2.0. See [LICENSE](LICENSE).
