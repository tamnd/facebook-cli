// Package cli builds the fb command tree on top of the fb library.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tamnd/facebook-cli/fb"
)

// App carries the resolved configuration and shared client for a command run.
type App struct {
	Cfg    fb.Config
	Client *fb.Client
	Out    *Output
	Limit  int
	since  time.Time
	until  time.Time
	dryRun bool
	g      *globalFlags
}

// globalFlags holds the persistent flag values before they fold into Cfg.
type globalFlags struct {
	output    string
	fields    string
	noHeader  bool
	template  string
	limit     int
	since     string
	until     string
	cookie    string
	cookieF   string
	rate      time.Duration
	retries   int
	timeout   time.Duration
	workers   int
	noCache   bool
	cacheTTL  time.Duration
	surface   string
	lang      string
	quiet     bool
	verbose   int
	color     string
	proxy     string
	userAgent string
	raw       bool
	dryRun    bool
	yes       bool
}

// Root builds the root command and its whole subtree.
func Root() *cobra.Command {
	g := &globalFlags{}
	app := &App{g: g}

	root := &cobra.Command{
		Use:   "fb",
		Short: "A delightful command line for Facebook",
		Long: `fb turns facebook.com into a fast, scriptable command line.

Resolve a Page, profile, or group to a rich record; stream its whole feed;
pull every comment, reaction, photo, and event; and build datasets, all from
one binary. Reads are anonymous where Facebook allows it and use your session
cookie where it does not (pass --cookie or set FACEBOOK_COOKIE).

Quick start:
  fb page nasa                       a Page's full profile
  fb page nasa --posts --limit 20    its twenty most recent posts
  fb post <url> --comments           a post and its comment thread
  fb id <anything>                   classify any Facebook id or URL`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return app.init(g)
		},
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&g.output, "output", "o", "auto", "table|json|jsonl|csv|tsv|yaml|url|raw")
	pf.StringVar(&g.fields, "fields", "", "comma-separated columns to keep/order")
	pf.BoolVar(&g.noHeader, "no-header", false, "omit the header row (table/csv/tsv)")
	pf.StringVar(&g.template, "template", "", "Go text/template applied per record")
	pf.IntVarP(&g.limit, "limit", "n", 0, "max records emitted (0 = unlimited)")
	pf.StringVar(&g.since, "since", "", "stop walking a feed older than this date (YYYY-MM-DD)")
	pf.StringVar(&g.until, "until", "", "skip feed items newer than this date")
	pf.StringVar(&g.cookie, "cookie", "", "cookie header value (c_user=...; xs=...)")
	pf.StringVar(&g.cookieF, "cookie-file", "", "path to a cookie file (header | cookies.txt | JSON)")
	pf.DurationVar(&g.rate, "rate", fb.DefaultDelay, "min delay between requests")
	pf.IntVar(&g.retries, "retries", fb.DefaultRetries, "retry attempts on 429/5xx")
	pf.DurationVar(&g.timeout, "timeout", fb.DefaultTimeout, "per-request timeout")
	pf.IntVarP(&g.workers, "workers", "j", fb.DefaultWorkers, "concurrency for fan-out commands")
	pf.BoolVar(&g.noCache, "no-cache", false, "bypass the on-disk cache")
	pf.DurationVar(&g.cacheTTL, "cache-ttl", time.Hour, "cache freshness window")
	pf.StringVar(&g.surface, "surface", "auto", "mbasic|mobile|auto")
	pf.StringVar(&g.lang, "lang", "en-US", "Accept-Language / locale")
	pf.BoolVarP(&g.quiet, "quiet", "q", false, "suppress progress on stderr")
	pf.CountVarP(&g.verbose, "verbose", "v", "increase verbosity (repeatable)")
	pf.StringVar(&g.color, "color", "auto", "color output: auto|always|never")
	pf.StringVar(&g.proxy, "proxy", "", "HTTP/SOCKS proxy URL")
	pf.StringVar(&g.userAgent, "user-agent", "", "override the default rotating UA set")
	pf.BoolVar(&g.raw, "raw", false, "print the upstream HTML/JSON untouched")
	pf.BoolVar(&g.dryRun, "dry-run", false, "print the requests that would be made, do nothing")
	pf.BoolVarP(&g.yes, "yes", "y", false, "assume yes to prompts")

	root.AddCommand(
		newPageCmd(app),
		newProfileCmd(app),
		newGroupCmd(app),
		newPostCmd(app),
		newCommentsCmd(app),
		newReactionsCmd(app),
		newPhotosCmd(app),
		newPhotoCmd(app),
		newVideoCmd(app),
		newVideosCmd(app),
		newEventCmd(app),
		newEventsCmd(app),
		newSearchCmd(app),
		newFeedCmd(app),
		newIDCmd(app),
		newSeedCmd(app),
		newCrawlCmd(app),
		newArchiveCmd(app),
		newDBCmd(app),
		newConfigCmd(app),
		newCacheCmd(app),
		newWhoamiCmd(app),
		newManCmd(),
		newVersionCmd(),
	)
	return root
}

func (a *App) init(g *globalFlags) error {
	cfg := fb.DefaultConfig()
	cfg.Cookie = g.cookie
	cfg.CookieFile = g.cookieF
	cfg.Delay = g.rate
	cfg.Retries = g.retries
	cfg.Timeout = g.timeout
	cfg.Workers = g.workers
	cfg.NoCache = g.noCache
	cfg.CacheTTL = g.cacheTTL
	cfg.UserAgent = g.userAgent
	cfg.Proxy = g.proxy
	cfg.Lang = g.lang
	cfg.Verbose = g.verbose
	switch g.surface {
	case "mbasic":
		cfg.Surface = fb.SurfaceMBasic
	case "mobile":
		cfg.Surface = fb.SurfaceMobile
	default:
		cfg.Surface = fb.SurfaceAuto
	}

	client, err := fb.NewClient(cfg)
	if err != nil {
		return err
	}
	a.Cfg = cfg
	a.Client = client
	a.Limit = g.limit
	a.dryRun = g.dryRun
	a.since = parseDate(g.since)
	a.until = parseDate(g.until)
	a.Out = newOutput(g)
	return nil
}

func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, f := range []string{"2006-01-02", "2006/01/02", "01/02/2006"} {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// progress writes a status line to stderr unless --quiet is set.
func (a *App) progress(format string, args ...any) {
	if a.g != nil && a.g.quiet {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "[fb] "+format+"\n", args...)
}

// listOpts builds fb.ListOptions from the app's resolved flags.
func (a *App) listOpts() fb.ListOptions {
	return fb.ListOptions{Limit: a.Limit, Since: a.since, Until: a.until}
}

// readArgsOrStdin returns args, or lines from stdin when the single arg is "-".
func readArgsOrStdin(args []string) []string {
	if len(args) == 1 && args[0] == "-" {
		var out []string
		sc := bufio.NewScanner(os.Stdin)
		sc.Buffer(make([]byte, 1024*1024), 1024*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				out = append(out, line)
			}
		}
		return out
	}
	return args
}
