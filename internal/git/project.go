package git

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Project is a made or adopted project's git repository: the port.Project
// update merges into (decision 38). It reads as a Worktree reads, and
// merges with git's own merge through its command line, as git merge
// would: the two renders are written to the project's objects as trees no
// ref or commit names (git hash-object, then git update-index and git
// write-tree on an index of its own), git merge-tree merges them into HEAD,
// the old render the base, and git read-tree takes the merged tree into
// the index and the working tree, each conflict then given its stages as
// git merge leaves them. No commit is made and HEAD never moves.
//
// Each file of a render is written to a folder of its own, Scratch, for
// git to hash, all in one run of git hash-object, with no filter of the
// project's: the blob is the rendered bytes, as new commits them. The
// working tree is written by git, so the project's own attributes and
// core.autocrlf decide its line endings, as they do for git merge.
type Project struct {
	*Worktree
	Scratch string
}

var _ port.Project = (*Project)(nil)

// OpenProject is the repository the folder dir is in, not looked at yet,
// scratch an empty folder of update's own.
func OpenProject(dir, scratch string) *Project {
	return &Project{Worktree: OpenWorktree(dir), Scratch: scratch}
}

// Committed is the file path's contents in HEAD's tree, false when it
// holds nothing there.
func (p *Project) Committed(path string) ([]byte, bool, error) {
	g := command{dir: p.top, env: readOnly}
	oid, err := g.output("rev-parse", "--verify", "--quiet", "HEAD:"+path)
	if exitCode(err) == 1 {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	data, err := g.output("cat-file", "blob", strings.TrimSpace(string(oid)))
	return data, err == nil, err
}

// Merge merges into HEAD the change from base to target, as port.Project
// says.
func (p *Project) Merge(base, target []port.File) (port.Merged, error) {
	oids, err := p.hash(append(base[:len(base):len(base)], target...))
	if err != nil {
		return port.Merged{}, err
	}
	baseTree, err := p.tree("base", base, oids[:len(base)])
	if err != nil {
		return port.Merged{}, err
	}
	targetTree, err := p.tree("target", target, oids[len(base):])
	if err != nil {
		return port.Merged{}, err
	}
	// git merge refreshes the index first, as read-tree needs: a file whose
	// stat git has not recorded since it was written is otherwise not up to
	// date, though its contents are.
	if _, err := run(p.top, "update-index", "-q", "--refresh"); err != nil {
		return port.Merged{}, err
	}
	out, err := run(p.top, "merge-tree", "-z", "--write-tree", "--merge-base="+baseTree, "HEAD", targetTree)
	if err != nil && exitCode(err) != 1 {
		return port.Merged{}, err
	}
	conflicted := err != nil
	records := strings.Split(string(out), "\x00")
	merged := records[0]
	if _, err := run(p.top, "read-tree", "-m", "-u", "HEAD", merged); err != nil {
		return port.Merged{}, err
	}
	var conflicts []port.Unmerged
	if conflicted {
		if conflicts, err = p.unmerge(records[1:]); err != nil {
			return port.Merged{}, err
		}
	}
	changes, err := p.changes(merged)
	if err != nil {
		return port.Merged{}, err
	}
	return port.Merged{Changes: changes, Conflicts: conflicts}, nil
}

// hash writes each file's contents to the project's objects, as a blob, and
// gives each one's object name, in order: one git hash-object over files
// written to Scratch, a symbolic link's target as a file's contents, as
// git stores a link.
func (p *Project) hash(files []port.File) ([]string, error) {
	dir, err := os.MkdirTemp(p.Scratch, "blobs-")
	if err != nil {
		return nil, err
	}
	var paths strings.Builder
	for i, f := range files {
		name := filepath.Join(dir, strconv.Itoa(i))
		if err := os.WriteFile(name, f.Data, 0o644); err != nil {
			return nil, err
		}
		paths.WriteString(name + "\n")
	}
	out, err := command{dir: p.top, stdin: strings.NewReader(paths.String())}.output("hash-object", "-w", "--no-filters", "--stdin-paths")
	if err != nil {
		return nil, err
	}
	oids := strings.Fields(string(out))
	if len(oids) != len(files) {
		return nil, fmt.Errorf("git hash-object named %d objects for %d files", len(oids), len(files))
	}
	return oids, nil
}

// tree writes files, each with its object of oids, as a tree of the
// project's objects, on an index of its own, named name, in Scratch, and
// gives the tree's object name.
func (p *Project) tree(name string, files []port.File, oids []string) (string, error) {
	var info bytes.Buffer
	for i, f := range files {
		fmt.Fprintf(&info, "%s %s\t%s\x00", gitModeOf(f.Mode), oids[i], f.Path)
	}
	g := command{dir: p.top, env: []string{"GIT_INDEX_FILE=" + filepath.Join(p.Scratch, name+".index")}, stdin: &info}
	if _, err := g.output("update-index", "--add", "-z", "--index-info"); err != nil {
		return "", err
	}
	g.stdin = nil
	out, err := g.output("write-tree")
	return strings.TrimSpace(string(out)), err
}

// gitModeOf is a file's mode as a tree records it: 120000 a symbolic link,
// 100755 an executable file, 100644 any other.
func gitModeOf(m fs.FileMode) string {
	switch {
	case m&fs.ModeSymlink != 0:
		return "120000"
	case m&0o111 != 0:
		return "100755"
	default:
		return "100644"
	}
}

// unmerge gives each conflicted path its stages in the index, as git merge
// leaves a conflict: info is merge-tree's conflicted file info, each record
// a mode, an object, a stage and a path, up to an empty one, and each
// path's stage 0 entry, read-tree's, is removed first. It returns the
// conflicted paths, sorted.
func (p *Project) unmerge(info []string) ([]port.Unmerged, error) {
	var removals, stages bytes.Buffer
	seen := map[string]bool{}
	var paths []string
	for _, s := range info[:slices.Index(info, "")] {
		meta, path, _ := strings.Cut(s, "\t")
		fields := strings.Fields(meta)
		if len(fields) != 3 {
			return nil, fmt.Errorf("reading the merge's conflicts: git merge-tree wrote %q", s)
		}
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
			fmt.Fprintf(&removals, "0 %s\t%s\x00", fields[1], path)
		}
		stages.WriteString(s + "\x00")
	}
	removals.Write(stages.Bytes())
	if _, err := (command{dir: p.top, stdin: &removals}).output("update-index", "-z", "--index-info"); err != nil {
		return nil, err
	}
	slices.Sort(paths)
	conflicts := make([]port.Unmerged, len(paths))
	for i, path := range paths {
		conflicts[i] = port.Unmerged{Path: path}
	}
	return conflicts, nil
}

