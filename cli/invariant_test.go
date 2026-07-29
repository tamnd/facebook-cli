package cli

import (
	"strings"
	"testing"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/facebook-cli/fb"
)

// invariant_test.go checks the three tables against each other: the commands
// somebody types, the routing table `fb routes` prints, and the operations
// `fb serve` and `fb mcp` answer with.
//
// They are three views of one thing and they drift apart quietly. A command
// added without a route reads as unsupported in the table that documents the
// tool; a route naming a command that does not exist sends somebody to a
// command that will not run.

// commandPaths walks the hand-written tree and returns every path a person can
// type, "group feed" included.
func commandPaths() map[string]bool {
	out := map[string]bool{}
	var walk func(prefix string, cmds []kit.Command)
	walk = func(prefix string, cmds []kit.Command) {
		for _, c := range cmds {
			name := strings.Fields(c.Use)[0]
			path := strings.TrimSpace(prefix + " " + name)
			out[path] = true
			walk(path, c.Sub)
		}
	}
	walk("", readCommands())
	walk("", metaCommands())
	walk("", authCommands())
	walk("", configCommands())
	return out
}

// baseCommand drops the flag off a routing row: "page --about" is the page
// command with a flag, not a command of its own.
func baseCommand(s string) string {
	var parts []string
	for _, f := range strings.Fields(s) {
		if strings.HasPrefix(f, "-") {
			break
		}
		parts = append(parts, f)
	}
	return strings.Join(parts, " ")
}

func TestEveryRoutedCommandExists(t *testing.T) {
	have := commandPaths()
	for _, o := range fb.Operations {
		if cmd := baseCommand(o.Command); !have[cmd] {
			t.Errorf("fb routes names %q, which is not a command", o.Command)
		}
	}
}

func TestEveryRoutedSurfaceExists(t *testing.T) {
	for _, o := range fb.Operations {
		if o.Surface == "" {
			t.Errorf("routing row %q names no surface", o.Command)
			continue
		}
		if s, ok := fb.SurfaceByID(o.Surface); !ok {
			t.Errorf("routing row %q names surface %q, which is not in fb surfaces", o.Command, o.Surface)
		} else if s.Tier > o.Tier {
			t.Errorf("routing row %q is tier %d on surface %s, which is tier %d", o.Command, o.Tier, s.ID, s.Tier)
		}
	}
}

// networked is every command that asks Facebook something. The rest print what
// fb already knows or touch this machine only, and neither needs a route.
var networked = []string{
	"page", "profile", "feed", "post", "comments", "reactions",
	"photo", "photos", "photos --album", "photo --download", "video", "video --download", "reel", "videos",
	"group", "group feed", "event", "events", "discover", "search", "id --resolve",
	"edges", "graph",
}

func TestEveryNetworkedCommandIsRouted(t *testing.T) {
	routed := map[string]bool{}
	for _, o := range fb.Operations {
		routed[baseCommand(o.Command)] = true
	}
	// profile and reel are the second word for a read that is already routed
	// under its other name, so they route through it.
	alias := map[string]string{"profile": "page", "reel": "video"}
	for _, c := range networked {
		want := baseCommand(c)
		if a, ok := alias[want]; ok {
			want = a
		}
		if !routed[want] {
			t.Errorf("`fb %s` asks Facebook something and fb routes does not say what answers it", c)
		}
	}
}

// TestNothingServedWrites is the rule from doc 05 section 5: the credential and
// disk commands stay on the command line. A write op registered here would be
// reachable over a network port the moment somebody passes --allow-writes.
func TestNothingServedWrites(t *testing.T) {
	for _, op := range New().Ops() {
		if op.Meta().Write {
			t.Errorf("operation %q writes, so it does not belong on the serve and MCP surfaces", op.Meta().Name)
		}
	}
}

// TestEveryOpIsAlsoACommand keeps the two vocabularies together: a tool an
// agent can call under one name and a person cannot call at all is a tool
// nobody can talk about.
func TestEveryOpIsAlsoACommand(t *testing.T) {
	have := commandPaths()
	// These two are flags on a command rather than commands. Over HTTP and MCP
	// there are no flags, so each gets its own op.
	flagShaped := map[string]bool{"playlists": true, "event suggested": true}
	for _, op := range New().Ops() {
		m := op.Meta()
		path := m.Name
		if m.Parent != "" {
			path = m.Parent + " " + m.Name
		}
		if !have[path] && !flagShaped[path] {
			t.Errorf("operation %q has no command, so `fb %s` does not work", path, path)
		}
	}
}

// TestEveryOpIsOffTheCommandLine checks the NoCLI half of the split. Without it
// the generated subcommand shadows the hand-written one and `fb page nasa`
// prints a reflected dump instead of the table.
func TestEveryOpIsOffTheCommandLine(t *testing.T) {
	for _, op := range New().Ops() {
		if !op.Meta().NoCLI {
			t.Errorf("operation %q would shadow the hand-written command", op.Meta().Name)
		}
	}
}
