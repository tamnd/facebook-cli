package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
	"github.com/tamnd/any-cli/kit/render"
	"github.com/tamnd/facebook-cli/fb"
)

// engine.go is the per-run state every command works through.
//
// The commands are kit escape hatches rather than reflected operations, because
// a reflected Profile is forty columns with JSON in the cells and what a person
// typing `fb page nasa` wants is eight. The same reads are registered as
// operations too, with NoCLI set, and that is what `fb serve` and `fb mcp`
// answer with. So the operation serves and the command stays typed.

// Row is one output record: a curated column set for the table, csv and url
// views, plus the whole typed object for json, jsonl and templates. It is kit's
// render.Record, so every row builder feeds the shared renderer and no command
// has formatting code of its own.
type Row = render.Record

// The fb-only global flags, bound on the root in root.go. kit already provides
// -o, --fields, --template, --no-header, -n, --rate, --retries, --timeout,
// --data-dir, --no-cache, -q, -v, --color and --dry-run, so fb adds these.
var (
	flagTier     string
	flagProxy    string
	flagCacheTTL time.Duration
)

// App is what a command's Run works with: the resolved config, the engine, and
// the output settings kit resolved once for the run.
type App struct {
	actx   context.Context
	st     *kit.State
	cfg    fb.Config
	eng    *fb.Engine
	engErr error
	limit  int
	quiet  bool
	// target is the thing this run was asked for, set once a command has parsed
	// its argument. The failure record names it, so a caller reading a stream of
	// them can tell which request went wrong.
	target string
	// warned is the set of missed-surface notes already printed this run.
	warned map[string]bool
}

// appFromCtx assembles the run's App from the resolved kit State.
func appFromCtx(ctx context.Context) *App {
	st := kit.FromContext(ctx)
	a := &App{actx: ctx, st: st}
	if st == nil {
		// No pre-run hook ran, which should not happen in normal use. Plain
		// defaults keep the command doing something sensible.
		a.cfg = fbConfig(kit.Config{})
		return a
	}
	a.cfg = fbConfig(st.Config)
	a.limit = st.Globals.Limit
	a.quiet = st.Config.Quiet
	return a
}

// fbConfig folds the kit globals and the fb-only flags into an fb.Config.
//
// kit's defaults are not fb's, and it says so by leaving the fields at zero
// when the user passed nothing, so a zero here means "fb decides" rather than
// "the user asked for zero".
func fbConfig(kc kit.Config) fb.Config {
	c := fb.Defaults()
	c.Tier = flagTier
	c.Proxy = flagProxy
	if flagCacheTTL > 0 {
		c.CacheTTL = flagCacheTTL
	}
	if kc.Rate > 0 {
		c.Rate = kc.Rate
	}
	if kc.Retries >= 0 {
		c.Retries = kc.Retries
	}
	if kc.Timeout > 0 {
		c.Timeout = kc.Timeout
	}
	c.NoCache = kc.NoCache
	c.DataDir = kc.DataDir
	if c.DataDir == "" {
		c.DataDir = fb.DefaultDataDir()
	}
	if kc.Verbose > 0 {
		c.Verbose = func(format string, args ...any) {
			_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
		}
	}
	// The session is read last, once --data-dir has settled where it is.
	if s, err := fb.LoadSession(c.DataDir); err == nil {
		c.Cookies = s.Cookie()
	}
	return c
}

// ctx returns the run context, which carries cancellation from the signal
// handler.
func (a *App) ctx() context.Context { return a.actx }

// engine builds the engine once. It can fail, because --tier is validated when
// the engine is built and a typo there has to stop the run rather than quietly
// read at the default tier.
func (a *App) engine() (*fb.Engine, error) {
	if a.eng == nil && a.engErr == nil {
		a.eng, a.engErr = fb.NewEngine(a.cfg)
	}
	return a.eng, a.engErr
}

// out builds the renderer over stdout from the run's resolved output settings,
// so the binary, an ant host and serve all format records identically.
func (a *App) out() (*render.Renderer, error) {
	if err := a.checkFormat(); err != nil {
		return nil, err
	}
	if a.st == nil {
		return render.New(render.Options{Format: render.List, Writer: os.Stdout})
	}
	o := a.st.Output
	return render.New(render.Options{
		Format:   a.format(),
		IsTTY:    o.IsTTY,
		Color:    o.Color,
		Fields:   o.Fields,
		NoHeader: o.NoHeader,
		Template: o.Template,
		Width:    o.Width,
		Writer:   os.Stdout,
	})
}

// format is what this run renders as. With no -o, fb prints the readable list
// view on a terminal and jsonl when piped, so a script that pipes fb gets
// something machine-readable without asking.
func (a *App) format() render.Format {
	if a.st == nil {
		return render.List
	}
	o := a.st.Output
	f := render.Format(o.Format)
	if o.Template == "" && (f == "" || f == render.Auto) {
		if o.IsTTY {
			return render.List
		}
		return render.JSONL
	}
	return f
}

// outputFormats is every value -o takes.
var outputFormats = []render.Format{
	render.Auto, render.List, render.Table, render.Markdown, render.JSON,
	render.JSONL, render.CSV, render.TSV, render.URL, render.Raw,
}

