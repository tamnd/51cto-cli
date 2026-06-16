// Command cto is a single-binary command line for 51cto.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/51cto-cli/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// kit builds the command tree from the registry, adds the serve and mcp
	// surfaces, wraps it in fang for help and completion, and maps the typed
	// error taxonomy to exit codes. The release ldflags set cli.Version.
	os.Exit(kit.Run(ctx, cli.NewApp()))
}
