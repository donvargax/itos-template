// Package check is the slice of itos-template check (decision 16): its
// command as kong reads it, the slice's input with no copy of its flags,
// and the proof of a template. It renders every combination the template's
// manifest allows, each as new renders it (the same code, package
// template's Render) in a temporary folder of its own, runs that render's
// checks in it and removes it. It imports no other slice.
//
// A check is a list of words run with no shell (os/exec), so it means the
// same on every system; each word has the literals replaced by the answers
// as a text file's contents are. A combination's checks stop at the first
// that fails, as a CI job's steps do: what follows a failed build only
// fails after it. Every combination is checked whatever failed before it.
//
// The report goes to stdout, one block per combination as it is checked,
// in the format docs/manifest.md gives ("What check reports").
package check

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"unicode"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/problem"
	"github.com/donvargax/itos-template/internal/render"
	"github.com/donvargax/itos-template/internal/tempdir"
	"github.com/donvargax/itos-template/internal/template"
)

// Command is itos-template check: its flags, the slice's input as kong
// reads it (decision 4). It never asks: it is a template's CI step.
type Command struct {
	Template string   `arg:"" optional:"" help:"The template: anything git clone takes, a path or a URL. The folder check runs in when not given."`
	Answer   []string `help:"An answer to one of the template's questions, as name=answer; once for each. check never asks: every answer is given, or taken with --defaults." placeholder:"NAME=ANSWER" sep:"none"`
	Defaults bool     `help:"Take a missing answer's default." negatable:"" env:"ITOS_TEMPLATE_DEFAULTS"`
}

// Help is check's detail in its --help: its report and its exit codes
// (docs/CLI.md, rules 10 and 14).
func (c *Command) Help() string {
	return `Renders every combination the template's manifest (itos-template.yaml) allows, each stack alone and each stack with every set of its features whose needs are chosen too, less those it lists as unsupported; each as new renders it, from the template's branch heads, in a temporary folder removed after its checks. In each render it runs the checks the manifest names, the root's, then the stack's, then the features' in the manifest's order, with no shell, the answers in place of the literals in their words. A combination's checks stop at the first that fails; every combination is checked whatever failed before it.

The report, on stdout, gives each combination a line, "go + cli: passed" or "go + cli: failed", then a line for each check as it ran, indented two spaces: "passed: <words>", "failed: <words>" followed by its output, each line of it indented four spaces after a "|", or "skipped: <words>" after a failure; a render that failed shows "not rendered" and why. An empty line and the count of the combinations that passed end it. docs/manifest.md describes it whole.

Exit codes: 0 every combination rendered and every check passed; 1 a check or a render failed; 2 a usage or manifest error: an answer missing or not one its question takes, a manifest refused; 3 git cannot reach the template, or cannot run, or a render could not be written; 70 an internal error.

Examples:
  itos-template check --answer name=blue-fox --defaults
  itos-template check https://github.com/you/template.git --answer name=blue-fox --answer module=example.com/blue/fox

Report issues at https://github.com/donvargax/itos-template/issues.`
}

// Run checks the template, its report on stdout, and returns check's exit
// code; a failure before any combination is rendered is reported on stderr.
func (c *Command) Run(stdout, stderr io.Writer) int {
	code, err := c.check(stdout)
	if err != nil {
		return problem.As(err).Report(stdout, stderr, false)
	}
	return code
}

// identity is who each render's first commit is by: a render no one keeps,
// so a CI with no git identity configured can check a template.
var identity = []string{
	"GIT_AUTHOR_NAME=itos-template check", "GIT_AUTHOR_EMAIL=itos-template@localhost",
	"GIT_COMMITTER_NAME=itos-template check", "GIT_COMMITTER_EMAIL=itos-template@localhost",
}

// check checks the template c names, the folder it runs in when none,
// writing the report to report, and returns check's exit code. A failure
// before any combination is rendered (the template unreachable, its
// manifest refused, an answer missing) is a *problem.Failure, nothing
// reported.
func (c *Command) check(report io.Writer) (int, error) {
	name := c.Template
	if name == "" {
		name = "."
	}
	src, err := template.Open(name)
	if err != nil {
		return 0, err
	}
	defer src.Close()
	answers, err := answer.Resolve(src.Manifest, c.Answer, c.Defaults)
	if err != nil {
		return 0, err
	}
	replacer := render.NewReplacer(src.Manifest.Replacements(answers))

	worst := 0 // the exit code: 0, else 1, else 3 or 70 for a render the environment or a defect of ours stopped
	passed := 0
	combinations := src.Manifest.Combinations()
	for _, combination := range combinations {
		result := checkCombination(src, combination, answers, replacer)
		if result.passed() {
			passed++
		} else {
			worst = max(worst, problem.CodeRefused)
			if result.renderCode == problem.CodeEnvironment || result.renderCode == problem.CodeInternal {
				worst = max(worst, result.renderCode)
			}
		}
		if _, err := io.WriteString(report, result.String()); err != nil {
			return problem.CodeInternal, fmt.Errorf("cannot write the report: %w", err)
		}
	}
	noun := "combinations"
	if len(combinations) == 1 {
		noun = "combination"
	}
	if _, err := fmt.Fprintf(report, "\n%d of %d %s passed.\n", passed, len(combinations), noun); err != nil {
		return problem.CodeInternal, fmt.Errorf("cannot write the report: %w", err)
	}
	return worst, nil
}

