package git

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

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
// reads yet: the idea outside-test-gaps. Nor does a scenario reach another
// OS's branch on the OS it runs on, which decision 26 has a test reach by
// handing the OS in as a value, or a git output no real git writes, which a
// stub git writes here (stubGit) (T-48).

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

// The release fixture's tags are lightweight; a person's may be annotated,
// or name a tree, which no release can be, and a tag may be off its branch:
// none a scenario reaches.
func TestTagsGiveTheCommitEachNamesAndIsAncestorReadsAHistory(t *testing.T) {
	dir := template(t)
	main := strings.TrimSpace(git(t, dir, "rev-parse", "main"))
	stack := strings.TrimSpace(git(t, dir, "rev-parse", "stack/go"))
	git(t, dir, "tag", "main/v1.0.0", "main")
	git(t, dir, "tag", "-a", "-m", "v1.0.0", "stack/go/v1.0.0", "stack/go")
	git(t, dir, "tag", "tree", "main^{tree}")
	r, err := Clone(dir, filepath.Join(t.TempDir(), "clone"))
	if err != nil {
		t.Fatal(err)
	}
	tags, err := r.Tags()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, tag := range tags {
		got[tag.Name] = tag.Commit
	}
	if len(got) != 2 || got["main/v1.0.0"] != main || got["stack/go/v1.0.0"] != stack {
		t.Errorf("Tags = %v, not main/v1.0.0 at %s and stack/go/v1.0.0 at %s", got, main, stack)
	}
	for _, c := range []struct {
		commit, of string
		want       bool
	}{{main, stack, true}, {stack, stack, true}, {stack, main, false}} {
		if is, err := r.IsAncestor(c.commit, c.of); err != nil || is != c.want {
			t.Errorf("IsAncestor(%s, %s) = %v, %v", c.commit, c.of, is, err)
		}
	}
	if _, err := r.IsAncestor("nosuch", stack); err == nil {
		t.Error("IsAncestor of no commit gave no error")
	}
}

// check --ref in a checkout reads the release from the tags it holds.
func TestCloneHereCopiesTheTags(t *testing.T) {
	dir := template(t)
	git(t, dir, "tag", "main/v1.0.0", "main")
	r, err := CloneHere(dir, filepath.Join(t.TempDir(), "copy"), "main")
	if err != nil {
		t.Fatal(err)
	}
	tags, err := r.Tags()
	if err != nil || len(tags) != 1 || tags[0].Name != "main/v1.0.0" {
		t.Errorf("Tags = %v, %v", tags, err)
	}
}

