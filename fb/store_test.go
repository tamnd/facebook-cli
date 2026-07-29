package fb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tamnd/facebook-cli/pkg/graph"
)

// store_test.go holds the store to the two promises the schema makes: a claim
// is identified by its source as well as by its three parts, and a node met
// twice keeps the better of the two sightings.

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := OpenStore(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func claim(from, pred, to, source string) graph.Edge {
	return graph.Edge{From: from, Predicate: pred, To: to, Source: source, Surface: surfaceComet}
}

// TestTwoSurfacesSayingOneThingStayTwoRows is the reason the source is in the
// primary key. Merging them would throw away the evidence that two pages agree,
// and with it any chance of noticing the day they stop.
func TestTwoSurfacesSayingOneThingStayTwoRows(t *testing.T) {
	st := openTestStore(t)
	n, err := st.PutClaims([]graph.Edge{
		claim("fb://profile/1", graph.Authored, "fb://post/2", "https://www.facebook.com/nasa"),
		claim("fb://profile/1", graph.Authored, "fb://post/2", "https://www.facebook.com/permalink.php?story_fbid=2&id=1"),
	})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if n != 2 {
		t.Fatalf("wrote %d claims, want 2", n)
	}
	got, err := st.AllClaims(context.Background())
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("store kept %d claims, want 2: the source is part of a claim's identity", len(got))
	}
}

// TestTheSameClaimFromTheSameSourceIsOneRow is the other half. A crawl that
// reads a page twice has not learned anything twice.
func TestTheSameClaimFromTheSameSourceIsOneRow(t *testing.T) {
	st := openTestStore(t)
	e := claim("fb://profile/1", graph.Mentions, "fb://profile/3", "https://www.facebook.com/nasa")
	for range 3 {
		if _, err := st.PutClaims([]graph.Edge{e}); err != nil {
			t.Fatalf("put: %v", err)
		}
	}
	got, _ := st.AllClaims(context.Background())
	if len(got) != 1 {
		t.Fatalf("store kept %d rows for one claim read three times, want 1", len(got))
	}
}

// TestASightingWithNoRecordKeepsTheOneThereIs is the node half. Nearly every
// node is met as somebody else's object before it is ever read, and whichever
// order those happen in the fetched record is the one worth keeping.
func TestASightingWithNoRecordKeepsTheOneThereIs(t *testing.T) {
	st := openTestStore(t)
	if err := st.PutNode("fb://profile/1", "page", Profile{ID: "1", Name: "NASA"}); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := st.PutNode("fb://profile/1", "profile", nil); err != nil {
		t.Fatalf("put: %v", err)
	}
	var rec string
	var first, last int64
	row := st.DB().QueryRow(`SELECT record, first_seen, last_seen FROM nodes WHERE uri = ?`, "fb://profile/1")
	if err := row.Scan(&rec, &first, &last); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if rec == "" {
		t.Fatal("a later sighting with no record erased the record that was there")
	}
	if last < first {
		t.Fatalf("last_seen %d is before first_seen %d", last, first)
	}
}

// TestTheReadLogIsWhatMakesASourceCheckable: a record names the URLs it came
// from and only this table says what those URLs answered.
func TestTheReadLogIsWhatMakesASourceCheckable(t *testing.T) {
	st := openTestStore(t)
	reads := []Read{
		{URL: "https://www.facebook.com/nasa", Surface: surfaceComet, Status: 200, Bytes: 1000, At: time.Now()},
		{URL: "https://www.facebook.com/gone", Surface: surfaceComet, Status: 404, Bytes: 12, At: time.Now()},
		{URL: "https://www.facebook.com/also-gone", Surface: surfaceComet, Status: 404, Bytes: 12, At: time.Now()},
	}
	for _, r := range reads {
		if err := st.LogRead(r); err != nil {
			t.Fatalf("log: %v", err)
		}
	}
	stats, err := st.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	want := map[string]int{"s1 404": 2, "s1 200": 1, "all": 3}
	for _, s := range stats {
		if s.Section != "reads" {
			continue
		}
		if n, ok := want[s.Key]; ok {
			if s.Count != n {
				t.Errorf("reads %q counted %d, want %d", s.Key, s.Count, n)
			}
			delete(want, s.Key)
		}
	}
	if len(want) > 0 {
		t.Errorf("stats did not report %v", want)
	}
}