// changes are how the tree merged changes each path from HEAD's, as git
// diff-tree names them, renames followed: each a status, then its path, a
// rename's two, the path it had first. git always writes them so.
func (p *Project) changes(merged string) ([]port.Change, error) {
	out, err := p.list("diff-tree", "-r", "-z", "-M", "--name-status", "HEAD", merged)
	if err != nil {
		return nil, err
	}
	kinds := map[byte]port.ChangeKind{'A': port.Added, 'D': port.Removed, 'R': port.Renamed}
	var changes []port.Change
	for i := 0; i < len(out); i++ {
		kind, ok := kinds[out[i][0]]
		if !ok {
			kind = port.Modified
		}
		c := port.Change{Kind: kind}
		if kind == port.Renamed {
			i++
			c.From = out[i]
		}
		i++
		c.Path = out[i]
		changes = append(changes, c)
	}
	slices.SortFunc(changes, func(a, b port.Change) int { return strings.Compare(a.Path, b.Path) })
	return changes, nil
}

// MergeVersion is the oldest git whose merge-tree takes a tree for its
// merge base and for what it merges (git merge-tree --write-tree
// --merge-base=<tree> <commit> <tree>), as update merges two renders no
// commit holds: 2.45.0, tried against 2.44.0, which refuses the tree.
var MergeVersion = [2]int{2, 45}

// Supports is nil when the git itos-template runs is MergeVersion or newer,
// a *TooOld otherwise, or a *Missing when it cannot be run.
func Supports() error {
	return supports(run("", "version"))
}

// supports is nil when out, what git version wrote, names MergeVersion or a
// newer one, a *TooOld otherwise; err, when git version failed.
func supports(out []byte, err error) error {
	if err != nil {
		return err
	}
	line := strings.TrimSpace(string(out))
	if slices.Compare(version(line), MergeVersion[:]) < 0 {
		return &TooOld{Version: line, Need: fmt.Sprintf("%d.%d", MergeVersion[0], MergeVersion[1])}
	}
	return nil
}

var versionLine = regexp.MustCompile(`^git version (\d+)\.(\d+)`)

// version reads the major and minor version of git version's line: git
// version 2.45.0, or 2.45.1.windows.1, or 2.39.3 (Apple Git-146); none, so
// older than any, for a line it cannot read.
func version(line string) []int {
	m := versionLine.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return []int{major, minor}
}
