---
title: "CLI"
description: "Every command and subcommand, with the flags that matter."
weight: 10
---

```
fb <command> [args] [flags]
```

Run `fb <command> --help` for the full flag list on any command. This page is
the map.

## Commands

| Command | What it does |
|---|---|
| `page <slug\|id\|url>...` | A Page, fully resolved |
| `profile <user\|id\|url>` | A person's public profile |
| `group <id\|slug\|url>` | A group and its feed |
| `post <url\|id>...` | One or more posts, fully resolved |
| `comments <post>` | Every comment and reply on a post |
| `reactions <post>` | Who reacted and how |
| `photos <handle>` | Stream a handle's photos |
| `photo <fbid\|url>` | One photo, full metadata |
| `videos <page>` | Stream a Page's videos and reels |
| `video <id\|url>` | One video or reel |
| `events <page>` | A Page's public events |
| `event <id\|url>` | One public event, full |
| `search <query>` | Search across Facebook's surfaces |
| `feed <handle>...` | Stream the feed of any handle |
| `id <thing>` | Classify any Facebook id or URL |
| `seed <root> <arg>` | Expand a root into a stream of URLs |
| `crawl` | Fetch a stream of URLs into records (and a DB) |
| `db query <sql>` | Query the local SQLite store |
| `whoami` | Report the loaded session |
| `config show\|path` | Show resolved configuration and paths |
| `cache dir\|clear` | Inspect and clear the on-disk cache |
| `completion <shell>` | Generate a shell completion script |
| `version` | Print version, commit, and build date |

## Entity commands

`page`, `profile`, and `group` resolve metadata by default and stream a feed
with `--posts`.

| Flag | Commands | Meaning |
|---|---|---|
| `--posts` | page, profile, group | Stream the feed instead of metadata |
| `--about` | page, profile | Metadata / intro only |
| `--photos` | page, profile | Stream photos |
| `--videos` | page | Stream videos and reels |
| `--events` | page | Stream public events |

## Post commands

| Flag | Commands | Meaning |
|---|---|---|
| `--comments` | post | Also stream the comment thread |
| `--replies` | post, comments | Expand nested replies |
| `--reactions` | post | Emit the reaction breakdown |
| `--no-detail` | post | Counters only, faster |
| `--order` | comments | `chrono` or `ranked` |
| `--list` | reactions | Emit every reactor as a row |
| `--type` | reactions, search | Filter to one type |

## Discovery and bulk

| Flag | Commands | Meaning |
|---|---|---|
| `--type` | search, seed | Result type to search or seed |
| `--streams` | video | Emit playable source URLs |
| `--from` | crawl | Read URLs from a file instead of stdin |
| `--db` | crawl, db | SQLite store path |

## Global flags

These apply to every command. See [configuration](/reference/configuration/) for
the full list and [output](/reference/output/) for the formatting flags.

| Flag | Meaning |
|---|---|
| `-o, --output` | Output format (default auto) |
| `--fields` | Comma-separated columns to keep/order |
| `--template` | Go text/template applied per record |
| `-n, --limit` | Maximum records (`0` = unlimited) |
| `--since` / `--until` | Bound a feed by date |
| `--cookie` / `--cookie-file` | Session cookie |
| `--surface` | `mbasic`, `mobile`, or `auto` |
| `--rate` | Minimum delay between requests |
| `-j, --workers` | Concurrency for fan-out commands |
| `--no-cache` | Bypass the on-disk cache |
| `--raw` | Print the upstream HTML untouched |
| `-v, --verbose` | Increase verbosity (repeatable) |
