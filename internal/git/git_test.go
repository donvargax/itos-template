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

// Decision 18 leaves infra to the scenarios, which run the real git on
// three systems. What this file holds no scenario reaches: the harness
// hides every itos and every GIT_* variable of the caller's
// (features/README.md), so it can link no itos as git nor point git at
// another repository; and none yet renders a merge of two features
// changing one file, a conflict, a symbolic link or a submodule, nor reads
// a commit's modes or author, so those stay here until the idea
// scenario-gaps gives them scenarios.

func TestIsItosFollowsLinksToAFileNamedItos(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need a privilege on windows")
	}
	dir := t.TempDir()
	itos := filepath.Join(dir, "itos")
	if err := os.WriteFile(itos, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(dir, "git")
	if err := os.Symlink(itos, shim); err != nil {
		t.Fatal(err)
	}
	again := filepath.Join(dir, "git2")
	if err := os.Symlink("git", again); err != nil {
		t.Fatal(err)
	}
	if !IsItos(shim) || !IsItos(again) {
		t.Error("a link to itos, directly or through another link, is not read as an itos")
	}
	if IsItos(itos) {
		t.Error("a script named itos, not a Go build of itos, is read as one")
	}
}

func TestGitRunsInItsFolderNotTheCallersRepository(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_DIR", filepath.Join(dir, "elsewhere"))
	if _, err := run(dir, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Errorf("git ran on the caller's GIT_DIR, not in its folder: %v", err)
	}
	_, err := run(dir, "rev-parse", "--verify", "nosuch")
	var failed *Failed
	if !errors.As(err, &failed) || failed.Code <= 0 {
		t.Errorf("a failing git command gave %v", err)
	}
}

func TestFailedSaysGitsLastLine(t *testing.T) {
	e := &Failed{Args: []string{"clone"}, Code: 128, Stderr: "Cloning into 'x'...\nfatal: repository 'x' does not exist\n\n"}
	if got := e.Error(); got != "fatal: repository 'x' does not exist" {
		t.Errorf("Error() = %q", got)
	}
	if got := (&Failed{Args: []string{"var", "X"}, Code: 1}).Error(); got != "git var X exited 1" {
		t.Errorf("Error() = %q", got)
	}
}

// isolate runs git with no config of the caller's and a fixed identity.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, v := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(v, "git tests")
	}
	for _, v := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(v, "git@localhost")
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := run(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func write(t *testing.T, dir, p, data string, mode os.FileMode) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
}

// template makes a template repository: main with a CRLF text file and a
// binary one; stack/go adding an executable script, a symbolic link to it
// and a submodule; go/a and go/b each changing another line of a file of
// main's, go/c the same line as go/a.
func template(t *testing.T) string {
	t.Helper()
	isolate(t)
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "core.autocrlf", "false")
	write(t, dir, "README.md", "# acme-widget\r\n", 0o644)
	write(t, dir, "logo.bin", "\x00acme-widget", 0o644)
	write(t, dir, "shared.txt", "one\n2\n3\n4\ntwo\n", 0o644)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "main")
	git(t, dir, "checkout", "-q", "-b", "stack/go")
	write(t, dir, "run.sh", "#!/bin/sh\n", 0o755)
	git(t, dir, "add", "-A")
	git(t, dir, "update-index", "--chmod=+x", "run.sh")
	link := strings.TrimSpace(gitIn(t, dir, "run.sh", "hash-object", "-w", "--stdin"))
	git(t, dir, "update-index", "--add", "--cacheinfo", "120000,"+link+",run")
	git(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))+",vendor/lib")
	git(t, dir, "commit", "-q", "-m", "stack/go")
	git(t, dir, "checkout", "-q", "-f", "-b", "go/a")
	write(t, dir, "shared.txt", "ONE\n2\n3\n4\ntwo\n", 0o644)
	git(t, dir, "commit", "-q", "-am", "go/a")
	git(t, dir, "checkout", "-q", "-f", "-b", "go/b", "stack/go")
	write(t, dir, "shared.txt", "one\n2\n3\n4\nTWO\n", 0o644)
	git(t, dir, "commit", "-q", "-am", "go/b")
	git(t, dir, "checkout", "-q", "-f", "-b", "go/c", "stack/go")
	write(t, dir, "shared.txt", "uno\n2\n3\n4\ntwo\n", 0o644)
	git(t, dir, "commit", "-q", "-am", "go/c")
	git(t, dir, "checkout", "-q", "-f", "main")
	return dir
}