// A conflict names the commit, by its place among those merged, whose
// merge left it: the domain names its branch by that place.
func TestMergeNamesTheCommitThatConflicts(t *testing.T) {
	dir := template(t)
	git(t, dir, "checkout", "-q", "-b", "other", "main")
	write(t, dir, "README.md", "other\n", 0o644)
	git(t, dir, "commit", "-q", "-a", "-m", "other")
	git(t, dir, "checkout", "-q", "stack/go")
	write(t, dir, "README.md", "stack\n", 0o644)
	git(t, dir, "commit", "-q", "-a", "-m", "stack")
	r, err := Clone(dir, filepath.Join(t.TempDir(), "clone"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Merge([]string{"main", "stack/go", "other"})
	var conflict *port.Conflict
	if !errors.As(err, &conflict) || conflict.At != 2 || len(conflict.Paths) != 1 || conflict.Paths[0] != "README.md" {
		t.Errorf("Merge = %v", err)
	}
}

// ID-RELEASE-14 refuses a repository with no origin with exit 1, naming
// origin, and a repository whose origin HasOrigin missed would be refused
// the same way, each listed branch then not on origin: no scenario tells
// them apart.
func TestHasOriginReadsTheRemoteOriginsURL(t *testing.T) {
	dir := template(t)
	g := OpenTagger(dir)
	if _, err := g.Top(); err != nil {
		t.Fatal(err)
	}
	if ok, err := g.HasOrigin(); ok || err != nil {
		t.Errorf("HasOrigin with no remote = %v, %v", ok, err)
	}
	git(t, dir, "remote", "add", "origin", "https://example.invalid/acme.git")
	if ok, err := g.HasOrigin(); !ok || err != nil {
		t.Errorf("HasOrigin with an origin = %v, %v", ok, err)
	}
}

// Release's root is the branch origin's HEAD names (bug-7). No scenario
// releases from a clone that recorded none and whose origin, asked, names
// one (check's ID-CHECK-18 asks it, but check reads no bool); and an origin
// naming none read as one naming "" would leave the copy rootless, refused
// with exit 2 saying it has no default branch as ID-RELEASE-17 is: no
// scenario tells them apart.
func TestDefaultBranchIsTheBranchOriginsHeadNames(t *testing.T) {
	origin := template(t)
	dir := filepath.Join(t.TempDir(), "clone")
	git(t, "", "clone", "-q", "--", origin, dir)
	git(t, dir, "checkout", "-q", "stack/go")
	g := OpenTagger(dir)
	if _, err := g.Top(); err != nil {
		t.Fatal(err)
	}
	if b, ok := g.DefaultBranch(); b != "main" || !ok {
		t.Errorf("DefaultBranch as git clone recorded it = %q, %v", b, ok)
	}
	git(t, dir, "remote", "set-head", "origin", "-d")
	if b, ok := g.DefaultBranch(); b != "main" || !ok {
		t.Errorf("DefaultBranch as origin says it = %q, %v", b, ok)
	}
	git(t, origin, "checkout", "-q", "--detach")
	if b, ok := g.DefaultBranch(); b != "" || ok {
		t.Errorf("DefaultBranch of an origin whose HEAD names no branch = %q, %v", b, ok)
	}
	git(t, dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	if b, ok := g.DefaultBranch(); b != "" || ok {
		t.Errorf("DefaultBranch of an origin that cannot be reached = %q, %v", b, ok)
	}
}

// The stub git: this test binary, run with stubOut set, writes its value,
// unquoted as a Go string, and exits 0, or 2 with stubFails set too.
const (
	stubOut   = "ITOS_TEMPLATE_TEST_STUB_GIT_OUT"
	stubFails = "ITOS_TEMPLATE_TEST_STUB_GIT_FAILS"
)

// TestMain runs the tests, or runs as the stub git: it reads its input, as
// git cat-file --batch does, writes what it was given, and exits 0, or 2
// to fail.
func TestMain(m *testing.M) {
	out, ok := os.LookupEnv(stubOut)
	if !ok {
		os.Exit(m.Run())
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	out, err := strconv.Unquote(out)
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(3)
	}
	_, _ = os.Stdout.WriteString(out)
	if os.Getenv(stubFails) != "" {
		os.Exit(2)
	}
	os.Exit(0)
}

// useGit has the package run the program at p as git for the rest of the
// test.
func useGit(t *testing.T, p string) {
	t.Helper()
	was := Bin
	Bin = func() string { return p }
	t.Cleanup(func() { Bin = was })
}

// stubGit has the package run the stub git for the rest of the test, every
// command writing out, and failing with exit 2 when fails.
func stubGit(t *testing.T, out string, fails bool) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(stubOut, strconv.Quote(out)) // a variable holds no NUL
	// A test binary built for coverage writes it where GOCOVERDIR says, and
	// the stub's, of another counter mode, must not join the coverage a
	// whole-suite mutation run collects there from itos-template's own
	// binary.
	t.Setenv("GOCOVERDIR", t.TempDir())
	if fails {
		t.Setenv(stubFails, "1")
	}
	useGit(t, exe)
}

// A run on one OS never takes another's branch, nor do the scenarios on it:
// names takes the OS and PATHEXT as values (decision 26) so that every OS's
// names are read on any.
func TestNamesAreGitOrOnWindowsGitWithEachExtension(t *testing.T) {
	for _, c := range []struct {
		goos, pathext string
		want          []string
	}{
		{"linux", ".EXE", []string{"git"}},
		{"darwin", "", []string{"git"}},
		{"windows", "", []string{"git.com", "git.exe", "git.bat", "git.cmd"}},
		{"windows", ".EXE;;.Cmd;", []string{"git.exe", "git.cmd"}},
	} {
		if got := names(c.goos, c.pathext); !slices.Equal(got, c.want) {
			t.Errorf("names(%q, %q) = %q, want %q", c.goos, c.pathext, got, c.want)
		}
	}
}

// bin is itos's rule for the real git (its bug 45). No scenario reaches
// its choices: the harness hides ITOS_GIT and every itos of the caller's
// (features/README.md), and runs with a PATH of absolute folders, each git
// on it a program.
func TestBinIsItosGitElseTheFirstProgramNamedGitInAnAbsolutePathFolder(t *testing.T) {
	goos := runtime.GOOS // a real OS, its files stat-ed as they are
	t.Setenv("PATHEXT", ".exe")
	name := names(goos, ".exe")[0]
	top := t.TempDir()
	t.Chdir(top)
	for _, dir := range []string{"relative", "absolute"} {
		write(t, top, dir+"/"+name, "#!/bin/sh\n", 0o755)
	}
	if err := os.MkdirAll(filepath.Join(top, "folder", name), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", strings.Join([]string{
		"", "relative", filepath.Join(top, "none"), filepath.Join(top, "folder"), filepath.Join(top, "absolute"),
	}, string(os.PathListSeparator)))
	t.Setenv("ITOS_GIT", "")
	if got, want := bin(goos), filepath.Join(top, "absolute", name); got != want {
		t.Errorf("bin on the PATH = %s, want %s", got, want)
	}
	chosen := filepath.Join(top, "chosen")
	t.Setenv("ITOS_GIT", chosen)
	if got := bin(goos); got != chosen {
		t.Errorf("bin with ITOS_GIT = %s, want %s", got, chosen)
	}
}

// fileInfo is a stub of a file's os.FileInfo, of its mode alone.
type fileInfo struct{ mode fs.FileMode }

