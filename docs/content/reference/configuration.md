---
title: "Configuration"
description: "Where fb keeps things, what the request defaults are, and how the cache and the tier flag behave."
weight: 20
---

fb needs no configuration to work.
This page is what it resolved and how to change it.

```sh
fb config show
```

```json
{"cache":"true","cache_dir":"/Users/you/.local/share/fb/cache","cache_ttl":"15m0s",
 "data_dir":"/Users/you/.local/share/fb","rate":"1s","retries":"2",
 "session":"/Users/you/.local/share/fb/session.json","signed_in":"no",
 "tier":"auto","timeout":"30s"}
```

`fb config path` prints just the data directory, which is the one you want in a script.

## One directory

Everything fb keeps lives under the data directory: the page cache, the session file, and any store you did not give an absolute path to.

```
~/.local/share/fb/
  cache/          the page cache, keyed by request URL
  session.json    the two cookies, if you imported them
  store.db        the default crawl store
```

Resolution order:

1. `--data-dir`
2. `FB_DATA_DIR`
3. `$XDG_DATA_HOME/fb`
4. `~/.local/share/fb`

One directory rather than the usual three is deliberate.
Splitting cache from data means a Docker user has to mount two volumes and will mount one, and the failure is silent: reads work and the session vanishes between runs.
`FB_DATA_DIR` is the only environment variable fb reads.

## Request defaults

| Flag | Default | What it does |
|---|---|---|
| `--rate` | `1s` | Minimum delay between requests |
| `--retries` | `2` | Retries on rate limit or 5xx |
| `--timeout` | `30s` | Per-request timeout |
| `--proxy` | none | An HTTP proxy for every request |

The retry count is low on purpose.
Facebook's refusals are mostly not transient: a `refusalCode 1675004` arriving as HTTP 200 is an allowlist decision, and retrying it is asking the same question louder.
fb recognises that one and does not retry it at all.

`--rate` is the flag to reach for when crawling.
Facebook throttles per URL rather than per address, so the way to stay under it is to spread the same requests over more time, not to spread them over more URLs.

## Caching

| Flag | Default | What it does |
|---|---|---|
| `--cache-ttl` | `15m` | How long a cached page counts as fresh |
| `--no-cache` | off | Bypass the cache for this run |

```sh
fb cache
fb cache clear
```

```json
{"bytes":"22574780","dir":"/Users/you/.local/share/fb/cache","files":"74","ttl":"15m0s"}
```

The cache is keyed by the request URL, which has a consequence worth knowing: `fb page nasa` and `fb page 100044561550831` are different keys for the same Page, because they are different requests.

A long `--cache-ttl` is the way to re-read what you already fetched without spending a request, which is useful when the throttle has closed and you want to work on what you have:

```sh
fb page 100044561550831 --cache-ttl 720h
```

A cache that will not open is reported and the run carries on without one.
Being unable to write a cache file is not a reason to refuse to read a page.

`fb archive` ignores all of this and always fetches, because a capture of a cached page is not a capture.

## The tier flag

`--tier` is two flags sharing one string.

A number caps what the run may use:

```sh
fb page nasa --tier 0    # read the way a machine with no cookies would
```

That is how you check a tier 0 claim while having a session imported.
The default is `auto`, which means tier 1 if cookies are stored and tier 0 if not, so the common case needs no flag.

A name pins one surface, and pinning fails rather than falling back:

```sh
fb page nasa --tier og        # answer from the meta head alone
fb page nasa --tier comet     # the Relay store alone
fb page nasa --tier embed     # the plugin surface alone
```

`comet`, `graphql`, `og`, `embed`, `session`.
This is how you find out which surface a field actually came from, which matters because the `via` map on a record is only as trustworthy as the thing that fills it.

Anything else is a usage error listing the valid values, rather than a typo that quietly renders the default.

## Output

| Flag | Default | What it does |
|---|---|---|
| `-o --output` | `auto` | The format. See [Output](/reference/output/) |
| `--fields` | all | Which columns, in which order |
| `--no-header` | off | Omit the header row |
| `--template` | none | A Go template applied per record |
| `--color` | `auto` | `NO_COLOR` is honoured |
| `-n --limit` | `0` | Stop after N records, 0 for no limit |

A bad `--template` fails before any request is made rather than after the fetch, which is the difference between a typo costing nothing and a typo costing a page you now have to wait to read again.
