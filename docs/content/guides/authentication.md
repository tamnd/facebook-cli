---
title: "Authentication"
description: "Supply a Facebook session cookie, in any of three formats, to unlock full reads."
weight: 10
---

Anonymous Facebook serves a login wall or a "content unavailable" shell for most
reads. A session cookie from a browser where you are logged in unlocks full
Pages, feeds, comment threads, and reaction lists. fb never logs in for you; it
reuses a cookie you already have.

## Getting a cookie

Log in to Facebook in a browser, open the developer tools, and copy the `Cookie`
request header for a facebook.com request, or export your cookies with a
cookies.txt extension. The values that matter are `c_user`, `xs`, `datr`, and
`fr`.

## Supplying it

fb reads the cookie from the first of these that is set:

1. `--cookie "c_user=...; xs=..."`
2. `--cookie-file <path>`
3. `FACEBOOK_COOKIE` environment variable
4. `FACEBOOK_COOKIE_FILE` environment variable

The environment variable is usually the most convenient:

```sh
export FACEBOOK_COOKIE="c_user=100000000000000; xs=...; datr=...; fr=..."
fb whoami
```

```
AUTHENTICATED  USER
true           100000000000000
```

## Cookie file formats

`--cookie-file` auto-detects three formats, so most exports just work:

- A **raw header line**: the value of the `Cookie:` header, with or without the
  `Cookie:` prefix.
- A **Netscape `cookies.txt`**: the tab-separated format browser extensions
  export. fb keeps only the facebook.com cookies.
- A **JSON export**: the array of `{name, value, domain}` objects extension
  exports produce.

```sh
fb --cookie-file ~/fb-cookies.txt page nasa --posts
```

## What a session changes

| Without a session | With a session |
| --- | --- |
| Most reads exit `4` (login wall) | Full Pages, feeds, and threads |
| `fb id` still works (no network) | `fb id` unchanged |
| `fb whoami` reports `false` | `fb whoami` reports your uid |

## Staying polite

A real session is tied to your account, so fetch at a human pace. fb defaults to
a two-second delay between requests (`--rate`) and caches responses on disk so
re-runs do not hit Facebook again. Raising the rate or running many workers
against one session risks a temporary block, which fb surfaces as exit `5`.

## Security

Treat a cookie like a password: it grants access to your account. Prefer
`--cookie-file` or the environment variable over `--cookie` on the command line,
since a literal flag value can land in your shell history. fb only ever sends the
cookie to facebook.com hosts.
