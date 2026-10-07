package git

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Repo is a template cloned bare into a folder of its own: the template's
// port.Repository.
//
// The template is cloned bare and merged with git merge-tree, so no working
// tree is checked out and nothing of the person's git config (core.autocrlf,
// filters) changes a byte: a render is read from the merged tree's blobs,
// and is the same on every system and every machine, as an update needs it
// (decision 3).
type Repo struct {
	dir string
}

var _ port.Repository = (*Repo)(nil)

// Clone clones the template name, anything git clone takes, bare into dir,
// which must not exist. A failure is an *Unreachable, or a *Missing when git
// cannot be run.
func Clone(name, dir string) (*Repo, error) {
	_, err := run("", "clone", "--bare", "--quiet", "--", name, dir)
	var failed *Failed
	if errors.As(err, &failed) {
		return nil, &Unreachable{Name: name, Err: failed}
	}
	if err != nil {
		return nil, err
	}
	return &Repo{dir: dir}, nil
}

func (r *Repo) git(args ...string) (string, error) {
	out, err := run(r.dir, args...)
	return strings.TrimSpace(string(out)), err
}

// DefaultBranch is the branch the template's HEAD names: its root branch.
func (r *Repo) DefaultBranch() (string, error) {
	return r.git("symbolic-ref", "--short", "HEAD")
}

// Commit is the commit a branch is at, and false when the template has no
// such branch.
func (r *Repo) Commit(branch string) (string, bool, error) {
	sha, err := r.git("rev-parse", "--verify", "--quiet", "refs/heads/"+branch+"^{commit}")
	if err != nil {
		if exitCode(err) == 1 {
			return "", false, nil
		}
		return "", false, err
	}
	return sha, true, nil
}

// File is a file's contents at a commit, and false when it has no such
// file.
func (r *Repo) File(commit, path string) ([]byte, bool, error) {
	out, err := run(r.dir, "cat-file", "blob", commit+":"+path)
	if err != nil {
		if exitCode(err) == 128 {
			return nil, false, nil
		}
		return nil, false, err
	}
	return out, true, nil
}

// identity is who the merge commits a render makes are by: commits of the
// clone, never the made project's.
var identity = []string{
	"GIT_AUTHOR_NAME=itos-template", "GIT_AUTHOR_EMAIL=itos-template@localhost",
	"GIT_COMMITTER_NAME=itos-template", "GIT_COMMITTER_EMAIL=itos-template@localhost",
}

// Merge merges each of branches after the first, in order, into the first,
// as git merge does, and returns the merged tree's files, every file's
// contents read by one git cat-file. Each merge after the first is made
// from a commit of the one before, so git finds each merge's base as it
// would in a working tree. A merge with conflicts is a *port.Conflict.
func (r *Repo) Merge(branches []string) ([]port.File, error) {
	commit := "refs/heads/" + branches[0]
	tree, err := r.git("rev-parse", commit+"^{tree}")
	if err != nil {
		return nil, err
	}
	for _, branch := range branches[1:] {
		out, err := run(r.dir, "merge-tree", "--write-tree", "--name-only", "--no-messages", commit, "refs/heads/"+branch)
		if exitCode(err) == 1 {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			return nil, &port.Conflict{Branch: branch, Paths: lines[1:]}
		}
		if err != nil {
			return nil, err
		}
		tree, _, _ = strings.Cut(strings.TrimSpace(string(out)), "\n")
		next, err := command{dir: r.dir, env: identity}.output("commit-tree", tree, "-p", commit, "-p", "refs/heads/"+branch, "-m", "Merge "+branch)
		if err != nil {
			return nil, err
		}
		commit = strings.TrimSpace(string(next))
	}
	return r.files(tree)
}

// files are every file of tree, in git's order, each with its contents but
// a submodule's, which the clone does not hold.
func (r *Repo) files(tree string) ([]port.File, error) {
	out, err := run(r.dir, "ls-tree", "-r", "-z", "--full-tree", tree)
	if err != nil {
		return nil, err
	}
	var files []port.File
	var oids []string
	var blobs []int // the index in files of each of oids
	for _, record := range strings.Split(string(out), "\x00") {
		if record == "" {
			continue
		}
		meta, path, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("reading the template's files: git ls-tree wrote %q", record)
		}
		f := port.File{Path: path, Mode: mode(fields[0])}
		if f.Mode != fs.ModeIrregular {
			oids = append(oids, fields[2])
			blobs = append(blobs, len(files))
		}
		files = append(files, f)
	}
	err = r.blobs(oids, func(i int, data []byte) error {
		files[blobs[i]].Data = data
		return nil
	})
	return files, err
}

// mode is the port's mode of a tree entry's, as git records it: 100755 an
// executable file, 120000 a symbolic link, 160000 a submodule's commit, any
// other a file.
func mode(git string) fs.FileMode {
	switch git {
	case "100755":
		return 0o755
	case "120000":
		return fs.ModeSymlink | 0o777
	case "160000":
		return fs.ModeIrregular
	default:
		return 0o644
	}
}

// blobs calls each with the contents of each object of oids, in order, read
// by one git cat-file.
func (r *Repo) blobs(oids []string, each func(i int, data []byte) error) error {
	if len(oids) == 0 {
		return nil
	}
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		c := command{dir: r.dir, stdin: strings.NewReader(strings.Join(oids, "\n") + "\n")}
		err := c.stream(pw, "cat-file", "--batch")
		_ = pw.CloseWithError(err)
		done <- err
	}()
	br := bufio.NewReader(pr)
	var readErr error
	for i := range oids {
		if readErr = readBlob(br, oids[i], func(data []byte) error { return each(i, data) }); readErr != nil {
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
