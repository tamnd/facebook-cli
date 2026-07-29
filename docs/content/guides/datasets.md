---
title: "Building a store"
description: "fb crawl walks the claim graph into SQLite, fb db stats says what landed, fb query is plain SQL, and fb export writes the whole thing as RDF."
weight: 65
---

`fb graph` walks and prints.
`fb crawl` walks and keeps.

```sh
fb crawl nasa --depth 1 --budget 20 --store ./nasa.db
```

```
read fb://profile/100044561550831: 7 claims, 0 of 20 requests spent
read fb://post/1587860636042640: 15 claims, 0 of 20 requests spent
read fb://photo/1587860609375976: 12 claims, 0 of 20 requests spent
refused https://www.facebook.com/photo/?fbid=1496429661852405: photo 1496429661852405 was not found: the permalink carried no media
refused https://www.facebook.com/photo/?fbid=416661036495945: photo 416661036495945 was not found: the permalink carried no media
```

Three reads produced 23 nodes and 34 claims, which is the whole point of storing claims rather than pages.
Most of what a store knows about is something nobody fetched.

## The budget

`--budget` is counted in requests, not nodes.

Requests are the unit Facebook's throttling is written in, so a walk that promises you fifty nodes is a walk that promises to spend an unknown amount of the thing you actually have a limited supply of.
A cache hit is not a request and does not count against the budget, which is why the run above spent zero of twenty.

The frontier is ordered by what a fetch is worth, measured against the captured pages rather than guessed, so a budget that runs out has been spent on the pages that say the most.
A post permalink outranks an album page because one of them carries twenty comments and the other carries nothing.

## The manifest

Every crawl ends with one JSON line saying what it did:

```json
{"seeds":["100044561550831"],"depth":1,"budget":20,"spent":0,
 "nodes_read":3,"nodes":23,"claims":34,"surfaces":["s1","s3"],
 "refusals":[{"uri":"fb://photo/1496429661852405","ref":"https://www.facebook.com/photo/?fbid=1496429661852405",
              "reason":"photo 1496429661852405 was not found: the permalink carried no media"}],
 "stopped":"depth","store":"/tmp/doc.db",
 "started_at":"2026-07-29T18:00:21.246808Z","finished_at":"2026-07-29T18:00:21.293642Z"}
```

`stopped` says which limit ended it: `depth`, `budget` or `limit`.
`refusals` names every node that would not read and why.

A crawl that hit the wall halfway through and a crawl that finished produce stores that look identical from the inside, and the manifest is the only thing that tells them apart.
Keep it next to the store.

## Seeds

```sh
fb crawl nasa spacex --depth 2 --budget 200 --store ./space.db
fb crawl --seed-directory --letter A --depth 1 --budget 50 --store ./a.db
```

Seeds are references in the same syntax every other command takes, so a handle, a numeric id, a permalink and a group URL all work and can be mixed.
`--seed-directory` starts from the Pages directory instead, which is the one surface with no profile behind it.
`--letter` picks which letter page.

`--resolve-opaque` is worth knowing if your crawl passes through a group: a group feed gives post authors as `pfbid` tokens, and that flag spends a request each to turn them into profiles you can key on.
It is off by default because it multiplies the request count.

## What is in the store

```sh
fb db stats --store ./nasa.db
```

```
╭─────────┬───────────────┬───────╮
│ SECTION │ KEY           │ COUNT │
├─────────┼───────────────┼───────┤
│ nodes   │ comment       │ 10    │
│ nodes   │ photo         │ 4     │
│ nodes   │ profile       │ 4     │
│ nodes   │ external      │ 2     │
│ nodes   │ album         │ 1     │
│ nodes   │ page          │ 1     │
│ nodes   │ post          │ 1     │
│ nodes   │ all           │ 23    │
│ claims  │ comments_on   │ 16    │
│ claims  │ commented     │ 5     │
│ claims  │ attaches      │ 3     │
│ claims  │ authored      │ 2     │
│ claims  │ covers        │ 2     │
│ claims  │ links_to      │ 2     │
│ claims  │ delegates_to  │ 1     │
│ claims  │ in_album      │ 1     │
│ claims  │ next_in_album │ 1     │
│ claims  │ owns          │ 1     │
│ claims  │ all           │ 34    │
│ reads   │ all           │ 0     │
╰─────────┴───────────────┴───────╯
```

Three sections, because a store answers three different questions.
`nodes` is what it knows about, `claims` is what it knows, and `reads` is what it spent to find out.

## Three tables

