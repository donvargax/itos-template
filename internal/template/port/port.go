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

// Checkout is an existing project's git repository, as adopt reads it at
// its top: what it is, and the files of its working tree as git sees them.
// Nothing of it is changed but the one new file Create writes.
type Checkout interface {
	// Look says what the folder the checkout was opened in is.
	Look() (CheckoutState, error)
	// Files are the working tree's files: every file git tracks that the
	// working tree holds, every file it holds that git neither tracks nor
	// ignores, and of the paths also, each that it holds though git ignores
	// it. Each has its path from the top, with /; its mode, git's index's for
	// a tracked file; and its contents, the index's blob for a tracked file
	// the working tree holds unchanged, else the working tree's. A submodule
	// is fs.ModeIrregular, with no contents.
	Files(also []string) ([]File, error)
	// Holds is whether the top holds anything at path, with /, a file git
	// ignores too.
	Holds(path string) (bool, error)
	// Create writes data as the new file path, with /, at the top, refusing
	// when there is one already; a failure leaves no file there.
	Create(path string, data []byte) error
}

// CheckoutState is what the folder a Checkout was opened in is: in a git
// repository or not, and when not, why, as git says; when it is, whether
// it has a working tree (a bare repository has none), its top, whether HEAD
// names a commit, and the paths the index or the working tree change from
// HEAD, untracked files git does not ignore included.
type CheckoutState struct {
	Repository bool
	Why        string
	WorkTree   bool
	Top        string
	Committed  bool
	Changed    []string
}

// Project is a made or adopted project's git repository, opened in a folder
// of it, as update reads it and merges a template's change into it: HEAD's
// tree read, and the index and the working tree written as git merge
// writes them. HEAD, every ref and every commit are left as they are; the
// objects a merge writes are referred to by nothing but the index.
type Project interface {
	// Look says what the folder the project was opened in is, as a
	// Checkout's Look says it.
	Look() (CheckoutState, error)
	// Committed is the file path's contents, with /, in HEAD's tree, and
	// false when the tree holds no file there.
	Committed(path string) ([]byte, bool, error)
	Merger
}

// Merger merges a template's render into a project's HEAD, as update merges
// a newer release in and adopt --merge its baseline: HEAD, every ref and
// every commit are left as they are; the objects a merge writes are referred
// to by nothing but the index.
type Merger interface {
	// CanMerge is nil when Merge can merge here, else why not: a git too
	// old to merge trees no commit holds.
	CanMerge() error
	// Merge merges into HEAD the change from base to target, each a whole
	// tree's files with / in their paths, base the merge's base, as git's
	// merge does: what merges cleanly is written to the working tree and
	// staged, and a conflict is left as git merge leaves one, marked in
	// the working tree's file and unmerged in the index (stages 1, 2 and 3:
	// base, HEAD and target), its markers naming HEAD's side HEAD, as git
	// merge does, and the renders as labels says. An empty base merges
	// target in whole: a path both HEAD and target hold otherwise is a
	// conflict both added, with no stage 1. It returns what the merge
	// changes from HEAD and what it leaves conflicted.
	Merge(base, target []File, labels Labels) (Merged, error)
}

// Labels are how a merge's conflict markers name the two renders it
// merges, as git merge names a branch: Base the base's, which a conflict
// style showing the base (diff3, zdiff3) marks, and Target the target's.
type Labels struct {
	Base, Target string
}

// Merged is what a Project's Merge did: each path it changed from HEAD,
// sorted by path, and each path it left conflicted, sorted.
type Merged struct {
	Changes   []Change
	Conflicts []Unmerged
}

// Change is a path a merge changes from HEAD, with /: how, and for a rename
// the path it had.
type Change struct {
	Path string     `json:"path"`
	Kind ChangeKind `json:"kind"`
	From string     `json:"from,omitempty"`
}

// ChangeKind is how a merge changes a path from HEAD.
type ChangeKind string

// The kinds of a Change. A change of a file's mode or type is Modified.
const (
	Added    ChangeKind = "added"
	Modified ChangeKind = "modified"
	Removed  ChangeKind = "removed"
	Renamed  ChangeKind = "renamed"
)

// Unmerged is a path, with /, a merge left conflicted.
type Unmerged struct {
	Path string `json:"path"`
}

// Differ shows how a file changes from one side to the other, as git's own
// diff shows it, a/ the side it changes from and b/ the side it changes to.
type Differ interface {
	// Diff is the diff of from to to, either nil when that side has no such
	// file, each a text file.
	Diff(from, to *File) (string, error)
}

// Tagger is a template's own git repository, the checkout or clone release
// runs in, where a release is tagged (decision 36): its branches read beside
// origin's remote-tracking ones, and annotated tags made and deleted.
// Nothing else of it is changed: no branch, no index, no working tree.
type Tagger interface {
	// Top is the repository's top folder, or a bare repository's own: a
	// *NoRepository when the folder it was opened in is in none.
	Top() (string, error)
	// HasOrigin is whether the repository has a remote named origin.
	HasOrigin() (bool, error)
	// DefaultBranch is the branch origin's HEAD names, the template's
	// default branch, whatever branch is checked out: as git clone recorded
	// it, else as origin says it; false when neither names one.
	DefaultBranch() (string, bool)
	// Heads are the commit the branch's local head is at and the one
	// origin's remote-tracking branch of that name is at, each "" when there
	// is none.
	Heads(branch string) (local, origin string, err error)
	// IsAncestor is whether commit is of or one of of's ancestors.
	IsAncestor(commit, of string) (bool, error)
	// Tag makes name an annotated tag of commit, its message message. A tag
	// there already is refused, as git refuses one.
	Tag(name, commit, message string) error
	// Untag deletes the tag name.
	Untag(name string) error
}

// NoRepository is a folder in no git repository, and why, as git says.
type NoRepository struct{ Why string }

func (e *NoRepository) Error() string { return "in no git repository: " + e.Why }

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
// the question's check takes the answer, saying why it did not, and
// returns that answer, the last its check took.
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
