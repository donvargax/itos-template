package syncing

import "strings"

// Error is why a branch cannot be synced, a sealed set (decision 17):
// internal/cli gives each kind its exit code and its line.
//
//sumtype:decl
type Error interface {
	error
	syncingError()
}

// NotRepository is a folder in no git repository, and why, as git says.
type NotRepository struct{ Why string }

// NoWorkTree is a git repository with no working tree: a bare one, or its
// .git folder.
type NoWorkTree struct{}

// NoCommit is a repository at Top whose HEAD names no commit.
type NoCommit struct{ Top string }

// NoOrigin is a repository with no remote named origin, whose HEAD names
// the root.
type NoOrigin struct{}

// NoDefaultBranch is a repository whose origin names no default branch,
// the root, neither as git clone recorded it nor when asked.
type NoDefaultBranch struct{}

// NotLocal is a branch sync reads, the root or one above the branch checked
// out, that the clone holds no local branch of.
type NotLocal struct{ Branch string }

// Detached is a HEAD that names no branch, Branches those sync can merge
// into, in the manifest's order.
type Detached struct{ Branches []string }

// Unnamed is the branch checked out, Branch, which the manifest does not
// name, Branches those sync can merge into, in the manifest's order.
type Unnamed struct {
	Branch   string
	Branches []string
}

// Behind is a branch above the one checked out, Branch, that lacks commits
// of the branches it descends from Lacks names.
type Behind struct {
	Branch string
	Lacks  []string
}

// Merging is a clone with a merge in progress already.
type Merging struct{}

// Dirty is a clone whose index or working tree changes Paths from HEAD, an
// untracked file git does not ignore among them.
type Dirty struct{ Paths []string }

// Conflicted is a merge of the branch Branch into Into that left conflicts
// in Paths.
type Conflicted struct {
	Branch, Into string
	Paths        []string
}

func (*NotRepository) syncingError()   {}
func (*NoWorkTree) syncingError()      {}
func (*NoCommit) syncingError()        {}
func (*NoOrigin) syncingError()        {}
func (*NoDefaultBranch) syncingError() {}
func (*NotLocal) syncingError()        {}
func (*Detached) syncingError()        {}
func (*Unnamed) syncingError()         {}
func (*Behind) syncingError()          {}
func (*Merging) syncingError()         {}
func (*Dirty) syncingError()           {}
func (*Conflicted) syncingError()      {}

func (e *NotRepository) Error() string { return "in no git repository: " + e.Why }
func (*NoWorkTree) Error() string      { return "a git repository with no working tree" }
func (e *NoCommit) Error() string      { return "no commit in " + e.Top }
func (*NoOrigin) Error() string        { return "no remote origin" }
func (*NoDefaultBranch) Error() string { return "origin names no default branch" }
func (e *NotLocal) Error() string      { return "no local branch " + e.Branch }
func (*Detached) Error() string        { return "HEAD names no branch" }
func (e *Unnamed) Error() string       { return "the manifest names no branch " + e.Branch }
func (e *Behind) Error() string        { return e.Branch + " lacks " + strings.Join(e.Lacks, ", ") }
func (*Merging) Error() string         { return "a merge is in progress" }
func (e *Dirty) Error() string         { return "changes from HEAD in " + strings.Join(e.Paths, ", ") }
func (e *Conflicted) Error() string    { return "conflicts in " + strings.Join(e.Paths, ", ") }
