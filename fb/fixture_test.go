package fb

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture_test.go loads the captured pages the fixture suite runs on.
//
// Every fixture is a real response, gzipped because a Comet page is the better
// part of a megabyte. None are hand-written: `fb archive` produced them, and
// capture.txt beside them records the command and the date.

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name+".html.gz"))
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gunzip %s: %v", name, err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return b
}

// fixtureDocs is the whole page plane in one call: extract, decode, collect,
// stitch.
func fixtureDocs(t *testing.T, name string) map[string]*Document {
	t.Helper()
	return documents(relayPayloads(decodeBlocks(dataSJS(fixture(t, name)))))
}

// TestEveryFixtureHasACaptureCommand. A fixture is a page as it was on one day,
// and the day comes. When it does, whoever has to update it needs to know what
// URL produced it, and a megabyte of minified Comet HTML does not say: the
// og:url on the reel fixture names the video permalink and the one on the video
// permalink names the reel, so even reading the file gets it wrong.
func TestEveryFixtureHasACaptureCommand(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "capture.txt"))
	if err != nil {
		t.Fatalf("read capture.txt: %v", err)
	}
	recorded := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 3 {
			t.Errorf("capture.txt line %q is not name, date, command", line)
			continue
		}
		if !strings.HasPrefix(cols[2], "fb archive ") {
			t.Errorf("%s records the command %q, and a fixture is made with fb archive", cols[0], cols[2])
		}
		recorded[cols[0]] = true
	}
	files, err := filepath.Glob(filepath.Join("testdata", "*.html.gz"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures, so this test is checking nothing")
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".html.gz")
		if !recorded[name] {
			t.Errorf("%s has no row in capture.txt, so nobody can recapture it", name)
		}
		delete(recorded, name)
	}
	for name := range recorded {
		t.Errorf("capture.txt records %s, which is not a fixture here", name)
	}
}
