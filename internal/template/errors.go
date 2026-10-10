package template

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos-template/internal/manifest"
)

// Error is why a template cannot be opened, a combination of it chosen or
// rendered, a sealed set (decision 17) of plain domain types: internal/cli
// gives each kind its exit code and its line, and gochecksumtype refuses a
// switch that leaves one out. A choice's problems come joined
// (errors.Join), every one of them, so one run names all there are to fix.
//
// An Error's own message is read wherever one is shown unclassified, as a
// joined error is. A stack or a feature the person named and the template
// lacks is quoted with every character outside ASCII escaped (%+q), as
// package answer quotes theirs (bug-5): a Hangul filler or a right-to-left
// override in it reaches the reader as an escape, never as itself. What the
// template's own manifest and branches give (a stack or feature found in
// it, a branch, a root, a path) is the template's and kept as it is. The
// template's name, the path or URL the person gave, is kept too: how a
// refusal echoes a path is the person's call (echo-paths-escaped).
//
//sumtype:decl
type Error interface {
	error
	templateError()
}

// NoStack is a choice that names no stack, where none can be asked: the
// template's stacks.
type NoStack struct{ Stacks []string }

// UnknownStack is a stack the template does not have, named on the command
// line or answered on a terminal, and the stacks it does.
type UnknownStack struct {
	Name   string
	Stacks []string
}

// UnknownFeature is a feature the template does not have, and the features
// of the stack chosen (Stack), if it has any.
type UnknownFeature struct {
	Name, Stack string
	Known       []string
}

// OtherStack is a feature of another stack than the one chosen (Stack).
type OtherStack struct {
	Feature manifest.Feature
	Stack   string
}

// Needs is a feature chosen without a feature it needs.
type Needs struct{ Feature, Need string }

// Unsupported is a combination the manifest lists as unsupported.
type Unsupported struct{ Combination manifest.Combination }

// NoRoot is a template whose HEAD names no branch to read its manifest
// from.
type NoRoot struct{ Template string }

// EmptyRoot is a template whose root branch has no commit.
type EmptyRoot struct{ Template, Root string }

// NoManifest is a template with no manifest on its root branch.
type NoManifest struct{ Template, Root string }

// ManifestInvalid is a template's manifest, on its root branch, that cannot
// be used: every problem found in it.
type ManifestInvalid struct {
	Root     string
	Problems []string
}

// NoBranch is a branch the manifest implies (a stack's, a feature's) that
// the template does not have.
type NoBranch struct{ Branch string }

// MergeConflict is merging the template's Branch into the branches merged
// before it (Into, the stack's first) leaving conflicts in Paths: a defect
// of the template, never the person's.
type MergeConflict struct {
	Into   []string
	Branch string
	Paths  []string
}

// HoldsRecord is a template holding File, the file a made project records
// its render in.
type HoldsRecord struct{ File string }

func (*NoStack) templateError()         {}
func (*UnknownStack) templateError()    {}
func (*UnknownFeature) templateError()  {}
func (*OtherStack) templateError()      {}
func (*Needs) templateError()           {}
func (*Unsupported) templateError()     {}
func (*NoRoot) templateError()          {}
func (*EmptyRoot) templateError()       {}
func (*NoManifest) templateError()      {}
func (*ManifestInvalid) templateError() {}
func (*NoBranch) templateError()        {}
func (*MergeConflict) templateError()   {}
func (*HoldsRecord) templateError()     {}

func (e *NoStack) Error() string        { return "no stack chosen" }
func (e *UnknownStack) Error() string   { return fmt.Sprintf("no stack %+q", e.Name) }
func (e *UnknownFeature) Error() string { return fmt.Sprintf("no feature %+q", e.Name) }
func (e *OtherStack) Error() string {
	return fmt.Sprintf("the feature %s of another stack than %s", e.Feature.Branch(), e.Stack)
}
func (e *Needs) Error() string { return fmt.Sprintf("the feature %s without %s", e.Feature, e.Need) }
func (e *Unsupported) Error() string {
	return "the unsupported combination " + e.Combination.Name()
}
func (e *NoRoot) Error() string    { return "no root branch in " + e.Template }
func (e *EmptyRoot) Error() string { return "no commit on " + e.Root + " in " + e.Template }
func (e *NoManifest) Error() string {
	return "no " + manifest.File + " on " + e.Root + " in " + e.Template
}
func (e *ManifestInvalid) Error() string {
	return manifest.File + " on " + e.Root + ": " + strings.Join(e.Problems, "; ")
}
func (e *NoBranch) Error() string { return "no branch " + e.Branch }
func (e *MergeConflict) Error() string {
	return fmt.Sprintf("conflicts merging %s into %s in %s", e.Branch, strings.Join(e.Into, " + "), strings.Join(e.Paths, ", "))
}
func (e *HoldsRecord) Error() string { return "the template holds " + e.File }
