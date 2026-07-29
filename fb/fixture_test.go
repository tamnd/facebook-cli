package fb

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
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
	defer f.Close()
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
