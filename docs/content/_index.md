---
title: "fb"
description: "A delightful, scriptable command line for Facebook. Resolve a Page, profile, or group, stream its feed, and pull every comment, reaction, photo, video, and event into clean structured data, all from one binary."
heroTitle: "Facebook, from the command line"
heroLead: "fb is a single pure-Go binary that turns facebook.com into typed, scriptable data. Resolve any Page, profile, or group, walk its whole feed, and pull comments, reactions, photos, videos, and events in the output format you want, with no browser and no API key."
heroPrimaryURL: "/getting-started/quick-start/"
heroPrimaryText: "Get started"
---

Pulling data out of Facebook usually means a headless browser, a brittle pile of
selectors, or the Graph API with its app review and tokens. fb takes a different
route: it reads the no-JavaScript HTML surface the way a basic phone browser
does, parses it into typed records, and renders them as a table, JSON, CSV, or
just URLs.

```bash
fb page nasa                       # a Page's full profile
fb page nasa --posts --limit 20    # its twenty most recent posts
fb post <url> --comments           # a post and its comment thread
fb id <anything>                   # classify any Facebook id or URL
```

The binary is pure Go with no runtime dependencies and no browser. fb reads the
public pages Facebook serves to search engines, so there is no login and no
cookie. When content is private, fb is explicit about the login wall rather than
silently returning nothing.

## What you can do with it

- **Resolve anything.** Turn a slug, numeric id, `pfbid` token, `story_fbid`,
  group id, or a `fb.watch`/`fb.me`/`share/` short link into a typed identity,
  then into a full record.
- **Stream feeds.** Walk a Page, profile, or group feed to any depth, stopping
  by `--limit` or by date with `--since`/`--until`.
- **Read a post in full.** Pull a post's text, creation time, reaction, comment,
  and share counts, its media, and the preview comments Facebook renders.
- **Collect media.** Stream a handle's photos, videos, and reels, and resolve a
  single photo or video to its full metadata and playable sources.
- **Build datasets.** Expand a root into a stream of URLs and crawl them into a
  local SQLite database you can query with SQL.

## Where to go next

- New here? Start with the [introduction](/getting-started/introduction/) for
  the mental model, then the [quick start](/getting-started/quick-start/).
- Want to install it? See [installation](/getting-started/installation/).
- Looking for a specific task? The [guides](/guides/) cover how fb reads
  Facebook, pages and profiles, posts and comments, media, search, and datasets.
- Need every flag? The [CLI reference](/reference/cli/) is the full surface.
