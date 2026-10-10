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
		// ID-NEW-60 and ID-NEW-62 read these refusals' codes and the version
		// and branch they name, not their rules; no scenario's template has
		// no release a --ref names, nor two branches without their tags.
		{&template.NoRelease{Version: "v9.9.9"}, 2, "release-unknown", `the template has no release "v9.9.9": it has none; a release is one version tagged <branch>/<version> on every branch its manifest lists`},
		{&template.NoRelease{Version: "v9.9.9", Releases: []string{"v1.0.0"}}, 2, "release-unknown", `the template has no release "v9.9.9": its releases are v1.0.0; a release is one version tagged <branch>/<version> on every branch its manifest lists`},
		{&template.Incomplete{Version: "v1.1.0", Branches: []string{"go/cli", "go/web"}}, 1, "release-incomplete", "the template's release v1.1.0 is incomplete: no tag go/cli/v1.1.0 on the branch go/cli, no tag go/web/v1.1.0 on the branch go/web; a release tags every branch its manifest lists <branch>/<version>: tag each in the template, or name another release with --ref"},
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

// An answer not taken is quoted with every character outside ASCII
// escaped: a Hangul filler is printable to Go, so plain quoting would show
// the very character refused.
func TestANotTakenAnswerIsQuotedEscaped(t *testing.T) {
	err := &answer.NotTaken{Name: "module", Answer: "blue\u3164fox", Reason: errors.New("cause")}
	if got, want := Message(err), `the answer to module, "blue\u3164fox", is not one it takes: cause`; got != want {
		t.Errorf("NotTaken says\n%s, not\n%s", got, want)
	}
}

// Asking happens only on a terminal, which no scenario has: the wording of
// what the domain asks, held whole here, byte for byte, through the domain
// asking it and prompt asking what this words.
func TestTheAskerWordsWhatTheDomainAsks(t *testing.T) {
	m, err := manifest.Parse([]byte(`version: 2
stacks:
  - name: go
  - name: python
questions:
  - name: owner
    literal: Acme Corp
    question: Owner?
    default: Nobody
`))
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	ui := &UI{In: strings.NewReader("rust\ngo\n\n"), Stderr: &stderr, Terminal: true}
	c, answers, err := template.Choose(m, manifest.Choice{}, nil, false, ui.Asker())
	if err != nil || c.Name() != "go" || answers["owner"] != "Nobody" {
		t.Fatalf("Choose = %s, %v, %v", c.Name(), answers, err)
	}
	want := "Which stack (go, python)? " +
		"  \"rust\" is not an answer it takes: the stacks are go, python\n" +
		"Which stack (go, python)? " +
		"Owner? [Nobody] "
	if stderr.String() != want {
		t.Errorf("asked\n%q, not\n%q", stderr.String(), want)
	}
	if (&UI{Terminal: false}).Asker() != nil {
		t.Error("an asker with no terminal")
	}
}

// A refusal echoing what the person gave, an answer with no =, a question
// the template does not ask, or a --stack or a --feature it does not have,
// quotes it with every character outside ASCII escaped, as an answer not
// taken is: Go prints a Hangul filler as itself under %q, and a right-to-left
// override reaching a terminal raw can turn the rest of the line around
// (bug-5). ID-NEW-54 reads the first two; the sentences are checked whole.
func TestARefusalEchoesWhatThePersonGaveEscaped(t *testing.T) {
	questions := []string{"name", "module"}
	cases := []struct {
		err     error
		message string
	}{
		{&answer.Malformed{Given: "blue\u3164fox"}, `--answer takes name=answer, and "blue\u3164fox" has no =`},
		{&answer.Malformed{Given: "blue\u202efox"}, `--answer takes name=answer, and "blue\u202efox" has no =`},
		{&answer.Unknown{Name: "na\u3164me", Questions: questions}, `the template asks no question "na\u3164me": its questions are name, module`},
		{&answer.Unknown{Name: "na\u202eme", Questions: questions}, `the template asks no question "na\u202eme": its questions are name, module`},
		{&template.UnknownStack{Name: "g\u3164o", Stacks: []string{"go", "python"}}, `the template has no stack "g\u3164o": its stacks are go, python`},
		{&template.UnknownStack{Name: "g\u202eo", Stacks: []string{"go", "python"}}, `the template has no stack "g\u202eo": its stacks are go, python`},
		{&template.UnknownFeature{Name: "c\u3164li", Stack: "go"}, `the template has no feature "c\u3164li"`},
		{&template.UnknownFeature{Name: "c\u202eli", Stack: "go"}, `the template has no feature "c\u202eli"`},
		{&template.UnknownFeature{Name: "c\u3164li", Stack: "go", Known: []string{"cli", "web"}}, `the template has no feature "c\u3164li": the stack go's features are cli, web`},
		{&template.UnknownFeature{Name: "c\u202eli", Stack: "go", Known: []string{"cli", "web"}}, `the template has no feature "c\u202eli": the stack go's features are cli, web`},
	}
	for _, c := range cases {
		if got := Message(c.err); got != c.message {
			t.Errorf("%T says\n%+q, not\n%+q", c.err, got, c.message)
		}
	}
}

