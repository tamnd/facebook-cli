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

## The crawler surface

When a search engine fetches a Facebook page, Facebook serves it server-rendered
HTML: the public version of a Page, profile, group, or post, with the text,
counts, media, and a few preview comments baked into the page. fb reads that same
surface. It presents itself as a web crawler, parses the rendered HTML into typed
records, and turns them into JSON, a table, or CSV. There is no login, no cookie,
and no browser to drive.

A few metadata reads still fall back to the stripped-down `mbasic.facebook.com`
and `m.facebook.com` surfaces; fb picks the surface automatically and `--surface`
overrides it.

## What is public, and what is not

fb only sees what Facebook puts on the public crawler surface. That is plenty for
public Pages, profiles, groups, and posts, but it has limits: a feed exposes the
most recent posts rather than the full history, and a post carries a few preview
comments rather than its whole thread. Private profiles and groups, content
behind a login wall, and per-reactor lists are not reachable, and fb reports that
with a distinct exit code rather than pretending there was no data. See
[how fb reads Facebook](/guides/authentication/) for the full picture.

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
