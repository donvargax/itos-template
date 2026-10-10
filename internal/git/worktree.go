package git

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Worktree is an existing project's git repository, read at its top as git
// sees it: the port.Checkout adopt reads. Nothing in it is written but the
// one new file Create makes: no git command it runs takes a lock or writes
// the index (GIT_OPTIONAL_LOCKS=0 keeps git status from refreshing it), a
// ref or an object.
type Worktree struct {
	dir string // the folder it was opened in
	top string // its top, once Look found it
}

var _ port.Checkout = (*Worktree)(nil)

// OpenWorktree is the repository the folder dir is in, not looked at yet.
func OpenWorktree(dir string) *Worktree { return &Worktree{dir: dir} }

// readOnly is the environment of a git command that must write nothing to
// the repository it reads.
var readOnly = []string{"GIT_OPTIONAL_LOCKS=0"}

// Look says what the folder the Worktree was opened in is, as git says it:
// in no repository when git rev-parse refuses it, with git's reason; a bare
// one, or a .git folder, when git gives it no top. Its changes are what git
// status lists, renames as two paths, untracked files it does not ignore
// each named.
func (w *Worktree) Look() (port.CheckoutState, error) {
	out, err := run(w.dir, "rev-parse", "--is-bare-repository")
	var failed *Failed
	if errors.As(err, &failed) {
		return port.CheckoutState{Why: failed.Error()}, nil
	}
	if err != nil {
		return port.CheckoutState{}, err
	}
	state := port.CheckoutState{Repository: true}
	if strings.TrimSpace(string(out)) == "true" {
		return state, nil
	}
	top, err := run(w.dir, "rev-parse", "--show-toplevel")
	if errors.As(err, &failed) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	state.WorkTree, w.top = true, strings.TrimSpace(string(top))
	state.Top = w.top
	if _, err := run(w.top, "rev-parse", "--verify", "--quiet", "HEAD^{commit}"); err == nil {
		state.Committed = true
	} else if exitCode(err) != 1 {
		return state, err
	}
	status, err := command{dir: w.top, env: readOnly}.output("status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=all")
	if err != nil {
		return state, err
	}
	// Each entry is its two status letters, a space and its path.
	for _, entry := range strings.Split(string(status), "\x00") {
		if entry != "" {
			state.Changed = append(state.Changed, entry[3:])
		}
	}
	return state, nil
}

// Files are the working tree's files as port.Checkout says: the index's
// entries (git ls-files), each read from the index's blob, all by one git
// cat-file, unless git diff-files says the working tree holds it otherwise,
// then from the working tree, or left out when it holds none; the
// untracked files git does not ignore (git ls-files --others
// --exclude-standard) and each of also it holds, from the working tree.
func (w *Worktree) Files(also []string) ([]port.File, error) {
	index, err := w.list("ls-files", "-z", "--stage")
	if err != nil {
		return nil, err
	}
	changed, err := w.list("diff-files", "-z", "--name-only")
	if err != nil {
		return nil, err
	}
	differs := map[string]bool{}
	for _, p := range changed {
		differs[p] = true
	}
	seen := map[string]bool{}
	var files []port.File
	var oids []string
	var blobs []int // the index in files of each of oids
	for _, entry := range index {
		meta, p, _ := strings.Cut(entry, "\t")
		fields := strings.Fields(meta)
		if len(fields) != 3 {
			return nil, fmt.Errorf("reading the project's index: git ls-files wrote %q", entry)
		}
		if seen[p] {
			continue // a conflict's other stages
		}
		seen[p] = true
		m := mode(fields[0])
		switch {
		case m == fs.ModeIrregular:
			files = append(files, port.File{Path: p, Mode: m})
		case differs[p]:
			f, ok, err := w.read(p)
			if err != nil {
				return nil, err
			}
			if ok {
				f.Mode = m
				files = append(files, f)
			}
		default:
			oids = append(oids, fields[1])
			blobs = append(blobs, len(files))
			files = append(files, port.File{Path: p, Mode: m})
		}
	}
	if err := readBlobs(w.top, oids, func(i int, data []byte) error {
		files[blobs[i]].Data = data
		return nil
	}); err != nil {
		return nil, err
	}
	untracked, err := w.list("ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	for _, p := range append(untracked, also...) {
		p = strings.TrimSuffix(p, "/") // a repository of its own, untracked
		if seen[p] {
			continue
		}
		seen[p] = true
		f, ok, err := w.read(p)
		if err != nil {
			return nil, err
		}
		if ok {
			files = append(files, f)
		}
	}
	return files, nil
}

