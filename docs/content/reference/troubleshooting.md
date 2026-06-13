---
title: "Troubleshooting"
description: "Exit codes, the login wall, rate limits, and common errors."
weight: 40
---

fb uses distinct exit codes so scripts can tell apart "no session", "not found",
and a real failure. When something goes wrong, the exit code is the first thing
to check.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Generic error |
| `2` | Usage error (bad flags or arguments) |
| `3` | Content not found or unavailable anonymously |
| `4` | Login wall: a session cookie is required |
| `5` | Rate limited |
| `6` | Network error |

## "Login wall" (exit 4)

The command needs a session and you have not supplied one, or the cookie has
expired. Set a fresh cookie and confirm it loaded:

```sh
export FACEBOOK_COOKIE="c_user=...; xs=..."
fb whoami
```

If `fb whoami` reports `false`, the cookie did not parse; see
[authentication](/guides/authentication/) for the accepted formats. A cookie
also expires when you log out of that browser session, so re-copy it if reads
that worked yesterday now wall.

## "Content unavailable" (exit 3)

Facebook returned an error or "not available" page. Anonymously this is the
normal response for most content, so the usual fix is to add a session. With a
session, exit `3` means the specific item is gone, private, or region-blocked.

## Rate limited (exit 5)

Facebook is throttling the session. fb already retries with backoff; if it still
exits `5`, slow down: raise `--rate`, lower `-j/--workers`, and avoid running
several large crawls against one session at once. A session that is pushed too
hard can be temporarily blocked.

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
