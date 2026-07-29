package cli

import (
	"context"
	"time"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/facebook-cli/fb"
)

// cmd_auth.go is Tier 1: two cookies, copied out of a browser you are already
// signed into.
//
// fb never asks for a password, never automates a login and never touches a
// checkpoint. What a session buys is written down in doc 05 section 6 and it is
// three things: the timeline past the first post, the About section as a query,
// and search at all. Everything else in this tool works signed out.

func authCommands() []kit.Command {
	return []kit.Command{
		{
			Use:   "auth",
			Short: "Manage your Facebook session",
			Sub: []kit.Command{
				newAuthImportCmd(),
				newAuthStatusCmd(),
				newAuthLogoutCmd(),
			},
		},
		newWhoamiCmd(),
	}
}

func newAuthImportCmd() kit.Command {
	var cUser, xs string
	return kit.Command{
		Use:   "import",
		Short: "Store the two cookies that make Tier 1 work",
		Long: "import takes c_user and xs out of a browser you are signed into. Facebook accepts that " +
			"pair on its own, so those two are all fb asks for and all it keeps. They are written " +
			"0600 in the data directory, they go into a request header and nowhere else, and " +
			"`fb config path` says where they live.",
		Args:  kit.NoArgs,
		Write: true,
		Flags: func(f *kit.FlagSet) {
			f.StringVar(&cUser, "c-user", "", "the c_user cookie, which is your account id")
			f.StringVar(&xs, "xs", "", "the xs cookie, which is the session")
		},
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			s := fb.Session{CUser: cUser, XS: xs, ImportedAt: time.Now().UTC()}
			if err := fb.SaveSession(a.cfg.DataDir, s); err != nil {
				return mapErr(err)
			}
			st, err := fb.SessionStatus(a.cfg.DataDir)
			if err != nil {
				return mapErr(err)
			}
			return a.emitOne(statusRow(st))
		},
	}
}

func newAuthStatusCmd() kit.Command {
	return kit.Command{
		Use:   "status",
		Short: "Say whether a session is stored, and whose",
		Args:  kit.NoArgs,
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			st, err := fb.SessionStatus(a.cfg.DataDir)
			if err != nil {
				return mapErr(err)
			}
			return a.emitOne(statusRow(st))
		},
	}
}

func newAuthLogoutCmd() kit.Command {
	return kit.Command{
		Use:     "logout",
		Aliases: []string{"clear"},
		Short:   "Forget the stored session",
		Long: "logout deletes the local cookie file. It does not sign the browser out and does not tell " +
			"Facebook anything, because it never talks to Facebook at all.",
		Args:  kit.NoArgs,
		Write: true,
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			if err := fb.ClearSession(a.cfg.DataDir); err != nil {
				return mapErr(err)
			}
			st, err := fb.SessionStatus(a.cfg.DataDir)
			if err != nil {
				return mapErr(err)
			}
			return a.emitOne(statusRow(st))
		},
	}
}

// newWhoamiCmd proves the stored session against the live site, which is the
// only way to tell an expired cookie from a working one.
func newWhoamiCmd() kit.Command {
	return kit.Command{
		Use:   "whoami",
		Short: "Prove the stored session against Facebook",
		Args:  kit.NoArgs,
		Run: func(ctx context.Context, args []string) error {
			a := appFromCtx(ctx)
			a.target = "me"
			if a.cfg.Cookies == "" {
				return mapErr(fb.NeedAuth("there is no session stored: run fb auth import first"))
			}
			eng, err := a.engine()
			if err != nil {
				return a.done(err)
			}
			p, err := eng.Profile(a.ctx(), "me", fb.ProfileOptions{NoPosts: true})
			if err != nil {
				return a.done(err)
			}
			return a.done(a.emitOne(profileRow(p)))
		},
	}
}
