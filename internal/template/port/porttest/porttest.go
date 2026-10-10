// Package porttest holds fakes of the ports, for the domain's tests
// (decision 18): working, lighter implementations, as an in-memory database
// stands in for a real one, never mocks. A Repository holds each branch as
// an fstest.MapFS and really merges them; a Disk really holds what is
// written to it; Git really commits what is on that Disk; Programs really
// run, as Go functions; a Terminal really asks. A test asserts on what they
// end up holding (the files written, the commit, what the terminal was asked)
// and on what the domain returns, never on which calls were made.
//
// It is test support, imported by tests alone, and does no I/O.
package porttest

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"testing/fstest"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Repository is a template's repository held in memory: each branch a tree
// of files, its commit named after it, and each tag a tree of its own, its
// commit named after the tag, on the history of the branch it says.
type Repository struct {
	Root     string // the branch HEAD names; "" when it names none
	Branches map[string]fstest.MapFS
	Tagged   map[string]Tag // each tag by its name
}

// Tag is a tag of a Repository: the tree of the commit it names, and the
// branch whose history holds that commit, "" for none.
type Tag struct {
	Tree fstest.MapFS
	On   string
}

var _ port.Repository = (*Repository)(nil)

// DefaultBranch is Root.
func (r *Repository) DefaultBranch() (string, error) {
	if r.Root == "" {
		return "", errors.New("HEAD names no branch")
	}
	return r.Root, nil
}

// Commit is the branch's commit, "commit of <branch>".
func (r *Repository) Commit(branch string) (string, bool, error) {
	if _, ok := r.Branches[branch]; !ok {
		return "", false, nil
	}
	return "commit of " + branch, true, nil
}

// tagCommit is the commit of the tag name: "tag <name>".
func tagCommit(name string) string { return "tag " + name }

// tree is the tree of commit, a branch's or a tag's.
func (r *Repository) tree(commit string) (fstest.MapFS, error) {
	if name, ok := strings.CutPrefix(commit, "tag "); ok {
		if tag, ok := r.Tagged[name]; ok {
			return tag.Tree, nil
		}
	}
	if tree, ok := r.Branches[strings.TrimPrefix(commit, "commit of ")]; ok {
		return tree, nil
	}
	return nil, fmt.Errorf("no commit %s", commit)
}

// File is a file of the tree of commit, a branch's or a tag's.
func (r *Repository) File(commit, name string) ([]byte, bool, error) {
	tree, err := r.tree(commit)
	if err != nil {
		return nil, false, err
	}
	f, ok := tree[name]
	if !ok {
		return nil, false, nil
	}
	return f.Data, true, nil
}

// Merge lays the tree of each commit after the first over the files of
// those before it, a file both hold the same being one file; a file the
// commit holds otherwise than the commits before it is a conflict, a
// *port.Conflict naming every such file. The files come in path order, as
// git's are.
func (r *Repository) Merge(commits []string) ([]port.File, error) {
	merged := map[string]port.File{}
	for i, c := range commits {
		tree, err := r.tree(c)
		if err != nil {
			return nil, err
		}
		var conflicts []string
		for _, p := range slices.Sorted(maps.Keys(tree)) {
			f := file(p, tree[p])
			if before, ok := merged[p]; ok && (before.Mode != f.Mode || string(before.Data) != string(f.Data)) {
				conflicts = append(conflicts, p)
				continue
			}
			merged[p] = f
		}
		if len(conflicts) > 0 {
			return nil, &port.Conflict{At: i, Paths: conflicts}
		}
	}
	var files []port.File
	for _, p := range slices.Sorted(maps.Keys(merged)) {
		files = append(files, merged[p])
	}
	return files, nil
}

// Tags are each of Tagged, its commit "tag <name>", in name order.
func (r *Repository) Tags() ([]port.Tag, error) {
	var tags []port.Tag
	for _, name := range slices.Sorted(maps.Keys(r.Tagged)) {
		tags = append(tags, port.Tag{Name: name, Commit: tagCommit(name)})
	}
	return tags, nil
}

// IsAncestor is whether commit is of, or a tag's on the branch whose head
// of is.
func (r *Repository) IsAncestor(commit, of string) (bool, error) {
	if commit == of {
		return true, nil
	}
	name, ok := strings.CutPrefix(commit, "tag ")
	tag, tagged := r.Tagged[name]
	return ok && tagged && tag.On != "" && "commit of "+tag.On == of, nil
}

// file is the port's file of a tree's: a mode of 0 is a plain file's.
func file(p string, f *fstest.MapFile) port.File {
	mode := f.Mode
	if mode == 0 {
		mode = 0o644
	}
	return port.File{Path: p, Mode: mode, Data: f.Data}
}

// Disk is a file system held in memory: each folder's files by their
// paths, with /, and the paths that are files.
type Disk struct {
	Folders map[string]map[string]port.File
	Files   map[string]bool
	Full    string // a file of a project whose write fails, as on a full disk
}

var _ port.Disk = (*Disk)(nil)

// NewDisk is an empty disk.
func NewDisk() *Disk {
	return &Disk{Folders: map[string]map[string]port.File{}, Files: map[string]bool{}}
}

// Look says what is at folder.
func (d *Disk) Look(folder string) (port.Contents, error) {
	if d.Files[folder] {
		return port.NotFolder, nil
	}
	files, ok := d.Folders[folder]
	switch {
	case !ok:
		return port.Missing, nil
	case len(files) == 0:
		return port.Empty, nil
	default:
		return port.Full, nil
	}
}

