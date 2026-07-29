---
title: "Capturing a read"
description: "fb archive writes one read to disk whole: the HTML, every Relay payload, the request headers, and the record fb parsed out."
weight: 70
---

Every other command prints what fb understood.
`fb archive` writes down what fb was given.

```sh
fb archive nasa --dir ./capture
```

```json
{"dir":"./capture","url":"https://www.facebook.com/profile.php?id=100044561550831",
 "final_url":"https://www.facebook.com/NASA/","status":200,"bytes":1117683,
 "operations":["ProfileCometHeaderQuery","ProfileCometTimelineFeedQuery",
               "ProfilePlusCometLoggedOutRootQuery","useCometLogInFormQuery"],
 "kind":"page","claims":7,"at":"2026-07-29T18:01:37.70869Z"}
```

## What lands on disk

```
capture/
  page.html            the bytes, 1.1 MB of them
  meta.json            the request, the response, and what was found in it
  record.json          the record fb parsed
  relay/
    ProfileCometHeaderQuery.json
    ProfileCometTimelineFeedQuery.json
    ProfilePlusCometLoggedOutRootQuery.json
    useCometLogInFormQuery.json
```

One file per Relay operation is the part that makes this useful.
A Comet page ships four or five query results inline in one HTML document, and reading them apart is most of the work of understanding a surface.
`relay/` has already done that: each file is one operation's payload, named after the operation, formatted so a diff between two captures is readable.

`meta.json` carries the URL asked for, the URL landed on after redirects, the status, the byte count, the tier, the timestamp, the user agent, the request headers exactly as sent, the operation list, the preloads with their doc ids and variables, and the OpenGraph head:

```json
"preloads": [{"op":"ProfileCometHeaderQuery","doc_id":"27168670956143595",
              "variables":{"scale":1,"selectedID":"100044561550831",
                           "selectedSpaceType":"profile","userID":"100044561550831"}}]
```

Those doc ids rotate every few weeks.
Having the one that was live when the capture was taken is the difference between a bug report you can act on and a bug report you can only sympathise with.

## The cookie is redacted

`request_headers` is in the capture because half the questions about a read are questions about what was sent.
The `Cookie` header is redacted on the way out, and a test asserts it, because an archive is a thing people paste into issues.

## Never from the cache

`fb archive` bypasses the page cache unconditionally, and there is no flag to change that.
A capture of a cached page is a capture of what the cache had, which is not the question anybody archiving a page is asking.

That also means archiving spends a real request every time, so it is the one command to be careful with when the throttle is close.

## A page that will not parse still archives

This is the case the command exists for.

If fb cannot make a record out of the page, the parse error is written in beside the bytes rather than the command failing and leaving you with nothing.
You get the HTML, the Relay payloads, the headers, and the error, which is everything needed to work out whether Facebook changed the shape or fb read it wrong.

## Where the fixtures came from

Every fixture in the repository is an `fb archive` capture with the volatile parts scrubbed.
The parser tests read those captures back and assert against the records, so the tests exercise the same bytes Facebook actually served rather than a hand-written approximation of them.

That is also the shape a good bug report takes here: run `fb archive` on the page that misbehaved, look through `meta.json` for anything you would rather not publish, and attach the directory.

## Where it writes

`--dir` picks the directory.
Without it, the capture goes to a dated directory under the data directory, which is `~/.local/share/fb` unless `FB_DATA_DIR` says otherwise.

```sh
fb archive nasa
fb archive 'https://www.facebook.com/NASA/posts/1587860636042640/' --dir ./post-capture
fb archive 1526795995494848 --dir ./event-capture
```

Any reference the read commands take, `fb archive` takes.
