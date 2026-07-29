package fb

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// relay.go turns the JSON blocks from sjs.go into stitched GraphQL documents.
//
// A query's results do not arrive as one object. ProfileCometHeaderQuery comes
// back as eleven payloads: one root with a "data" key, and ten deferred
// fragments each carrying a "label", a "path" into the root, and its own data.
// Facebook streams them because the browser renders as they land. We want the
// document the browser ends up with, so we stitch.
//
// The same stitcher serves the replayed GraphQL surface, where the payloads
// arrive as one JSON object per line instead of as script blocks. That is the
// reason this is one function and not two.

// payload is one Relay result: either a root or a deferred fragment.
type payload struct {
	Op       string          // operation name, from the preloader id
	Label    string          `json:"label"`
	Path     []any           `json:"path"`
	Data     json.RawMessage `json:"data"`
	Sequence int             // sequence_number from the __bbox, ordering key
}

// Document is one operation's results, stitched into the shape the browser
// would have rendered.
type Document struct {
	Op   string
	Data map[string]any
}

// relayPayloads walks decoded script blocks and collects every Relay result in
// them, keyed by operation.
//
// The walk is a tree walk rather than a fixed path. Facebook moves the nesting
// between routes, and a hardcoded index reads as "this route is walled" for as
// long as it takes somebody to check.
func relayPayloads(decoded []any) []payload {
	var out []payload
	for _, v := range decoded {
		collectRelay(v, &out)
	}
	return out
}

// collectRelay finds arrays whose first element names RelayPrefetchedStreamCache.
func collectRelay(v any, out *[]payload) {
	switch t := v.(type) {
	case map[string]any:
		for _, sub := range t {
			collectRelay(sub, out)
		}
	case []any:
		if p, ok := relayEntry(t); ok {
			*out = append(*out, p...)
			return
		}
		for _, sub := range t {
			collectRelay(sub, out)
		}
	}
}

// relayEntry reads one ["RelayPrefetchedStreamCache", "next", [], [id, {__bbox}]]
// array, if that is what it was handed.
func relayEntry(a []any) ([]payload, bool) {
	if len(a) < 4 {
		return nil, false
	}
	name, ok := a[0].(string)
	if !ok || moduleName(name) != "RelayPrefetchedStreamCache" {
		return nil, false
	}
	args, ok := a[3].([]any)
	if !ok || len(args) < 2 {
		return nil, false
	}
	id, _ := args[0].(string)
	op := opFromPreloaderID(id)
	box, ok := args[1].(map[string]any)
	if !ok {
		return nil, false
	}
	bbox, ok := box["__bbox"].(map[string]any)
	if !ok {
		return nil, false
	}
	result, ok := bbox["result"]
	if !ok {
		return nil, false
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, false
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, false
	}
	p.Op = op
	if seq, ok := bbox["sequence_number"].(float64); ok {
		p.Sequence = int(seq)
	}
	return []payload{p}, true
}

// moduleName strips the hash Facebook sometimes appends to a module name.
//
// A Page ships "RelayPrefetchedStreamCache". An event page ships
// "RelayPrefetchedStreamCache@4a1fb7054d45192b147ad69ff77e2f17". Matching on
// equality finds the first and silently misses the second.
func moduleName(s string) string {
	if i := strings.IndexByte(s, '@'); i >= 0 {
		return s[:i]
	}
	return s
}

// opFromPreloaderID reads the operation name out of
// "adp_ProfileCometHeaderQueryRelayPreloader_6a697058bcbae7f25800971". The
// trailing hex is a per-render id and is never part of a key.
func opFromPreloaderID(id string) string {
	s := strings.TrimPrefix(id, "adp_")
	if i := strings.Index(s, "RelayPreloader"); i > 0 {
		return s[:i]
	}
	return s
}

// stitch merges a set of payloads for one operation into a single document.
//
// The root is the payload with no label. Fragments are applied in
// sequence_number order, each merged into the node its path names, because two
// fragments can address the same path and the later one refines the earlier.
func stitch(op string, ps []payload) (*Document, error) {
	var root map[string]any
	var frags []payload
	for _, p := range ps {
		if p.Label == "" && p.Path == nil {
			if root == nil && len(p.Data) > 0 {
				if err := json.Unmarshal(p.Data, &root); err != nil {
					return nil, fmt.Errorf("%s: root data: %w", op, err)
				}
				continue
			}
		}
		frags = append(frags, p)
	}
	if root == nil {
		return nil, fmt.Errorf("%s: no root payload", op)
	}
	sort.SliceStable(frags, func(i, j int) bool { return frags[i].Sequence < frags[j].Sequence })
	for _, f := range frags {
		if len(f.Data) == 0 {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal(f.Data, &data); err != nil {
			continue
		}
		node := walkPath(root, f.Path)
		if node == nil {
			// The path names a node the root never established, which means
			// the page was truncated. Dropping the fragment is better than
			// inventing the parent it wanted.
			continue
		}
		mergeInto(node, data)
	}
	return &Document{Op: op, Data: root}, nil
}

// walkPath follows a Relay path (string keys and numeric indices) to the map it
// names, or nil when the path does not resolve.
func walkPath(root map[string]any, path []any) map[string]any {
	cur := any(root)
	for _, step := range path {
		switch s := step.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				return nil
			}
			cur, ok = m[s]
			if !ok {
				return nil
			}
		case float64:
			a, ok := cur.([]any)
			if !ok || int(s) >= len(a) || int(s) < 0 {
				return nil
			}
			cur = a[int(s)]
		default:
			return nil
		}
	}
	m, _ := cur.(map[string]any)
	return m
}

// mergeInto applies a fragment over a node. A key present in both is taken from
// the fragment, because the fragment is the refinement; a key that is a map on
// both sides merges recursively so a fragment refining one leaf does not drop
// its siblings.
func mergeInto(node, frag map[string]any) {
	for k, v := range frag {
		if sub, ok := v.(map[string]any); ok {
			if existing, ok := node[k].(map[string]any); ok {
				mergeInto(existing, sub)
				continue
			}
		}
		node[k] = v
	}
}

// documents groups payloads by operation and stitches each group.
func documents(ps []payload) map[string]*Document {
	byOp := map[string][]payload{}
	for _, p := range ps {
		byOp[p.Op] = append(byOp[p.Op], p)
	}
	out := map[string]*Document{}
	for op, group := range byOp {
		doc, err := stitch(op, group)
		if err != nil {
			continue
		}
		out[op] = doc
	}
	return out
}