// checkFormat refuses an -o that does not exist. The renderer falls through to
// jsonl on an unknown format, so `-o jsonl1` would print jsonl and exit 0,
// which reads as if the flag had been honoured.
func (a *App) checkFormat() error {
	if a.st == nil || a.st.Output.Format == "" {
		return nil
	}
	f := render.Format(a.st.Output.Format)
	if f == render.Template {
		if a.st.Output.Template == "" {
			return errs.Usage("the template format renders --template, so pass one: --template '{{.id}}'")
		}
		return nil
	}
	switch f {
	case "md", "section", "sections":
		return nil
	}
	for _, v := range outputFormats {
		if f == v {
			return nil
		}
	}
	names := make([]string, 0, len(outputFormats))
	for _, v := range outputFormats {
		if v != render.Auto {
			names = append(names, string(v))
		}
	}
	return errs.Usage("no output format %q; there is %s", f, strings.Join(names, ", "))
}

// rawOutput is the guard on a command that writes bytes rather than records:
// fb photos --download, fb video --download, fb archive. Giving somebody who
// asked for json a directory of jpegs and a zero exit code is the tool agreeing
// with a question it did not answer.
func (a *App) rawOutput(cmd string) error {
	if a.st == nil {
		return nil
	}
	o := a.st.Output
	switch {
	case o.Format != "" && render.Format(o.Format) != render.Auto:
		return errs.Usage("the %s command writes bytes, not records, so there is nothing for -o to format", cmd)
	case o.Template != "":
		return errs.Usage("the %s command writes bytes, not records, so there is nothing for --template to render", cmd)
	case len(o.Fields) > 0:
		return errs.Usage("the %s command writes bytes, not records, so there are no --fields to pick from", cmd)
	}
	return nil
}

// emitOne renders a single record.
func (a *App) emitOne(r Row) error {
	out, err := a.out()
	if err != nil {
		return err
	}
	if err := out.Emit(r); err != nil {
		return err
	}
	return out.Flush()
}

// emit renders a list of records.
func (a *App) emit(rows []Row) error {
	out, err := a.out()
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := out.Emit(r); err != nil {
			return err
		}
	}
	return out.Flush()
}

// fail writes the failure record from doc 05 section 7 to stdout, then hands
// the error on for the exit code.
//
// Only on jsonl, and deliberately. jsonl is what a script gets when it pipes
// fb, and one more line is always valid there. json cannot take a record after
// its array has closed, csv cannot take a row with different columns, and the
// human formats already print a better message on stderr.
func (a *App) fail(target string, err error) error {
	if err == nil {
		return nil
	}
	if a.format() == render.JSONL {
		if b, e := json.Marshal(fb.FailureOf(err, target, a.tier())); e == nil {
			// Nothing to do if stdout is gone: we are on our way out with an
			// error already, and a broken pipe here would replace it.
			_, _ = fmt.Fprintln(os.Stdout, string(b))
		}
	}
	return mapErr(err)
}

// done is the tail of every read.
func (a *App) done(err error) error { return a.fail(a.target, err) }

// tier is the tier this run reads at, for the failure record.
func (a *App) tier() int {
	if a.eng != nil {
		return a.eng.Tier()
	}
	if a.cfg.Cookies != "" && a.cfg.Tier != "0" {
		return 1
	}
	return 0
}

// logf prints progress to stderr unless --quiet.
func (a *App) logf(format string, args ...any) {
	if !a.quiet {
		_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

// warnMissed says on stderr what the record already says in its envelope: a
// surface was tried, did not answer, and the read carried on without it. It
// matters because the result of carrying on is a field that is absent, and an
// absent field otherwise reads as a fact about the page.
func (a *App) warnMissed(env fb.Envelope) {
	for _, m := range env.Missed {
		a.warnOnce(m.Surface + ": " + m.Reason)
	}
}

// warnOnce prints a warning the first time this run raises it. A feed is a
// hundred records that all went without the same surface, and a hundred
// identical warnings is a way of not being read.
func (a *App) warnOnce(note string) {
	if a.warned[note] {
		return
	}
	if a.warned == nil {
		a.warned = map[string]bool{}
	}
	a.warned[note] = true
	a.logf("warn: %s", note)
}

// mapErr turns an fb error into the kit error that carries the matching exit
// code (doc 05 section 7): no results 3, needs auth 4, rate limited 5, not
// found 6, unsupported 7, network 8.
//
// 4 and 7 are the pair that matter. 4 means a session would fix it and the
// message says so. 7 means nothing serves this at any tier, so nobody should go
// looking for a credential.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var (
		ue *fb.UsageError
		nr *fb.NoResultsError
		na *fb.NeedAuthError
		rl *fb.RateLimitedError
		nf *fb.NotFoundError
		un *fb.UnsupportedError
		ne *fb.NetworkError
	)
	switch {
	case errors.As(err, &ue):
		return errs.Usage("%s", ue.Error())
	case errors.As(err, &nr):
		return errs.NoResults("%s", nr.Error())
	case errors.As(err, &na):
		return errs.NeedAuth("%s", na.Error())
	case errors.As(err, &rl):
		return errs.RateLimited("%s", rl.Error())
	case errors.As(err, &nf):
		return errs.NotFound("%s", nf.Error())
	case errors.As(err, &un):
		return errs.Unsupported("%s", un.Error())
	case errors.As(err, &ne):
		return errs.Network("%s", ne.Error())
	}
	// A transport failure that never went through the client still gets 8.
	if n := fb.AsNetwork(err); n != nil {
		return errs.Network("%s", n.Error())
	}
	return err
}
