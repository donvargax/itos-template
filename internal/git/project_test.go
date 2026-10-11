package git

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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
	m, err := p.Merge(base, target, port.Labels{Base: "template v1.0.0", Target: "template v1.1.0"})
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

// madeProject makes a repository in a new folder whose one commit holds files,
// and opens it as a Project.
func madeProject(t *testing.T, files map[string]string, config ...string) (*Project, string) {
	t.Helper()
	isolate(t)
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	for i := 0; i+1 < len(config); i += 2 {
		git(t, dir, "config", config[i], config[i+1])
	}
	for p, data := range files {
		write(t, dir, p, data, 0o644)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "made")
	p := OpenProject(dir, t.TempDir())
	if _, err := p.Look(); err != nil {
		t.Fatal(err)
	}
	return p, dir
}

func file(path, data string) port.File { return port.File{Path: path, Mode: 0o644, Data: []byte(data)} }

// lines are the file path's lines in dir's working tree, each with its
// line ending.
func lines(t *testing.T, dir, path string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, path))
	if err != nil {
		t.Fatal(err)
	}
	return strings.SplitAfter(string(data), "\n")
}

var labels = port.Labels{Base: "template v1.0.0", Target: "template v1.1.0"}

// A content conflict's markers are labelled HEAD and as labels says, in
// each conflict style, a file both sides added too; one whose markers a
// conflict-marker-size attribute sized, a binary file and a modify/delete
// conflict either way are left as merge-tree wrote them; every stage is
// merge-tree's.
func TestMergeLabelsAContentConflictsMarkersInEachStyle(t *testing.T) {
	for _, style := range []string{"merge", "diff3", "zdiff3"} {
		t.Run(style, func(t *testing.T) {
			p, dir := madeProject(t, map[string]string{
				".gitattributes": "sized.txt conflict-marker-size=10\n",
				"clash.txt":      "a\nproject\nb\n",
				"sized.txt":      "a\nproject\nb\n",
				"added.txt":      "project\n",
				"bin.dat":        "bin\x00project",
				"kept.txt":       "project\n",
			}, "merge.conflictStyle", style)
			base := []port.File{file("clash.txt", "a\nv1.0.0\nb\n"), file("sized.txt", "a\nv1.0.0\nb\n"), file("bin.dat", "bin\x00base"),
				file("kept.txt", "base\n"), file("dropped.txt", "base\n")}
			target := []port.File{file("clash.txt", "a\nv1.1.0\nb\n"), file("sized.txt", "a\nv1.1.0\nb\n"), file("bin.dat", "bin\x00release"),
				file("added.txt", "release\n"), file("dropped.txt", "release\n")}
			m, err := p.Merge(base, target, labels)
			if err != nil {
				t.Fatal(err)
			}
			want := []port.Unmerged{{Path: "added.txt"}, {Path: "bin.dat"}, {Path: "clash.txt"}, {Path: "dropped.txt"}, {Path: "kept.txt"}, {Path: "sized.txt"}}
			if !slices.Equal(m.Conflicts, want) {
				t.Errorf("the conflicts are %v", m.Conflicts)
			}
			for path, data := range map[string]string{"kept.txt": "project\n", "dropped.txt": "release\n"} {
				if got := lines(t, dir, path); strings.Join(got, "") != data {
					t.Errorf("%s, a modify/delete conflict, holds %q", path, got)
				}
			}
			for _, path := range []string{"clash.txt", "added.txt"} {
				got := lines(t, dir, path)
				want := []string{"<<<<<<< HEAD\n", "=======\n", ">>>>>>> template v1.1.0\n"}
				if style != "merge" {
					want = append(want, "||||||| template v1.0.0\n")
				}
				for _, l := range want {
					if !slices.Contains(got, l) {
						t.Errorf("%s has no line %q: %q", path, l, got)
					}
				}
			}
			sized := lines(t, dir, "sized.txt")
			if !slices.Contains(sized, "<<<<<<<<<< HEAD\n") || slices.Contains(sized, ">>>>>>>>>> template v1.1.0\n") {
				t.Errorf("sized.txt is not as merge-tree writes it: %q", sized)
			}
			if data, err := os.ReadFile(filepath.Join(dir, "bin.dat")); err != nil || string(data) != "bin\x00project" {
				t.Errorf("bin.dat holds %q, %v", data, err)
			}
			for path, sides := range map[string][3]string{"clash.txt": {"a\nv1.0.0\nb\n", "a\nproject\nb\n", "a\nv1.1.0\nb\n"}, "added.txt": {"", "project\n", "release\n"}} {
				var want []string
				for i, data := range sides {
					if data != "" {
						oid := strings.TrimSpace(gitIn(t, dir, data, "hash-object", "--stdin"))
						want = append(want, "100644 "+oid+" "+strconv.Itoa(i+1)+"\t"+path+"\n")
					}
				}
				if got := git(t, dir, "ls-files", "--stage", "--", path); got != strings.Join(want, "") {
					t.Errorf("the index holds %s as\n%s", path, got)
				}
			}
		})
	}
}