// combinationResult is what checking one combination found.
type combinationResult struct {
	name       string
	renderErr  string // why the render failed, when it did
	renderCode int    // the render failure's exit code
	checks     []checkResult
}

type checkResult struct {
	words  []string // as it ran, the answers in place
	status string   // passed, failed or skipped
	output []byte   // what a failed check wrote, stdout and stderr as they came
}

func (r *combinationResult) passed() bool {
	if r.renderErr != "" {
		return false
	}
	for _, c := range r.checks {
		if c.status != "passed" {
			return false
		}
	}
	return true
}

// checkCombination renders c into a temporary folder, runs its checks there
// and removes it.
func checkCombination(src *template.Template, c manifest.Combination, answers map[string]string, r *render.Replacer) *combinationResult {
	result := &combinationResult{name: c.Name()}
	dir, err := tempdir.Make("itos-template-check-")
	if err != nil {
		result.renderErr = fmt.Sprintf("cannot make a temporary folder: %v", err)
		result.renderCode = problem.CodeEnvironment
		return result
	}
	defer func() {
		if err := tempdir.Remove(dir); err != nil {
			slog.Warn("cannot remove a render's temporary folder", "folder", dir, "error", err)
		}
	}()
	if _, err := src.Render(c, answers, dir, false, identity); err != nil {
		result.renderCode = problem.As(err).Code
		result.renderErr = err.Error()
		return result
	}
	failed := false
	for _, check := range src.Manifest.ChecksOf(c) {
		words := make([]string, len(check))
		for i, w := range check {
			words[i] = r.Text(w)
		}
		if failed {
			result.checks = append(result.checks, checkResult{words: words, status: "skipped"})
			continue
		}
		output, ok := runCheck(dir, words)
		status := "passed"
		if !ok {
			status, failed = "failed", true
		}
		result.checks = append(result.checks, checkResult{words: words, status: status, output: output})
	}
	return result
}

// runCheck runs words in dir, with no shell, and returns what it wrote and
// whether it exited 0. A program that cannot be started fails, its output
// the reason.
func runCheck(dir string, words []string) ([]byte, bool) {
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Dir = dir
	cmd.Env = git.Environ()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err == nil {
		return out.Bytes(), true
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		fmt.Fprintf(&out, "cannot run %s: %v\n", words[0], err)
	}
	return out.Bytes(), false
}

// String is the combination's block of the report: its name and whether it
// passed, then either "not rendered" and why, or each check, passed, failed
// or skipped, a failed one followed by its output, each line of it after
// "    |".
func (r *combinationResult) String() string {
	var b strings.Builder
	status := "passed"
	if !r.passed() {
		status = "failed"
	}
	fmt.Fprintf(&b, "%s: %s\n", r.name, status)
	if r.renderErr != "" {
		b.WriteString("  not rendered\n")
		writeOutput(&b, []byte(r.renderErr))
	}
	for _, c := range r.checks {
		fmt.Fprintf(&b, "  %s: %s\n", c.status, Words(c.words))
		if c.status == "failed" {
			writeOutput(&b, c.output)
		}
	}
	return b.String()
}

// writeOutput writes each line of out after "    |", a space before the
// line when it is not empty; a line ending is \n, \r\n or \r alone.
func writeOutput(b *strings.Builder, out []byte) {
	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return
	}
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			b.WriteString("    |\n")
			continue
		}
		b.WriteString("    | " + line + "\n")
	}
}

// Words writes a check's words as the report shows it: joined by spaces,
// a word written as it is unless it is empty or holds a space, a quote, a
// backslash or a character that does not print, which is written in double
// quotes as Go writes a string.
func Words(words []string) string {
	shown := make([]string, len(words))
	for i, w := range words {
		shown[i] = w
		if w == "" || strings.ContainsFunc(w, func(r rune) bool {
			return unicode.IsSpace(r) || r == '"' || r == '\'' || r == '\\' || !unicode.IsPrint(r)
		}) {
			shown[i] = strconv.Quote(w)
		}
	}
	return strings.Join(shown, " ")
}
