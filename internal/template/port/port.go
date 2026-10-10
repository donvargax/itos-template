// Package port is how the domain (template, project) reaches what is
// outside it (decision 17): a template's git repository, the folder a
// project is written into, the git that commits it, the programs a check
// runs, the temporary folders check renders in, and a person at a terminal.
// It holds only the interfaces infra implements and the plain types their
// signatures need, built of the standard library's.
//
// Infra implements each (internal/git, disk, program, tempdir, prompt),
// saying so with var _ port.X = (*Y)(nil), and imports no package of ours
// but this one; turning git's output and errors into these terms is its
// anti-corruption layer, kept there. The domain imports this and no infra,
// so its tests run it against the fakes of porttest (decision 18).
//
// Where an outside failure means something to the domain (a merge that
// leaves conflicts, a commit git refuses) the port names it with a type
// here, which the domain wraps in its own error; any other failure is
// infra's own, and passes through the domain untouched.
package port

import (
	"io/fs"
	"strconv"
	"strings"
)

// Repository is a template's git repository, cloned: its branches and tags
// read and its commits merged, never checked out.
type Repository interface {
	// DefaultBranch is the branch the repository's HEAD names: its root.
	DefaultBranch() (string, error)
	// Commit is the commit a branch is at, and false when there is no such
	// branch.
	Commit(branch string) (string, bool, error)
	// File is a file's contents at a commit, and false when the commit
	// holds no such file.
	File(commit, path string) ([]byte, bool, error)
	// Merge merges each of commits after the first, in order, into the
	// first, as git merge does, and returns the merged tree's files in git's
	// order. A merge that leaves conflicts is a *Conflict.
	Merge(commits []string) ([]File, error)
	// Tags are the repository's tags that name a commit, in no order.
	Tags() ([]Tag, error)
	// IsAncestor is whether commit is of or one of of's ancestors: on the
	// history of the commit of, a branch's head.
	IsAncestor(commit, of string) (bool, error)
}

// Tag is a tag of a repository: its name, without refs/tags/, so with the
// slashes it holds (stack/go/v1.2.0), and the commit it names, an
// annotated tag's own commit.
type Tag struct {
	Name, Commit string
}

// File is a file of a tree: its path, with / on every system; its mode; and
// its contents. The mode is 0o644, or 0o755 for a file git records as
// executable; fs.ModeSymlink for a symbolic link, its contents the target;
// fs.ModeIrregular for a submodule's commit, which has no contents.
type File struct {
	Path string
	Mode fs.FileMode
	Data []byte
}

// Conflict is merging the commit at At, an index of the commits Merge was
// given, into the commits merged before it leaving conflicts in Paths.
type Conflict struct {
	At    int
	Paths []string
}

func (c *Conflict) Error() string {
	return "merging commit " + strconv.Itoa(c.At) + " leaves conflicts in " + strings.Join(c.Paths, ", ")
}

// Disk is the file system a project's folder is on.
type Disk interface {
	// Look says what is at folder.
	Look(folder string) (Contents, error)
	// Write writes files into folder, making it and the folders the files
	// need, each file's mode as File says.
	Write(folder string, files []File) error
	// Clear removes what a failed Write left in folder: the folder itself
	// when made says Write made it, else everything in it.
	Clear(folder string, made bool)
}

// Contents is what is at a folder's path.
type Contents int

// What Look finds.
const (
	Missing   Contents = iota // nothing
	Empty                     // an empty folder
	Full                      // a folder with files in it
	NotFolder                 // a file
)

// Committer is the git that makes a project's folder a repository.
type Committer interface {
	// Identity is nil when git knows who commits, else why it does not.
	Identity() error
	// Commit makes folder a git repository and commits everything in it as
	// its first commit, with message, the files of executables (paths with
	// /) recorded executable whatever the file system says. The commit is
	// by by when it is given, else by whoever git's config names. It returns
	// the commit; a commit git refuses (a hook saying no) is a *Refused.
	Commit(folder, message string, executables []string, by *Identity) (string, error)
}

// Identity is who a commit is by.
type Identity struct {
	Name, Email string
}

// Refused is a commit git refused, and why.
type Refused struct{ Err error }

func (r *Refused) Error() string { return "git refused the commit: " + r.Err.Error() }

func (r *Refused) Unwrap() error { return r.Err }

// Runner runs the programs a template's checks name.
type Runner interface {
	// Run runs words, the program first, in dir, with no shell, and returns
	// what it wrote on its standard output and error, as they came, and
	// whether it exited 0. A program that cannot be started fails, its
	// output saying why.
	Run(dir string, words []string) ([]byte, bool)
}

// Folders makes and removes temporary folders.
type Folders interface {
	// Make makes a new, empty folder and returns its path.
	Make() (string, error)
	// Remove removes a folder Make made and everything in it. A folder
	// left behind is only a temporary one, so a failure is not returned.
	Remove(folder string)
}

// Asker asks a person, on a terminal, what a command line left out. What
// is asked is data, a Question; how it is worded is the UI's.
type Asker interface {
	Ask(q Question) (string, error)
}

// Question is what is asked, a sealed set: the choice of a stack, or an
// answer to one of the manifest's questions. The Asker asks again until
// the question's check takes the answer, saying why it did not.
//
//sumtype:decl
type Question interface{ question() }

// StackChoice is the choice of a stack among Stacks, in the manifest's
// order. Check refuses a stack not among them, with the domain's error
// carrying the stacks.
type StackChoice struct {
	Stacks []string
	Check  func(answer string) error
}

// Answer is an answer to one of the manifest's questions: its text, the
// template author's, its default, nil when it has none, and the check an
// answer must pass, nil when every answer does.
type Answer struct {
	Text    string
	Default *string
	Check   func(answer string) error
}

func (StackChoice) question() {}
func (Answer) question()      {}