// git checks out the labelled file as it checks out merge-tree's, so the
// person's line endings are kept: CRLF where core.autocrlf asks for it.
func TestMergeLabelsAConflictKeepingTheLineEndingsGitChecksOut(t *testing.T) {
	p, dir := madeProject(t, map[string]string{"clash.txt": "a\nproject\nb\n"}, "core.autocrlf", "true")
	if _, err := p.Merge([]port.File{file("clash.txt", "a\nv1.0.0\nb\n")}, []port.File{file("clash.txt", "a\nv1.1.0\nb\n")}, labels); err != nil {
		t.Fatal(err)
	}
	want := []string{"a\r\n", "<<<<<<< HEAD\r\n", "project\r\n", "=======\r\n", "v1.1.0\r\n", ">>>>>>> template v1.1.0\r\n", "b\r\n", ""}
	if got := lines(t, dir, "clash.txt"); !slices.Equal(got, want) {
		t.Errorf("clash.txt's lines are %q", got)
	}
}

// merge-tree writes each record a mode, an object, a stage of 1, 2 or 3,
// and a path; anything else is read as no conflict info at all.
func TestStagesOfRefusesARecordGitDoesNotWrite(t *testing.T) {
	for _, record := range []string{"100644 abc 0\tf", "100644 abc 4\tf", "100644 abc x\tf", "100644 1\tf", "\tf"} {
		if _, err := stagesOf([]string{record, ""}); err == nil || !strings.Contains(err.Error(), "git merge-tree wrote") {
			t.Errorf("%q gives %v", record, err)
		}
	}
	got, err := stagesOf([]string{"100644 b 1\tf", "100644 o 2\tf", "100644 t 3\tg", "", "ignored"})
	if err != nil || len(got) != 2 || got[0].objects != [3]string{"b", "o", ""} || got[1].objects != [3]string{"", "", "t"} || len(got[0].records) != 2 {
		t.Errorf("stagesOf = %+v, %v", got, err)
	}
}

// A file in the way of the release's folder is moved aside, as git merge
// moves it, its one stage HEAD's, and nothing relabelled.
func TestMergeLeavesAFileDirectoryConflictAsGitDoes(t *testing.T) {
	p, dir := madeProject(t, map[string]string{"d": "project\n"})
	m, err := p.Merge(nil, []port.File{file("d/x", "release\n")}, labels)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Conflicts, []port.Unmerged{{Path: "d~HEAD"}}) {
		t.Errorf("the conflicts are %v", m.Conflicts)
	}
	oid := strings.TrimSpace(gitIn(t, dir, "project\n", "hash-object", "--stdin"))
	if got := git(t, dir, "ls-files", "--stage", "--", "d~HEAD"); got != "100644 "+oid+" 2\td~HEAD\n" {
		t.Errorf("the index holds d~HEAD as %q", got)
	}
	if got := strings.Join(lines(t, dir, "d/x"), ""); got != "release\n" {
		t.Errorf("d/x holds %q", got)
	}
}

// git merge-file failing, on an object the project lacks, is an error.
func TestMergeFileFailsOnAnObjectTheProjectLacks(t *testing.T) {
	p, _ := madeProject(t, map[string]string{"a": "a\n"})
	missing := strings.Repeat("1", 40)
	if oid, err := p.mergeFile([3]string{missing, missing, missing}, [3]string{"b", "o", "t"}); oid != "" || exitCode(err) < 1 {
		t.Errorf("mergeFile = %q, %v", oid, err)
	}
}
