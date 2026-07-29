---
title: "Media"
description: "Photos with their alt text and album neighbours, videos with captions and transcripts, and the bytes with a provenance sidecar."
weight: 40
---

Photos and videos are the part of Facebook where the public surface is most generous.
Signed out, you get the full-resolution image URL, the alt text, the album, the neighbours, the reaction breakdown and the comments, and for a video you also get the media URL in two qualities, the caption track and Facebook's own transcript.

## The photo tab

```sh
fb photos nasa -n 20
fb photos nasa -n 20 --fields id,size,alt
```

The tab pages on a cursor, eight photos per request, and `-n` says how many to collect.

Alt text is the field worth knowing about.
NASA writes real descriptions, and they come back whole:

```
The Roman Space Telescope, several stories tall and covered in gray foil, towers over a group of
about a dozen technicians, wearing dark blue clean suits and full face masks; they are posing in
front of the telescope for the photo. The cavernous, white servicing room surrounds them.
Credit: NASA/Sydney Rohde (Rocz)
```

## An album

```sh
fb photos nasa --album 1587860609375976 -n 6
```

`--album` takes any photo in the set rather than the album id, and that is deliberate.
An album's own page carries nothing to a signed-out reader.
What does work is the neighbour chain the viewer's arrows use, so fb walks it one request per photo, which is why `--album` is slower than the tab and why `-n` matters more.

`fb photo <id>` prints `album_id`, so you can tell which set you are in before you walk it.

## One photo

```sh
fb photo 1587860609375976 -o json
```

```
id, url                     the photo and its permalink
image                       uri, width, height, alt, and whether the URL is signed
owner                       who posted it
album_id, album_kind        the set it belongs to
post                        the post it was attached to
caption                     the post's message, with entities
counts                      reactions by type, comments, shares
comments                    the comments the permalink ships
next                        the next photo in the album
```

`next` is what makes `--album` possible, and it is also a claim: `fb edges` emits it as `next_in_album`.

## Videos and reels

`fb video` and `fb reel` are the same command, and it takes all three of the routes Facebook serves one video under.

```sh
fb video 1380134307388381
fb video 'https://www.facebook.com/reel/1380134307388381/'
fb video 'https://www.facebook.com/watch/?v=1380134307388381'
```

fb reads the reel route first, because that is the one that carries the media URLs, the dimensions, the captions and the music.

```
id, url, kind               reel or video
owner, title, message
post_id                     the post the video hangs off
width, height               3840x2160
duration_seconds            112.946
created_at
thumbnail
sd_url, hd_url, dash_url    the media, in both qualities
captions                    the caption tracks, with locale and an .srt URL
music                       when there is a track attached
privacy
counts
```

## Transcripts

```sh
fb video 1380134307388381 --transcript
```

That fetches the watch route as well, which is where Facebook publishes its own machine transcript.
It is the only long-form text a video has, it is often a couple of thousand words, and it is not on the reel route at all:

```
I remember really well the first time
realizing that you could use math and science and technology
and laws of the universe to figure out how things work…
```

## The video tab

```sh
fb videos nasa -n 20
fb videos nasa --playlists
```

The tab is two queries stitched into one record: the video grid and the shows.
`--playlists` prints the shows instead of the videos.

Play counts come back on the tab, which is one of the few places Facebook publishes them:

```sh
fb videos nasa -n 2 --fields id,created,size,plays,reactions
```

```
╭──────────────────┬──────────────────┬───────────┬─────────┬───────────╮
│ ID               │ CREATED          │ SIZE      │ PLAYS   │ REACTIONS │
├──────────────────┼──────────────────┼───────────┼─────────┼───────────┤
│ 1380134307388381 │ 2026-07-23 16:31 │ 3840x2160 │ 457,806 │ 6,023     │
│ 1376651124309650 │ 2026-07-02 15:02 │ 1920x1080 │ 600,059 │ 6,616     │
╰──────────────────┴──────────────────┴───────────┴─────────┴───────────╯
```

## Downloading

```sh
fb photo 1587860609375976 --download ./out
fb video 1380134307388381 --download ./out
fb photos nasa --album 1587860609375976 -n 6 --download ./out
```

`--download` takes the directory as its value.
For a video it picks HD when there is one and falls back to SD.

Beside every file goes a `.json` sidecar with the record that produced it: the id, the owner, the alt text, the source URL and when it was fetched.
A folder of `.jpg` files with no provenance is a folder of files nobody can cite later, and the sidecar is the fix.

The command prints where the file landed and how big it is rather than the record.
The CDN URL is deliberately not a column: it is five hundred characters of signature, it stops working within the hour, and in a table it hides the two things you are actually reading.

## A note on CDN URLs

Every `scontent.*.fbcdn.net` URL Facebook hands out is signed and time-limited.
fb marks them `"signed": true` and keeps them anyway, because they are what the page said, but a URL you saved yesterday will not fetch today.
If you want the bytes, download them.
