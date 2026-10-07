package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/problem"
)

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := git.Run(dir, args...); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func writeFile(t *testing.T, dir, p, data string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newTemplate makes a template whose manifest is text: main with the manifest
// and README, stack/sh adding bin/acme-widget, sh/extra adding extra.txt.
// No git identity is set but the template's own commits', as in a CI that
// configures none.
func newTemplate(t *testing.T, text string) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	c := func(args ...string) {
		t.Helper()
		g := git.Command{Dir: dir, Env: []string{"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@localhost", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@localhost"}}
		if _, err := g.Output(args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	run(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "itos-template.yaml", text)
	writeFile(t, dir, "README.md", "acme-widget\n")
	c("add", "-A")
	c("commit", "-q", "-m", "main")
	c("checkout", "-q", "-b", "stack/sh")
	writeFile(t, dir, "bin/acme-widget", "echo acme-widget\n")
	c("add", "-A")
	c("commit", "-q", "-m", "stack/sh")
	c("checkout", "-q", "-b", "sh/extra")
	writeFile(t, dir, "extra.txt", "extra\n")
	c("add", "-A")
	c("commit", "-q", "-m", "sh/extra")
	c("checkout", "-q", "main")
	return dir
}

// gitPath is the real git's path, with / on every system, as YAML takes it
// in double quotes.
func gitPath() string { return strings.ReplaceAll(git.Bin(), `\`, `/`) }

// manifestText is a manifest whose checks run git, the real one, by its path:
// an itos linked as git on the PATH never runs in a unit test.
func manifestText(extraChecks string) string {
	g := gitPath()
	return `version: 2
checks: [["` + g + `", ls-files, --error-unmatch, README.md]]
stacks:
  - name: sh
    checks: [["` + g + `", ls-files, --error-unmatch, bin/acme-widget]]
features:
  - name: extra
    stack: sh
    checks:
` + extraChecks + `
questions:
  - name: name
    literal: acme-widget
    question: Name?
    case_forms: true
`
}

func TestRunChecksEveryCombinationTheAnswersInPlace(t *testing.T) {
	tpl := newTemplate(t, manifestText(`      - ["`+gitPath()+`", ls-files, --error-unmatch, extra.txt]`))
	var report strings.Builder
	code, err := Run(Options{Template: tpl, Answers: []string{"name=blue-fox"}}, &report)
	if err != nil || code != 0 {
		t.Fatalf("Run = %d, %v\n%s", code, err, report.String())
	}
	g := Words([]string{gitPath()})
	want := "sh: passed\n" +
		"  passed: " + g + " ls-files --error-unmatch README.md\n" +
		"  passed: " + g + " ls-files --error-unmatch bin/blue-fox\n" +
		"sh + extra: passed\n" +
		"  passed: " + g + " ls-files --error-unmatch README.md\n" +
		"  passed: " + g + " ls-files --error-unmatch bin/blue-fox\n" +
		"  passed: " + g + " ls-files --error-unmatch extra.txt\n" +
		"\n2 of 2 combinations passed.\n"
	if report.String() != want {
		t.Errorf("the report is\n%s\nnot\n%s", report.String(), want)
	}
}

func TestRunStopsACombinationAtItsFirstFailedCheckAndChecksTheRest(t *testing.T) {
	tpl := newTemplate(t, manifestText(`      - [itos-template-no-such-program, x]
      - [also-not-run]`))
	var report strings.Builder
	code, err := Run(Options{Template: tpl, Answers: []string{"name=blue-fox"}}, &report)
	if err != nil || code != problem.CodeRefused {
		t.Fatalf("Run = %d, %v\n%s", code, err, report.String())
	}
	text := report.String()
	for _, want := range []string{
		"sh: passed\n",
		"sh + extra: failed\n",
		"  failed: itos-template-no-such-program x\n    | cannot run itos-template-no-such-program: ",
		"  skipped: also-not-run\n",
		"\n1 of 2 combinations passed.\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the report does not hold %q:\n%s", want, text)
		}
	}
}

func TestRunRefusesMissingAnswersBeforeReportingAnything(t *testing.T) {
	tpl := newTemplate(t, manifestText(`      - [x]`))
	var report strings.Builder
	_, err := Run(Options{Template: tpl}, &report)
	if f, ok := err.(*problem.Failure); !ok || f.Code != problem.CodeUsage {
		t.Fatalf("Run = %v", err)
	}
	if report.Len() != 0 {
		t.Errorf("a report was written:\n%s", report.String())
	}
}

func TestWordsQuotesAWordOnlyWhenItMustBe(t *testing.T) {
	got := Words([]string{"sh", "-c", "go test ./...", "", `a"b`, "tab\there", "plain/path.go"})
	want := `sh -c "go test ./..." "" "a\"b" "tab\there" plain/path.go`
	if got != want {
		t.Errorf("Words = %s, want %s", got, want)
	}
}

func TestTheReportWritesOutputLinesAfterABar(t *testing.T) {
	r := &combinationResult{name: "go + cli", checks: []checkResult{
		{words: []string{"go", "vet"}, status: "passed"},
		{words: []string{"go", "test"}, status: "failed", output: []byte("one\r\n\r\ntwo\rthree\n")},
		{words: []string{"go", "build"}, status: "skipped"},
	}}
	want := "go + cli: failed\n  passed: go vet\n  failed: go test\n    | one\n    |\n    | two\n    | three\n  skipped: go build\n"
	if got := r.String(); got != want {
		t.Errorf("the block is\n%q, not\n%q", got, want)
	}
	r = &combinationResult{name: "go", renderErr: "merging leaves conflicts"}
	if got, want := r.String(), "go: failed\n  not rendered\n    | merging leaves conflicts\n"; got != want {
		t.Errorf("the block is %q, not %q", got, want)
	}
}
