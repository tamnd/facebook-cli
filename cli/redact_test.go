package cli

import (
	"bytes"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/any-cli/kit/render"
	"github.com/tamnd/facebook-cli/fb"
)

// redact_test.go is one rule with no exceptions: the xs cookie never reaches
// stdout.
//
// It is worth a test of its own rather than a review, because the leak is one
// careless field away. The obvious way to render a session is to marshal it,
// and a marshalled Session prints the cookie into whatever somebody piped fb
// into. That is why Status is a separate type, and this is what keeps it one.

const secret = "AbCdEf:ThisIsTheSessionCookieValue"

// formats is every way fb can print a record. The template one renders the
// whole value through text/template, which is the format most likely to reach
// a field nobody meant to expose.
var formats = []render.Format{
	render.List, render.Table, render.Markdown, render.JSON,
	render.JSONL, render.CSV, render.TSV, render.URL,
}

func renderRow(t *testing.T, f render.Format, r Row) string {
	t.Helper()
	var buf bytes.Buffer
	out, err := render.New(render.Options{Format: f, Writer: &buf})
	if err != nil {
		t.Fatalf("render.New(%s): %v", f, err)
	}
	if err := out.Emit(r); err != nil {
		t.Fatalf("emit as %s: %v", f, err)
	}
	if err := out.Flush(); err != nil {
		t.Fatalf("flush as %s: %v", f, err)
	}
	return buf.String()
}

func TestSessionStatusNeverPrintsTheCookie(t *testing.T) {
	dir := t.TempDir()
	s := fb.Session{CUser: "100000000000001", XS: secret, ImportedAt: time.Now().UTC()}
	if err := fb.SaveSession(dir, s); err != nil {
		t.Fatalf("save: %v", err)
	}
	st, err := fb.SessionStatus(dir)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !st.Present {
		t.Fatal("a session was just written and status says there is none")
	}
	row := statusRow(st)
	for _, f := range formats {
		if got := renderRow(t, f, row); strings.Contains(got, secret) {
			t.Errorf("fb auth status prints the xs cookie as %s:\n%s", f, got)
		}
	}
	// The account id is the point of the command, so it has to survive.
	if got := renderRow(t, render.JSON, row); !strings.Contains(got, "100000000000001") {
		t.Errorf("fb auth status hides the account id it exists to print:\n%s", got)
	}
}

func TestConfigShowNeverPrintsTheCookie(t *testing.T) {
	c := fb.Defaults()
	c.DataDir = t.TempDir()
	c.Cookies = "c_user=100000000000001; xs=" + secret
	a := &App{cfg: c}
	row := kv(
		"data_dir", a.cfg.DataDir,
		"session", fb.SessionPath(a.cfg.DataDir),
		"signed_in", yesno(a.cfg.Cookies != ""),
	)
	for _, f := range formats {
		if got := renderRow(t, f, row); strings.Contains(got, secret) {
			t.Errorf("fb config show prints the xs cookie as %s:\n%s", f, got)
		}
	}
}

// TestSessionFileIsPrivate checks the mode rather than the contents. The cookie
// is in that file by design, and the thing standing between it and every other
// account on the machine is the permission bits.
func TestSessionFileIsPrivate(t *testing.T) {
	dir := t.TempDir()
	if err := fb.SaveSession(dir, fb.Session{CUser: "1", XS: secret}); err != nil {
		t.Fatalf("save: %v", err)
	}
	fi, err := os.Stat(fb.SessionPath(dir))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := fi.Mode().Perm(); mode != fs.FileMode(0o600) {
		t.Errorf("the session file is %04o, which is readable by somebody else; want 0600", mode)
	}
}
