package git

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/template/port"
)

// A git older than update's merge needs is refused, naming its version and
// the one needed; no scenario can run another git than the machine's.
func TestSupportsRefusesAGitOlderThanTheMergeNeeds(t *testing.T) {
	for line, ok := range map[string]bool{
		"git version 2.45.0\n":                 true,
		"git version 2.45.1.windows.1\n":       true,
		"git version 2.56.0":                   true,
		"git version 3.0.0":                    true,
		"git version 2.44.9\n":                 false,
		"git version 2.39.3 (Apple Git-146)\n": false,
		"git version 1.99.0":                   false,
		"no git at all":                        false,
	} {
		err := supports([]byte(line), nil)
		var old *TooOld
		switch {
		case ok && err != nil:
			t.Errorf("%q is refused: %v", line, err)
		case !ok && (!errors.As(err, &old) || old.Need != "2.45" || old.Version != strings.TrimSpace(line)):
			t.Errorf("%q gives %v", line, err)
		case !ok && old.Error() != old.Version+", older than 2.45":
			t.Errorf("TooOld says %q", old.Error())
		}
	}
	missing := &Missing{Err: errors.New("no git")}
	if err := supports(nil, missing); err != missing {
		t.Errorf("a git that cannot run gives %v", err)
	}
}

// The rest of Project the scenarios run against the real git; what no
// scenario reads is here: a HEAD that cannot be read, and the changes a
// merge makes, which only --json's changed holds and no scenario reads.

func TestCommittedFailsOutsideARepository(t *testing.T) {
	p := OpenProject(t.TempDir(), t.TempDir())
	p.top = p.dir
	if data, ok, err := p.Committed(".itos-template.yaml"); err == nil || ok || data != nil {
		t.Errorf("Committed outside a repository = %q, %v, %v", data, ok, err)
	}
}

func TestMergeNamesEachChangeAndConflictAsGitDoes(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	readme := "# blue-fox\n\nline 2\nline 3\nline 4\nline 5\n"
	for p, data := range map[string]string{"README.md": readme, "gone.txt": "gone\n", "clash.txt": "project\n", "same.txt": "same\n"} {
		write(t, dir, p, data, 0o644)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "made")
	p := OpenProject(dir, t.TempDir())
	if _, err := p.Look(); err != nil {
		t.Fatal(err)
	}
	file := func(path, data string) port.File { return port.File{Path: path, Mode: 0o644, Data: []byte(data)} }
	base := []port.File{file("README.md", readme), file("gone.txt", "gone\n"), file("clash.txt", "base\n"), file("same.txt", "same\n")}
	target := []port.File{file("README.txt", readme), file("clash.txt", "release\n"), file("same.txt", "same\n"), file("added.txt", "added\n"),
		{Path: "run.sh", Mode: 0o755, Data: []byte("#!/bin/sh\n")}}
	m, err := p.Merge(base, target)
	if err != nil {
		t.Fatal(err)
	}
	want := []port.Change{
		{Path: "README.txt", Kind: port.Renamed, From: "README.md"},
		{Path: "added.txt", Kind: port.Added},
		{Path: "clash.txt", Kind: port.Modified},
		{Path: "gone.txt", Kind: port.Removed},
		{Path: "run.sh", Kind: port.Added},
	}
	if !slices.Equal(m.Changes, want) {
		t.Errorf("the changes are %v, not %v", m.Changes, want)
	}
	if !slices.Equal(m.Conflicts, []port.Unmerged{{Path: "clash.txt"}}) {
		t.Errorf("the conflicts are %v", m.Conflicts)
	}
	if got := git(t, dir, "ls-files", "--stage", "--", "clash.txt", "run.sh"); strings.Count(got, " 1\tclash.txt") != 1 ||
		strings.Count(got, " 2\tclash.txt") != 1 || strings.Count(got, " 3\tclash.txt") != 1 || !strings.Contains(got, "100755 ") {
		t.Errorf("the index holds\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "gone.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("gone.txt is still in the working tree: %v", err)
	}
	if head := strings.TrimSpace(git(t, dir, "rev-list", "--count", "--all")); head != "1" {
		t.Errorf("the repository holds %s commits", head)
	}
}
