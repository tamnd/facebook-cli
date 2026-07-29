---
title: "Quick start"
description: "From a fresh install to a page, a post, its reactions and a small graph, in six commands."
weight: 30
---

Six commands, a couple of minutes, no credential of any kind.

## 1. Check the binary

```sh
fb --version
fb id nasa
```

`fb id` classifies a handle, an id, a permalink or a short link without touching the network, so it works everywhere and is the fastest sanity check.

```json
{"input":"nasa","kind":"handle","handle":"nasa","url":"https://www.facebook.com/nasa","command":"page",
 "note":"a handle is an alias and not an identity: it names a profile until the profile changes it"}
```

## 2. See what fb would do

```sh
fb explain 'https://www.facebook.com/NASA/posts/1587860636042640/'
```

```json
{"ref":{"kind":"post","id":"1587860636042640","handle":"NASA","command":"post"},
 "route":{"command":"post","surface":"s1","operation":"CometSinglePostDialogContentQuery","tier":0,"depth":"whole"}}
```

Also offline.
It says which command handles that URL, which surface fb would read, which GraphQL operation carries the answer, and how much of the answer that surface gives.
`fb routes` prints the same thing for every command at once.

## 3. Read a page

```sh
fb page nasa --fields id,handle,name,likes,followers,category
```

```
╭─────────────────┬────────┬──────────────────────────────────────────────────────┬────────────┬────────────┬─────────────────────────╮
│ ID              │ HANDLE │ NAME                                                 │ LIKES      │ FOLLOWERS  │ CATEGORY                │
├─────────────────┼────────┼──────────────────────────────────────────────────────┼────────────┼────────────┼─────────────────────────┤
│ 100044561550831 │ NASA   │ NASA - National Aeronautics and Space Administration │ 28,660,579 │ 28,000,000 │ Government organisation │
╰─────────────────┴────────┴──────────────────────────────────────────────────────┴────────────┴────────────┴─────────────────────────╯
```

`--fields` picks columns.
Drop it and you get the default set; add `-o json` and you get the whole record, which for a page is the bio, the contact block, the websites, the cover and avatar with their dimensions, the tabs the page has, and the envelope saying where each of those came from.

`fb page nasa --about` reads the About tab instead, which is where the long-form profile lives.

## 4. Read a post, whole

```sh
fb post 'https://www.facebook.com/NASA/posts/1587860636042640/' \
  --fields id,created,author,reactions,comments,shares
```

```
╭──────────────────┬──────────────────┬──────────────────────────────────────────────────────┬───────────┬──────────┬────────╮
│ ID               │ CREATED          │ AUTHOR                                               │ REACTIONS │ COMMENTS │ SHARES │
├──────────────────┼──────────────────┼──────────────────────────────────────────────────────┼───────────┼──────────┼────────┤
│ 1587860636042640 │ 2026-07-27 21:35 │ NASA - National Aeronautics and Space Administration │ 13,054    │ 532      │ 1,058  │
╰──────────────────┴──────────────────┴──────────────────────────────────────────────────────┴───────────┴──────────┴────────╯
```

The same permalink carries the comments and the reaction breakdown, so those are two more commands over one fetch:

```sh
fb comments 'https://www.facebook.com/NASA/posts/1587860636042640/' -n 5
fb reactions 'https://www.facebook.com/NASA/posts/1587860636042640/'
```

```
╭───────┬───────┬───────╮
│ TYPE  │ COUNT │ SHARE │
├───────┼───────┼───────┤
│ Like  │ 10997 │ 0.842 │
│ Love  │ 1722  │ 0.132 │
│ Care  │ 153   │ 0.012 │
│ Haha  │ 97    │ 0.007 │
│ Wow   │ 73    │ 0.006 │
│ Angry │ 8     │ 0.001 │
│ Sad   │ 4     │ 0.000 │
╰───────┴───────┴───────╯
```

Seven types, broken out, signed out.

## 5. Read the media tabs

```sh
fb photos nasa -n 5 --fields id,size,alt
fb videos nasa -n 5 --fields id,created,plays,title
fb events nasa --fields id,name,start
```

Photos come with the alt text NASA wrote, which is often a paragraph of real description.
Videos come with their play count and, with `fb video <id> --transcript`, Facebook's own caption track.
`--download <dir>` on `fb photo` or `fb video` writes the bytes plus a JSON sidecar recording where they came from.

## 6. Turn it into a graph

Every read makes claims about other things.
`fb edges` prints just those:

```sh
fb edges nasa --fields predicate,to,note
```

```
╭──────────────┬────────────────────────────────────────────────────────────────────────────────┬──────────────────────────────────────────────────────╮
│ PREDICATE    │ TO                                                                             │ NOTE                                                 │
├──────────────┼────────────────────────────────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ delegates_to │ fb://page/54971236771                                                          │ NASA - National Aeronautics and Space Administration │
│ covers       │ fb://photo/1496429661852405                                                    │                                                      │
│ covers       │ fb://photo/416661036495945                                                     │                                                      │
│ links_to     │ fb://external/5bc0ad51cb2d1c745e141f87a537df877be241e909da9e7059bd133937fc133a │ https://www.nasa.gov/nasa-app/                       │
│ links_to     │ fb://external/639a6da94698ad1a2274a8d490f69c4a97615e451e0f70ebbf0d35b4c43c7a5f │ https://www.nasa.gov/                                │
│ authored     │ fb://post/1587860636042640                                                     │ NASA - National Aeronautics and Space Administration │
│ attaches     │ fb://photo/1587860609375976                                                    │                                                      │
╰──────────────┴────────────────────────────────────────────────────────────────────────────────┴──────────────────────────────────────────────────────╯
```

An external site gets a URI too, hashed so the same link off two different pages is the same node.

`fb crawl` walks those claims and keeps what it finds:

```sh
fb crawl nasa --depth 1 --store nasa.db
fb db stats --store nasa.db
fb query "select kind, count(*) as n from nodes group by 1 order by n desc" --store nasa.db
```

```
╭──────────┬────╮
│ KIND     │ N  │
├──────────┼────┤
│ comment  │ 10 │
│ profile  │ 4  │
│ photo    │ 4  │
│ external │ 2  │
│ post     │ 1  │
│ page     │ 1  │
│ album    │ 1  │
╰──────────┴────╯
```

One hop off one page, and there is already a small graph: the page, the page it delegates to, its cover photos, the two sites it links to, its newest post, that post's photo, that photo's album, and ten commenters.
`fb export --store nasa.db --format turtle` writes the whole thing as RDF over schema.org.

## Where to go next

- [Sessions and tiers](/guides/authentication/): what the two cookies buy and what they do not.
- [Pages and profiles](/guides/pages-and-profiles/), [posts and comments](/guides/posts-and-comments/), [media](/guides/media/): the read commands, one at a time.
- [The claim graph](/guides/discovering/) and [building a store](/guides/datasets/): edges, crawl, query, export.
- [Output formats](/reference/output/): tables, JSON, JSONL, CSV, URLs and templates.
