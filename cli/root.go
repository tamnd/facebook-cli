package cli

import (
	"time"

	"github.com/tamnd/any-cli/kit"
)

// Build metadata, stamped with -ldflags. goreleaser targets
// github.com/tamnd/facebook-cli/cli.{Version,Commit,Date}.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// New builds the app: the identity, the fb-only global flags, and every command.
//
// cli touches kit only here. kit wraps cobra and fang internally, so a command
// in this package names none of them.
func New() *kit.App {
	app := kit.New(kit.Identity{
		Binary: "fb",
		Short:  "A fast, read-only command line for Facebook",
		Long: "fb reads Facebook's public pages: the Relay data the site ships to a signed-out browser, " +
			"the meta head beside it, and the embed plugins. There is no app id, no developer app and " +
			"no API token anywhere in this tool, and there is nothing to sign up for. Two cookies from " +
			"a browser you are already signed into unlock the timeline past the first post; everything " +
			"else works with nothing at all.",
		Version: Version,
		Site:    "https://www.facebook.com",
		Repo:    "https://github.com/tamnd/facebook-cli",
	}, kit.WithDefaults(withFBDefaults))

	app.GlobalFlags(bindFBFlags)

	app.CommandGroup("auth", "Manage your Facebook session (Tier 1)")
	app.CommandGroup("config", "Show where fb keeps things")
	app.CommandGroup("cache", "Inspect or clear the page cache")

	for _, c := range readCommands() {
		app.AddCommand(c)
	}
	for _, c := range metaCommands() {
		app.AddCommand(c)
	}
	for _, c := range authCommands() {
		app.AddCommand(c)
	}
	for _, c := range configCommands() {
		app.AddCommand(c)
	}
	return app
}

// withFBDefaults overlays fb's request defaults onto the kit baseline, so help
// and `fb config show` read the same whether or not the user passed a flag.
func withFBDefaults(c *kit.Config) {
	c.Rate = time.Second
	c.Retries = 2
	c.Timeout = 30 * time.Second
}

// bindFBFlags registers the fb-only persistent flags. kit already provides
// -o/--output, --fields, --template, --no-header, -n/--limit, --rate,
// --retries, --timeout, --data-dir, --no-cache, -q/--quiet, -v, --color and
// --dry-run, so fb adds these three.
func bindFBFlags(f *kit.FlagSet) {
	f.StringVar(&flagTier, "tier", "", "cap the tier (0|1) or pin one surface (comet|graphql|og|embed|session)")
	f.StringVar(&flagProxy, "proxy", "", "an HTTP proxy to send every request through")
	f.DurationVar(&flagCacheTTL, "cache-ttl", 0, "how long a cached page counts as fresh (default 15m)")
}
