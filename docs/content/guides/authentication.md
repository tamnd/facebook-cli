---
title: "Sessions and tiers"
description: "Tier 0 needs nothing at all. Tier 1 is two cookies you paste in yourself, and this is what they buy."
weight: 10
---

fb has two tiers and no third.

```sh
fb tiers
```

```
╭──────┬─────────┬────────────────────────────────────────────────┬────────╮
│ TIER │ NAME    │ NEEDS                                          │ ACTIVE │
├──────┼─────────┼────────────────────────────────────────────────┼────────┤
│ 0    │ public  │ nothing at all                                 │ true   │
│ 1    │ session │ two cookies from a browser you are signed into │        │
╰──────┴─────────┴────────────────────────────────────────────────┴────────╯
```

There is no app id, no developer app, no API token and no OAuth flow anywhere in this tool.
There is nothing to sign up for and nothing to wait for review on.

## What tier 0 gives you

Almost everything, and it needs nothing at all:

- A page or profile whole, including the About tab.
- A post whole, with its reaction breakdown by type and the comments the permalink ships.
- The photo tab, an album walked photo by photo, and the image bytes.
- The video tab, a single video or reel, its captions, and Facebook's own transcript.
- The events tab and an event permalink.
- The public shell of a group: name, id, member and post counts, privacy, the description.
- The Pages directory, which is where a crawl gets its seeds.

## What tier 1 adds

Three things, and that is the honest list:

- **The timeline past the first post.** Signed out, Facebook ships exactly one post with a profile page and refuses to page. With a session, `fb feed` pages properly.
- **A group's discussion.** `fb group feed` needs a session for most groups.
- **Search.** Facebook answers a signed-out search with a 404, so `fb search` exits 4 rather than pretending.

That is it.
A session does not unlock private profiles, private groups you are not in, per-reactor lists, or anyone's messages.
fb reads what your account can already see in a browser, and nothing else.

## Importing a session

You paste the cookies in yourself.
fb never asks for your password and never signs in on your behalf.

Open facebook.com in a browser you are signed into, open the developer tools, find the cookies for `www.facebook.com`, and copy two of them: `c_user`, which is your numeric account id, and `xs`, which is the session itself.

```sh
fb auth import --c-user 100044561550831 --xs '<the xs value>'
fb auth status
fb whoami
```

`fb auth status` reads the stored file and says whether one is there and whose it is.
`fb whoami` spends a request to prove it against Facebook, which is the difference between a session that exists and a session that works.

```sh
fb auth logout
```

deletes the file.

## Where the cookies live, and where they do not go

The session is written to `session.json` under the data directory, which `fb config path` will print.
It is sent to facebook.com and to nowhere else.

`fb archive` is the one command that writes request headers to disk, and it redacts the cookie before it does.
There is a test that builds an engine with a fake `xs`, runs the archive header path, and fails if the secret appears in what would be written.

## Capping the tier

`--tier` is a ceiling, not a switch:

```sh
fb page nasa --tier 0        # read as if signed out, even with a session stored
fb feed nasa --tier 1        # allow the session, which is the default when one exists
```

`--tier 0` is the useful one.
It is how you check whether a field really is public before building something on it, and how you reproduce what somebody without a session would see.

You can also pin one surface: `--tier comet`, `graphql`, `og`, `embed` or `session`.
That is a debugging tool.
Pinning `og` reads only the meta head, which is a quick way to see which fields come from there and which come from the Relay store.

## When Facebook says no

Facebook throttles by URL, and it throttles a signed-out reader harder than a signed-in one.
When it decides to, it answers a perfectly ordinary GET with the log-in page, HTTP 200 and all.
fb calls that what it is:

```
Facebook answered every request for profile nasa with the log-in page: import a session with `fb auth import`
```

That exits 4.
It usually means throttling rather than a page that genuinely needs a session, and the giveaway is that other routes on the same profile keep answering: with `/nasa` walled, `/nasa/photos`, `/nasa/videos` and `/nasa/events` carried on fine, and `/nasa` came back about fifteen minutes later.

There is a second refusal worth naming.
Facebook sometimes returns HTTP 200 with a body saying "Rate limit exceeded" and an error code of 1675004.
That one is not a rate limit at all, it is an allowlist refusal for an operation signed-out readers are not given, so fb maps it to exit 4 and never retries it.

## Staying polite

fb waits a second between requests by default, and caches every page it fetches for fifteen minutes, so re-running a command is free.

```sh
fb --rate 2s crawl nasa --depth 2      # slower
fb --no-cache page nasa                # ignore the cache for this read
fb cache                               # how big it is
fb cache clear                         # empty it
```

A crawl's `--budget` is counted in requests rather than nodes, for the same reason: requests are the unit the throttling is written in, and a cache hit does not count against it.
