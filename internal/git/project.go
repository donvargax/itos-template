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
// update merges into (decision 38), and the port.Merger adopt --merge
// merges its baseline into, over an empty base. It reads as a Worktree reads, and
// merges with git's own merge through its command line, as git merge
// would: the two renders are written to the project's objects as trees no
// ref or commit names (git hash-object, then git update-index and git
// write-tree on an index of its own), git merge-tree merges them into HEAD,
// the old render the base, and git read-tree takes the merged tree into
// the index and the working tree, each conflict then given its stages as
// git merge leaves them. No commit is made and HEAD never moves.
//
// git merge-tree labels a conflict's markers with its arguments, the
// renders' tree ids, and takes no label of its own (slice-15). So each
// conflicted file it merged by its contents is merged again by git
// merge-file, over the same three stages, with the histogram diff
// merge-tree's merge uses and the person's merge.conflictStyle, labelled
// HEAD and as the update names the renders, before git read-tree takes the
// merged tree; git checks out every file, so its line endings are what
// they were. A file is merged again only where git merge-file, given
// merge-tree's own labels, writes merge-tree's very file: one a merge
// driver or a conflict-marker-size attribute decided, or a rename's
// markers, which name its paths too, is left as merge-tree wrote it, as is
// a binary file, which merge-tree leaves as HEAD has it. The index stages
// are merge-tree's, whatever the file.
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

// CanMerge is nil when the git itos-template runs can merge as Merge
// does (Supports).
func (p *Project) CanMerge() error { return Supports() }

