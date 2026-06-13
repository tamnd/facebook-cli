---
title: "How fb reads Facebook"
description: "fb reads the server-rendered pages Facebook serves to search engines, so there is no login and no browser."
weight: 10
---

fb does not log in. It reads the same server-rendered pages Facebook serves to
search engines: the public version of a Page, profile, group, or post, with the
text, counts, media, and a few preview comments baked into the HTML. There is no
cookie, no password, and no browser to drive.

```sh
fb whoami
```

```
MODE       USER_AGENT
anonymous  Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)
```

## What you get

For a public Page, profile, group, or post, fb returns the same fields you would
read on the page itself:

- Page and profile records: name, about text, like and follower counts, avatar.
- Posts: text, creation time, reaction, comment, and share counts, attached
  media, and outbound links.
- A handful of preview comments per post, with the commenter's name.

`fb id` classifies any URL or id offline and never touches the network.

## The trade-off: depth

Reading the crawler surface keeps fb to a single binary with no login, at the
cost of depth. Two limits are worth knowing:

- **Recent posts only.** A feed exposes roughly the most recent posts, not the
  full history. There is no deep pagination, so `--limit` above what the page
  carries simply returns what is there.
- **Preview comments only.** Each post carries a few preview comments, not the
  full thread, and the commenter attribution is approximate. Comment timestamps
  are not exposed on this surface.

## What stays private

Anything Facebook does not put on the public crawler page is not reachable: the
content behind a login wall, private groups and profiles, full comment threads,
and per-reactor lists. When a target is private or removed, fb exits `4` (login
wall) or `3` (content unavailable) rather than guessing.

## Staying polite

fb defaults to a two-second delay between requests (`--rate`) and caches
responses on disk, so re-runs do not hit Facebook again. Raising the rate or
running many workers risks a temporary block, which fb surfaces as exit `5`.
Fetch at a human pace.
