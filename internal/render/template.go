// Package render makes a project's files from a template's branches: the
// template cloned by git (decision 9), the stack's branch merged with the
// chosen features' branches by git (decision 2), and each literal replaced
// by its answer in the merged tree's file contents and names (decision 1).
//
// The template is cloned bare and merged with git merge-tree, so no working
// tree is checked out and nothing of the person's git config (core.autocrlf,
// filters) changes a byte: the render is read from the merged tree's blobs,
// and is the same on every system and every machine, as an update needs it
// (decision 3).
package render

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/donvargax/itos-template/internal/git"
)

// Template is a template cloned bare into a folder of its own.
type Template struct {
	Dir string
}

// UnreachableError is a template git could not clone.
type UnreachableError struct {
	Name string
	Err  error
}

func (e *UnreachableError) Error() string {
	return fmt.Sprintf("git cannot reach the template %s: %v", e.Name, e.Err)
}

func (e *UnreachableError) Unwrap() error { return e.Err }

// Clone clones the template name, anything git clone takes, bare into dir,
// which must not exist. A failure is an *UnreachableError, or a *git.Error
// with Missing when git cannot be run.
func Clone(name, dir string) (*Template, error) {
	_, err := git.Run("", "clone", "--bare", "--quiet", "--", name, dir)
	var e *git.Error
	if errors.As(err, &e) && e.Missing {
		return nil, err
	}
	if err != nil {
		return nil, &UnreachableError{Name: name, Err: err}
	}
	return &Template{Dir: dir}, nil
}

func (t *Template) git(args ...string) (string, error) {
	out, err := git.Run(t.Dir, args...)
	return strings.TrimSpace(string(out)), err
}

// DefaultBranch is the branch the template's HEAD names: its root branch.
func (t *Template) DefaultBranch() (string, error) {
	return t.git("symbolic-ref", "--short", "HEAD")
}

// Commit is the commit a branch is at, and false when the template has no
// such branch.
func (t *Template) Commit(branch string) (string, bool, error) {
	sha, err := t.git("rev-parse", "--verify", "--quiet", "refs/heads/"+branch+"^{commit}")
	if err != nil {
		if git.Code(err) == 1 {
			return "", false, nil
		}
		return "", false, err
	}
	return sha, true, nil
}

// File is a file's contents at a commit, and false when it has no such
// file.
func (t *Template) File(commit, path string) ([]byte, bool, error) {
	out, err := git.Run(t.Dir, "cat-file", "blob", commit+":"+path)
	if err != nil {
		if git.Code(err) == 128 {
			return nil, false, nil
		}
		return nil, false, err
	}
	return out, true, nil
}

// ConflictError is a merge of two of the template's branches that git
// leaves with conflicts: a defect of the template, never the person's.
type ConflictError struct {
	Into, Branch string
	Paths        []string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("merging the template's branch %s into %s leaves conflicts in %s: the template's branches must merge cleanly; merge them in the template and resolve them there",
		e.Branch, e.Into, strings.Join(e.Paths, ", "))
}

// identity is who the merge commits a render makes are by: commits of the
// clone, never the made project's.
var identity = []string{
	"GIT_AUTHOR_NAME=itos-template", "GIT_AUTHOR_EMAIL=itos-template@localhost",
	"GIT_COMMITTER_NAME=itos-template", "GIT_COMMITTER_EMAIL=itos-template@localhost",
}

// Merge merges each of branches, in order, into the commit base, as git
// merge does, and returns the merged tree. Each merge after the first is
// made from a commit of the one before, so git finds each merge's base as it
// would in a working tree. A merge with conflicts is a *ConflictError.
func (t *Template) Merge(base, baseName string, branches []string) (string, error) {
	commit := base
	tree, err := t.git("rev-parse", base+"^{tree}")
	if err != nil {
		return "", err
	}
	into := baseName
	for _, branch := range branches {
		out, err := git.Run(t.Dir, "merge-tree", "--write-tree", "--name-only", "--no-messages", commit, "refs/heads/"+branch)
		if git.Code(err) == 1 {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			return "", &ConflictError{Into: into, Branch: branch, Paths: lines[1:]}
		}
		if err != nil {
			return "", err
		}
		tree, _, _ = strings.Cut(strings.TrimSpace(string(out)), "\n")
		c := git.Command{Dir: t.Dir, Env: identity}
		next, err := c.Output("commit-tree", tree, "-p", commit, "-p", "refs/heads/"+branch, "-m", "Merge "+branch)
		if err != nil {
			return "", err
		}
		commit = strings.TrimSpace(string(next))
		into += " + " + branch
	}
	return tree, nil
}

// Entry is a file of a tree: its mode as git records it (100644, 100755,
// 120000 for a symbolic link, 160000 for a submodule), its object and its
// path, with /.
type Entry struct {
	Mode string
	OID  string
	Path string
}

// Executable is whether git records the entry as an executable file.
func (e Entry) Executable() bool { return e.Mode == "100755" }

// Link is whether the entry is a symbolic link, its contents its target.
func (e Entry) Link() bool { return e.Mode == "120000" }

// Submodule is whether the entry is a submodule's commit.
func (e Entry) Submodule() bool { return e.Mode == "160000" }

// Entries are every file of tree, in git's order.
func (t *Template) Entries(tree string) ([]Entry, error) {
	out, err := git.Run(t.Dir, "ls-tree", "-r", "-z", "--full-tree", tree)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, record := range strings.Split(string(out), "\x00") {
		if record == "" {
			continue
		}
		meta, path, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("reading the template's files: git ls-tree wrote %q", record)
		}
		entries = append(entries, Entry{Mode: fields[0], OID: fields[2], Path: path})
	}
	return entries, nil
}

// Blobs calls each with the contents of each object of oids, in order, read
// by one git cat-file.
func (t *Template) Blobs(oids []string, each func(i int, data []byte) error) error {
	if len(oids) == 0 {
		return nil
	}
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		c := git.Command{Dir: t.Dir, Stdin: strings.NewReader(strings.Join(oids, "\n") + "\n")}
		err := c.Stream(pw, "cat-file", "--batch")
		_ = pw.CloseWithError(err)
		done <- err
	}()
	r := bufio.NewReader(pr)
	var readErr error
	for i := range oids {
		if readErr = readBlob(r, oids[i], func(data []byte) error { return each(i, data) }); readErr != nil {
			break
		}
	}
	_ = pr.CloseWithError(errors.New("read"))
	if err := <-done; err != nil && readErr == nil {
		return err
	}
	return readErr
}

func readBlob(r *bufio.Reader, oid string, each func([]byte) error) error {
	header, err := r.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading the object %s: %w", oid, err)
	}
	fields := strings.Fields(header)
	if len(fields) != 3 || fields[1] != "blob" {
		return fmt.Errorf("reading the object %s: git cat-file wrote %q", oid, strings.TrimSpace(header))
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil {
		return fmt.Errorf("reading the object %s: %w", oid, err)
	}
	data := make([]byte, size+1)
	if _, err := io.ReadFull(r, data); err != nil {
		return fmt.Errorf("reading the object %s: %w", oid, err)
	}
	return each(data[:size])
}

// Remove removes the clone.
func (t *Template) Remove() error { return os.RemoveAll(t.Dir) }

// isText is whether data is a text file's contents, as git tells them: no
// NUL in its first 8000 bytes.
func isText(data []byte) bool {
	return !bytes.Contains(data[:min(len(data), 8000)], []byte{0})
}

// osPath is a tree's path, with /, as a path of this system under dir.
func osPath(dir, p string) string { return filepath.Join(dir, filepath.FromSlash(p)) }
