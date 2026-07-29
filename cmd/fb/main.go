// Command fb is a single-binary command line for Facebook.
package main

import (
	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/facebook-cli/cli"
)

func main() {
	// kit.Main builds the command tree, wires the signal handler, and exits with
	// the code the error taxonomy asked for.
	kit.Main(cli.New())
}
