---
title: "Troubleshooting"
description: "The nine exit codes, what each one means for a script, and what to do about the wall."
weight: 40
---

Every failure is classified before it is printed, so a script can branch on the exit code without reading the sentence.

## Exit codes

| Code | Meaning | What to do |
|---|---|---|
| `0` | Success | |
| `1` | Something unclassified went wrong | Rerun with `-vv` and file it |
| `2` | Usage: bad flags or arguments | The message names the fix |
| `3` | The read worked and there was nothing in it | Usually nothing; it is a real answer |
| `4` | Needs a session, or Facebook refused | `fb auth import`, or wait |
| `5` | Rate limited after the retries | Raise `--rate`, wait |
| `6` | The thing does not exist | Check the id |
| `7` | fb cannot do this here | The message says why |
| `8` | Network | Your problem, not Facebook's |

An empty result is exit 3 and not an error.
A Page with no events has no events, and a command that treated that as a failure would make every honest empty answer look like a bug.

## Exit 4 is two different things

This is the one worth understanding, because the same code covers a wall you can climb and a wall you cannot.

**A private thing.** A private profile, a closed group, a post visible only to friends.
No session fixes this unless the session belongs to somebody who can see it.

**A refusal.** Facebook decided this address has asked enough and served the log-in page instead of the data.
The content is public and will be readable again later.

fb says which it is:

```
Facebook answered every request for group 1443890352589739 with the log-in page.
It cuts a signed-out reader off from the group route after a few dozen requests, whatever the
group, and it does not lift for a while: wait, or import a session with `fb auth import`.
```

### The throttle is per URL

Measured rather than assumed: with `/nasa` walled, `/nasa/photos`, `/nasa/videos` and `/nasa/events` all answered normally, and `/nasa` came back on its own after about fifteen minutes.

So when one read walls:

- The other tabs of the same profile are probably fine.
- Waiting works, and about fifteen minutes is the usual figure for a profile route.
- The `/groups/` route is the exception and stays shut a good deal longer.
- Hammering it makes it worse and there is no trick that does not.

### refusalCode 1675004

Facebook has a refusal that arrives as HTTP 200 with a body saying "Rate limit exceeded".
It is not a rate limit.
It is an allowlist decision about the GraphQL operation you asked for, and it will say the same thing tomorrow.

fb recognises it, maps it to exit 4 rather than 5, and does not retry it.
Retrying an allowlist refusal is asking the same question louder.

## Exit 2: a post id on its own

```sh
fb post 1587860636042640
```

```
Facebook has no route that takes a post id alone: pass --author, or give the whole permalink URL.
```

A post id means nothing without the profile it belongs to, and Facebook has no URL that takes one.
fb refuses rather than guessing, because a guess here reads somebody else's post and tells you it is yours.

## Exit 6: not found

```sh
fb photo 1496429661852405
```

```
Photo 1496429661852405 was not found: the permalink carried no media.
```

Worth knowing: a Page's cover and avatar have photo ids that do not resolve as photo permalinks.
They are real ids naming real images and they have no page of their own, so a crawl records them as a refusal and carries on.

## Exit 7: not here

`fb search` without a session, an album whose page carries nothing, paging when `--tier` has pinned a surface that cannot page.

```
paging needs the GraphQL surface, which this --tier does not allow
```

The message always says what would make it work.

## Seeing what happened

```sh
fb page nasa -v
fb page nasa -vv
```

`-v` logs the requests.
`-vv` logs every URL, every cache hit, and every Relay operation found in the page.

When a read comes back thinner than you expected, `fb explain` is faster than either:

```sh
fb explain 'https://www.facebook.com/NASA/posts/1587860636042640/'
```

It says which surface fb would use and which operation carries the answer, without asking Facebook anything, so you can tell a thin surface from a wrong reference before spending a request.

## When the answer is stale

```sh
fb page nasa --no-cache
fb cache clear
```

The default freshness window is fifteen minutes.

## When you think fb parsed it wrong

```sh
fb archive <the thing that misbehaved> --dir ./capture
```

That writes the HTML, every Relay payload one file per operation, the request headers with the cookie redacted, and the record fb made.
A page fb cannot parse at all still archives, with the parse error beside the bytes.

Attach the directory to the issue.
It is the same format the fixtures in the repository are in, so a capture can become a regression test the same day.