func (f fileInfo) Name() string       { return "git" }
func (f fileInfo) Size() int64        { return 0 }
func (f fileInfo) Mode() fs.FileMode  { return f.mode }
func (f fileInfo) ModTime() time.Time { return time.Time{} }
func (f fileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fileInfo) Sys() any           { return nil }

// As names: executable takes the OS as a value (decision 26), so windows,
// which keeps no execute bit, is read on any OS.
func TestExecutableIsARegularFileWithAnExecuteBitOrOnWindowsAny(t *testing.T) {
	for _, c := range []struct {
		goos string
		mode fs.FileMode
		want bool
	}{
		{"linux", 0o755, true},
		{"linux", 0o100, true},
		{"linux", 0o644, false},
		{"linux", fs.ModeDir | 0o755, false},
		{"windows", 0o644, true},
		{"windows", fs.ModeDir | 0o755, false},
	} {
		if got := executable(c.goos, fileInfo{c.mode}); got != c.want {
			t.Errorf("executable(%q, %v) = %v", c.goos, c.mode, got)
		}
	}
}

// No scenario makes Identity's git fail to run: a git missing fails new's
// clone before it, and a missing temporary folder fails new's own folder
// for the clone. A folder git cannot start in is one here.
func TestIdentityGivesTheErrorOfAGitThatCannotRun(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone")
	for _, v := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(v, gone)
	}
	var missing *Missing
	if err := (Committer{}).Identity(); !errors.As(err, &missing) {
		t.Errorf("Identity where git cannot run = %v, not a *Missing", err)
	}
}

// The scenarios run where the file system keeps the execute bit a render
// writes but on windows, so only there does git take it from the
// executables Commit is given; a file written without its bit is that case
// on any OS.
func TestCommitRecordsAnExecutableWrittenWithoutItsBit(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	write(t, dir, "run.sh", "#!/bin/sh\n", 0o644)
	if _, err := (Committer{}).Commit(dir, "chore: make it\n", []string{"run.sh"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := git(t, dir, "ls-files", "--stage", "--", "run.sh"); !strings.HasPrefix(got, "100755 ") {
		t.Errorf("run.sh is recorded %q, not executable", got)
	}
}

// git ls-tree writes every record as its mode, type and object, a tab and
// the path; a record of another shape is no real git's, so a stub's.
func TestFilesRefusesATreeRecordOfAnotherShape(t *testing.T) {
	for _, record := range []string{"100644 blob 0123", "100644 blob\tREADME.md"} {
		t.Run(record, func(t *testing.T) {
			stubGit(t, record+"\x00", false)
			files, err := (&Repo{dir: t.TempDir()}).files("HEAD")
			if want := fmt.Sprintf("git ls-tree wrote %q", record); err == nil || !strings.Contains(err.Error(), want) || files != nil {
				t.Errorf("files = %v, %v, not %s", files, err, want)
			}
		})
	}
}

// A git cat-file that wrote every object and then failed, or that wrote
// something other than a blob, is no real git's: a stub's.
func TestBlobsGivesGitsFailureOnlyOnceEveryObjectIsRead(t *testing.T) {
	read := func() ([]string, error) {
		var got []string
		err := (&Repo{dir: t.TempDir()}).blobs([]string{"0123"}, func(_ int, data []byte) error {
			got = append(got, string(data))
			return nil
		})
		return got, err
	}
	t.Run("every object read", func(t *testing.T) {
		stubGit(t, "0123 blob 1\nx\n", true)
		got, err := read()
		var failed *Failed
		if !errors.As(err, &failed) || failed.Code != 2 || !slices.Equal(got, []string{"x"}) {
			t.Errorf("blobs = %q, %v, not x and git's exit 2", got, err)
		}
	})
	t.Run("an object not read", func(t *testing.T) {
		stubGit(t, "0123 tree 1\nx\n", true)
		got, err := read()
		var failed *Failed
		if errors.As(err, &failed) || err == nil || !strings.Contains(err.Error(), "git cat-file wrote") || got != nil {
			t.Errorf("blobs = %q, %v, not the object it could not read", got, err)
		}
	})
}

// Every tree a scenario renders or reads holds a file, a template its
// manifest at least; blobs of none runs no git, so it reads none where git
// cannot run.
func TestBlobsOfNoObjectRunsNoGit(t *testing.T) {
	useGit(t, filepath.Join(t.TempDir(), "no-git"))
	err := (&Repo{dir: t.TempDir()}).blobs(nil, func(int, []byte) error {
		t.Error("blobs of no object gave one")
		return nil
	})
	if err != nil {
		t.Errorf("blobs of no object = %v", err)
	}
}

// git cat-file --batch writes a blob of every object here; any other type
// is no real git's.
func TestReadBlobRefusesAnObjectNotABlob(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("0123 tree 3\nabc\n"))
	err := readBlob(r, "0123", func([]byte) error {
		t.Error("readBlob gave a tree's contents")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), `git cat-file wrote "0123 tree 3"`) {
		t.Errorf("readBlob of a tree = %v", err)
	}
}