// Merge merges into HEAD the change from base to target, as port.Project
// says.
func (p *Project) Merge(base, target []port.File, labels port.Labels) (port.Merged, error) {
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
	var stages []stage
	if conflicted {
		if stages, err = stagesOf(records[1:]); err != nil {
			return port.Merged{}, err
		}
		given := [3]string{baseTree, "HEAD", targetTree}
		if merged, err = p.relabel(merged, stages, given, [3]string{labels.Base, "HEAD", labels.Target}); err != nil {
			return port.Merged{}, err
		}
	}
	if _, err := run(p.top, "read-tree", "-m", "-u", "HEAD", merged); err != nil {
		return port.Merged{}, err
	}
	var conflicts []port.Unmerged
	if conflicted {
		if conflicts, err = p.unmerge(stages); err != nil {
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

// stage is a conflicted path as git merge-tree's conflicted file info
// gives it: its records, each a mode, an object, a stage and the path, and
// the object of each stage, base, HEAD and target, "" for a side with none.
type stage struct {
	path    string
	records []string
	objects [3]string
}

// stagesOf reads info, merge-tree's conflicted file info up to an empty
// record, as each path's stages, in its order.
func stagesOf(info []string) ([]stage, error) {
	var stages []stage
	at := map[string]int{}
	for _, s := range info[:slices.Index(info, "")] {
		meta, path, _ := strings.Cut(s, "\t")
		fields := strings.Fields(meta)
		n := 0
		if len(fields) == 3 {
			n, _ = strconv.Atoi(fields[2])
		}
		if n < 1 || n > 3 {
			return nil, fmt.Errorf("reading the merge's conflicts: git merge-tree wrote %q", s)
		}
		i, ok := at[path]
		if !ok {
			i, at[path] = len(stages), len(stages)
			stages = append(stages, stage{path: path})
		}
		stages[i].records = append(stages[i].records, s)
		stages[i].objects[n-1] = fields[1]
	}
	return stages, nil
}

// relabel is the tree merged, merge-tree's, each conflicted file of it
// with a HEAD and a target side merged again by git merge-file, labelled
// labels (base, HEAD and target), where merging it again with the labels
// merge-tree was given, given, writes the very file the tree holds. A file
// both sides added is merged over an empty base, as merge-tree merges it.
// The tree is written on an index of its own, in Scratch.
func (p *Project) relabel(merged string, stages []stage, given, labels [3]string) (string, error) {
	g := command{dir: p.top, env: []string{"GIT_INDEX_FILE=" + filepath.Join(p.Scratch, "merged.index")}}
	if _, err := g.output("read-tree", merged); err != nil {
		return "", err
	}
	list, err := g.output("ls-files", "--stage", "-z")
	if err != nil {
		return "", err
	}
	// Each entry is a mode, an object, a stage and the path; held keeps
	// each path's mode and object, none for a path the tree does not hold.
	held := map[string][2]string{}
	for _, entry := range strings.Split(string(list), "\x00") {
		meta, path, _ := strings.Cut(entry, "\t")
		if fields := strings.Fields(meta); len(fields) == 3 {
			held[path] = [2]string{fields[0], fields[1]}
		}
	}
	var again bytes.Buffer
	for _, s := range stages {
		entry, sides := held[s.path], s.objects
		// A file merge-tree left as HEAD has it, a binary one, holds no
		// markers, nor does one that lacks a side.
		if sides[1] == "" || sides[2] == "" || entry[1] == sides[1] {
			continue
		}
		if sides[0] == "" {
			if sides[0], err = p.emptyBlob(); err != nil {
				return "", err
			}
		}
		same, err := p.mergeFile(sides, given)
		if err != nil {
			return "", err
		}
		if same != entry[1] {
			continue
		}
		relabelled, err := p.mergeFile(sides, labels)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&again, "%s %s\t%s\x00", entry[0], relabelled, s.path)
	}
	g.stdin = &again
	if _, err := g.output("update-index", "-z", "--index-info"); err != nil {
		return "", err
	}
	g.stdin = nil
	out, err := g.output("write-tree")
	return strings.TrimSpace(string(out)), err
}

// mergeFile is the object git merge-file writes merging sides, the objects
// of the base, HEAD and the target, labelled labels, with the histogram
// diff merge-tree's merge uses and the person's merge.conflictStyle,
// conflicts and all.
func (p *Project) mergeFile(sides, labels [3]string) (string, error) {
	out, err := run(p.top, "merge-file", "--object-id", "--diff-algorithm=histogram",
		"-L", labels[1], "-L", labels[0], "-L", labels[2], sides[1], sides[0], sides[2])
	// It exits with the number of conflicts it left, so it failed only
	// when it wrote no object.
	if oid := strings.TrimSpace(string(out)); oid != "" {
		return oid, nil
	}
	return "", err
}

// emptyBlob is the empty file's object, written to the project's objects.
func (p *Project) emptyBlob() (string, error) {
	out, err := command{dir: p.top, stdin: strings.NewReader("")}.output("hash-object", "-w", "--stdin")
	return strings.TrimSpace(string(out)), err
}

// unmerge gives each conflicted path its stages in the index, as git merge
// leaves a conflict, each path's stage 0 entry, read-tree's, removed first.
// It returns the conflicted paths, sorted.
func (p *Project) unmerge(stages []stage) ([]port.Unmerged, error) {
	var removals, records bytes.Buffer
	conflicts := make([]port.Unmerged, len(stages))
	for i, s := range stages {
		fmt.Fprintf(&removals, "0 %s\t%s\x00", strings.Fields(s.records[0])[1], s.path)
		for _, r := range s.records {
			records.WriteString(r + "\x00")
		}
		conflicts[i] = port.Unmerged{Path: s.path}
	}
	removals.Write(records.Bytes())
	if _, err := (command{dir: p.top, stdin: &removals}).output("update-index", "-z", "--index-info"); err != nil {
		return nil, err
	}
	slices.SortFunc(conflicts, func(a, b port.Unmerged) int { return strings.Compare(a.Path, b.Path) })
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
// commit holds: 2.45.0, tried against 2.44.0, which refuses the tree. What
// labels a conflict, git merge-file's --object-id (2.43) and
// --diff-algorithm (2.44), is older; tried on 2.45.4.
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
