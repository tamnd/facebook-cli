---
title: "Pages and profiles"
description: "Read a page or profile whole, read its About tab, and walk its timeline as far as the tier allows."
weight: 20
---

`fb page` and `fb profile` are the same command under two names.
Facebook stopped drawing a hard line between a Page and a person's profile years ago, and so does fb: both come back as a Profile record with a `kind` field saying which it is.

## The record

```sh
fb page nasa
fb profile zuck
fb page 100044561550831        # a numeric id works too
```

```
╭─────────────────┬────────┬──────────────────────────────────────────────────────┬────────────┬────────────┬─────────────────────────╮
│ ID              │ HANDLE │ NAME                                                 │ LIKES      │ FOLLOWERS  │ CATEGORY                │
├─────────────────┼────────┼──────────────────────────────────────────────────────┼────────────┼────────────┼─────────────────────────┤
│ 100044561550831 │ NASA   │ NASA - National Aeronautics and Space Administration │ 28,660,579 │ 28,000,000 │ Government organisation │
╰─────────────────┴────────┴──────────────────────────────────────────────────────┴────────────┴────────────┴─────────────────────────╯
```

The table is a summary.
`-o json` is the record, and for NASA it holds:

```
tier, surfaces, sources, via, fetched_at   the envelope
id, handle, name, url, kind                who it is
delegate_page_id                           the old-style page id behind a new-style profile
verified                                   the blue tick
bio                                        the short text under the name
category, category_raw                     "Government organisation", and the raw "Page · Government organisation"
likes, talking_about, followers            the three counts
websites, email, contact_other             the contact block
avatar, cover                              with dimensions and the focus point
tabs                                       the tabs this profile has, with their URLs
posts, posts_cursor, posts_truncated       the newest post, and whether there is more
```

`fb fields profile` will print the whole list with how often each one was filled across the captured pages.

## The handle is not the identity

A handle is an alias.
It names a profile until whoever owns it changes it, and then it names something else or nothing.

```sh
fb id nasa
```

```json
{"input":"nasa","kind":"handle","handle":"nasa","url":"https://www.facebook.com/nasa","command":"page",
 "note":"a handle is an alias and not an identity: it names a profile until the profile changes it"}
```

The numeric id is the identity.
Anything you are keeping, key on that.
`fb id nasa --resolve` spends one request to turn the handle into the id.

There is a second id worth knowing about.
A modern profile often carries a `delegate_page_id`, which is the old-style Page id for the same thing, and Facebook uses both in different places.
fb records both rather than picking one, because a claim made about `54971236771` and a claim made about `100044561550831` are claims about the same organisation.

## Two counts that are not the same count

`likes` and `followers` are different numbers and they come from different surfaces.
The `via` field says which:

```json
"via": {"followers": "s1", "likes": "s3", "talking_about": "s3"}
```

Followers came from the Relay store, likes and the talking-about number came from the OpenGraph meta head.
When the two disagree, they disagree because Facebook rounds them differently in the two places, not because fb got one wrong.

## The About tab

```sh
fb page nasa --about
```

That fetches a second page, which is where the long-form profile lives: the full category list, the address, the phone number, the email, every website, the founded date, and the free-text sections.

```sh
fb page nasa --about --no-posts
```

`--no-posts` skips the timeline parse when you only want the profile, which is one less thing to go wrong and slightly faster.

`--tab <name>` reads one named tab from the `tabs` list instead of the main page.

## The timeline

```sh
fb feed nasa
fb feed nasa -n 50
```

Signed out, this is what happens:

```
note: Tier 0 serves one post per profile; the timeline query is refused signed out.
      Run fb auth import to read further, or use fb group feed which is not walled.
```

One post, a note on stderr saying why, and exit 0.
One post is a real answer, not an error, and the record's `missed` field carries the same sentence so a program reading the JSON sees it too.

With a session imported, `fb feed` pages properly and `-n` means what it says.
See [sessions and tiers](/guides/authentication/).

The newest post also comes back attached to `fb page`, so if one post is all you need there is no reason to run `fb feed` at all.

## Reading several

Every read command takes one reference, and the shell is the loop:

```sh
for h in nasa spacex esa; do fb page "$h" -o jsonl; done > pages.jsonl
```

`-o jsonl` is one record per line, which is the format to reach for when you are appending.
