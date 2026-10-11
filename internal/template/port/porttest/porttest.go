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
// commit named after the tag, on the history of the branch it says. A
// method Fails names by its name fails with that error, as a git that
// cannot be read does.
type Repository struct {
	Root     string // the branch HEAD names; "" when it names none
	Branches map[string]fstest.MapFS
	Tagged   map[string]Tag   // each tag by its name
	NoTags   error            // why the tags cannot be read, when they cannot
	Fails    map[string]error // by method: Commit, Merge
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

// Commit is the branch's commit, "commit of <branch>", or the error
// Fails gives Commit.
func (r *Repository) Commit(branch string) (string, bool, error) {
	if _, ok := r.Branches[branch]; !ok || r.Fails["Commit"] != nil {
		return "", false, r.Fails["Commit"]
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
	if err := r.Fails["Merge"]; err != nil {
		return nil, err
	}
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

// Tags are each of Tagged, its commit "tag <name>", in name order, or
// NoTags when it is set.
func (r *Repository) Tags() ([]port.Tag, error) {
	if r.NoTags != nil {
		return nil, r.NoTags
	}
	var tags []port.Tag
	for _, name := range slices.Sorted(maps.Keys(r.Tagged)) {
		tags = append(tags, port.Tag{Name: name, Commit: tagCommit(name)})
	}
	return tags, nil
}

// IsAncestor is whether commit is of, or a tag's on the branch whose head
// of is; either being no commit is an error, as git's.
func (r *Repository) IsAncestor(commit, of string) (bool, error) {
	for _, c := range []string{commit, of} {
		if _, err := r.tree(c); err != nil {
			return false, err
		}
	}
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

// Checkout is an existing project's repository held in memory: what Look
// says of it, every file of its working tree, the paths of them git
// ignores, and Fail, why its files cannot be read, when they cannot. Create
// writes into Tree, as the working tree gains a file.
type Checkout struct {
	State   port.CheckoutState
	Tree    map[string]port.File
	Ignored map[string]bool
	Fail    error
	// Unwritable is why Create cannot write, when it cannot.
	Unwritable error
}

var _ port.Checkout = (*Checkout)(nil)

// Look is State.
func (c *Checkout) Look() (port.CheckoutState, error) { return c.State, nil }

// Files are the files of Tree, in path order, but those git ignores that
// also does not name.
func (c *Checkout) Files(also []string) ([]port.File, error) {
	if c.Fail != nil {
		return nil, c.Fail
	}
	var files []port.File
	for _, p := range slices.Sorted(maps.Keys(c.Tree)) {
		if !c.Ignored[p] || slices.Contains(also, p) {
			files = append(files, c.Tree[p])
		}
	}
	return files, nil
}

// Holds is whether Tree holds p, ignored or not.
func (c *Checkout) Holds(p string) (bool, error) {
	_, ok := c.Tree[p]
	return ok, c.Fail
}

// Create adds the file p to Tree, of mode 0o644, unless Unwritable says it
// cannot, or Tree holds p already, an error wrapping fs.ErrExist.
func (c *Checkout) Create(p string, data []byte) error {
	if c.Unwritable != nil {
		return c.Unwritable
	}
	if _, ok := c.Tree[p]; ok {
		return fmt.Errorf("create %s: %w", p, fs.ErrExist)
	}
	c.Tree[p] = port.File{Path: p, Mode: 0o644, Data: data}
	return nil
}

// Project is a made project's repository held in memory: what Look says of
// it, HEAD's tree, and, once Merge has run, its working tree and the
// conflicted paths' index stages. Merge merges each path as a whole file,
// as git does when both sides change a file, so a change both sides make
// to one file conflicts, however far apart its lines; it follows no
// rename. Fail is why HEAD cannot be read or merged into, when it cannot.
type Project struct {
	State port.CheckoutState
	Head  map[string]port.File
	Fail  error
	// Unable is why it cannot merge at all, as a git too old cannot, when
	// it cannot.
	Unable error

	// Tree is the working tree after Merge, each path's file; a conflicted
	// one is HEAD's side, or the target's when HEAD has none.
	Tree map[string]port.File
	// Stages are the conflicted paths' index entries after Merge: base,
	// HEAD and target, nil for a side with no file.
	Stages map[string][3]*port.File
	// Merges counts the merges made.
	Merges int
	// Labels are the labels the last Merge was given.
	Labels port.Labels
}

var _ port.Project = (*Project)(nil)

// Look is State.
func (p *Project) Look() (port.CheckoutState, error) { return p.State, nil }

// CanMerge is Unable.
func (p *Project) CanMerge() error { return p.Unable }

// Committed is Head's file path, or Fail.
func (p *Project) Committed(path string) ([]byte, bool, error) {
	if p.Fail != nil {
		return nil, false, p.Fail
	}
	f, ok := p.Head[path]
	return f.Data, ok, nil
}

// Merge merges every path of base, Head and target: the side that changed
// it from base wins, both changing it the same way is one change, and
// both changing it otherwise is a conflict. labels are kept in Labels.
func (p *Project) Merge(base, target []port.File, labels port.Labels) (port.Merged, error) {
	if p.Fail != nil {
		return port.Merged{}, p.Fail
	}
	p.Merges++
	p.Labels = labels
	b, t := byPath(base), byPath(target)
	paths := map[string]struct{}{}
	for _, m := range []map[string]port.File{b, p.Head, t} {
		for path := range m {
			paths[path] = struct{}{}
		}
	}
	p.Tree, p.Stages = map[string]port.File{}, map[string][3]*port.File{}
	var merged port.Merged
	for _, path := range slices.Sorted(maps.Keys(paths)) {
		sides := [3]*port.File{side(b, path), side(p.Head, path), side(t, path)}
		result := sides[1]
		switch {
		case same(sides[0], sides[1]):
			result = sides[2]
		case !same(sides[1], sides[2]) && !same(sides[0], sides[2]):
			p.Stages[path] = sides
			merged.Conflicts = append(merged.Conflicts, port.Unmerged{Path: path})
			if result == nil {
				result = sides[2]
			}
		}
		if result != nil {
			p.Tree[path] = *result
		}
		if change, ok := changeOf(path, sides[1], result); ok {
			merged.Changes = append(merged.Changes, change)
		}
	}
	return merged, nil
}

func byPath(files []port.File) map[string]port.File {
	m := map[string]port.File{}
	for _, f := range files {
		m[f.Path] = f
	}
	return m
}

func side(m map[string]port.File, path string) *port.File {
	f, ok := m[path]
	if !ok {
		return nil
	}
	return &f
}

func same(a, b *port.File) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Mode == b.Mode && string(a.Data) == string(b.Data)
}

// changeOf is how path changes from HEAD's file to the merge's, false when
// it does not.
func changeOf(path string, head, result *port.File) (port.Change, bool) {
	switch {
	case same(head, result):
		return port.Change{}, false
	case head == nil:
		return port.Change{Path: path, Kind: port.Added}, true
	case result == nil:
		return port.Change{Path: path, Kind: port.Removed}, true
	}
	return port.Change{Path: path, Kind: port.Modified}, true
}

// Differ is a diff as a fake writes it: each side's path and contents,
// "a/<path>: <contents>" then "b/<path>: <contents>", a side with no file
// "a: none"; or Fail, why it cannot, when it cannot.
type Differ struct{ Fail error }

var _ port.Differ = Differ{}

// Diff is from then to, as Differ writes them.
func (d Differ) Diff(from, to *port.File) (string, error) {
	if d.Fail != nil {
		return "", d.Fail
	}
	return diffSide("a", from) + diffSide("b", to), nil
}

func diffSide(name string, f *port.File) string {
	if f == nil {
		return name + ": none\n"
	}
	return name + "/" + f.Path + ": " + string(f.Data)
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

// Tagger is a template's own repository held in memory, where a release
// is tagged: its local branches and origin's, each at a commit; each
// commit's parent, a history to tell ahead from behind; and its tags,
// really made and deleted. A tag Refused names, or one there already, is
// refused as git refuses one; one Stuck names stays when deleted. A method
// Fails names by its name fails with that error, as a git that cannot be
// read does.
type Tagger struct {
	Folder  string // its top; "" for a folder in no repository
	Remote  bool   // whether it has an origin
	Default string // the branch origin's HEAD names; "" for none
	Local   map[string]string
	Origin  map[string]string
	Parents map[string]string // each commit's parent
	Tags    map[string]Tagged
	Refused map[string]error
	Stuck   map[string]error
	Fails   map[string]error // by method: Top, HasOrigin, Heads, IsAncestor
}

// Tagged is a tag a Tagger holds: the commit it names and its message.
type Tagged struct{ Commit, Message string }

var _ port.Tagger = (*Tagger)(nil)

// Top is Folder, or a *port.NoRepository when it is "".
func (t *Tagger) Top() (string, error) {
	if err := t.Fails["Top"]; err != nil {
		return "", err
	}
	if t.Folder == "" {
		return "", &port.NoRepository{Why: "not a git repository"}
	}
	return t.Folder, nil
}

// HasOrigin is Remote.
func (t *Tagger) HasOrigin() (bool, error) { return t.Remote, t.Fails["HasOrigin"] }

// DefaultBranch is Default, and false when it is "".
func (t *Tagger) DefaultBranch() (string, bool) { return t.Default, t.Default != "" }

// Heads are the branch's commits in Local and Origin.
func (t *Tagger) Heads(branch string) (string, string, error) {
	return t.Local[branch], t.Origin[branch], t.Fails["Heads"]
}

// IsAncestor is whether commit is of, or found walking of's Parents.
func (t *Tagger) IsAncestor(commit, of string) (bool, error) {
	if err := t.Fails["IsAncestor"]; err != nil {
		return false, err
	}
	for c := of; c != ""; c = t.Parents[c] {
		if c == commit {
			return true, nil
		}
	}
	return false, nil
}

// Tag makes the tag name, unless Refused names it or it is there already.
func (t *Tagger) Tag(name, commit, message string) error {
	if err := t.Refused[name]; err != nil {
		return err
	}
	if _, ok := t.Tags[name]; ok {
		return fmt.Errorf("tag '%s' already exists", name)
	}
	if t.Tags == nil {
		t.Tags = map[string]Tagged{}
	}
	t.Tags[name] = Tagged{Commit: commit, Message: message}
	return nil
}

// Untag deletes the tag name, unless Stuck names it.
func (t *Tagger) Untag(name string) error {
	if err := t.Stuck[name]; err != nil {
		return err
	}
	delete(t.Tags, name)
	return nil
}

// The modes of a fake tree's files but a plain one's (0, or 0o644): an
// executable file's, a symbolic link's (its Data the target) and a
// submodule's.
const (
	Executable = fs.FileMode(0o755)
	Link       = fs.ModeSymlink | 0o777
	Submodule  = fs.ModeIrregular
)
