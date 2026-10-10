package git

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/template/port"
)

// What adopt's scenarios do not reach of the project's working tree: their
// fixtures hold no symbolic link, submodule, untracked executable, nested
// repository or unmerged index entry, and no record appears between adopt
// looking for one and writing it.

// project is a repository with a commit: a.txt, run.sh recorded
// executable, a submodule's gitlink at sub, uninitialised, and a .gitignore
// ignoring ignored.txt.
func project(t *testing.T) string {
	t.Helper()
	isolate(t)
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "a.txt", "a\n", 0o644)
	write(t, dir, "run.sh", "#!/bin/sh\n", 0o755)
	write(t, dir, ".gitignore", "ignored.txt\n", 0o644)
	git(t, dir, "add", "-A")
	git(t, dir, "update-index", "--chmod=+x", "--", "run.sh")
	git(t, dir, "commit", "-q", "-m", "start")
	head := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))
	git(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+head+",sub")
	git(t, dir, "commit", "-q", "-m", "add a submodule")
	// An uninitialised submodule is an empty folder, as a clone leaves one.
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func looked(t *testing.T, dir string) (*Worktree, port.CheckoutState) {
	t.Helper()
	w := OpenWorktree(dir)
	state, err := w.Look()
	if err != nil {
		t.Fatal(err)
	}
	return w, state
}

func filesByPath(t *testing.T, w *Worktree, also ...string) map[string]port.File {
	t.Helper()
	files, err := w.Files(also)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]port.File{}
	for _, f := range files {
		if _, twice := got[f.Path]; twice {
			t.Errorf("Files gave %s twice", f.Path)
		}
		got[f.Path] = f
	}
	return got
}

func TestFilesReadTheWorkingTreeAsGitSeesIt(t *testing.T) {
	dir := project(t)
	write(t, dir, "a.txt", "changed\n", 0o644)
	write(t, dir, "new.sh", "#!/bin/sh\n", 0o755)
	write(t, dir, "new.txt", "new\n", 0o644)
	write(t, dir, "ignored.txt", "secret\n", 0o644)
	w, _ := looked(t, dir)
	got := filesByPath(t, w, "new.txt", "ignored.txt", "nowhere.txt")
	if f := got["a.txt"]; string(f.Data) != "changed\n" || f.Mode != 0o644 {
		t.Errorf("a changed file is %v %q, not the working tree's", f.Mode, f.Data)
	}
	if f := got["run.sh"]; string(f.Data) != "#!/bin/sh\n" || f.Mode != 0o755 {
		t.Errorf("an executable is %v %q", f.Mode, f.Data)
	}
	if f := got["sub"]; f.Mode != fs.ModeIrregular || f.Data != nil {
		t.Errorf("the submodule is %v %q", f.Mode, f.Data)
	}
	if f := got["new.txt"]; f.Mode != 0o644 || string(f.Data) != "new\n" {
		t.Errorf("an untracked file is %v %q", f.Mode, f.Data)
	}
	if f := got["ignored.txt"]; string(f.Data) != "secret\n" {
		t.Errorf("an ignored file also names is %q", f.Data)
	}
	if _, ok := got["nowhere.txt"]; ok {
		t.Error("a file the working tree does not hold is listed")
	}
	if runtime.GOOS != "windows" {
		if f := got["new.sh"]; f.Mode != 0o755 {
			t.Errorf("an untracked executable is %v", f.Mode)
		}
	}
	if _, ok := filesByPath(t, w)["ignored.txt"]; ok {
		t.Error("an ignored file is listed when also does not name it")
	}
}

// A file the working tree removed is left out, as is one an unmerged entry
// holds twice in the index, listed once.
func TestFilesLeaveOutARemovedFileAndListAnUnmergedOneOnce(t *testing.T) {
	dir := project(t)
	if err := os.Remove(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatal(err)
	}
	oid := strings.TrimSpace(gitIn(t, dir, "both\n", "hash-object", "-w", "--stdin"))
	gitIn(t, dir, "0 0000000000000000000000000000000000000000\tc.txt\n100644 "+oid+" 2\tc.txt\n100644 "+oid+" 3\tc.txt\n", "update-index", "--index-info")
	write(t, dir, "c.txt", "both\n", 0o644)
	w, _ := looked(t, dir)
	got := filesByPath(t, w)
	if _, ok := got["a.txt"]; ok {
		t.Error("a removed file is listed")
	}
	if f, ok := got["c.txt"]; !ok || string(f.Data) != "both\n" {
		t.Errorf("an unmerged file is %q", f.Data)
	}
}