// TestAQueryCannotWrite is the guard on `fb query`. The string is arbitrary SQL
// somebody typed, and a crawl thrown away by a finger slip is not recoverable.
func TestAQueryCannotWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := st.PutClaims([]graph.Edge{claim("fb://profile/1", graph.Authored, "fb://post/2", "u")}); err != nil {
		t.Fatalf("put: %v", err)
	}
	st.Close()

	ro, err := OpenStoreRO(path)
	if err != nil {
		t.Fatalf("open read-only: %v", err)
	}
	defer ro.Close()
	if _, err := ro.Query(context.Background(), "delete from claims"); err == nil {
		t.Fatal("a read-only store accepted a delete")
	}
	if _, err := ro.Query(context.Background(), "select count(*) from claims"); err != nil {
		t.Fatalf("a read-only store refused a select: %v", err)
	}
}

// TestAMissingStoreSaysWhereOneComesFrom. "no such file" is true and useless;
// the answer somebody needs is the name of the command that makes one.
func TestAMissingStoreSaysWhereOneComesFrom(t *testing.T) {
	_, err := OpenStoreRO(filepath.Join(t.TempDir(), "nothing.db"))
	if err == nil {
		t.Fatal("opening a store that is not there succeeded")
	}
	if ExitCode(err) != 6 {
		t.Errorf("exit code %d, want 6 for a store that is not there", ExitCode(err))
	}
}

// TestAnExportIsInAFixedOrder. A dump that reorders itself cannot be diffed,
// and a diff is how somebody notices a page started saying something different.
func TestAnExportIsInAFixedOrder(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.PutClaims([]graph.Edge{
		claim("fb://profile/9", graph.Mentions, "fb://profile/1", "u2"),
		claim("fb://profile/1", graph.Authored, "fb://post/5", "u1"),
		claim("fb://profile/1", graph.Authored, "fb://post/5", "u0"),
	}); err != nil {
		t.Fatalf("put: %v", err)
	}
	want := [][2]string{
		{"fb://profile/1", "u0"},
		{"fb://profile/1", "u1"},
		{"fb://profile/9", "u2"},
	}
	for range 2 {
		got, err := st.AllClaims(context.Background())
		if err != nil {
			t.Fatalf("all: %v", err)
		}
		for i, w := range want {
			if got[i].From != w[0] || got[i].Source != w[1] {
				t.Fatalf("claim %d is %s from %s, want %s from %s", i, got[i].From, got[i].Source, w[0], w[1])
			}
		}
	}
}

// TestTheStoreKnowsAPageFromAPerson: the URI space is one on purpose, so the
// kind column is the only place the difference lives, and it is what an export
// needs to write schema:Organization instead of schema:Person.
func TestTheStoreKnowsAPageFromAPerson(t *testing.T) {
	st := openTestStore(t)
	if err := st.PutNode("fb://profile/1", "page", Profile{ID: "1"}); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := st.PutNode("fb://profile/2", "", nil); err != nil {
		t.Fatalf("put: %v", err)
	}
	kinds, err := st.Kinds(context.Background())
	if err != nil {
		t.Fatalf("kinds: %v", err)
	}
	if kinds["fb://profile/1"] != "page" {
		t.Errorf("kind is %q, want page", kinds["fb://profile/1"])
	}
	if _, ok := kinds["fb://profile/2"]; ok {
		t.Error("a node nobody has read claimed a kind")
	}
}
