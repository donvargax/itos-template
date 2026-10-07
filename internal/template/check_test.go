package template

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

// checked checks tpl with the fakes, has the one program its checks run, and
// returns every result and what is left on the disk.
func checked(t *testing.T, tpl *Template, setup func(*porttest.Disk, *porttest.Git, *porttest.Folders)) ([]Result, *porttest.Disk, *porttest.Git) {
	t.Helper()
	w, d, g := writer()
	folders := &porttest.Folders{Disk: d}
	if setup != nil {
		setup(d, g, folders)
	}
	var results []Result
	err := tpl.Check(answers, folders, w, porttest.Programs{"has": porttest.Has(d)}, func(r Result) error {
		results = append(results, r)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return results, d, g
}

// report is each result as its combination's name and how each check
// ended, the words in place.
func report(results []Result) []string {
	var lines []string
	for _, r := range results {
		status := "failed"
		if r.Passed() {
			status = "passed"
		}
		lines = append(lines, r.Combination.Name()+": "+status)
		for _, c := range r.Checks {
			lines = append(lines, "  "+[]string{"passed", "failed", "skipped"}[c.Status]+": "+c.Check.String())
		}
	}
	return lines
}

func TestCheckRendersEveryCombinationAndRunsItsChecksTheAnswersInPlace(t *testing.T) {
	results, d, g := checked(t, open(t, acme()), nil)
	want := []string{
		"sh: passed",
		"  passed: has README.md",
		"  passed: has bin/blue-fox",
		"sh + extra: passed",
		"  passed: has README.md",
		"  passed: has bin/blue-fox",
		"  passed: has extra.txt",
		"sh + extra + more: passed",
		"  passed: has README.md",
		"  passed: has bin/blue-fox",
		"  passed: has extra.txt",
		"py: passed",
		"  passed: has README.md",
		"py + tool: passed",
		"  passed: has README.md",
	}
	if got := report(results); !slices.Equal(got, want) {
		t.Errorf("the report is\n%s", strings.Join(got, "\n"))
	}
	if len(d.Folders) != 0 {
		t.Errorf("the renders' folders are left: %v", d.Folders)
	}
	for folder, c := range g.Commits {
		if c.By != checker {
			t.Errorf("the render in %s is committed by %+v", folder, c.By)
		}
		if _, ok := c.Files["more.txt"]; ok && folder != "tmp/3" {
			t.Errorf("more.txt is in the render in %s", folder)
		}
	}
	if len(g.Commits) != 5 {
		t.Errorf("%d renders were committed", len(g.Commits))
	}
}

// A CI that sets no git identity can check a template.
func TestCheckRendersWhenGitKnowsNoOne(t *testing.T) {
	results, _, _ := checked(t, open(t, acme()), func(_ *porttest.Disk, g *porttest.Git, _ *porttest.Folders) { g.Who = nil })
	for _, r := range results {
		if !r.Passed() {
			t.Errorf("%s: %v", r.Combination.Name(), r.Err)
		}
	}
}

func TestCheckStopsACombinationAtItsFirstFailedCheckAndChecksTheRest(t *testing.T) {
	text := strings.Replace(manifestText, "checks: [[has, extra.txt]]", "checks: [[nosuch, x], [has, extra.txt]]", 1)
	results, _, _ := checked(t, open(t, withManifest(acme(), text)), nil)
	want := []string{
		"sh: passed",
		"  passed: has README.md",
		"  passed: has bin/blue-fox",
		"sh + extra: failed",
		"  passed: has README.md",
		"  passed: has bin/blue-fox",
		"  failed: nosuch x",
		"  skipped: has extra.txt",
		"sh + extra + more: failed",
		"  passed: has README.md",
		"  passed: has bin/blue-fox",
		"  failed: nosuch x",
		"  skipped: has extra.txt",
		"py: passed",
		"  passed: has README.md",
		"py + tool: passed",
		"  passed: has README.md",
	}
	if got := report(results); !slices.Equal(got, want) {
		t.Errorf("the report is\n%s", strings.Join(got, "\n"))
	}
	failed := results[1].Checks[2]
	if !slices.Equal(failed.Lines(), []string{"cannot run nosuch: no such program"}) || results[1].Checks[3].Output != nil {
		t.Errorf("the failed check wrote %q, the skipped one %q", failed.Output, results[1].Checks[3].Output)
	}
}

// A render that fails fails its combination, which runs no check; the
// others are still checked.
func TestCheckGivesARenderThatFailedItsError(t *testing.T) {
	repo := acme()
	repo.Branches["sh/more"]["extra.txt"] = &fstest.MapFile{Data: []byte("changed\n")}
	results, d, _ := checked(t, open(t, repo), nil)
	more := results[2]
	var conflict *MergeConflict
	if more.Passed() || !errors.As(more.Err, &conflict) || len(more.Checks) != 0 {
		t.Errorf("sh + extra + more is %+v", more)
	}
	if !results[0].Passed() || !results[3].Passed() || len(results) != 5 {
		t.Errorf("the report is\n%s", strings.Join(report(results), "\n"))
	}
	if len(d.Folders) != 0 {
		t.Errorf("the renders' folders are left: %v", d.Folders)
	}
}

func TestCheckGivesAFolderThatCannotBeMadeAsEachRendersError(t *testing.T) {
	full := errors.New("no space left")
	results, _, _ := checked(t, open(t, acme()), func(_ *porttest.Disk, _ *porttest.Git, f *porttest.Folders) { f.Fail = full })
	for _, r := range results {
		if !errors.Is(r.Err, full) || r.Passed() {
			t.Errorf("%s: %v", r.Combination.Name(), r.Err)
		}
	}
}

func TestCheckStopsAtAnErrorEachReturns(t *testing.T) {
	w, d, _ := writer()
	stop := errors.New("the report cannot be written")
	n := 0
	err := open(t, acme()).Check(answers, &porttest.Folders{Disk: d}, w, porttest.Programs{"has": porttest.Has(d)}, func(Result) error {
		n++
		return stop
	})
	if !errors.Is(err, stop) || n != 1 {
		t.Errorf("Check = %v after %d results", err, n)
	}
}

func TestLinesReadEveryLineEnding(t *testing.T) {
	r := Ran{Output: []byte("one\r\n\r\ntwo\rthree\n")}
	if got := r.Lines(); !slices.Equal(got, []string{"one", "", "two", "three"}) {
		t.Errorf("Lines = %q", got)
	}
	if got := Lines("no line ending"); !slices.Equal(got, []string{"no line ending"}) {
		t.Errorf("Lines = %q", got)
	}
	if got := Lines(""); got != nil {
		t.Errorf("no output is %q", got)
	}
}

func TestAResultPassesOnlyWhenRenderedAndEveryCheckPassed(t *testing.T) {
	cases := map[bool]Result{
		true:  {Checks: []Ran{{Status: Passed}}},
		false: {Checks: []Ran{{Status: Passed}, {Status: Skipped}}},
	}
	for want, r := range cases {
		if r.Passed() != want {
			t.Errorf("%+v passed: %v", r, r.Passed())
		}
	}
	if (Result{Err: &HoldsRecord{File: project.RecordFile}}).Passed() {
		t.Error("a combination not rendered passed")
	}
}