// ErrFull is the failure of a write of the file Full names.
var ErrFull = errors.New("no space left on the disk")

// Write writes files into folder, making it; the file Full names fails, the
// files before it written.
func (d *Disk) Write(folder string, files []port.File) error {
	if d.Folders[folder] == nil {
		d.Folders[folder] = map[string]port.File{}
	}
	for _, f := range files {
		if f.Path == d.Full {
			return ErrFull
		}
		d.Folders[folder][f.Path] = f
	}
	return nil
}

// Clear removes folder when made, else everything in it.
func (d *Disk) Clear(folder string, made bool) {
	if made {
		delete(d.Folders, folder)
		return
	}
	d.Folders[folder] = map[string]port.File{}
}

// Git is the git that commits a folder of Disk: each folder's commit, a
// snapshot of its files then.
type Git struct {
	Disk    *Disk
	Who     *port.Identity // who git's config names; nil when it names no one
	Refuse  error          // why a hook refuses every commit, when one does
	Commits map[string]Commit
}

var _ port.Committer = (*Git)(nil)

// Commit is a folder's first commit: its files, each executable one with
// mode 0o755 whatever the disk says, its message and who it is by.
type Commit struct {
	Files   map[string]port.File
	Message string
	By      port.Identity
}

// NewGit is a git that knows who commits, committing folders of d.
func NewGit(d *Disk) *Git {
	return &Git{Disk: d, Who: &port.Identity{Name: "Someone", Email: "someone@localhost"}, Commits: map[string]Commit{}}
}

// ErrNoIdentity is a git whose config names no one.
var ErrNoIdentity = errors.New("git knows no one")

// Identity is nil when Who names someone.
func (g *Git) Identity() error {
	if g.Who == nil {
		return ErrNoIdentity
	}
	return nil
}

// Commit commits everything in folder, its commit "<n> of <folder>".
func (g *Git) Commit(folder, message string, executables []string, by *port.Identity) (string, error) {
	if g.Refuse != nil {
		return "", &port.Refused{Err: g.Refuse}
	}
	if by == nil {
		if g.Who == nil {
			return "", ErrNoIdentity
		}
		by = g.Who
	}
	files := maps.Clone(g.Disk.Folders[folder])
	for _, p := range executables {
		f := files[p]
		f.Mode = 0o755
		files[p] = f
	}
	g.Commits[folder] = Commit{Files: files, Message: message, By: *by}
	return "commit of " + folder, nil
}

// Programs are programs a check can run, by name, each a Go function of the
// folder it runs in and its arguments, returning what it writes and whether
// it exits 0.
type Programs map[string]func(dir string, args []string) ([]byte, bool)

var _ port.Runner = Programs(nil)

// Run runs words[0] of p; one p has not is a program that cannot be
// started.
func (p Programs) Run(dir string, words []string) ([]byte, bool) {
	program, ok := p[words[0]]
	if !ok {
		return []byte("cannot run " + words[0] + ": no such program\n"), false
	}
	return program(dir, words[1:])
}

// Has is a program that exits 0 when every file its arguments name is in
// the folder of d it runs in, and says which is not otherwise.
func Has(d *Disk) func(dir string, args []string) ([]byte, bool) {
	return func(dir string, args []string) ([]byte, bool) {
		for _, a := range args {
			if _, ok := d.Folders[dir][a]; !ok {
				return []byte("no " + a + "\n"), false
			}
		}
		return nil, true
	}
}

// Folders are temporary folders made on Disk, "tmp/1", "tmp/2"…
type Folders struct {
	Disk *Disk
	Fail error // why no folder can be made, when none can
	made int
}

var _ port.Folders = (*Folders)(nil)

// Make makes the next empty folder.
func (f *Folders) Make() (string, error) {
	if f.Fail != nil {
		return "", f.Fail
	}
	f.made++
	dir := path.Join("tmp", fmt.Sprint(f.made))
	f.Disk.Folders[dir] = map[string]port.File{}
	return dir, nil
}

// Remove removes folder from Disk.
func (f *Folders) Remove(folder string) { delete(f.Disk.Folders, folder) }

// Terminal is a person at a terminal: the lines they type, in order, and
// what they were asked, as data: each question, once each time it was
// asked, and why each answer its check refused was refused. The wording is
// the UI's, so it is not here.
type Terminal struct {
	Lines   []string
	Asked   []port.Question
	Refused []error
}

var _ port.Asker = (*Terminal)(nil)

// ErrInputEnded is the person typing nothing more.
var ErrInputEnded = errors.New("the input ended")

// Ask takes the next line, an empty one an answer's default, until q's
// check takes one.
func (t *Terminal) Ask(q port.Question) (string, error) {
	var def *string
	var check func(string) error
	switch q := q.(type) {
	case port.StackChoice:
		check = q.Check
	case port.Answer:
		def, check = q.Default, q.Check
	}
	for {
		t.Asked = append(t.Asked, q)
		if len(t.Lines) == 0 {
			return "", ErrInputEnded
		}
		answer := t.Lines[0]
		t.Lines = t.Lines[1:]
		if answer == "" && def != nil {
			answer = *def
		}
		if check == nil {
			return answer, nil
		}
		if err := check(answer); err != nil {
			t.Refused = append(t.Refused, err)
			continue
		}
		return answer, nil
	}
}

// The modes of a fake tree's files but a plain one's (0, or 0o644): an
// executable file's, a symbolic link's (its Data the target) and a
// submodule's.
const (
	Executable = fs.FileMode(0o755)
	Link       = fs.ModeSymlink | 0o777
	Submodule  = fs.ModeIrregular
)
