---
title: "HTTP and MCP"
description: "The same eighteen read operations over HTTP as NDJSON, or over stdio as MCP tools, generated from the operations rather than written twice."
weight: 75
---

The commands are not the program.
Underneath there are eighteen read operations, and the CLI is one of three ways to reach them.

## fb serve

```sh
fb serve --addr :8080
```

```
fb serving on :8080
```

Every read operation gets a route, records come back as NDJSON, one JSON object per line, flushed as they are produced.
A long crawl is consumable while it runs rather than after it finishes.

```sh
curl 'localhost:8080/v1/page/100044561550831'
curl 'localhost:8080/v1/page?ref=nasa&about=true'
curl 'localhost:8080/v1/comments?ref=https://www.facebook.com/NASA/posts/1587860636042640/&limit=50'
curl 'localhost:8080/v1/edges?ref=nasa'
```

Positional arguments work either as the path tail or as a query parameter.
The query form is the one to use for a reference containing a slash, which is most of them, because a URL in a path is a fight nobody wins.

The routes:

```
/v1/page        /v1/feed        /v1/photos      /v1/photo
/v1/post        /v1/comments    /v1/reactions   /v1/videos
/v1/video       /v1/playlists   /v1/events      /v1/event
/v1/event/suggested             /v1/group       /v1/group/feed
/v1/graph       /v1/edges       /v1/discover
```

Two more that are not operations:

```sh
curl localhost:8080/healthz
curl localhost:8080/v1/openapi.json
```

```json
{"binary":"fb","status":"ok"}
```

`openapi.json` is an OpenAPI 3.1 document generated from the operations, not maintained beside them, so it cannot describe a parameter the code does not have.
Point a client generator at it and the client is correct by construction.

### Errors

An error before any record is written comes back as JSON with the status the exit code maps to:

```
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{"error":"Facebook answered every request for profile doesnotexist000 with the log-in page: import a session with `fb auth import`","kind":4}
```

`kind` is the same number the CLI would have exited with, so a client can switch on it without parsing English.

Once the first record has been written the status is already 200 and there is no taking it back, which is the honest cost of streaming.

### Reads only, unless you say otherwise

`fb serve` exposes read operations and nothing else by default.
`--allow-writes` opens the rest, and there is nothing in fb today that writes to Facebook, so in practice it is a switch waiting for a feature.

There is no authentication on the listener.
Bind it to localhost or put something in front of it.
Anyone who can reach the port can spend your rate limit and, if you have imported a session, read as you.

## fb mcp

```sh
fb mcp
```

The same eighteen operations as MCP tools over stdio.

```json
{"protocolVersion":"2024-11-05","serverInfo":{"name":"fb","version":"0.3.0"},
 "capabilities":{"tools":{}},
 "instructions":"A fast, read-only command line for Facebook"}
```

Each tool carries the operation's own summary:

```
page        Read a Page or profile whole
feed        Read a profile's timeline (one post signed out)
photos      Read a profile's photo tab
videos      Read a profile's video tab, grid and shows together
playlists   Read the shows a profile groups its videos into
events      Read a profile's events tab
```

Wiring it into Claude Code:

```sh
claude mcp add fb -- fb mcp
```

Or by hand, in any MCP client's config:

```json
{"mcpServers": {"fb": {"command": "fb", "args": ["mcp"]}}}
```

The schemas are generated from the same parameter definitions the flags come from, so a tool call and a command line take exactly the same arguments under the same names.

## Why this is not three implementations

An operation declares its parameters, its output schema and what it does, once.
The CLI turns that into flags and a help page, `fb serve` turns it into a route and an OpenAPI entry, `fb mcp` turns it into a tool and a JSON schema.

Which means a new read command arrives on all three surfaces at once, and the three cannot describe the same operation differently, because there is only one description.
A test asserts every operation has a summary and a schema, so nothing reaches the tool list as an unlabelled `page2`.
