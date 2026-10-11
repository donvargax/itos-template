package releasing

import (
	"strconv"
	"strings"
)

// Error is why a release cannot be cut, a sealed set (decision 17):
// internal/cli gives each kind its exit code and its line.
//
//sumtype:decl
type Error interface {
	error
	releasingError()
}

// BadVersion is a version that is not vX.Y.Z with an optional pre-release.
type BadVersion struct{ Version string }

// NotRepository is a folder in no git repository, and why, as git says.
type NotRepository struct{ Why string }

// NoOrigin is a repository with no remote named origin to publish to.
type NoOrigin struct{}

// NoDefaultBranch is a repository whose origin names no default branch, the
// root a release tags, neither as git clone recorded it nor when asked.
type NoDefaultBranch struct{}

// Used is a version the branches the manifest lists carry already, Tags
// each such tag.
type Used struct{ Tags []string }

// Unsynced is a listed branch whose local head is not origin's, How saying
// how it differs.
type Unsynced struct {
	Branch string
	How    How
}

// How is how a branch's local head differs from origin's.
type How string

// The ways a local head differs from origin's: past it, before it, each
// past a commit of the other, or with no head of origin's at all.
const (
	Ahead       How = "ahead of"
	Behind      How = "behind"
	Diverged    How = "diverged from"
	Unpublished How = "not on"
)

// Failed is a check in which Failed of All combinations failed.
type Failed struct{ Failed, All int }

// Refused is the tag Tag the repository refused, and why; Left, the tags
// made before it that could not be deleted after, the rest deleted.
type Refused struct {
	Tag  string
	Err  error
	Left []string
}

func (*BadVersion) releasingError()      {}
func (*NotRepository) releasingError()   {}
func (*NoOrigin) releasingError()        {}
func (*NoDefaultBranch) releasingError() {}
func (*Used) releasingError()            {}
func (*Unsynced) releasingError()        {}
func (*Failed) releasingError()          {}
func (*Refused) releasingError()         {}

func (e *BadVersion) Error() string    { return "the version " + strconv.Quote(e.Version) }
func (e *NotRepository) Error() string { return "in no git repository: " + e.Why }
func (*NoOrigin) Error() string        { return "no remote origin" }
func (*NoDefaultBranch) Error() string { return "origin names no default branch" }
func (e *Used) Error() string          { return "the tags " + strings.Join(e.Tags, ", ") + " exist" }
func (e *Unsynced) Error() string      { return e.Branch + " is " + string(e.How) + " origin's" }
func (e *Failed) Error() string {
	return strconv.Itoa(e.Failed) + " of " + strconv.Itoa(e.All) + " combinations failed"
}
func (e *Refused) Error() string { return "git refused the tag " + e.Tag + ": " + e.Err.Error() }

func (e *Refused) Unwrap() error { return e.Err }
