package fb

import (
	"regexp"
	"sort"
	"strconv"
)

// tokens.go pulls the request tokens out of a Comet page.
//
// Replaying an operation on surface 2 needs more than a doc id. The server
// checks a per-page CSRF token (lsd), a checksum of it (jazoest), and a set of
// build identifiers that tell it which revision of the site is asking. None of
// these are secret and none identify a person: they are all in the HTML of a
// page anybody can fetch signed out. They are extracted rather than pinned,
// because a pinned revision goes stale in days.
//
// The one that is not obvious is jazoest. It is the string "2" followed by the
// sum of the byte values of the lsd token, and Facebook computes it in
// JavaScript on every request. Getting it wrong is a 400 with no explanation.

// Tokens is what a page said about itself.
type Tokens struct {
	LSD     string `json:"lsd"`
	Jazoest string `json:"jazoest"`
	Rev     string `json:"rev"`     // __rev and __spin_r, the site revision
	SpinB   string `json:"spin_b"`  // release branch, "trunk"
	SpinT   string `json:"spin_t"`  // build timestamp
	Haste   string `json:"haste"`   // __hs
	HSI     string `json:"hsi"`     // __hsi
	DTSG    string `json:"dtsg"`    // fb_dtsg, present only with a session
	UserID  string `json:"user_id"` // "0" signed out
	Locale  string `json:"locale"`  // from og:locale
}

var (
	reLSD     = regexp.MustCompile(`"LSD",\[\],\{"token":"([^"]+)"`)
	reSpinR   = regexp.MustCompile(`"__spin_r":(\d+)`)
	reSpinB   = regexp.MustCompile(`"__spin_b":"([^"]+)"`)
	reSpinT   = regexp.MustCompile(`"__spin_t":(\d+)`)
	reHaste   = regexp.MustCompile(`"haste_session":"([^"]+)"`)
	reHSI     = regexp.MustCompile(`"hsi":"([^"]+)"`)
	reDTSG    = regexp.MustCompile(`"DTSGInitialData",\[\],\{"token":"([^"]+)"`)
	reUserID  = regexp.MustCompile(`"USER_ID":"(\d+)"`)
	reRevFall = regexp.MustCompile(`"server_revision":(\d+)`)
)

// parseTokens reads the tokens out of page HTML.
func parseTokens(html []byte) Tokens {
	first := func(re *regexp.Regexp) string {
		if m := re.FindSubmatch(html); m != nil {
			return string(m[1])
		}
		return ""
	}
	t := Tokens{
		LSD:    first(reLSD),
		Rev:    first(reSpinR),
		SpinB:  first(reSpinB),
		SpinT:  first(reSpinT),
		Haste:  first(reHaste),
		HSI:    first(reHSI),
		DTSG:   first(reDTSG),
		UserID: first(reUserID),
	}
	if t.Rev == "" {
		t.Rev = first(reRevFall)
	}
	if t.UserID == "" {
		t.UserID = "0"
	}
	t.Jazoest = jazoest(t.LSD)
	return t
}

// jazoest is "2" followed by the sum of the byte values of the token.
//
// The leading 2 is a version marker Facebook's own code writes; it is not the
// sum and it is not optional.
func jazoest(token string) string {
	if token == "" {
		return ""
	}
	sum := 0
	for i := 0; i < len(token); i++ {
		sum += int(token[i])
	}
	return "2" + strconv.Itoa(sum)
}

// usable reports whether these tokens are enough to replay an operation.
func (t Tokens) usable() bool { return t.LSD != "" && t.Rev != "" }

// Preload is one operation descriptor the page shipped: the persisted query id
// and the exact variables the server was called with.
//
// This is the whole basis of the replay surface. The rule from spec 3004 doc 01
// section 3 is that fb only ever replays an operation the page itself shipped,
// with the variables the page itself sent, changing only the count and the
// cursor. Hand-building variables does not work: the __relay_internal__pv__*
// provider flags are required and unguessable, and leaving one out gets "A
// server error occurred" rather than a useful message.
type Preload struct {
	Op        string         `json:"op"`
	DocID     string         `json:"doc_id"`
	Variables map[string]any `json:"variables"`
	ActorID   string         `json:"actor_id,omitempty"`
	PreloadID string         `json:"preloader_id,omitempty"`
}

// preloads harvests every operation descriptor in the decoded blocks.
func preloads(decoded []any) []Preload {
	var out []Preload
	seen := map[string]bool{}
	for _, v := range decoded {
		collectPreloads(v, &out, seen)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Op < out[j].Op })
	return out
}

func collectPreloads(v any, out *[]Preload, seen map[string]bool) {
	switch t := v.(type) {
	case map[string]any:
		if id, ok := t["queryID"].(string); ok && id != "" {
			p := Preload{
				DocID:     id,
				Op:        digStr(t, "queryName"),
				ActorID:   digStr(t, "actorID"),
				PreloadID: digStr(t, "preloaderID"),
			}
			if p.Op == "" {
				p.Op = opFromPreloaderID(p.PreloadID)
			}
			p.Variables, _ = t["variables"].(map[string]any)
			// The same operation is described more than once on a page, once
			// per component that might need it. They carry the same doc id, so
			// the first wins and the rest are noise.
			if p.Op != "" && !seen[p.Op] {
				seen[p.Op] = true
				*out = append(*out, p)
			}
			return
		}
		for _, sub := range t {
			collectPreloads(sub, out, seen)
		}
	case []any:
		for _, sub := range t {
			collectPreloads(sub, out, seen)
		}
	}
}
