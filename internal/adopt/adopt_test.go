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
		preview(&cli.UI{Stdout: &out}, a, "v1.0.0", dryRun)
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
