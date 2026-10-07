package release

import (
	"fmt"
	"sort"
)

// Build is what a binary is built from, as the release cut compares it
// (decision 23): each module it links, by path, at its version (a
// replacement's path and version after " => "), and the Go toolchain that
// builds it. The main module is not one of the modules: its own code changes
// with every commit, and a commit's type says whether that is a release.
type Build struct {
	Modules   map[string]string
	Toolchain string
}

// Moved lists what differs from old to new, in a line each for the release
// cut's reason: every linked module whose version moved, that came or that
// went, by path, then the toolchain. None means the binary is built from the
// same modules with the same toolchain, whatever else the range changed.
func Moved(old, new Build) []string {
	var paths []string
	for p := range old.Modules {
		paths = append(paths, p)
	}
	for p := range new.Modules {
		if _, ok := old.Modules[p]; !ok {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	var moved []string
	for _, p := range paths {
		was, linked := old.Modules[p]
		is, links := new.Modules[p]
		switch {
		case !linked:
			moved = append(moved, fmt.Sprintf("%s %s, newly linked", p, is))
		case !links:
			moved = append(moved, fmt.Sprintf("%s %s, no longer linked", p, was))
		case was != is:
			moved = append(moved, fmt.Sprintf("%s %s to %s", p, was, is))
		}
	}
	if old.Toolchain != new.Toolchain {
		moved = append(moved, fmt.Sprintf("the toolchain %s to %s", old.Toolchain, new.Toolchain))
	}
	return moved
}
