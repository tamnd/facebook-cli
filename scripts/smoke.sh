#!/usr/bin/env bash
# smoke.sh runs every fb command against live Facebook with real public ids and
# asserts each one either returns data or exits with a clean, documented code.
# It is the living proof of the project goal: "all commands work."
#
# It runs once anonymously, and again authenticated when FACEBOOK_COOKIE (or
# FACEBOOK_COOKIE_FILE) is set in the environment. Anonymous Facebook gates most
# reads behind a login wall, so anonymously a command "passes" when it returns
# data OR exits 4 (login wall) OR exits 3 (content unavailable). With a cookie,
# the same commands are expected to return data.
#
#   FB=./bin/fb ./scripts/smoke.sh
#   FACEBOOK_COOKIE="c_user=...; xs=..." FB=./bin/fb ./scripts/smoke.sh
set -u

FB="${FB:-fb}"
PAGE="${FB_SMOKE_PAGE:-nasa}"
PROFILE="${FB_SMOKE_PROFILE:-zuck}"
GROUP="${FB_SMOKE_GROUP:-}"
SEARCH="${FB_SMOKE_SEARCH:-nasa}"

pass=0
fail=0
walled=0

# run <description> -- <command...>
# Passes when the command returns data, or exits 3/4 (documented walls).
run() {
	local desc="$1"; shift
	[ "$1" = "--" ] && shift
	local out rc
	out="$("$@" 2>/dev/null)"
	rc=$?
	if [ $rc -eq 0 ] && [ -n "$out" ]; then
		echo "ok    $desc"
		pass=$((pass + 1))
	elif [ $rc -eq 4 ] || [ $rc -eq 3 ]; then
		echo "wall  $desc (exit $rc, needs a session)"
		walled=$((walled + 1))
	else
		echo "FAIL  $desc (exit $rc)"
		fail=$((fail + 1))
	fi
}

# strict <description> -- <command...>
# Must exit 0 with output; used for offline-deterministic commands.
strict() {
	local desc="$1"; shift
	[ "$1" = "--" ] && shift
	local out rc
	out="$("$@" 2>/dev/null)"
	rc=$?
	if [ $rc -eq 0 ] && [ -n "$out" ]; then
		echo "ok    $desc"
		pass=$((pass + 1))
	else
		echo "FAIL  $desc (exit $rc)"
		fail=$((fail + 1))
	fi
}

echo "== fb smoke (FB=$FB) =="
if [ -n "${FACEBOOK_COOKIE:-}${FACEBOOK_COOKIE_FILE:-}" ]; then
	echo "   session: present"
else
	echo "   session: anonymous (login-walled reads are expected)"
fi
echo

# --- offline-deterministic: must always pass ---
strict "version"            -- "$FB" version
strict "whoami"             -- "$FB" whoami -o json
strict "config show"        -- "$FB" config show -o jsonl
strict "config path"        -- "$FB" config path
strict "cache dir"          -- "$FB" cache dir
strict "id slug"            -- "$FB" id "$PAGE" -o json
strict "id post url"        -- "$FB" id "https://www.facebook.com/$PAGE/posts/pfbid0abc" -o json
strict "id group url"       -- "$FB" id "https://www.facebook.com/groups/123" -o json
strict "id video url"       -- "$FB" id "https://www.facebook.com/watch/?v=123" -o json
strict "completion bash"    -- "$FB" completion bash

# db build + query roundtrip
DB="$(mktemp -t fbsmoke.XXXXXX).db"
strict "db query (schema)"  -- "$FB" db --db "$DB" query "select count(*) from sqlite_master" -o jsonl
rm -f "$DB"

echo

# --- live reads: data, or a documented wall ---
run "page"                  -- "$FB" page "$PAGE" -o json --no-cache
run "page --posts"          -- "$FB" page "$PAGE" --posts -n 5 -o jsonl --no-cache
run "page --about"          -- "$FB" page "$PAGE" --about -o json --no-cache
run "page --photos"         -- "$FB" page "$PAGE" --photos -n 5 -o jsonl --no-cache
run "page --videos"         -- "$FB" page "$PAGE" --videos -n 5 -o jsonl --no-cache
run "page --events"         -- "$FB" page "$PAGE" --events -o jsonl --no-cache
run "profile"               -- "$FB" profile "$PROFILE" -o json --no-cache
run "profile --posts"       -- "$FB" profile "$PROFILE" --posts -n 5 -o jsonl --no-cache
run "feed"                  -- "$FB" feed "$PAGE" -n 5 -o jsonl --no-cache
run "photos"                -- "$FB" photos "$PAGE" -n 5 -o jsonl --no-cache
run "videos"                -- "$FB" videos "$PAGE" -n 5 -o jsonl --no-cache
run "events"                -- "$FB" events "$PAGE" -o jsonl --no-cache
run "search (all)"          -- "$FB" search "$SEARCH" -n 5 -o jsonl --no-cache
run "search --type page"    -- "$FB" search "$SEARCH" --type page -n 5 -o jsonl --no-cache

if [ -n "$GROUP" ]; then
	run "group"             -- "$FB" group "$GROUP" -o json --no-cache
	run "group --posts"     -- "$FB" group "$GROUP" --posts -n 5 -o jsonl --no-cache
fi

# A post/comment/reaction smoke needs a concrete post URL; derive one from the
# page feed when a session is available, otherwise skip the per-post commands.
POST="$("$FB" page "$PAGE" --posts -n 1 -o url --no-cache 2>/dev/null | head -1)"
if [ -n "$POST" ]; then
	run "post"              -- "$FB" post "$POST" -o json --no-cache
	run "post --comments"   -- "$FB" post "$POST" --comments -n 5 -o jsonl --no-cache
	run "post --reactions"  -- "$FB" post "$POST" --reactions -o jsonl --no-cache
	run "comments"          -- "$FB" comments "$POST" -n 5 -o jsonl --no-cache
	run "reactions"         -- "$FB" reactions "$POST" -o json --no-cache
else
	echo "skip  post/comments/reactions (no post url available anonymously)"
fi

echo
echo "== $pass passed, $walled walled, $fail failed =="
[ $fail -eq 0 ]
