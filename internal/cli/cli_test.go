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

// Decision 18 leaves the UI to the scenarios, and they hold every exit code
// the help names; none yet reads --json's rule IDs, which are the contract
// (docs/CLI.md), nor most of the lines, so the kinds' codes, rules and
// sentences stay here until the idea scenario-gaps gives them scenarios.
func TestEachKindHasItsCodeRuleAndSentence(t *testing.T) {
	deflt := "example.com/you/project"
	module := &manifest.Question{Name: "module", Question: "Module?", Default: &deflt}
	name := &manifest.Question{Name: "name", Question: "Name?"}
	cause := errors.New("cause")
	failed := &git.Failed{Args: []string{"x"}, Code: 128, Stderr: "fatal: no\n"}
	cases := []struct {
		err           error
		code          int
		rule, message string
	}{
		{&template.NoStack{Stacks: []string{"go", "py"}}, 2, "stack-missing", "no stack chosen: name one with --stack (go, py)"},
		{&template.UnknownStack{Name: "rust", Stacks: []string{"go"}}, 2, "stack-unknown", "the template has no stack rust: its stacks are go"},
		{&template.UnknownFeature{Name: "x"}, 2, "feature-unknown", "the template has no feature x"},
		{&template.UnknownFeature{Name: "x", Stack: "go", Known: []string{"cli", "web"}}, 2, "feature-unknown", "the template has no feature x: the stack go's features are cli, web"},
		{&template.OtherStack{Feature: manifest.Feature{Name: "cli", Stack: "py"}, Stack: "go"}, 1, "feature-other-stack", "the feature py/cli is the stack py's, not the stack go's: a project has one stack's features"},
		{&template.Needs{Feature: "web", Need: "cli"}, 1, "feature-needs", "the feature web needs the feature cli: choose it too, with --feature cli"},
		{&template.Unsupported{Combination: manifest.Combination{Stack: manifest.Stack{Name: "go"}, Features: []manifest.Feature{{Name: "cli"}}}}, 1, "combination-unsupported", "the template does not support go + cli: its manifest lists that combination as unsupported"},
		{&template.NoRoot{Template: "t"}, 2, "manifest-missing", "the template t has no default branch to read its manifest, itos-template.yaml, from"},
		{&template.EmptyRoot{Template: "t", Root: "main"}, 2, "manifest-missing", "the template t's default branch, main, has no commit to read its manifest, itos-template.yaml, from"},
		{&template.NoManifest{Template: "t", Root: "main"}, 2, "manifest-missing", "the template t has no itos-template.yaml on its root branch, main: a template names its stacks, features and questions there (docs/manifest.md)"},
		{&template.ManifestInvalid{Root: "main", Problems: []string{"a", "b"}}, 2, "manifest-invalid", "the template's itos-template.yaml on main: a\nthe template's itos-template.yaml on main: b"},
		{&template.NoBranch{Branch: "stack/go"}, 2, "manifest-branch-missing", "the template's manifest lists go, but the template has no branch stack/go"},
		{&template.MergeConflict{Into: []string{"stack/go", "go/a"}, Branch: "go/c", Paths: []string{"a", "b"}}, 1, "merge-conflict", "merging the template's branch go/c into stack/go + go/a leaves conflicts in a, b: the template's branches must merge cleanly; merge them in the template and resolve them there"},
		{&template.HoldsRecord{File: ".itos-template.yaml"}, 1, "template-defect", "the template holds .itos-template.yaml, the file a made project records its render in: leave it out of the template"},
		{&render.BadName{Path: "acme/x", Would: "..", Element: ".."}, 2, "answer-name", `the template's acme/x would be named "..", which no file can be: the answers make it ".."`},
		{&render.SameName{First: "a", Second: "b", To: "c"}, 2, "answer-name", "the template's a and b would both be c: give answers that tell them apart"},
		{&render.FileIsFolder{File: "a", Folder: "b", Of: "c"}, 2, "answer-name", "the template's file a would be b, a folder of c: give answers that tell them apart"},
		{&render.Submodule{Path: "vendor/lib"}, 1, "template-defect", "the template holds the submodule vendor/lib, which new cannot render"},
		{&answer.Malformed{Given: "x"}, 2, "answer-malformed", `--answer takes name=answer, and "x" has no =`},
		{&answer.Unknown{Name: "zz", Questions: []string{"name", "module"}}, 2, "answer-unknown", "the template asks no question zz: its questions are name, module"},
		{&answer.Twice{Name: "name"}, 2, "answer-twice", "the answer to name is given twice"},
		{&answer.NotTaken{Name: "name", Answer: "Bad", Reason: cause}, 2, "answer-malformed", `the answer to name, "Bad", is not one it takes: cause`},
		{&answer.Missing{Question: name}, 2, "answer-missing", "no answer to name (Name?): give one with --answer name=<answer>"},
		{&answer.Missing{Question: module}, 2, "answer-missing", "no answer to module (Module?): give one with --answer module=<answer>, or take its default, example.com/you/project, with --defaults"},
		{&answer.NotAnswered{Question: name, Err: cause}, 2, "answer-missing", "no answer to name (Name?): cause"},
		{&project.NotEmpty{Folder: "made"}, 1, "folder-not-empty", "the folder made has files in it: new writes a project into a missing or empty folder"},
		{&project.NotFolder{Folder: "made"}, 1, "folder-not-empty", "made is a file: new writes a project into a missing or empty folder"},
		{&project.CommitRefused{Err: failed}, 1, "commit-refused", "git refused the project's first commit: fatal: no"},
		{&git.Missing{Err: cause}, 3, "git-missing", "cannot run git, which new clones, merges and commits with: install git, or put it on the PATH (cannot run git: cause)"},
		{&git.Unreachable{Name: "t", Err: failed}, 3, "template-unreachable", "git cannot reach the template t: fatal: no. Name a path or a URL git clone takes."},
		{failed, 70, "internal", "fatal: no"},
		{&git.NoIdentity{Err: failed}, 3, "git-identity", "git does not know who you are, so it cannot make the project's first commit: set user.name and user.email in git's config (fatal: no)"},
		{&disk.Unreadable{Folder: "made", Err: cause}, 3, "folder-unreadable", "cannot read the folder made: cause"},
		{&disk.Unmakeable{Folder: "made", Err: cause}, 3, "folder-unwritable", "cannot make the folder made: cause"},
		{&disk.Unwritable{Folder: "made", Err: cause}, 3, "folder-unwritable", "cannot write the project in made: cause"},
		{&disk.Unwritable{Folder: "made", Link: "run", Err: cause}, 3, "folder-unwritable", "cannot write the project in made: cannot make the symbolic link run: cause"},
		{&tempdir.Unmakeable{Err: cause}, 3, "temporary-folder", "cannot make a temporary folder: cause"},
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
		wantErr := "itos-template: " + strings.ReplaceAll(c.message, "\n", "\nitos-template: ") + "\n"
		if stderr.String() != wantErr {
			t.Errorf("%T: stderr is\n%s", c.err, stderr.String())
		}
		if !strings.HasPrefix(stdout.String(), `{"schema":1,"ok":false,"problems":[{"rule":"`+c.rule+`","message":`) {
			t.Errorf("%T: --json is %s", c.err, stdout.String())
		}
	}
}

func TestJoinedProblemsAreEachALineUnderTheHighestCode(t *testing.T) {
	err := errors.Join(&template.UnknownStack{Name: "rust", Stacks: []string{"go"}}, &answer.Twice{Name: "name"})
	var stdout, stderr strings.Builder
	if code := (&UI{Stdout: &stdout, Stderr: &stderr}).Fail(err, true); code != 2 {
		t.Errorf("exit %d", code)
	}
	want := `{"schema":1,"ok":false,"problems":[{"rule":"stack-unknown","message":"the template has no stack rust: its stacks are go"},{"rule":"answer-twice","message":"the answer to name is given twice"}]}` + "\n"
	if stdout.String() != want {
		t.Errorf("--json is %s", stdout.String())
	}
	if stderr.String() != "itos-template: the template has no stack rust: its stacks are go\nitos-template: the answer to name is given twice\n" {
		t.Errorf("stderr is %s", stderr.String())
	}
}
