---
title: "Output"
description: "Ten formats, column selection, templates, and the envelope every record carries."
weight: 30
---

Every command renders through one formatter, so the flags on this page work on all of them.

## Formats

```
auto  table  markdown  list  json  jsonl  csv  tsv  url  raw
```

`auto` is the default: a table on a terminal, JSON Lines into a pipe.
Which means a command reads well when you type it and pipes cleanly when you do not, without a flag either way.

```sh
fb videos nasa -n 2 --fields id,created,plays -o markdown
```

```
| id               | created          | plays   |
|------------------|------------------|---------|
| 1380134307388381 | 2026-07-23 16:31 | 457,806 |
| 1376651124309650 | 2026-07-02 15:02 | 600,059 |
```

```sh
fb videos nasa -n 2 --fields id,created,plays -o csv
```

```
id,created,plays
1380134307388381,2026-07-23 16:31,"457,806"
1376651124309650,2026-07-02 15:02,"600,059"
```

`-o list` is the one to reach for when a record is too wide for a terminal:

```sh
fb videos nasa -n 1 -o list
```

```
## 1380134307388381
- **created**: 2026-07-23 16:31
- **size**: 3840x2160
- **plays**: 457,806
- **reactions**: 6,023
- **title**: NASA Astronaut Chris Williams: Thinking Like a Scientist
- **url**: https://www.facebook.com/NASA/videos/nasa-astronaut-chris-williams-thinking-like-a-scientist/1380134307388381/
```

`-o url` prints one URL per record, which is what makes the shell a pipeline:

```sh
fb photos nasa -n 20 -o url | while read u; do fb photo "$u" -o jsonl; done > photos.jsonl
```

```
https://www.facebook.com/NASA/videos/nasa-astronaut-chris-williams-thinking-like-a-scientist/1380134307388381/
https://www.facebook.com/NASA/videos/whats-up-july-2026/1376651124309650/
```

## Table columns are not record fields

This is the thing to know before writing a script.

A table shows a chosen handful of columns, formatted for reading: numbers get thousands separators, timestamps get shortened, a count of zero renders as blank and a count that is genuinely unknown renders as `n/a`.
The JSON is the record, whole and unformatted.

```sh
fb videos nasa -n 1 -o json
```

```json
{"tier":0,"surfaces":["s1"],"id":"1380134307388381","url":"…","kind":"video",
 "owner":{…},"title":"…","message":{…},"post_id":"…","width":3840,"height":2160,
 "created_at":"2026-07-23T16:31:00Z","thumbnail":"…","sd_url":"…","counts":{…}}
```

`plays` is a column on the videos table and `counts` is the field in the record.
`--fields` names columns, and a template names record fields.
Never assume one list is the other.

## --fields

```sh
fb videos nasa -n 2 --fields id,created,plays
fb comments "$URL" --fields author,replies,body
fb edges nasa --fields predicate,to,note
```

Keeps and orders exactly the columns you name, for table, markdown, CSV and TSV.
`--no-header` drops the header row.

`fb fields <kind>` lists what a record kind has.

## --template

A Go [text/template](https://pkg.go.dev/text/template) applied to each record, one line per record.
The names are the JSON field names, lowercase, and nesting works:

```sh
fb page 100044561550831 --template '{{.name}} has {{.followers}} followers'
```

```
NASA - National Aeronautics and Space Administration has 28000000 followers
```

```sh
fb post "$URL" --template '{{.id}} {{.counts.reactions}}'
```

```
1587860636042640 14120
```

Note the unformatted `28000000`, which is the point: a template gives you the record, not the table's rendering of it.

A bad template is caught before any request goes out.

## The envelope

Every record carries the same six fields before its own:

```json
{"tier":0,"surfaces":["s1","s3"],
 "sources":["https://www.facebook.com/profile.php?id=100044561550831",
            "https://www.facebook.com/NASA/"],
 "via":{"followers":"s1","likes":"s3","talking_about":"s3"},
 "fetched_at":"2026-07-29T18:02:57.973905Z"}
```

| Field | What it says |
|---|---|
| `tier` | Which tier answered |
| `surfaces` | Which of the eight surfaces were read |
| `sources` | The URLs, in order |
| `via` | Which surface each contested field came from |
| `missed` | What the read knew it did not get |
| `fetched_at` | When |

`via` only lists fields where the answer depends on which surface you ask, which is why `likes` is in it and `id` is not.

`missed` is the one to check in a pipeline.
A post with 532 comments read from a permalink that ships twenty says so there, and a consumer that ignores it will conclude the post had twenty comments.

## Empty is not zero

Tables render a real zero as blank and an unknown as `n/a`, and those are different facts.
An event with nobody going shows a blank; an event whose meta sentence did not carry the numbers shows `n/a`.

In JSON the same distinction is `0` versus the field being absent.
Nothing is defaulted to zero on the way out, because a zero fb invented is indistinguishable from a zero Facebook reported.
