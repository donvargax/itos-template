package git

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Decision 19 leaves infra to the scenarios, which run the real git on
// three systems; slice-3 gave them what this file held. What stays, no
// scenario reaches: the harness hides every itos and every GIT_* variable
// of the caller's (features/README.md), so it can link no itos as git nor
// point git at another repository; a git command that should not fail,
// failing, is an internal error (exit 70), which no scenario can make; and
// a symbolic link read from a template waits for the idea
// template-special-files. A merge keeping a CRLF file's line endings, and
// a commit by the identity check gives, a scenario could reach but none
// reads yet: the idea outside-test-gaps.

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

// A *Failed is an internal error, which no scenario can make: its sentence
// is git's last line.
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

// template makes a template repository: main with a CRLF text file;
// stack/go adding a script and a symbolic link to it.
func template(t *testing.T) string {
	t.Helper()
	isolate(t)
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "core.autocrlf", "false")
	write(t, dir, "README.md", "# acme-widget\r\n", 0o644)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "main")
	git(t, dir, "checkout", "-q", "-b", "stack/go")
	write(t, dir, "run.sh", "#!/bin/sh\n", 0o644)
	git(t, dir, "add", "-A")
	link := strings.TrimSpace(gitIn(t, dir, "run.sh", "hash-object", "-w", "--stdin"))
	git(t, dir, "update-index", "--add", "--cacheinfo", "120000,"+link+",run")
	git(t, dir, "commit", "-q", "-m", "stack/go")
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

// A symbolic link waits for the idea template-special-files, and no
// scenario yet reads a template's CRLF file (the idea outside-test-gaps).
func TestMergeKeepsASymbolicLinkAndALinesEnding(t *testing.T) {
	files, err := clone(t).Merge([]string{"stack/go"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]port.File{}
	for _, f := range files {
		got[f.Path] = f
	}
	if f := got["README.md"]; string(f.Data) != "# acme-widget\r\n" {
		t.Errorf("README.md is %q: CRLF lost", f.Data)
	}
	if f := got["run"]; f.Mode&fs.ModeSymlink == 0 || string(f.Data) != "run.sh" {
		t.Errorf("the link run is %q, %v", f.Data, f.Mode)
	}
}

// ID-NEW-23 holds a project's first commit by whoever git says; check
// commits its renders by its own identity, so it runs where git knows no
// one, which no scenario runs check in yet (the idea outside-test-gaps).
func TestCommitIsByTheIdentityGiven(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	write(t, dir, "README.md", "x\n", 0o644)
	sha, err := Committer{}.Commit(dir, "chore: make it\n", nil, &port.Identity{Name: "check", Email: "check@localhost"})
	if err != nil {
		t.Fatal(err)
	}
	if head := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD")); head != sha {
		t.Errorf("Commit = %s, HEAD %s", sha, head)
	}
	if who := strings.TrimSpace(git(t, dir, "log", "-1", "--format=%an <%ae>%n%cn <%ce>")); who != "check <check@localhost>\ncheck <check@localhost>" {
		t.Errorf("the commit is by %s", who)
	}
}
