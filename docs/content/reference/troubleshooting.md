---
title: "Troubleshooting"
description: "Exit codes, the login wall, rate limits, and common errors."
weight: 40
---

fb uses distinct exit codes so scripts can tell apart "private", "not found",
and a real failure. When something goes wrong, the exit code is the first thing
to check.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Generic error |
| `2` | Usage error (bad flags or arguments) |
| `3` | Content not found or unavailable anonymously |
| `4` | Login wall: the content is not public |
| `5` | Rate limited |
| `6` | Network error |

## "Login wall" (exit 4)

Facebook only puts public content on the crawler surface fb reads. Exit `4`
means the target is behind a login wall: a private profile or group, or a Page
that is not visible to anonymous visitors. There is no cookie to set; the content
is simply not reachable this way. Confirm fb is reading anonymously and check the
URL it fetched:

```sh
fb whoami
fb page nasa -vv
```

See [how fb reads Facebook](/guides/authentication/) for what the crawler
surface exposes and what stays private.

## "Content unavailable" (exit 3)

Facebook returned an error or "not available" page. The specific item is gone,
private, or region-blocked. Add `-v` to see the URL fetched and confirm it is the
one you meant.

## Rate limited (exit 5)

Facebook is throttling the crawler. fb already retries with backoff; if it still
exits `5`, slow down: raise `--rate`, lower `-j/--workers`, and avoid running
several large crawls at once. A crawler that is pushed too hard can be
temporarily blocked.

## Nothing comes back, but exit 0

A feed or list legitimately had no items (an account with no public posts, a Page
with no events). That is a clean, empty result, not an error. Add `-v` to see the
URLs fetched and confirm fb reached the page you expected.

## Seeing what fb does

`-v` (repeatable) logs request activity to stderr; `-vv` logs every URL and cache
hit. `--raw` prints the upstream HTML untouched so you can inspect exactly what
Facebook returned:

```sh
fb page nasa -vv
fb page nasa --raw | head
```

## Stale cache

If a command returns data you know is out of date, the cache is serving an old
response. Bypass it for one run with `--no-cache`, or clear it entirely:

```sh
fb page nasa --posts --no-cache
fb cache clear
```