// A template URL's credential never reaches what we print (bug-4): not
// through the name it was given, nor through what git said of it, which a
// git that echoes the userinfo, or a git command failing with nothing on
// its standard error and so named by its arguments, would carry. ID-NEW-41
// and ID-CHECK-23 hold the name on the real git, whose fatal line leaves
// the userinfo out; the rest is held here, on stderr, in --json and in the
// message check's report shows, with the scenarios' fake token.
func TestNoMessageEchoesATemplateURLsCredential(t *testing.T) {
	const token = "ghp_EXAMPLETOKENNOTREAL"
	name := "https://x-access-token:" + token + "@127.0.0.1:1/acme.git"
	echoed := &git.Failed{Args: []string{"clone"}, Code: 128, Stderr: "fatal: unable to access '" + name + "/': refused\n"}
	silent := &git.Failed{Args: []string{"clone", "--bare", "--quiet", "--", name, "t.git"}, Code: 128}
	cases := []struct {
		err     error
		message string
	}{
		{&git.Unreachable{Name: name, Err: echoed}, "git cannot reach the template https://127.0.0.1:1/acme.git: fatal: unable to access 'https://127.0.0.1:1/acme.git/': refused. Name a path or a URL git clone takes."},
		{&git.Unreachable{Name: name, Err: silent}, "git cannot reach the template https://127.0.0.1:1/acme.git: git clone --bare --quiet -- https://127.0.0.1:1/acme.git t.git exited 128. Name a path or a URL git clone takes."},
		{&template.NoRoot{Template: name}, "the template https://127.0.0.1:1/acme.git has no default branch to read its manifest, itos-template.yaml, from"},
		{&template.EmptyRoot{Template: name, Root: "main"}, "the template https://127.0.0.1:1/acme.git's default branch, main, has no commit to read its manifest, itos-template.yaml, from"},
		{&template.NoManifest{Template: name, Root: "main"}, "the template https://127.0.0.1:1/acme.git has no itos-template.yaml on its root branch, main: a template names its stacks, features and questions there (docs/manifest.md)"},
		{silent, "git clone --bare --quiet -- https://127.0.0.1:1/acme.git t.git exited 128"},
		{fmt.Errorf("cloning %s: %w", name, errors.New("cause")), "cloning https://127.0.0.1:1/acme.git: cause"},
		{errors.Join(&template.NoRoot{Template: name}, &template.NoRoot{Template: name}), "the template https://127.0.0.1:1/acme.git has no default branch to read its manifest, itos-template.yaml, from\nthe template https://127.0.0.1:1/acme.git has no default branch to read its manifest, itos-template.yaml, from"},
	}
	for _, c := range cases {
		if got := Message(c.err); got != c.message {
			t.Errorf("%T says\n%s, not\n%s", c.err, got, c.message)
		}
		var stdout, stderr strings.Builder
		(&UI{Stdout: &stdout, Stderr: &stderr}).Fail(c.err, true)
		if strings.Contains(stdout.String(), token) || strings.Contains(stderr.String(), token) {
			t.Errorf("%T prints the token: --json %s, stderr %s", c.err, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr strings.Builder
	(&UI{Stdout: &stdout, Stderr: &stderr}).Usage(fmt.Errorf("unexpected argument %s", name), true)
	if want := "itos-template: unexpected argument https://127.0.0.1:1/acme.git\n"; stderr.String() != want {
		t.Errorf("a usage error says\n%s, not\n%s", stderr.String(), want)
	}
	if strings.Contains(stdout.String(), token) {
		t.Errorf("a usage error's --json prints the token: %s", stdout.String())
	}
}
