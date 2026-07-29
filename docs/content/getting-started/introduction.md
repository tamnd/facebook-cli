---
title: "Introduction"
description: "Where fb gets its data, what the two tiers mean, and how a Facebook page becomes a typed record."
weight: 10
---

There are two usual ways to read [Facebook](https://facebook.com) from a script, and both are bad.
The Graph API means registering an app, getting it reviewed, and carrying a token that expires, all before you can read one public post.
Driving a headless browser means a pile of CSS selectors that break the next time somebody ships a redesign.

fb takes a third route, and it is the one that has been sitting in plain sight the whole time.

## The page ships its own data

Load facebook.com signed out and look at the HTML.
Most of it is the app shell, but scattered through it are blocks like this:

```html
<script type="application/json" data-sjs>{"require":[["RelayPrefetchedStreamCache", ...
```

That is the Relay store.
It is the result of the GraphQL queries the page ran on the server, serialised into the document so the JavaScript app can start without a round trip.
The name, the id, the follower count, the post text with its link ranges, the reaction breakdown, the photo URLs: all of it is in there as JSON, already structured, before any pixel is drawn.

fb reads that.
Not the rendered text, not the DOM, the same data the page renders itself from.
When a redesign moves a heading around, the store does not care.

Facebook splits one page's data across several of those blocks, so fb extracts every block, decodes it, collects the query results by their operation name, stitches the pieces of one operation back together, and parses the result into a typed record.
That pipeline is the whole tool.

## Eight surfaces

The inline store is the main one, but it is not the only place Facebook leaves data lying around.
`fb surfaces` lists all of them:

```
╭────┬──────────────────┬──────────────────────┬──────┬───────────────────────────────────────────╮
│ ID │ NAME             │ HOST                 │ TIER │ FORMAT                                    │
├────┼──────────────────┼──────────────────────┼──────┼───────────────────────────────────────────┤
│ s1 │ Comet page       │ www.facebook.com     │ 0    │ HTML carrying Relay query results as JSON │
│ s2 │ Comet GraphQL    │ /api/graphql/        │ 0    │ JSON, one object per line                 │
│ s3 │ OpenGraph head   │ the same HTML        │ 0    │ meta tags                                 │
│ s4 │ Embed plugins    │ /plugins/*.php       │ 0    │ HTML                                      │
│ s5 │ Picture redirect │ graph.facebook.com   │ 0    │ 302 to the CDN                            │
│ s6 │ Media CDN        │ scontent.*.fbcdn.net │ 0    │ bytes                                     │
│ s7 │ Pages directory  │ /directory/pages/    │ 0    │ HTML, anchors only                        │
│ s8 │ Session Comet    │ www.facebook.com     │ 1    │ as surfaces 1 and 2                       │
╰────┴──────────────────┴──────────────────────┴──────┴───────────────────────────────────────────╯
```

Surface 2 deserves a note, because it is the one non-GET in the tool.
A page that ships a Relay store also ships the doc id and the variables it used, so fb can take that exact query, change the cursor, and ask for the next page of results.
It is a replay of a query the page just handed over, not an invented one, and no doc id is ever hardcoded because Facebook rotates them every few weeks.

## Two tiers

```
╭──────┬─────────┬────────────────────────────────────────────────┬────────╮
│ TIER │ NAME    │ NEEDS                                          │ ACTIVE │
├──────┼─────────┼────────────────────────────────────────────────┼────────┤
│ 0    │ public  │ nothing at all                                 │ true   │
│ 1    │ session │ two cookies from a browser you are signed into │        │
╰──────┴─────────┴────────────────────────────────────────────────┴────────╯
```

Tier 0 is the default and it needs nothing.
Pages, posts with their comments and reactions, photos, videos with transcripts, events, the public shell of a group, and the pages directory all read at tier 0.

Tier 1 is two cookies, `c_user` and `xs`, that you paste in yourself from a browser you are already signed into.
It buys the timeline past the first post, group discussions, and search.
fb never asks for your password, never logs in for you, and never sends a cookie anywhere but facebook.com.
See [sessions and tiers](/guides/authentication/) for what each one costs.

`fb routes` prints the whole table: which surface and which GraphQL operation answers each command, at which tier, and how deep the answer goes.

## What comes back

Everything starts with a reference.
A handle, a numeric id, a `pfbid` permalink, a `story_fbid` query, a group id, a `fb.watch` short link: they all classify to a typed reference that says what kind of thing it is and which command reads it.

```sh
fb id nasa
fb id 'https://fb.watch/abc123/'
fb explain 'https://www.facebook.com/NASA/posts/1587860636042640/'
```

None of those three touch the network.
`fb id` says what the string is, and `fb explain` goes one step further and says what fb would do about it:

```json
{"ref":{"kind":"post","id":"1587860636042640","handle":"NASA","command":"post"},
 "route":{"command":"post","surface":"s1","operation":"CometSinglePostDialogContentQuery","tier":0,"depth":"whole"}}
```

From a reference, fb produces one of a closed set of records: Profile, Post, Comment, Photo, Video, Event, Group, Section, Directory.
Each is a plain struct with JSON tags, and `fb fields <kind>` will list every field on any of them.

## Every record says what it did not see

This is the part that matters more than it sounds.
A scraper that half worked and a scraper that fully worked look identical from the outside, because both return a record.
So every record fb produces carries an envelope:

- `tier`: the tier it was read at.
- `surfaces`: which of the eight answered, as ids.
- `sources`: the URLs behind it.
- `via`: for the interesting fields, which surface each one came from.
- `missed`: what fb knows it did not get, in words.
- `fetched_at`: when.

Read NASA signed out and `missed` will say the timeline stops after the first post, because signed out it does.
That is a different thing from NASA having posted once, and the record is explicit about which it is.

## Read-only, and checked

fb never posts, likes, comments, follows or joins anything.
Every request is a GET except the one replay described above, and that is not a promise in a README.
A test parses the package as source and fails the build if a second non-GET appears anywhere, if the replay's URL stops being the GraphQL endpoint, if any string in the package ends in `Mutation`, or if any registered operation is anything but a read.

## Where to go next

Install the binary in [installation](/getting-started/installation/), then run through the [quick start](/getting-started/quick-start/).
