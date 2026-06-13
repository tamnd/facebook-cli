---
title: "Configuration"
description: "Paths, request behavior, and every global flag."
weight: 20
---

fb works with sensible defaults out of the box. This page covers where it keeps
state and the global flags that change its behavior.

## Paths

fb follows the XDG base directory conventions:

- **Cache**: `$XDG_CACHE_HOME/fb`, or `~/.cache/fb`. Cached HTTP responses keyed
  by URL.
- **Data**: `$XDG_DATA_HOME/fb`, or `~/.local/share/fb`. Persistent state.

See the resolved paths and settings any time:

```sh
fb config show
fb config path
```

## Access

fb reads Facebook anonymously, as a web crawler, with no login and no cookie.
`fb whoami` reports the mode and the user agent in use. See
[how fb reads Facebook](/guides/authentication/) for what that surface exposes
and what stays private.

## Request behavior

| Flag | Default | Meaning |
|---|---|---|
| `--rate` | `2s` | Minimum delay between requests |
| `--retries` | `4` | Retry attempts on 429 / 5xx |
| `--timeout` | `30s` | Per-request timeout |
| `-j, --workers` | `2` | Concurrency for fan-out commands |
| `--surface` | `auto` | `mbasic`, `mobile`, or `auto` |
| `--lang` | `en-US` | `Accept-Language` / locale |
| `--proxy` | none | HTTP/SOCKS proxy URL |
| `--user-agent` | rotating | Override the default UA set |

The defaults are deliberately polite. Facebook is stricter than most targets, so
the two-second rate and modest worker count keep the crawler out of trouble.

## Caching

Responses are cached on disk by URL and reused within the freshness window:

| Flag | Default | Meaning |
|---|---|---|
| `--cache-ttl` | `1h` | How long a cached response stays fresh |
| `--no-cache` | off | Bypass the cache for this run |

Clear it with `fb cache clear`, or see where it lives with `fb cache dir`.

## Output and limits

| Flag | Default | Meaning |
|---|---|---|
| `-o, --output` | `auto` | Output format |
| `--fields` | all | Columns to keep and order |
| `--no-header` | off | Omit the header row |
| `--template` | none | Go template per record |
| `-n, --limit` | `0` | Maximum records (`0` = unlimited) |
| `--since` / `--until` | none | Bound a feed by date (`YYYY-MM-DD`) |

## Behavior

| Flag | Meaning |
|---|---|
| `--raw` | Print the upstream HTML/JSON untouched |
| `--dry-run` | Print the requests that would be made, do nothing |
| `-q, --quiet` | Suppress progress on stderr |
| `-v, --verbose` | Increase verbosity (repeatable: `-v`, `-vv`) |
| `--color` | `auto`, `always`, or `never` |
| `-y, --yes` | Assume yes to prompts |
