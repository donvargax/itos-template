package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/disk"
	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/render"
	"github.com/donvargax/itos-template/internal/tempdir"
	"github.com/donvargax/itos-template/internal/template"
)

// Decision 19 leaves the UI to the scenarios, and slice-3's read every
// kind a scenario can make by its exit code and its --json rule ID, the
// contract (docs/CLI.md), and what its message names. The kinds here are
// the ones no scenario reads, each saying why: what no scenario can make
// (asking on a terminal, git refusing a first commit, a disk or a
// temporary folder failing, internal errors), and six kinds a scenario
// could make but none reads the rule of yet (the idea outside-test-gaps).
// Their sentences are checked whole, as nothing else checks them.
func TestEachKindNoScenarioReadsHasItsCodeRuleAndSentence(t *testing.T) {
	name := &manifest.Question{Name: "name", Question: "Name?"}
	cause := errors.New("cause")
	failed := &git.Failed{Args: []string{"x"}, Code: 128, Stderr: "fatal: no\n"}
	cases := []struct {
		err           error
		code          int
		rule, message string
	}{
		// ID-CHECK-05 reads this refusal's code, not its rule.
		{&template.Unsupported{Combination: manifest.Combination{Stack: manifest.Stack{Name: "go"}, Features: []manifest.Feature{{Name: "cli"}}}}, 1, "combination-unsupported", "the template does not support go + cli: its manifest lists that combination as unsupported"},
		// ID-NEW-19 and ID-CHECK-08 read this refusal's code, not its rule.
		{&git.Unreachable{Name: "t", Err: failed}, 3, "template-unreachable", "git cannot reach the template t: fatal: no. Name a path or a URL git clone takes."},
		// ID-NEW-10 reads this refusal's code, not its rule; ID-NEW-29 reads
		// the rule of a file in the folder's place.
		{&project.NotEmpty{Folder: "made"}, 1, "folder-not-empty", "the folder made has files in it: new writes a project into a missing or empty folder"},
		// A template whose HEAD names no branch: no scenario's fixture has one.
		{&template.NoRoot{Template: "t"}, 2, "manifest-missing", "the template t has no default branch to read its manifest, itos-template.yaml, from"},
		// No fixture's literal is in a name an answer can make "..".
		{&render.BadName{Path: "acme/x", Would: "..", Element: ".."}, 2, "answer-name", `the template's acme/x would be named "..", which no file can be: the answers make it ".."`},
		// ID-NEW-33 reads this refusal's code, not its rule.
		{&render.FileIsFolder{File: "a", Folder: "b", Of: "c"}, 2, "answer-name", "the template's file a would be b, a folder of c: give answers that tell them apart"},
		// Asking happens only on a terminal, which no scenario has.
		{&answer.NotAnswered{Question: name, Err: cause}, 2, "answer-missing", "no answer to name (Name?): cause"},
		// git refuses a first commit only where a hook or a config of the
		// person's says so, and the harness gives git none.
		{&project.CommitRefused{Err: failed}, 1, "commit-refused", "git refused the project's first commit: fatal: no"},
		// A git command that should not fail, failing: a defect.
		{failed, 70, "internal", "fatal: no"},
		// The disk and the temporary folder fail where the system refuses,
		// which no scenario can make happen on all three systems.
		{&disk.Unreadable{Folder: "made", Err: cause}, 3, "folder-unreadable", "cannot read the folder made: cause"},
		{&disk.Unmakeable{Folder: "made", Err: cause}, 3, "folder-unwritable", "cannot make the folder made: cause"},
		{&disk.Unwritable{Folder: "made", Err: cause}, 3, "folder-unwritable", "cannot write the project in made: cause"},
		// windows refusing a symbolic link waits for template-special-files.
		{&disk.Unwritable{Folder: "made", Link: "run", Err: cause}, 3, "folder-unwritable", "cannot write the project in made: cannot make the symbolic link run: cause"},
		{&tempdir.Unmakeable{Err: cause}, 3, "temporary-folder", "cannot make a temporary folder: cause"},
		// An error no switch classifies: a defect.
		{fmt.Errorf("cannot write the report: %w", cause), 70, "internal", "cannot write the report: cause"},
	}
	for _, c := range cases {
		var stdout, stderr strings.Builder
		ui := &UI{Stdout: &stdout, Stderr: &stderr}
		if code := ui.Fail(c.err, true); code != c.code || Code(c.err) != c.code {
			t.Errorf("%T: exit %d, not %d", c.err, code, c.code)
		}
		if got := Message(c.err); got != c.message {
			t.Errorf("%T says\n%s, not\n%s", c.err, got, c.message)
		}
		wantErr := "itos-template: " + c.message + "\n"
		if stderr.String() != wantErr {
			t.Errorf("%T: stderr is\n%s", c.err, stderr.String())
		}
		if !strings.HasPrefix(stdout.String(), `{"schema":1,"ok":false,"problems":[{"rule":"`+c.rule+`","message":`) {
			t.Errorf("%T: --json is %s", c.err, stdout.String())
		}
	}
}
