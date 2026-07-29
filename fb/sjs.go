package fb

import (
	"encoding/json"
	"strconv"
	"strings"
)

// sjs.go pulls the JSON blocks out of a Comet page.
//
// A signed-out facebook.com page is a shell plus its Relay store, and the store
// arrives as <script type="application/json" data-sjs> blocks. A Page ships 84
// of them. Four hold query results; the rest are bootloader manifests and
// logging config.
//
// This is a byte scan and not a HTML parse on purpose. goquery costs 40ms and
// 12MB to build a tree we throw away, and the block bodies are already
// JSON-escaped by the server so there is no entity decoding to do.

// scriptBlock is one <script type="application/json" data-sjs> body, with the
// length the server claimed for it.
type scriptBlock struct {
	Body    []byte
	Claimed int // data-content-len, -1 when the attribute was absent
}

// dataSJS finds every application/json script block carrying data-sjs.
func dataSJS(html []byte) []scriptBlock {
	var out []scriptBlock
	s := string(html)
	for i := 0; ; {
		j := strings.Index(s[i:], "<script")
		if j < 0 {
			break
		}
		start := i + j
		gt := strings.IndexByte(s[start:], '>')
		if gt < 0 {
			break
		}
		tag := s[start : start+gt]
		i = start + gt + 1
		if !strings.Contains(tag, `type="application/json"`) || !strings.Contains(tag, "data-sjs") {
			continue
		}
		end := strings.Index(s[i:], "</script>")
		if end < 0 {
			break
		}
		out = append(out, scriptBlock{
			Body:    []byte(s[i : i+end]),
			Claimed: attrInt(tag, "data-content-len"),
		})
		i += end + len("</script>")
	}
	return out
}

// attrInt reads a numeric attribute out of a raw tag, returning -1 when it is
// absent or unparseable.
func attrInt(tag, name string) int {
	k := strings.Index(tag, name+`="`)
	if k < 0 {
		return -1
	}
	rest := tag[k+len(name)+2:]
	q := strings.IndexByte(rest, '"')
	if q < 0 {
		return -1
	}
	n, err := strconv.Atoi(rest[:q])
	if err != nil {
		return -1
	}
	return n
}

// truncated reports whether the server's own byte count disagrees with what we
// hold. A truncated page does not announce itself any other way.
func (b scriptBlock) truncated() bool {
	return b.Claimed >= 0 && b.Claimed != len(b.Body)
}

// decodeBlocks parses each block body. A block that does not parse is skipped
// rather than failing the page, because one bad block among 84 is a block we do
// not need; the caller decides whether what it wanted came back.
func decodeBlocks(blocks []scriptBlock) []any {
	out := make([]any, 0, len(blocks))
	for _, b := range blocks {
		if b.truncated() {
			continue
		}
		var v any
		if err := json.Unmarshal(b.Body, &v); err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}
