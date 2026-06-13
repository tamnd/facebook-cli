---
title: "Media"
description: "Stream a handle's photos, videos, and reels, resolve a single item, and list public events."
weight: 40
---

fb treats photos, videos, reels, and events as first-class records. The list
commands stream them; the singular commands resolve one item in full.

## Photos

Stream a Page or profile's photos:

```sh
fb photos nasa --limit 100 -o jsonl
```

Each record carries the photo id, the full-resolution image URL, a caption when
present, and the owner. To collect just the image URLs:

```sh
fb photos nasa --limit 200 -o url
```

Resolve one photo to its full metadata by `fbid` or URL:

```sh
fb photo "fbid=10160000000000000" -o json
```

## Videos and reels

Stream a Page's videos and reels:

```sh
fb videos nasa -o jsonl
```

Resolve one video or reel, including a short link, to its record:

```sh
fb video "https://fb.watch/xxxxx" -o json
```

A video record can include playable source URLs. Emit them with `--streams`:

```sh
fb video "https://fb.watch/xxxxx" --streams -o jsonl
```

```json
{"quality":"hd","url":"https://video.xx.fbcdn.net/..."}
{"quality":"sd","url":"https://video.xx.fbcdn.net/..."}
```

## Events

List a Page's upcoming public events:

```sh
fb events nasa -o jsonl
```

Resolve one event by id or URL to its full record (name, description, start
time, going and interested counts, online flag):

```sh
fb event 1234567890 -o json
```

## From a Page

If you start from a Page, the `page` command has `--photos`, `--videos`, and
`--events` shortcuts that stream the same records without naming the handle
twice:

```sh
fb page nasa --photos --limit 50 -o jsonl
```
