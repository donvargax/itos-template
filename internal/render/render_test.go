package render

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/git"
)

// isolate runs git with no config of the caller's and a fixed identity.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, v := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(v, "render tests")
	}
	for _, v := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(v, "render@localhost")
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git.Run(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func write(t *testing.T, dir, p string, data string, mode os.FileMode) {
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
// binary one, stack/go adding an executable script under a folder named
// after the literal, go/a and go/b each changing one file of main's.
func template(t *testing.T) string {
	t.Helper()
	isolate(t)
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	run(t, dir, "config", "core.autocrlf", "false")
	write(t, dir, "README.md", "# acme-widget\r\nAcmeWidget, ACME_WIDGET\r\n", 0o644)
	write(t, dir, "logo.bin", "\x00acme-widget", 0o644)
	write(t, dir, "shared.txt", "one\n2\n3\n4\ntwo\n", 0o644)
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "main")
	run(t, dir, "checkout", "-q", "-b", "stack/go")
	write(t, dir, "acme-widget/run.sh", "#!/bin/sh\necho acme_widget\n", 0o755)
	run(t, dir, "add", "-A")
	run(t, dir, "update-index", "--chmod=+x", "acme-widget/run.sh")
	run(t, dir, "commit", "-q", "-m", "stack/go")
	run(t, dir, "checkout", "-q", "-b", "go/a")
	write(t, dir, "shared.txt", "ONE\n2\n3\n4\ntwo\n", 0o644)
	run(t, dir, "commit", "-q", "-am", "go/a")
	run(t, dir, "checkout", "-q", "-b", "go/b", "stack/go")
	write(t, dir, "shared.txt", "one\n2\n3\n4\nTWO\n", 0o644)
	run(t, dir, "commit", "-q", "-am", "go/b")
	run(t, dir, "checkout", "-q", "-b", "go/c", "stack/go")
	write(t, dir, "shared.txt", "uno\n2\n3\n4\ntwo\n", 0o644)
	run(t, dir, "commit", "-q", "-am", "go/c")
	run(t, dir, "checkout", "-q", "main")
	return dir
}

func clone(t *testing.T) *Template {
	t.Helper()
	tpl, err := Clone(template(t), filepath.Join(t.TempDir(), "clone"))
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

var acme = NewReplacer([]string{
	"acme-widget", "blue-fox", "acme_widget", "blue_fox", "ACME_WIDGET", "BLUE_FOX",
	"acmeWidget", "blueFox", "AcmeWidget", "BlueFox",
})

func TestRenderMergesReplacesAndKeepsBytesAndModes(t *testing.T) {
	tpl := clone(t)
	if b, _ := tpl.DefaultBranch(); b != "main" {
		t.Errorf("DefaultBranch = %q", b)
	}
	base, ok, err := tpl.Commit("stack/go")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	tree, err := tpl.Merge(base, "stack/go", []string{"go/a", "go/b"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := tpl.Plan(tree, func(string) bool { return true }, acme)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	executables, err := plan.Write(out)
	if err != nil {
		t.Fatal(err)
	}
	read := func(p string) string {
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if got := read("README.md"); got != "# blue-fox\r\nBlueFox, BLUE_FOX\r\n" {
		t.Errorf("README.md is %q: CRLF and the replacements not as they should be", got)
	}
	if got := read("logo.bin"); got != "\x00acme-widget" {
		t.Errorf("logo.bin is %q: a binary file is never replaced in", got)
	}
	if got := read("shared.txt"); got != "ONE\n2\n3\n4\nTWO\n" {
		t.Errorf("shared.txt is %q, not both features' changes", got)
	}
	if got := read("blue-fox/run.sh"); got != "#!/bin/sh\necho blue_fox\n" {
		t.Errorf("blue-fox/run.sh is %q", got)
	}
	if !slices.Equal(executables, []string{"blue-fox/run.sh"}) {
		t.Errorf("executables %q", executables)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(out, "blue-fox", "run.sh"))
		if err != nil || info.Mode().Perm()&0o111 == 0 {
			t.Errorf("run.sh is not executable: %v %v", info.Mode(), err)
		}
	}
}

func TestPlanLeavesOutWhatKeepRefuses(t *testing.T) {
	tpl := clone(t)
	base, _, _ := tpl.Commit("main")
	tree, err := tpl.Merge(base, "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := tpl.Plan(tree, func(p string) bool { return p != "logo.bin" }, acme)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range plan.Files {
		paths = append(paths, f.To)
	}
	if !slices.Equal(paths, []string{"README.md", "shared.txt"}) {
		t.Errorf("plan %q", paths)
	}
}

func TestMergeRefusesConflictsNamingTheBranches(t *testing.T) {
	tpl := clone(t)
	base, _, _ := tpl.Commit("stack/go")
	_, err := tpl.Merge(base, "stack/go", []string{"go/a", "go/c"})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Branch != "go/c" || !slices.Equal(conflict.Paths, []string{"shared.txt"}) {
		t.Fatalf("Merge = %v", err)
	}
	if !IsDefect(err) {
		t.Error("a conflict is not read as the template's defect")
	}
}

func TestPathReplacesEachElementAndRefusesImpossibleNames(t *testing.T) {
	if got, err := acme.Path("cmd/acme-widget/acme_widget.go"); err != nil || got != "cmd/blue-fox/blue_fox.go" {
		t.Errorf("Path = %q, %v", got, err)
	}
	slash := NewReplacer([]string{"acme", "a/b"})
	if _, err := slash.Path("acme/x"); err == nil {
		t.Error("an answer making a / in a name is taken")
	}
	dot := NewReplacer([]string{"acme", ".."})
	if _, err := dot.Path("acme/x"); err == nil {
		t.Error("an answer making .. a name is taken")
	}
}

func TestPlanRefusesTwoFilesOfOneName(t *testing.T) {
	tpl := clone(t)
	base, _, _ := tpl.Commit("main")
	tree, _ := tpl.Merge(base, "main", nil)
	same := NewReplacer([]string{"README.md", "shared.txt"})
	_, err := tpl.Plan(tree, func(string) bool { return true }, same)
	var name *NameError
	if !errors.As(err, &name) {
		t.Fatalf("Plan = %v", err)
	}
}

func TestCloneRefusesATemplateGitCannotReach(t *testing.T) {
	isolate(t)
	_, err := Clone(filepath.Join(t.TempDir(), "nosuch"), filepath.Join(t.TempDir(), "clone"))
	var unreachable *UnreachableError
	if !errors.As(err, &unreachable) {
		t.Fatalf("Clone = %v", err)
	}
}

func TestContentsLeavesABinaryFileAsItIs(t *testing.T) {
	data := append([]byte(strings.Repeat("x", 9000)), []byte("acme-widget")...)
	data[8500] = 0
	if got := acme.Contents(data); string(got[len(got)-8:]) != "blue-fox" {
		t.Error("a NUL past the first 8000 bytes made a text file binary")
	}
	data[10] = 0
	if got := acme.Contents(data); string(got[len(got)-11:]) != "acme-widget" {
		t.Error("a NUL in the first 8000 bytes did not make the file binary")
	}
}
