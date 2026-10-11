package adopt

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/adoption"
	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/cli"
	"github.com/donvargax/itos-template/internal/project"
)

// Decision 19 leaves the UI to the scenarios. What stays here is the
// preview's wording no scenario reads: it is for people, never the
// contract (docs/CLI.md), and the scenarios' fixtures hold no symbolic
// link or submodule, nor a template with no feature or no question.

func TestGitModeNamesEachModeAsGitWritesIt(t *testing.T) {
	for mode, want := range map[fs.FileMode]string{
		0o644: "100644", 0o755: "100755", 0o700: "100755",
		fs.ModeSymlink | 0o777: "120000", fs.ModeIrregular: "160000",
	} {
		if got := gitMode(mode); got != want {
			t.Errorf("gitMode(%v) = %s, not %s", mode, got, want)
		}
	}
}

func TestADifferencesNotesSayBinaryAndItsModes(t *testing.T) {
	for _, c := range []struct {
		d    adoption.Difference
		want string
	}{
		{adoption.Difference{}, ""},
		{adoption.Difference{Binary: true}, " (binary)"},
		{adoption.Difference{ModeChanged: true, From: 0o644, To: 0o755}, " (mode 100644 to 100755)"},
		{adoption.Difference{Binary: true, ModeChanged: true, From: 0o644, To: fs.ModeSymlink}, " (binary, mode 100644 to 120000)"},
	} {
		if got := notes(c.d); got != c.want {
			t.Errorf("notes(%+v) = %q, not %q", c.d, got, c.want)
		}
	}
}

func TestThePreviewNamesTheBaselineAndWhatWasDone(t *testing.T) {
	show := func(a *adoption.Adoption, dryRun bool) string {
		var out strings.Builder
		preview(&cli.UI{Stdout: &out}, a, "v1.0.0", dryRun, false)
		return out.String()
	}
	bare := &adoption.Adoption{Written: true, Baseline: &project.Record{Template: "/t", Release: "v1.0.0", Stack: "go"}}
	got := show(bare, false)
	for _, want := range []string{"the stack go, no features.", "Its answers: none.", "Wrote " + project.RecordFile} {
		if !strings.Contains(got, want) {
			t.Errorf("the preview does not say %q:\n%s", want, got)
		}
	}
	full := &adoption.Adoption{Dirty: true, Baseline: &project.Record{
		Template: "/t", Stack: "go", Features: []string{"cli", "web"}, Answers: answer.Set{"name": "blue-fox", "module": "m"},
		Commits: map[string]string{"stack/go": "2", "main": "1"},
	}}
	got = show(full, true)
	for _, want := range []string{"the features cli, web.", "Its answers: module=m, name=blue-fox.", "  main 1\n  stack/go 2\n", "dirty", "A dry run: nothing written."} {
		if !strings.Contains(got, want) {
			t.Errorf("the preview does not say %q:\n%s", want, got)
		}
	}
	one := &adoption.Adoption{Baseline: &project.Record{Template: "/t", Stack: "go", Features: []string{"cli"}, Answers: answer.Set{"name": "blue-fox"}}}
	got = show(one, false)
	for _, want := range []string{"the features cli.", "Its answers: name=blue-fox."} {
		if !strings.Contains(got, want) {
			t.Errorf("the preview does not say %q:\n%s", want, got)
		}
	}
	if got := show(full, false); strings.Contains(got, "Wrote") || strings.Contains(got, "dry run") {
		t.Errorf("a refusal's preview says it wrote, or ran dry:\n%s", got)
	}
}

// A template URL holding a credential is recorded without it, which adopt
// says once on stderr, as new does; no adopt scenario names such a URL.
func TestAdoptSaysWhenTheRecordLeavesACredentialOut(t *testing.T) {
	var stderr strings.Builder
	credentialLeftOut(&cli.UI{Stderr: &stderr})("https://host/acme.git")
	want := "itos-template: the record names the template https://host/acme.git, the credential its URL held left out: itos-template update will reach the template through git's credential helper (git help credentials)\n"
	if stderr.String() != want {
		t.Errorf("adopt says %q", stderr.String())
	}
}

// A merge's preview says it merged and staged the baseline, never that it
// wrote the record unstaged, which no scenario reads.
func TestAMergesPreviewSaysItMergedAndStaged(t *testing.T) {
	var out strings.Builder
	a := &adoption.Adoption{Written: true, Baseline: &project.Record{Template: "/t", Release: "v1.0.0", Stack: "go"}}
	preview(&cli.UI{Stdout: &out}, a, "v1.0.0", false, true)
	got := out.String()
	if !strings.Contains(got, "Merged the baseline into the project with "+project.RecordFile+", staged and uncommitted") ||
		!strings.Contains(got, "git reset --hard") || strings.Contains(got, "unstaged") {
		t.Errorf("a merge's preview says\n%s", got)
	}
	out.Reset()
	a.Written = false
	preview(&cli.UI{Stdout: &out}, a, "v1.0.0", true, true)
	if got := out.String(); strings.Contains(got, "Merged") || !strings.Contains(got, "A dry run: nothing written.") {
		t.Errorf("a dry run's preview says it merged:\n%s", got)
	}
}

// adopt's --json object when it stopped before rendering the baseline,
// here at a template's name, which no scenario reads: baseline null, and
// the differences and conflicts empty lists, never null, as when it renders.
func TestAdoptsJSONBeforeTheBaselineHoldsEmptyLists(t *testing.T) {
	var stdout, stderr strings.Builder
	code := (&CLI{Template: "acme\twidget", Merge: true, JSON: true}).Run(&cli.UI{Stdout: &stdout, Stderr: &stderr})
	want := `{"schema":1,"ok":false,"written":false,"dirty":false,"baseline":null,"differences":[],"conflicts":[],"problems":[{"rule":"template-name",`
	if code != cli.CodeUsage || !strings.HasPrefix(stdout.String(), want) {
		t.Errorf("adopt --json exited %d with\n%s", code, stdout.String())
	}
}

// The preview names the paths the baseline declines, which no scenario
// reads, and says nothing of them when it declines none.
func TestThePreviewNamesTheDeclinedPaths(t *testing.T) {
	var out strings.Builder
	a := &adoption.Adoption{Baseline: &project.Record{Template: "/t", Stack: "go", Declined: []string{"docs", "cli.txt"}}}
	preview(&cli.UI{Stdout: &out}, a, "v1.0.0", true, true)
	if got := out.String(); !strings.Contains(got, "Its declined paths, left out of the comparison and the merge: docs, cli.txt.\n") {
		t.Errorf("the preview says\n%s", got)
	}
	out.Reset()
	a.Baseline.Declined = nil
	preview(&cli.UI{Stdout: &out}, a, "v1.0.0", true, true)
	if got := out.String(); strings.Contains(got, "declined") {
		t.Errorf("a preview declining nothing says\n%s", got)
	}
}
