package fb

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// dig.go is how every parser reads the Comet tree.
//
// The tree is deep, irregular, and full of nulls: a renderer that is present on
// one page is nil on the next, and a path that resolves for a Page does not for
// a person. Written with type assertions at each step, a parser becomes mostly
// error handling for cases that are not errors. These helpers return the zero
// value for a path that does not resolve, and the parsers decide what a zero
// means.

// dig follows string keys and integer-shaped keys through maps and slices.
// A key like "0" indexes a slice.
func dig(v any, path ...string) any {
	cur := v
	for _, k := range path {
		switch t := cur.(type) {
		case map[string]any:
			var ok bool
			cur, ok = t[k]
			if !ok {
				return nil
			}
		case []any:
			i, err := strconv.Atoi(k)
			if err != nil || i < 0 || i >= len(t) {
				return nil
			}
			cur = t[i]
		default:
			return nil
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}

// digStr reads a string at a path, or "".
func digStr(v any, path ...string) string {
	s, _ := dig(v, path...).(string)
	return s
}

// digInt reads a number at a path, or 0. JSON numbers arrive as float64; a
// count that arrives as a string (Facebook does this for a few) is parsed.
func digInt(v any, path ...string) int {
	switch t := dig(v, path...).(type) {
	case float64:
		return int(t)
	case string:
		n, err := strconv.Atoi(strings.ReplaceAll(t, ",", ""))
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}

// digFloat reads a float at a path, or 0.
func digFloat(v any, path ...string) float64 {
	f, _ := dig(v, path...).(float64)
	return f
}

// digBool reads a bool at a path, or false.
func digBool(v any, path ...string) bool {
	b, _ := dig(v, path...).(bool)
	return b
}

// digMap reads a map at a path, or nil.
func digMap(v any, path ...string) map[string]any {
	m, _ := dig(v, path...).(map[string]any)
	return m
}

// digSlice reads a slice at a path, or nil.
func digSlice(v any, path ...string) []any {
	s, _ := dig(v, path...).([]any)
	return s
}

// digMaps reads a slice of maps at a path, skipping entries that are not maps.
func digMaps(v any, path ...string) []map[string]any {
	raw := digSlice(v, path...)
	out := make([]map[string]any, 0, len(raw))
	for _, e := range raw {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// edges reads a Relay connection's nodes: {"edges": [{"node": {...}}]}.
func edges(v any, path ...string) []map[string]any {
	var out []map[string]any
	for _, e := range digMaps(v, append(path, "edges")...) {
		if n, ok := e["node"].(map[string]any); ok {
			out = append(out, n)
		}
	}
	return out
}

// digTime reads a unix-seconds timestamp, or the zero time. Facebook uses 0 for
// "no end time" as well as for the epoch, and no Facebook object predates 2004,
// so 0 means absent here.
func digTime(v any, path ...string) time.Time {
	n := digInt(v, path...)
	if n <= 0 {
		return time.Time{}
	}
	return time.Unix(int64(n), 0).UTC()
}

// firstOf returns the first path that resolves to a non-empty map. Several
// fields live at more than one path in the same document (a story's message is
// at three), and the parsers name their preference first.
func firstOf(v any, paths ...[]string) map[string]any {
	for _, p := range paths {
		if m := digMap(v, p...); len(m) > 0 {
			return m
		}
	}
	return nil
}

// sortedKeys returns a map's keys in order.
//
// Both walkers below descend into every child, and "first" only means anything
// if the children are visited in a fixed order. Ranging a Go map is randomised,
// so without this the same capture parsed twice can pick two different actors
// out of a story and the golden test flaps. It cost an afternoon to find.
func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// findFirst walks a subtree for the first map carrying the given __typename.
// Used where Facebook nests a renderer at a depth that changes between routes.
func findFirst(v any, typename string) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		if s, _ := t["__typename"].(string); s == typename {
			return t
		}
		for _, k := range sortedKeys(t) {
			if m := findFirst(t[k], typename); m != nil {
				return m
			}
		}
	case []any:
		for _, sub := range t {
			if m := findFirst(sub, typename); m != nil {
				return m
			}
		}
	}
	return nil
}

// findKey walks a subtree for the first map that has the given key set to a
// non-nil value, and returns that map.
func findKey(v any, key string) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		if got, ok := t[key]; ok && got != nil {
			return t
		}
		for _, k := range sortedKeys(t) {
			if m := findKey(t[k], key); m != nil {
				return m
			}
		}
	case []any:
		for _, sub := range t {
			if m := findKey(sub, key); m != nil {
				return m
			}
		}
	}
	return nil
}
