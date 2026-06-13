// Command fb is a single-binary command line for Facebook.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/fang"
	"github.com/tamnd/facebook-cli/cli"
	"github.com/tamnd/facebook-cli/fb"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := cli.Root()
	// fang gives styled help, errors, and shell completion for free; the command
	// tree and its exit-code mapping still live in the cli package.
	if err := fang.Execute(ctx, root,
		fang.WithVersion(cli.Version),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	); err != nil {
		var ce *fb.CodeError
		if errors.As(err, &ce) {
			os.Exit(ce.Code)
		}
		os.Exit(1)
	}
}
