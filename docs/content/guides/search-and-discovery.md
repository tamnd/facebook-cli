---
title: "Search and discovery"
description: "Search across Facebook's surfaces, and classify any id or URL with fb id."
weight: 50
---

Two commands cover discovery: `search` finds entities by keyword, and `id`
classifies any handle or URL into a typed identity with no network access.

## Searching

`search` queries across Facebook's surfaces and streams typed results:

```sh
fb search "climate" -o jsonl
```

Narrow to one kind with `--type`:

```sh
fb search "climate" --type page --limit 50 -o jsonl
fb search "nasa" --type group -o jsonl
```

The accepted types are `page`, `profile`, `group`, `post`, `photo`, `video`,
`event`, and `all` (the default). Each result carries its type, name, URL, and a
snippet, so you can pipe the URLs straight into another command:

```sh
fb search "nasa" --type page -o url | fb page - -o jsonl
```

## Classifying ids and URLs

`fb id` is the fastest command in the tool: it does no network work and no
login, it just parses. It recognises every shape Facebook uses:

```sh
fb id nasa
fb id "https://www.facebook.com/nasa/posts/pfbid0xyz"
fb id "story.php?story_fbid=111&id=222"
fb id "https://www.facebook.com/groups/123456789"
fb id "https://www.facebook.com/watch/?v=987654321"
fb id "profile.php?id=100000000000001"
```

Each prints a typed identity: the kind (page, profile, group, post, photo,
video, event), the ids it pulled out, and the canonical and mbasic URLs.

```json
{"input":"https://www.facebook.com/nasa/posts/pfbid0xyz","kind":"post","post_id":"pfbid0xyz","owner_id":"nasa","canonical_url":"https://www.facebook.com/nasa/posts/pfbid0xyz","mbasic_url":"https://mbasic.facebook.com/nasa/posts/pfbid0xyz"}
```

## Short links

`fb.watch`, `fb.me`, and `share/` links cannot be classified by their text
alone, so `fb id` follows the redirect to resolve them to a real id:

```sh
fb id "https://fb.watch/xxxxx" -o json
```

This is the one case where `fb id` makes a network request. Everything else is
pure parsing.
