package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/facebook-cli/fb"
)

// cmd_config.go is the two commands that talk about this machine rather than
// about Facebook: where fb keeps things, and what is in the cache.

func configCommands() []kit.Command {
	return []kit.Command{newConfigCmd(), newCacheCmd()}
}

func newConfigCmd() kit.Command {
	return kit.Command{
		Use:   "config",
		Short: "Show where fb keeps things and what it resolved",
		Sub: []kit.Command{
			{
				Use:   "path",
				Short: "Print the data directory",
				Args:  kit.NoArgs,
				Run: func(ctx context.Context, args []string) error {
					_, err := fmt.Fprintln(os.Stdout, appFromCtx(ctx).cfg.DataDir)
					return err
				},
			},
			{
				Use:   "show",
				Short: "Print the resolved configuration",
				Args:  kit.NoArgs,
				Run: func(ctx context.Context, args []string) error {
					a := appFromCtx(ctx)
					c := a.cfg
					return a.emitOne(kv(
						"data_dir", c.DataDir,
						"cache_dir", filepath.Join(c.DataDir, "cache"),
						"session", fb.SessionPath(c.DataDir),
						"signed_in", yesno(c.Cookies != ""),
						"tier", orAuto(c.Tier),
						"rate", c.Rate.String(),
						"retries", strconv.Itoa(c.Retries),
						"timeout", c.Timeout.String(),
						"cache_ttl", c.CacheTTL.String(),
						"cache", yn(!c.NoCache),
					))
				},
			},
		},
	}
}

func newCacheCmd() kit.Command {
	return kit.Command{
		Use:   "cache",
		Short: "Inspect or clear the page cache",
		Args:  kit.NoArgs,
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			eng, err := a.engine()
			if err != nil {
				return mapErr(err)
			}
			bytes, files := eng.Client().Cache.Size()
			return a.emitOne(kv(
				"dir", filepath.Join(a.cfg.DataDir, "cache"),
				"files", strconv.Itoa(files),
				"bytes", strconv.FormatInt(bytes, 10),
				"ttl", a.cfg.CacheTTL.String(),
			))
		},
		Sub: []kit.Command{
			{
				Use:   "clear",
				Short: "Delete every cached page",
				Args:  kit.NoArgs,
				Write: true,
				Run: func(ctx context.Context, args []string) error {
					a := appFromCtx(ctx)
					eng, err := a.engine()
					if err != nil {
						return mapErr(err)
					}
					if err := eng.Client().Cache.Clear(); err != nil {
						return mapErr(err)
					}
					a.logf("cache cleared")
					return nil
				},
			},
		},
	}
}

// kv builds a one-record row out of alternating keys and values, which is the
// shape both of these commands want: named settings rather than a list of
// things.
func kv(pairs ...string) Row {
	cols := make([]string, 0, len(pairs)/2)
	vals := make([]string, 0, len(pairs)/2)
	value := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		cols = append(cols, pairs[i])
		vals = append(vals, pairs[i+1])
		value[pairs[i]] = pairs[i+1]
	}
	return Row{Cols: cols, Vals: vals, Value: value}
}

func orAuto(s string) string {
	if s == "" {
		return "auto"
	}
	return s
}
