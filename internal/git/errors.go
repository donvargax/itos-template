package git

import (
	"fmt"
	"strings"
)

// Error is how git failed, a sealed set (decision 17): internal/cli gives
// each kind its exit code in one switch, and gochecksumtype refuses a
// switch that leaves one out. Each is read from what git did, never from
// its exit code passed through (docs/CLI.md, rule 31).
//
//sumtype:decl
type Error interface {
	error
	gitError()
}

// Missing is no git that could be run at all.
type Missing struct{ Err error }

// Unreachable is a template git could not clone.
type Unreachable struct {
	Name string
	Err  *Failed
}

// Failed is a git command that exited with an error: its arguments, its
// exit code and what it said on its standard error.
type Failed struct {
	Args   []string
	Code   int
	Stderr string
}

// NoIdentity is a git that does not know who commits, so cannot make a
// project's first commit.
type NoIdentity struct{ Err *Failed }

func (*Missing) gitError()     {}
func (*Unreachable) gitError() {}
func (*Failed) gitError()      {}
func (*NoIdentity) gitError()  {}

func (e *Missing) Error() string { return fmt.Sprintf("cannot run git: %v", e.Err) }

func (e *Missing) Unwrap() error { return e.Err }

func (e *Unreachable) Error() string {
	return fmt.Sprintf("git cannot clone %s: %v", e.Name, e.Err)
}

func (e *Unreachable) Unwrap() error { return e.Err }

// Error is what git said went wrong: the last line it wrote on its standard
// error that is not empty, else that it exited with its code.
func (e *Failed) Error() string {
	if line := e.lastLine(); line != "" {
		return line
	}
	return fmt.Sprintf("git %s exited %d", strings.Join(e.Args, " "), e.Code)
}

func (e *Failed) lastLine() string {
	lines := strings.Split(strings.TrimSpace(e.Stderr), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func (e *NoIdentity) Error() string {
	return fmt.Sprintf("git knows no one to commit as: %v", e.Err)
}

func (e *NoIdentity) Unwrap() error { return e.Err }
