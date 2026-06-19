// Package cli assembles the cto command tree from the fiftyone domain
// on top of the any-cli/kit framework.
package cli

import (
	"github.com/tamnd/any-cli/kit"
	fiftyone "github.com/tamnd/51cto-cli/51cto"
)

// Build metadata, set via -ldflags at release time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// NewApp assembles the kit application from the fiftyone domain. The
// domain's Register installs the client factory and every operation, so the
// binary and a host share one source of truth. kit.Run turns the App into the
// CLI plus the serve and mcp surfaces and the typed-error-to-exit-code mapping.
//
// To add a command, declare it in 51cto/domain.go with kit.Handle and it
// appears here automatically.
func NewApp() *kit.App {
	id := fiftyone.Domain{}.Info().Identity
	id.Version = Version

	app := kit.New(id)
	fiftyone.Domain{}.Register(app)
	app.AddCommand(newVersionCmd())
	return app
}
