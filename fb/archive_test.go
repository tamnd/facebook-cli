package fb

import (
	"strings"
	"testing"
)

// archive_test.go covers the one thing about a capture that has to be true
// before anybody sends one anywhere.
//
// An archive is a directory somebody attaches to a bug report. It holds the
// page bytes, the headers as sent and the parsed record, which is exactly what
// makes it useful and exactly what makes a leaked cookie in it a Facebook
// account handed to a stranger.

// TestAnArchiveNeverKeepsTheCookie checks the headers a capture records, which
// is the only place in an archive a cookie could get to: the page bytes come
// back from Facebook and the record is parsed from them, so neither has ever
// seen one.
func TestAnArchiveNeverKeepsTheCookie(t *testing.T) {
	const secret = "sb=aBcDeF; xs=42%3AsecretsessionvALUE%3A2%3A1700000000%3A-1%3A-1"
	cfg := Defaults()
	cfg.Cookies = secret
	e, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	h := archiveHeaders(e.Client())
	if len(h) == 0 {
		t.Fatal("no headers recorded, so this test is checking nothing")
	}
	got, ok := h["Cookie"]
	if !ok {
		t.Fatal("no Cookie header at all, and the archive should say a cookie was sent rather than hide that it was")
	}
	for _, v := range got {
		if strings.Contains(v, "secretsessionvALUE") || strings.Contains(v, "xs=") {
			t.Fatalf("the archive records the cookie: %q", v)
		}
	}
	for k, vs := range h {
		for _, v := range vs {
			if strings.Contains(v, "secretsessionvALUE") {
				t.Errorf("header %s carries the session cookie: %q", k, v)
			}
		}
	}
}

// TestAnArchiveWithoutTheCacheRefuses. The capture is two steps over one read:
// fetch the bytes, then parse them back through the ordinary reader so
// record.json is the record fb would really produce. The second step reads the
// cache the first step wrote. With the cache off it would fetch again, and the
// record beside page.html would describe a page nobody kept.
func TestAnArchiveWithoutTheCacheRefuses(t *testing.T) {
	cfg := Defaults()
	cfg.NoCache = true
	e, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	_, err = e.Archive(t.Context(), "nasa", t.TempDir())
	if err == nil {
		t.Fatal("archive ran with the cache off")
	}
	if got := ExitCode(err); got != 2 {
		t.Errorf("exit code %d, want 2: this is a usage problem, not a failed read", got)
	}
}
