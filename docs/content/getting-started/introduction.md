---
title: "Introduction"
description: "What fb reads, how Facebook is laid out from its point of view, and the model fb uses to make it scriptable."
weight: 10
---

[Facebook](https://facebook.com) is enormous and almost entirely built for a
JavaScript app, not for a quick question from a script. The official way in is
the Graph API, which means registering an app, getting it reviewed, and managing
tokens before you can read a single public post. fb closes that gap by reading
the surface Facebook still serves to a plain browser.

## The no-JavaScript surface

Facebook keeps a stripped-down, no-JavaScript version of the site at
`mbasic.facebook.com`. It is plain HTML: a Page is a heading and some text, a
feed is a list of posts with "see more" links, a comment thread is a list of
comments with a "view more comments" link. fb fetches those pages and parses the
HTML into typed records, the same way you would read them in an old phone
browser, but turned into JSON, a table, or CSV.

There is a richer mobile surface at `m.facebook.com` for cases the basic surface
cannot answer. fb picks the surface automatically; `--surface` overrides it.

## The login wall is load-bearing

Facebook shows very little to a logged-out visitor. Anonymously, most reads
return a login wall or a "content unavailable" shell, and fb reports that
honestly with a distinct exit code rather than pretending there was no data.

Depth scales with your session. Pass a cookie from a browser where you are
logged in and the same commands return full Pages, feeds, comment threads, and
reaction lists. See [authentication](/guides/authentication/) for how to supply
one.

## How fb models Facebook

Everything starts as an **identity**. A slug (`nasa`), a numeric id, a `pfbid`
token, a `story_fbid`, a group id, or a short link like `fb.watch/…` all classify
to a typed identity that says what kind of thing it is and how to reach it. `fb
id <anything>` shows exactly what fb sees, with no network access.

From an identity, fb fetches and parses one of a small, closed set of record
types:

- **Page**, **Profile**, **Group**: the entity and its metadata.
- **Post**: one story, with text, media, and counters.
- **Comment**, **Reaction**: the conversation under a post.
- **Photo**, **Video**, **Event**: media and listings.
- **SearchResult**, **Identity**: discovery and classification.

Every record is a plain struct with JSON tags. A field Facebook does not surface
stays empty rather than being guessed, so the data you get is the data Facebook
actually showed.

## Where to go next

Install the binary in [installation](/getting-started/installation/), then run
through the [quick start](/getting-started/quick-start/).
