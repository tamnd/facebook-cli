---
title: "Finding your way in"
description: "Classify anything you paste with fb id, ask what fb would do with fb explain, and find seeds with fb discover."
weight: 55
---

Four commands answer questions about fb rather than about Facebook, and three of them make no request at all.

## fb id: what is this string?

Facebook has a dozen ways of writing down a thing.
`fb id` takes any of them and says what it is, with no network access and no session:

```sh
fb id nasa
fb id 100044561550831
fb id 'https://www.facebook.com/permalink.php?id=100044561550831&story_fbid=1587860636042640'
fb id 'https://www.facebook.com/NASA/posts/pfbid0ikEX4Jf8...'
fb id 'UzpfSTEwMDA0NDU2MTU1MDgzMToxNTg3ODYwNjM2MDQyNjQwOjE1ODc4NjA2MzYwNDI2NDA='
fb id 'https://fb.watch/abc123/'
fb id 'https://www.facebook.com/groups/1443890352589739/'
```

Each answer says the kind, the ids it could pull out, the URL fb would use, and which command handles it.
What makes it worth running is the `note`, which is where the uncertainty goes:

```json
{"input":"nasa","kind":"handle","command":"page",
 "note":"a handle is an alias and not an identity: it names a profile until the profile changes it"}

{"input":"100044561550831","kind":"numeric","id":"100044561550831",
 "note":"15 digits starting 1000 is the shape of a profile id, but only a fetch confirms it"}

{"kind":"post","id":"pfbid0ikEX4Jf8…","handle":"NASA","opaque":true,
 "note":"the pfbid in this permalink is per-render: the numeric post id comes back with the fetch"}
```

A story key decodes without a request, which is a nice trick:

```json
{"kind":"story","id":"1587860636042640","author_id":"100044561550831","command":"post --author",
 "decoded":"S:_I100044561550831:1587860636042640:1587860636042640",
 "note":"base64 of \"S:_I100044561550831:1587860636042640:1587860636042640\": the author id in it names a profile nobody fetched"}
```

### --resolve

Three references cannot be settled by parsing: a handle, a `pfbid`, and a share link.

```sh
fb id nasa --resolve
fb id 'https://fb.watch/abc123/' --resolve
```

`--resolve` spends exactly one request to turn each into the numeric id behind it.
It is opt-in because everything else `fb id` does is free, and a command that silently sometimes hits the network is a command you cannot use in a loop.

## fb explain: what would fb do?

```sh
fb explain 'https://www.facebook.com/NASA/posts/1587860636042640/'
```

```json
{"ref":{"kind":"post","id":"1587860636042640","handle":"NASA","command":"post"},
 "route":{"command":"post","surface":"s1","operation":"CometSinglePostDialogContentQuery","tier":0,"depth":"whole"}}
```

`fb id` plus the routing decision: which surface fb would read, which GraphQL operation carries the answer, at which tier, and how deep the answer goes.
Also free.
This is the command to reach for when a read came back thinner than you expected and you want to know whether that is the surface's fault or yours.

## fb surfaces, fb tiers, fb routes, fb fields

Four tables, all built from the same Go values that drive the reads, so they cannot drift from the code the way a table in a document does.

```sh
fb surfaces              # the eight places fb reads, and what each is
fb tiers                 # the two tiers and what they need
fb routes                # command to surface to operation, at each tier
fb routes --questions    # the same thing keyed by what you want to know
fb fields                # the record kinds
fb fields profile        # every field on a Profile, and how often the fixtures filled it
```

`fb routes --questions` is the one to read first, because it is keyed by what you actually want rather than by what the command is called:

```
QUESTION                                                 TIER0  TIER1  NOTE
a Page or profile                                        s1 s3  s8     surface 3 carries the like and talking-about counts
a profile's newest post                                  s1     s8 s2  one post and a cursor logged out, paged with the cursor spent
a post whole                                             s1     s8     the permalink ships 20 comments
a post's reactions by type                               s1     s8     all seven types logged out
an album's photos                                        s1     s8     walked one request at a time through the neighbour chain
a profile's About                                        s1     s8 s2  the /about page only logged out: the About query is refused
a public group                                           s1     s8     tier 0 is best effort: Facebook cuts the group route off
a public event                                           s1 s3  s8     surface 3 carries the RSVP counts, rounded
the numeric id behind a handle, a pfbid or a share link  s1     s8     one request, and only for the three parsing cannot settle
what a page claims about everything else                 s1     s8     no request of its own
one read written down whole                              s1     s8     never from the cache
```

That is an excerpt, and `-o table` renders it as a proper table.

`fb fields` deserves a caveat, and it prints its own: a count of zero is a question rather than a verdict.
The census is measured against the captured pages in the repository, and a dozen captures is a dozen pages, not the whole of facebook.com.

## fb discover: where to start a crawl

```sh
fb discover
fb discover --letter A
```

The Pages directory is the one surface with no profile behind it, and it is where a crawl gets its seeds.

```
╭────────┬─────────────────────────────────────────────────╮
│ NAME   │ URL                                             │
├────────┼─────────────────────────────────────────────────┤
│ People │ https://www.facebook.com/directory/people/      │
│ Pages  │ https://www.facebook.com/directory/pages/       │
│ Places │ https://www.facebook.com/directory/places/      │
│ A      │ https://www.facebook.com/directory/pages/A      │
│ B      │ https://www.facebook.com/directory/pages/B      │
╰────────┴─────────────────────────────────────────────────╯
```

The index reads.
The letter pages behind it are blocked to a signed-out reader more often than not, and when that happens fb says so rather than returning an empty list.
`fb crawl --seed-directory` wires this straight into a crawl.

## fb search

```sh
fb search "climate"
```

```
Search needs a session: Facebook answers a signed-out search with 404, so run fb auth import first.
```

Exit 4.
There is no partial answer to give here and fb does not invent one.
With a session imported, search works.