// list is what git, run at the top, writes, split at each NUL.
func (w *Worktree) list(args ...string) ([]string, error) {
	out, err := command{dir: w.top, env: readOnly}.output(args...)
	if err != nil {
		return nil, err
	}
	var list []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			list = append(list, p)
		}
	}
	return list, nil
}

// read is the working tree's file p, with /, as the file system holds it:
// a link's target, a file's contents, its mode executable when any execute
// bit is set, a folder (a repository of its own) as a submodule is; false
// when there is nothing at p.
func (w *Worktree) read(p string) (port.File, bool, error) {
	full := w.path(p)
	info, err := os.Lstat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return port.File{}, false, nil
	}
	if err != nil {
		return port.File{}, false, &Unreadable{Path: p, Err: err}
	}
	f := port.File{Path: p, Mode: 0o644}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		f.Mode = fs.ModeSymlink | 0o777
		target, err := os.Readlink(full)
		if err != nil {
			return port.File{}, false, &Unreadable{Path: p, Err: err}
		}
		f.Data = []byte(filepath.ToSlash(target))
	case info.IsDir():
		f.Mode = fs.ModeIrregular
	default:
		if info.Mode()&0o111 != 0 {
			f.Mode = 0o755
		}
		if f.Data, err = os.ReadFile(full); err != nil {
			return port.File{}, false, &Unreadable{Path: p, Err: err}
		}
	}
	return f, true, nil
}

// path is p, with /, at the top, as the system writes a path.
func (w *Worktree) path(p string) string {
	return filepath.Join(filepath.FromSlash(w.top), filepath.FromSlash(path.Clean(p)))
}

// Holds is whether the top holds anything at p.
func (w *Worktree) Holds(p string) (bool, error) {
	_, err := os.Lstat(w.path(p))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, &Unreadable{Path: p, Err: err}
	}
	return true, nil
}

// Create writes data as the new file p at the top, made only when there is
// none (os.O_EXCL): an *Unwritable wrapping fs.ErrExist when there is one. A
// write that fails removes the file it made.
func (w *Worktree) Create(p string, data []byte) error {
	full := w.path(p)
	f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return &Unwritable{Path: p, Err: err}
	}
	_, wrote := f.Write(data)
	if err := errors.Join(wrote, f.Close()); err != nil {
		_ = os.Remove(full)
		return &Unwritable{Path: p, Err: err}
	}
	return nil
}

// Differ is git's own diff of two sides of a file, the port.Differ: git
// diff --no-index over two folders made for it in Dir, a/ holding the side
// it changes from and b/ the side it changes to, so it names them as git
// diff names a commit's sides. Each side is written as a file of mode 0644
// whatever its mode, which the difference names apart, and nothing of the
// person's git config changes what it shows: no colour, no external diff
// or text conversion, no line ending converted.
type Differ struct{ Dir string }

var _ port.Differ = Differ{}

// Diff is git's diff of from to to, either nil when that side has none.
func (d Differ) Diff(from, to *port.File) (string, error) {
	dir, err := os.MkdirTemp(d.Dir, "diff-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	for _, s := range []struct {
		name string
		f    *port.File
	}{{"a", from}, {"b", to}} {
		folder := filepath.Join(dir, s.name)
		if err := os.MkdirAll(folder, 0o755); err != nil {
			return "", err
		}
		if s.f == nil {
			continue
		}
		file := filepath.Join(folder, filepath.FromSlash(s.f.Path))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(file, s.f.Data, 0o644); err != nil {
			return "", err
		}
	}
	out, err := command{dir: dir}.output("-c", "core.autocrlf=false", "diff", "--no-index", "--no-prefix",
		"--no-color", "--no-ext-diff", "--no-textconv", "--", "a", "b")
	if exitCode(err) == 1 {
		return string(out), nil
	}
	return string(out), err
}
