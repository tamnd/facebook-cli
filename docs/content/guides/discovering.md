---
title: "Discovering"
description: "Walk the graph of pages, posts, authors, and comments breadth first, streaming one record per node."
weight: 55
---

Every other command answers one question about one object: a Page's feed, a
post's comments, the author of a story. `discover` chains them. From a seed it
follows the object's edges, and from each neighbor it follows theirs, hop by hop,
streaming one record per node as the node is reached.

```bash
fb discover nasa
```

A seed is anything `fb` can resolve to a page, profile, group, or post: a slug,
a numeric id, or any Facebook URL.

## The graph

There are five kinds of node. Three are **actors** that own a feed, and two are
the content hanging off them:

| Kind | What it is |
|---|---|
| `page` | a Page (org, brand, public figure) |
| `profile` | a person's public profile |
| `group` | a group |
| `post` | one story |
| `comment` | one preview comment under a post |

Between them `discover` follows three edges:

| Edge | From to | What it follows |
|---|---|---|
| `posts` | actor to post | an actor's recent feed |
| `author` | post to actor | the actor that posted a story |
| `comments` | post to comment | a post's preview comments (a leaf) |

You rarely name edges one at a time. `--follow` takes a **preset**:

| Preset | Expands to | Walk shape |
|---|---|---|
| `content` *(default)* | `posts` + `author` | actors and their posts, and from a post seed back to its author and on through their feed |
| `threads` | `posts` + `comments` | posts and the preview comments under them |
| `all` | every edge | the whole reachable neighborhood |

```bash
fb discover nasa                              # content (the default)
fb discover nasa --follow threads --depth 2   # posts, then their comments
fb discover nasa --follow all --depth 2
```

`--follow` also takes a single edge name, or a comma-separated mix of presets and
edges, so you can be exact:

```bash
fb discover "https://www.facebook.com/nasa/posts/123" --follow author
fb discover nasa --follow posts,comments
```

## Why comments need depth 2

`fb` reads the public pages Facebook serves to search engines, with no login (see
[how fb reads Facebook](/guides/authentication/)). That surface is a shallow star:
actors, their recent posts, and a few preview comments under each post. Three
things follow from that, and they shape every walk:

- **Comments are leaves.** A preview comment exposes the commenter's name and
  text, but no id or profile to hop to, so `discover` emits a comment and stops
  there. It never expands a comment.
- **A feed post's author points back where you came from.** When `discover` reads
  an actor's feed it tags each post with that actor as owner, so the `author` edge
  from a feed post lands on the actor you already have, and the walk dedups it.
- **`author` is a real hop from a post seed.** When the seed is a post URL, the
  owner is encoded in the URL, not something you walked to. Here `author` reaches
  a new actor, and one hop further `posts` reaches the rest of their feed. That is
  what `content` is tuned for.

The practical rule: comments sit one hop below their post. To reach them from an
actor seed, ask for `--depth 2`; seed a post directly and `--depth 1` is enough.

```bash
fb discover nasa --follow threads --depth 2                      # actor: needs depth 2
fb discover "https://www.facebook.com/nasa/posts/123" --follow threads  # post seed: depth 1
```

## Bounding the walk

Three independent limits keep a walk finite, so an unbounded `discover` always
terminates instead of spidering forever:

- `--depth` is how many hops to follow (default `1`; `0` emits only the seeds).
- `--fanout` caps neighbors per edge (default `25`; `0` means unlimited).
- `-n` caps the total nodes streamed (default `500`).

```bash
fb discover nasa --depth 2 --fanout 10 -n 200
```

## Reading the output

Each row is a node tagged with how it was reached: how deep, by which edge, and
the object itself. The full typed record rides along for `-o json` and `-o jsonl`,
and `-o url` prints one link per node:

```bash
fb discover nasa                 # the readable table
fb discover nasa -o jsonl        # one lossless object per line
fb discover nasa -o url          # one URL per node, to pipe onward
```

Seeds can come from stdin via `-`, so any command that emits URLs feeds a walk:

```bash
fb search "climate" -o url | fb discover - --depth 1
```

## When an edge is gated

A page that does not render for the anonymous crawler, or a feed that gets rate
limited mid-walk, is not fatal. The walk treats the two cases differently:

- A **seed** that cannot be fetched fails the walk, like any bad id.
- An edge that fails **deeper** in the walk becomes a one-line note on stderr and
  the walk carries on with the other edges. `-q` silences the notes.

## discover or crawl?

Both walk the graph from seeds, but they are built for different jobs:

- **`discover`** streams one record per node to stdout. It is for exploring,
  piping, and rendering in any output format. To keep a walk, redirect it:
  `fb discover nasa --depth 2 -o jsonl > graph.jsonl`.
- **`crawl`** fetches a queue of URLs into full records and a SQLite store,
  pulling attached data like comments. It is for building a dataset on disk. See
  [Datasets](/guides/datasets/).

`fb discover - -o url` is the bridge between them: a walk can produce the very URL
stream that `crawl` consumes.

```bash
fb discover nasa --depth 1 -o url | fb crawl --db nasa.db --comments
```
