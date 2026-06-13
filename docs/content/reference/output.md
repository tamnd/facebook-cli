---
title: "Output"
description: "Output formats, column selection, and per-record templates."
weight: 30
---

Every command renders through one formatter, so the flags below work everywhere.

## Formats

Pick a format with `-o`, or let fb choose: a table when writing to a terminal,
JSON Lines when piped.

| Format | Best for |
|---|---|
| `table` | Reading on a terminal (aligned columns) |
| `jsonl` | Piping: one JSON object per line |
| `json` | A single JSON array |
| `csv` | Spreadsheets |
| `tsv` | Tab-separated tools |
| `yaml` | YAML documents |
| `url` | Just the URL / permalink column |
| `raw` | The upstream HTML/JSON, untouched (`--raw`) |

```sh
fb page nasa --posts -o table
fb page nasa --posts -o jsonl
fb page nasa --posts -o json
fb page nasa --posts -o csv
fb page nasa --posts -o url
```

`auto` (the default) resolves to `table` on a terminal and `jsonl` when the
output is a pipe or file, so commands read well interactively and pipe cleanly
without a flag.

## Selecting columns

`--fields` keeps and orders just the columns you name, for table, CSV, and TSV:

```sh
fb page nasa --posts --fields permalink,reactions_count,comments_count
```

`--no-header` drops the header row, for feeding another tool:

```sh
fb page nasa --posts --fields permalink -o csv --no-header
```

## The url format

`-o url` prints one URL per record. It uses the record's `url` column, falling
back to `permalink`, then `canonical_url`, then `mbasic_url`, so it works across
every record type:

```sh
fb photos nasa --limit 100 -o url
fb search nasa --type page -o url | fb page - -o jsonl
```

## Templates

`--template` runs a Go [text/template](https://pkg.go.dev/text/template) over the
full record, one line per row. Fields are the struct fields (capitalised):

```sh
fb page nasa --posts --template '{{.Permalink}} {{.ReactionsCount}} {{.CommentsCount}}'
```

A template gives you the full record, including fields that are not shown as
table columns, so it is the most flexible way to reshape output without piping
through `jq`.