The store is an ordinary SQLite file with three tables, and no schema of fb's own beyond them.

`nodes` is `uri`, `kind`, `record`, `first_seen`, `last_seen`.
`record` is the full JSON of the read, envelope and all, or empty for a node nobody fetched.

`claims` is `from_uri`, `predicate`, `to_uri`, `source`, `surface`, `tier`, `seen_at`.

`reads` is one row per request the crawl actually spent.

## fb query

```sh
fb query "select kind, count(*) as n from nodes group by kind order by n desc" --store ./nasa.db
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

The string goes straight to SQLite.
There is no query dialect of fb's own here on purpose: the answer to "what does this store say" should be a query you already know how to write, and SQL is the one everybody has.

The store is opened read-only, so a finger slip that says `delete` is refused by the database rather than by a check in this tool.

Records are JSON in a column, which SQLite has functions for:

```sh
fb query "select kind, count(*) as total,
                 sum(case when record is null or record='' then 0 else 1 end) as with_record
          from nodes group by kind order by total desc" --store ./nasa.db
```

```
╭──────────┬───────┬─────────────╮
│ KIND     │ TOTAL │ WITH_RECORD │
├──────────┼───────┼─────────────┤
│ comment  │ 10    │ 0           │
│ profile  │ 4     │ 1           │
│ photo    │ 4     │ 1           │
│ external │ 2     │ 0           │
│ post     │ 1     │ 1           │
│ page     │ 1     │ 0           │
│ album    │ 1     │ 0           │
╰──────────┴───────┴─────────────╯
```

That is the query to run first on any store.
It separates what was read from what was only named, and a node with no record is a node the next crawl should visit.

### Why the same claim appears more than once

```sh
fb query "select predicate, source from claims
          where from_uri='fb://post/1587860636042640'" --store ./nasa.db
```

```
╭───────────┬───────────────────────────────────────────────────────────────────────────────────────╮
│ PREDICATE │ SOURCE                                                                                │
├───────────┼───────────────────────────────────────────────────────────────────────────────────────┤
│ attaches  │ https://www.facebook.com/permalink.php?id=100044561550831&story_fbid=1587860636042640 │
│ attaches  │ https://www.facebook.com/photo/?fbid=1587860609375976                                 │
│ attaches  │ https://www.facebook.com/profile.php?id=100044561550831                               │
╰───────────┴───────────────────────────────────────────────────────────────────────────────────────╯
```

One post attaching one photo, asserted by three different pages.
That is not a bug and it is not deduplicated away, because the source is part of the claim's identity: three independent pages agreeing is a stronger fact than one page saying it, and you can only see that if the rows survive.

`select distinct from_uri, predicate, to_uri from claims` is the view without provenance, and it is one line away whenever you want it.

## fb export

```sh
fb export --store ./nasa.db --format turtle
fb export --store ./nasa.db --format nt --no-provenance
fb export --store ./nasa.db --format jsonld
```

The whole store in the same vocabulary [`fb rdf`](/guides/discovering/) uses:

```turtle
@prefix fb: <https://tamnd.github.io/facebook-cli/ns#> .
@prefix prov: <http://www.w3.org/ns/prov#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix schema: <https://schema.org/> .

<fb://comment/1032857456032245>
    schema:parentItem <fb://photo/1587860609375976> ;
    rdf:type schema:Comment ;
    schema:parentItem <fb://post/1587860636042640> ;
    schema:author <fb://profile/100000776226475> .

<fb://photo/1587860609375976>
    rdf:type schema:ImageObject ;
    fb:inAlbum <fb://album/416661013162614> ;
    fb:nextInAlbum <fb://photo/1587644152730955> ;
    schema:creator <fb://profile/100044561550831> .
```

The kind a node was stored under is used where there is one, so a Page that was actually read comes out as an `Organization` rather than as a `Person`.
A node nobody read keeps whatever the claim implied.

The order is fixed, so two exports of the same store are the same bytes and a diff between them means something changed rather than that SQLite returned rows in a different order.

## Tee-ing without crawling

Every read command takes `--db`, which writes the record into a store as a side effect of printing it:

```sh
fb page nasa --db ./nasa.db
fb post "$URL" --db ./nasa.db
fb comments "$URL" -n 0 --db ./nasa.db
```

That is the way to build a store by hand when you know exactly which pages you want and a crawl would spend requests finding out.
Note that `--db` is the tee flag and `--store` is what `fb crawl`, `fb db`, `fb query` and `fb export` read and write.
They are different flags because tee-ing while crawling into a second store is a real thing to want.
