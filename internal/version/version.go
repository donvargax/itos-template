// Package version is the version itos-template says it is. Harvested from
// itos's internal/version.
//
// The version is the release's tag, written nowhere in the tree: a release's
// build stamps it into the binary with -ldflags
// "-X github.com/donvargax/itos-template/internal/version.stamp=<v>", as the
// acceptance harness stamps the binary it builds. A binary built without the
// stamp says the module version Go records: the release's for
// `go install github.com/donvargax/itos-template/cmd/itos-template@v<x>`, a
// pseudo-version from git (0.0.0-<time>-<commit>, +dirty with changes) for a
// go build in a checkout. One that records none (go run, go test, a build
// with -buildvcs=false) says Dev.
package version

import (
	"runtime/debug"
	"strings"
)

// stamp is the version, set at link time (-ldflags -X).
var stamp string

// Dev is what a binary says it is when it was built with neither the stamp
// nor a module version: Go's own word for it.
const Dev = "(devel)"

// Version is the version this binary says it is: the stamp, else the module
// version Go recorded, without its leading v, else Dev.
func Version() string {
	if stamp != "" {
		return stamp
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		return fromModule(info.Main.Version)
	}
	return Dev
}

// fromModule reads a module version as Go records it ("v0.6.0", or "(devel)"
// when it knows none).
func fromModule(v string) string {
	if v == "" || v == Dev {
		return Dev
	}
	return strings.TrimPrefix(v, "v")
}