// gitIn runs git in dir with stdin's contents.
func gitIn(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	out, err := command{dir: dir, stdin: strings.NewReader(stdin)}.output(args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func clone(t *testing.T) *Repo {
	t.Helper()
	r, err := Clone(template(t), filepath.Join(t.TempDir(), "clone"))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRepoReadsBranchesAndFiles(t *testing.T) {
	r := clone(t)
	if b, err := r.DefaultBranch(); err != nil || b != "main" {
		t.Errorf("DefaultBranch = %q, %v", b, err)
	}
	commit, ok, err := r.Commit("main")
	if err != nil || !ok {
		t.Fatalf("Commit(main) = %v, %v", ok, err)
	}
	if _, ok, err := r.Commit("go/nosuch"); ok || err != nil {
		t.Errorf("Commit(go/nosuch) = %v, %v", ok, err)
	}
	if data, ok, err := r.File(commit, "README.md"); !ok || err != nil || string(data) != "# acme-widget\r\n" {
		t.Errorf("File(README.md) = %q, %v, %v", data, ok, err)
	}
	if _, ok, err := r.File(commit, "nosuch"); ok || err != nil {
		t.Errorf("File(nosuch) = %v, %v", ok, err)
	}
}

func TestMergeCombinesTheBranchesKeepingBytesAndModes(t *testing.T) {
	files, err := clone(t).Merge([]string{"stack/go", "go/a", "go/b"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]port.File{}
	var paths []string
	for _, f := range files {
		got[f.Path] = f
		paths = append(paths, f.Path)
	}
	if !slices.Equal(paths, []string{"README.md", "logo.bin", "run", "run.sh", "shared.txt", "vendor/lib"}) {
		t.Errorf("the files are %q, in git's order", paths)
	}
	if f := got["shared.txt"]; string(f.Data) != "ONE\n2\n3\n4\nTWO\n" || f.Mode != 0o644 {
		t.Errorf("shared.txt is %q, %v, not both features' changes", f.Data, f.Mode)
	}
	if f := got["README.md"]; string(f.Data) != "# acme-widget\r\n" {
		t.Errorf("README.md is %q: CRLF lost", f.Data)
	}
	if f := got["logo.bin"]; string(f.Data) != "\x00acme-widget" {
		t.Errorf("logo.bin is %q", f.Data)
	}
	if f := got["run.sh"]; f.Mode != 0o755 {
		t.Errorf("run.sh's mode is %v", f.Mode)
	}
	if f := got["run"]; f.Mode&fs.ModeSymlink == 0 || string(f.Data) != "run.sh" {
		t.Errorf("the link run is %q, %v", f.Data, f.Mode)
	}
	if f := got["vendor/lib"]; f.Mode != fs.ModeIrregular || f.Data != nil {
		t.Errorf("the submodule is %q, %v", f.Data, f.Mode)
	}
}

func TestMergeRefusesConflictsNamingTheBranchAndThePaths(t *testing.T) {
	_, err := clone(t).Merge([]string{"stack/go", "go/a", "go/c"})
	var conflict *port.Conflict
	if !errors.As(err, &conflict) || conflict.Branch != "go/c" || !slices.Equal(conflict.Paths, []string{"shared.txt"}) {
		t.Fatalf("Merge = %v", err)
	}
}

// The commit records an executable as the template does, on every system:
// on windows, where the file system has no execute bit, too.
func TestCommitRecordsTheExecutablesAndWhoItIsBy(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	write(t, dir, "run.sh", "#!/bin/sh\n", 0o644)
	write(t, dir, "README.md", "x\r\n", 0o644)
	sha, err := Committer{}.Commit(dir, "chore: make it\n", []string{"run.sh"}, &port.Identity{Name: "check", Email: "check@localhost"})
	if err != nil {
		t.Fatal(err)
	}
	if head := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD")); head != sha {
		t.Errorf("Commit = %s, HEAD %s", sha, head)
	}
	if files := git(t, dir, "ls-files", "-s"); !strings.Contains(files, "100755 ") || !strings.Contains(files, "100644 ") {
		t.Errorf("the commit's files:\n%s", files)
	}
	if author := strings.TrimSpace(git(t, dir, "log", "-1", "--format=%an <%ae>")); author != "check <check@localhost>" {
		t.Errorf("the commit is by %s", author)
	}
}

func TestIdentitySaysWhenGitKnowsNoOne(t *testing.T) {
	isolate(t)
	if err := (Committer{}).Identity(); err != nil {
		t.Errorf("Identity = %v", err)
	}
	for _, v := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(v, "")
	}
	var none *NoIdentity
	if err := (Committer{}).Identity(); !errors.As(err, &none) {
		t.Errorf("Identity = %v", err)
	}
}
