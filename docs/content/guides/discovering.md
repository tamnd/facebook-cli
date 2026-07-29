---
title: "The claim graph"
description: "Every read makes claims about other things. fb edges prints them, fb graph walks them, and fb rdf writes them in a vocabulary something else can read."
weight: 60
---

A record is what one page said about itself.
A claim is what it said about something else, and that is the part that composes.

## fb edges

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

`fb edges` makes no request of its own.
It routes the reference the way the ordinary read command would, does that one read, and prints the claims instead of the record.

### URIs

Everything gets a `fb://kind/id` URI, including things nobody fetched.
That is the point: the same photo referenced from a post and from an album is one node, and it stays one node across two runs and across two machines.

An external site gets one too, hashed from the URL, so a link to nasa.gov off two different pages joins.

### The note

`note` carries what the claim knew that the two URIs do not: the name of a profile nobody fetched, the URL behind an external node.
It is deliberately not part of the claim's key, because a name is a fact about the node and not about the claim.

### Who said so

Every claim also carries `source`, `surface` and `tier`: the URL that asserted it, which of the eight surfaces it came off, and at which tier.

The source is part of the claim's identity rather than metadata hanging off it.
Two surfaces asserting the same edge stay two rows, so when they disagree the disagreement is something you can query instead of something the last write silently resolved.

Duplicates within one source are dropped, because a post's author appears in the story header, in every comment's parent and in the feedback node, and seeing `authored` three times tells you nothing the first told you.

## The predicates

Nineteen of them, and that is the whole list:

```
authored        profile -> post          mentions      post -> profile
links_to        post -> external         attaches      post -> photo, video
in_album        photo -> album           next_in_album photo -> photo
comments_on     comment -> post          commented     profile -> comment
posted_in       post -> group            hosts         profile -> event
announced_by    event -> post            located_at    event -> place
in_city         place -> place           suggests      event -> event
delegates_to    profile -> page          shares        post -> post
owns            profile -> photo, video  tagged_in     profile -> photo
covers          profile, group, event -> photo
```

## fb graph

```sh
fb graph nasa --depth 2 --budget 25
```

`fb edges` plus a walk.
`--depth` is how many hops out, and `--budget` caps the requests.

The budget is counted in requests rather than nodes, and that is on purpose: requests are the unit Facebook's throttling is written in, and a walk that promises you twenty nodes spends an unknown number of requests to get there.

A node that will not read is recorded as a miss and the walk carries on:

```
warn: s1: could not read fb://photo/1496429661852405: photo 1496429661852405 was not found:
      the permalink carried no media
```

An album has no page a signed-out reader can fetch, a group behind the wall stays behind it, and neither is a reason to stop the walk.

## fb rdf

```sh
fb rdf nasa --format turtle
fb rdf nasa --format nt
fb rdf nasa --format jsonld
fb rdf nasa --depth 2 --budget 25 --format turtle
```

The same claims, in a vocabulary something else can read.

```turtle
@prefix fb: <https://tamnd.github.io/facebook-cli/ns#> .
@prefix prov: <http://www.w3.org/ns/prov#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix schema: <https://schema.org/> .

<fb://profile/100044561550831>
    fb:delegatesTo <fb://page/54971236771> ;
    rdf:type schema:Person ;
    schema:image <fb://photo/1496429661852405> ;
    schema:citation <fb://external/639a6da94698ad1a2274a8d490f69c4a97615e451e0f70ebbf0d35b4c43c7a5f> .

<fb://post/1587860636042640>
    schema:author <fb://profile/100044561550831> ;
    rdf:type schema:SocialMediaPosting ;
    schema:associatedMedia <fb://photo/1587860609375976> .
```

### Why schema.org

Facebook already publishes OpenGraph on its own pages, and OpenGraph is RDFa, so there is a vendor-blessed vocabulary here before fb invents anything.
Where `og:` runs out, schema.org carries on.
That is also what [x-cli](https://github.com/tamnd/x-cli) exports into, so a store built from both tools joins rather than sitting in two piles.

The kinds map straight across: a profile is a `schema:Person`, a page or group is an `Organization`, a post is a `SocialMediaPosting`, a comment is a `Comment`, a photo is an `ImageObject`, a video is a `VideoObject`, an event is an `Event`, a place is a `Place`, an album is an `ImageGallery`.
An external URL gets no class at all, because it is somebody else's page and guessing what kind would be inventing a fact.

Some predicates turn round on the way.
fb writes `profile authored post`, because that is how a page reads.
schema.org defines `author` on the posting, so the triple comes out as `post schema:author profile`.

Predicates schema.org has no term for go in the `fb:` namespace: `fb:delegatesTo`, `fb:inAlbum`, `fb:nextInAlbum`, `fb:announcedBy`, `fb:suggests`.
The namespace is declared in the output rather than assumed, so somebody who has never heard of `fb:inAlbum` can work out where to look without asking.

### Provenance

Provenance is on by default.
In N-Triples it uses RDF-star, so each claim is annotated with the URL that asserted it:

```
<fb://profile/100044561550831> <…#delegatesTo> <fb://page/54971236771> .
<< <fb://profile/100044561550831> <…#delegatesTo> <fb://page/54971236771> >>
   <http://www.w3.org/ns/prov#wasDerivedFrom> <https://www.facebook.com/profile.php?id=100044561550831> .
```

A dump is then as auditable as the read it came from.
`--no-provenance` turns it off for anyone who would rather have the smaller file.

## Where next

`fb graph` walks and prints.
[`fb crawl`](/guides/datasets/) walks and keeps, into a SQLite store you can query with SQL and export whole.