func TestFilesReadALinkAndANestedRepository(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need a privilege on windows")
	}
	dir := project(t)
	if err := os.Symlink("a.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "nested")
	git(t, dir, "init", "-q", nested)
	w, _ := looked(t, dir)
	got := filesByPath(t, w)
	if f := got["link"]; f.Mode != fs.ModeSymlink|0o777 || string(f.Data) != "a.txt" {
		t.Errorf("the link is %v %q", f.Mode, f.Data)
	}
	if f := got["nested"]; f.Mode != fs.ModeIrregular {
		t.Errorf("the nested repository is %v", f.Mode)
	}
}

func TestLookNamesEveryChangeAndNoTopInAGitFolder(t *testing.T) {
	dir := project(t)
	write(t, dir, "a.txt", "changed\n", 0o644)
	write(t, dir, "run.sh", "changed\n", 0o755)
	git(t, dir, "add", "run.sh")
	write(t, dir, "new.txt", "new\n", 0o644)
	write(t, dir, "ignored.txt", "secret\n", 0o644)
	_, state := looked(t, dir)
	if !state.Repository || !state.WorkTree || !state.Committed {
		t.Errorf("the project is %+v", state)
	}
	if got := slices.Sorted(slices.Values(state.Changed)); !slices.Equal(got, []string{"a.txt", "new.txt", "run.sh"}) {
		t.Errorf("the changes are %q", got)
	}
	if _, state := looked(t, filepath.Join(dir, ".git")); !state.Repository || state.WorkTree {
		t.Errorf("a .git folder is %+v", state)
	}
}

func TestCreateWritesANewFileAndNeverReplacesOne(t *testing.T) {
	dir := project(t)
	w, _ := looked(t, dir)
	if held, err := w.Holds("rec.yaml"); err != nil || held {
		t.Errorf("Holds of no file = %v, %v", held, err)
	}
	if err := w.Create("rec.yaml", []byte("one\n")); err != nil {
		t.Fatal(err)
	}
	if held, err := w.Holds("rec.yaml"); err != nil || !held {
		t.Errorf("Holds of the file made = %v, %v", held, err)
	}
	err := w.Create("rec.yaml", []byte("two\n"))
	var unwritable *Unwritable
	if !errors.As(err, &unwritable) || !errors.Is(err, fs.ErrExist) {
		t.Errorf("Create over a file gave %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "rec.yaml")); string(data) != "one\n" {
		t.Errorf("the file holds %q", data)
	}
}

// git diff shows a side with no file as git shows a file added or deleted.
func TestDifferShowsAFileAddedAndOneUnchanged(t *testing.T) {
	isolate(t)
	d := Differ{Dir: t.TempDir()}
	out, err := d.Diff(nil, &port.File{Path: "docs/new.md", Data: []byte("new\n")})
	if err != nil || !strings.Contains(out, "--- /dev/null\n+++ b/docs/new.md\n") || !strings.Contains(out, "\n+new\n") {
		t.Errorf("the diff of a file added is %q, %v", out, err)
	}
	same := &port.File{Path: "x", Data: []byte("x\n")}
	if out, err := d.Diff(same, same); err != nil || out != "" {
		t.Errorf("the diff of a file unchanged is %q, %v", out, err)
	}
}

// A path through a file is one the system refuses to look at, not one that
// is missing: an *Unreadable naming it. Windows reads it as missing.
func TestFilesAndHoldsNameAPathTheSystemCannotRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows reads a path through a file as missing")
	}
	dir := project(t)
	w, _ := looked(t, dir)
	var unreadable *Unreadable
	if _, err := w.Files([]string{"a.txt/x"}); !errors.As(err, &unreadable) || unreadable.Path != "a.txt/x" {
		t.Errorf("Files of a path through a file gave %v", err)
	}
	if held, err := w.Holds("a.txt/x"); !errors.As(err, &unreadable) || held {
		t.Errorf("Holds of a path through a file gave %v, %v", held, err)
	}
}
