---
title: "fb"
description: "A read-only command line for Facebook. It reads the data facebook.com already ships to a signed-out browser and turns it into typed records, claims and RDF. No app id, no API token, nothing to sign up for."
heroTitle: "Facebook, as structured data"
heroLead: "fb is one pure-Go binary that reads the public data facebook.com already sends to a browser: pages, posts, comments, reactions, photos, videos, groups and events, as typed records you can pipe. There is no app id, no developer app and no API token anywhere in it."
heroPrimaryURL: "/getting-started/quick-start/"
heroPrimaryText: "Get started"
---

Reading Facebook usually means the Graph API with its app review and tokens, or a headless browser and a pile of selectors that break every quarter.
fb takes the third route.

A facebook.com page ships its own data inline.
The Relay store the page renders itself from is sitting in the HTML as JSON, and that is the same data, not a scrape of the rendered text.
fb reads that, parses it into typed records, and prints them as a table, JSON, CSV or just URLs.

```bash
fb page nasa                  # the profile record, whole
fb post <url>                 # a post with its comments and reactions
fb photos nasa --limit 20     # the photo tab
fb edges nasa                 # what that page claims about everything else
fb crawl nasa --depth 2       # walk those claims into a SQLite store
```

Nothing here needs a credential.
Two cookies from a browser you are already signed into unlock a few more reads, and fb is explicit about which ones and why.

## What it does

- **Reads a page whole.** Name, id, category, counts, bio, contact block, cover, avatar, the tabs it has, and the newest post.
- **Reads a post whole.** The message with links and mentions resolved to nodes, reactions broken down by type, share and comment counts, the media, and the comments the permalink ships.
- **Reads media.** Photos with alt text and album neighbours, videos with their captions and Facebook's own transcript, and the bytes with a provenance sidecar.
- **Says what it did not see.** Every record carries the tier it was read at, which surfaces answered, the URLs behind it, and what it missed.
- **Turns pages into a graph.** `fb edges` prints the claims one read makes, `fb crawl` walks them into a store, and `fb export` writes the lot as RDF over schema.org.
- **Answers to something else.** `fb serve` puts the same reads behind HTTP, and `fb mcp` puts them in front of an agent.

## Read-only, and checked

Every request fb makes is a GET, except one form post that replays a query the page itself shipped.
That is not a promise in a README.
A test reads the package as source and fails the build if a second non-GET appears, if an operation name ends in `Mutation`, or if any registered operation is anything but a read.

## Where to go next

- New here? [Introduction](/getting-started/introduction/) is the mental model, and it is worth ten minutes before anything else.
- Ready to run something? [Quick start](/getting-started/quick-start/).
- Installing? [Installation](/getting-started/installation/).
- Looking for one task? The [guides](/guides/) go command by command.
- Want every flag? The [CLI reference](/reference/cli/).
