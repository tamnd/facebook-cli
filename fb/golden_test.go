package fb

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// golden_test.go parses every fixture into the record it produces and compares
// that against a committed file.
//
//	go test ./fb -run Golden -update
//
// The diff is the point, and it is a different diff from the census next door.
// The census counts what is filled, so it catches a field that stopped being
// parsed at all. This catches a field that started being parsed differently: a
// name with the category glued onto it, a timestamp an hour out, a URL that
// grew a tracking parameter back. Neither of those changes a count.
//
// The parse functions are called rather than the engine, so there is no network
// and no clock in the output. A golden with a fetched_at in it would be a
// golden that fails every run, and the first fix anybody reaches for is to stop
// running it.

func goldenPath(kind, fixture string) string {
	return filepath.Join("testdata", "golden", kind+"_"+fixture+".json")
}

func TestGoldenRecords(t *testing.T) {
	for _, src := range censusSources() {
		for _, name := range src.from {
			t.Run(src.kind+"/"+name, func(t *testing.T) {
				got, err := json.MarshalIndent(src.parse(t, name), "", "  ")
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				got = append(got, '\n')
				path := goldenPath(src.kind, name)
				if *updateCensus {
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatalf("mkdir: %v", err)
					}
					if err := os.WriteFile(path, got, 0o644); err != nil {
						t.Fatalf("write %s: %v", path, err)
					}
					t.Logf("wrote %s", path)
					return
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read %s: %v (run go test ./fb -run Golden -update)", path, err)
				}
				if bytes.Equal(got, want) {
					return
				}
				t.Errorf("%s is not what the parser produces.\n%s\nRun: go test ./fb -run Golden -update, and read the diff before committing it.",
					path, firstDifference(want, got))
			})
		}
	}
}

// firstDifference names the first line that differs and prints both sides of
// it. A whole-record diff of a profile is four hundred lines and the useful
// part is one of them.
func firstDifference(want, got []byte) string {
	w := strings.Split(string(want), "\n")
	g := strings.Split(string(got), "\n")
	for i := range max(len(w), len(g)) {
		lw, lg := lineAt(w, i), lineAt(g, i)
		if lw == lg {
			continue
		}
		return "line " + strconv.Itoa(i+1) + ":\n  committed: " + strings.TrimSpace(lw) + "\n  parsed:    " + strings.TrimSpace(lg)
	}
	return "the files differ in whitespace only"
}

func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "(end of file)"
}
