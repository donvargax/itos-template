// Package check is the slice of itos-template check (decisions 16 and 17),
// in two thin layers over the domain, which does the work: package
// template's Check renders every combination the template's manifest
// allows, each as new renders it, scans it for leftover literals, runs its
// checks and gives each its result.
//
// Its UI is CLI, its flags as kong reads them, and Run, which assembles the
// Query from them, hands it to Handle and writes the report, a block per
// combination as it is checked, in the format docs/manifest.md gives ("What
// check reports"), and the exit code the results make. Its application is
// Query, a CQRS query of domain objects, and Handle, which only wires infra
// (git, disk, tempdir, program) to the domain (template, answer) and calls
// it in order. It imports no other slice.
package check

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/cli"
	"github.com/donvargax/itos-template/internal/disk"
	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/program"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/tempdir"
	"github.com/donvargax/itos-template/internal/template"
)

// ── UI ──

// CLI is itos-template check as kong reads it (decision 4): its flags. It
// never asks: it is a template's CI step.
type CLI struct {
	Template string   `arg:"" optional:"" help:"The template: anything git clone takes, a path or a URL. When not given, the repository check runs in, from its top, each branch its local one or else origin's."`
	Answer   []string `help:"An answer to one of the template's questions, as name=answer; once for each. check never asks: every answer is given, or taken with --defaults." placeholder:"NAME=ANSWER" sep:"none"`
	Ref      string   `help:"The template's release to check, by its version (v1.2.0), a pre-release too. Without it, the branch heads, which itos-template release will tag." placeholder:"VERSION"`
	Defaults bool     `help:"Take a missing answer's default." negatable:"" env:"ITOS_TEMPLATE_DEFAULTS"`
}

// Help is check's detail in its --help: its report and its exit codes
// (docs/CLI.md, rules 10 and 14).
func (c *CLI) Help() string {
	return `Renders every combination the template's manifest (itos-template.yaml) allows, each stack alone and each stack with every set of its features whose needs are chosen too, less those it lists as unsupported; each as new renders it, from the template's branch heads, what itos-template release will tag, or with --ref from that release (a version tagged <branch>/<version> on every branch the manifest lists), in a temporary folder removed after its checks. Each render is scanned for leftovers, a literal kept in a form no answer replaced ("Acme Widget" for acme-widget), each failing its combination. In each render it runs the checks the manifest names, the root's, then the stack's, then the features' in the manifest's order, with no shell, the answers in place of the literals in their words. A combination's checks stop at the first that fails; every combination is checked whatever failed before it. When no check is marked as scanning the renders for credentials (scans: [credentials]), check warns on stderr.

The report, on stdout, gives each combination a line, "go + cli: passed" or "go + cli: failed", then a line for each leftover, indented two spaces: "leftover: "<text>" at <path>:<line>" or "leftover: "<text>" in the path <path>"; then a line for each check as it ran, indented two spaces: "passed: <words>", "failed: <words>" followed by its output, each line of it indented four spaces after a "|", or "skipped: <words>" after a failure; a render that failed shows "not rendered" and why. An empty line and the count of the combinations that passed end it. docs/manifest.md describes it whole.

Exit codes: 0 every combination rendered, kept no literal and passed every check; 1 a render, a scan or a check failed, or the release --ref names is incomplete; 2 a usage or manifest error: a --ref no release has, an answer missing or not one its question takes, a manifest refused; 3 git cannot reach the template, or cannot run, or a render could not be written; 70 an internal error.

Examples:
  itos-template check --answer name=blue-fox --defaults
  itos-template check https://github.com/you/template.git --answer name=blue-fox --answer module=example.com/blue/fox
  itos-template check --answer name=blue-fox --defaults --ref v1.2.0

Report issues at https://github.com/donvargax/itos-template/issues.`
}

// Run checks the template, its report on stdout, and returns check's exit
// code: 0 when every combination passed, else 1, or 3 or 70 when the
// environment or a defect of ours stopped a render. A failure before any
// combination is checked is reported on stderr.
func (c *CLI) Run(ui *cli.UI) int {
	worst, passed, all := 0, 0, 0
	warn := func(m *manifest.Manifest) {
		if !m.Scans(manifest.Credentials) {
			ui.Line("warning: no check of the template is marked as scanning its renders for credentials: mark the one that does with scans: [" + manifest.Credentials + "], in a check's long form (docs/manifest.md)")
		}
	}
	err := Handle(Query{Template: c.Template, Answers: answer.Given(c.Answer), Defaults: c.Defaults, Ref: c.Ref}, warn, func(r template.Result) error {
		all++
		if r.Passed() {
			passed++
		}
		worst = max(worst, exitCode(r))
		if _, err := io.WriteString(ui.Stdout, block(r)); err != nil {
			return fmt.Errorf("cannot write the report: %w", err)
		}
		return nil
	})
	if err == nil {
		if _, werr := io.WriteString(ui.Stdout, summary(passed, all)); werr != nil {
			err = fmt.Errorf("cannot write the report: %w", werr)
		}
	}
	if err != nil {
		return ui.Fail(err, false)
	}
	return worst
}

// exitCode is the exit code r makes check end with: 0 when it passed, else
// 1, or 3 or 70 when the environment or a defect of ours stopped its
// render.
func exitCode(r template.Result) int {
	if r.Passed() {
		return 0
	}
	if r.Err != nil {
		if code := cli.Code(r.Err); code == cli.CodeEnvironment || code == cli.CodeInternal {
			return code
		}
	}
	return cli.CodeRefused
}

// summary is the report's end: an empty line and the count of the
// combinations that passed of all.
func summary(passed, all int) string {
	noun := "combinations"
	if all == 1 {
		noun = "combination"
	}
	return fmt.Sprintf("\n%d of %d %s passed.\n", passed, all, noun)
}

// block is a combination's block of the report: its name and whether it
// passed, then either "not rendered" and why, or each literal its render
// kept and each check, passed, failed or skipped, a failed one followed by
// its output, each line of it after "    |". A leftover is the text found,
// in double quotes, then where: " at " and the path, a colon and the line,
// or " in the path " and the path, written as a check's word is.
func block(r template.Result) string {
	var b strings.Builder
	status := "passed"
	if !r.Passed() {
		status = "failed"
	}
	fmt.Fprintf(&b, "%s: %s\n", r.Combination.Name(), status)
	if r.Err != nil {
		b.WriteString("  not rendered\n")
		writeLines(&b, template.Lines(cli.Message(r.Err)))
	}
	for _, l := range r.Leftovers {
		path := manifest.Words{l.Path}.String()
		if l.Line == 0 {
			fmt.Fprintf(&b, "  leftover: %q in the path %s\n", l.Text, path)
		} else {
			fmt.Fprintf(&b, "  leftover: %q at %s:%d\n", l.Text, path, l.Line)
		}
	}
	for _, c := range r.Checks {
		switch c.Status {
		case template.Passed:
			fmt.Fprintf(&b, "  passed: %s\n", c.Check)
		case template.Failed:
			fmt.Fprintf(&b, "  failed: %s\n", c.Check)
			writeLines(&b, c.Lines())
		case template.Skipped:
			fmt.Fprintf(&b, "  skipped: %s\n", c.Check)
		}
	}
	return b.String()
}

// writeLines writes each line after "    |", a space before the line when
// it is not empty.
func writeLines(b *strings.Builder, lines []string) {
	for _, line := range lines {
		if line == "" {
			b.WriteString("    |\n")
			continue
		}
		b.WriteString("    | " + line + "\n")
	}
}

// ── Application ──

// Query is check's query: the results of checking the template Template,
// the repository check runs in when it is empty (git.CloneHere), at its
// release Ref or else its branch heads, with Answers, Defaults taking a
// missing answer's default.
type Query struct {
	Template string
	Answers  answer.Given
	Defaults bool
	Ref      string
}

// Handle checks the template q names, giving its manifest to opened once
// its answers are resolved, then each combination's result to each as it
// is found.
//
// git clones the template by its name as given; each render records it,
// in its .itos-template.yaml and its first commit, by the name
// template.Recorded gives, from the folder check runs in, on this system,
// as new records it (check-record-name-slice): a URL's credential left out,
// so a template's own credential scan finds none in its renders, and a
// relative path made absolute. A render is thrown away, so nothing is said
// of a credential left out, as new says it. The repository check runs in,
// when none is named, is git.Here from that repository's top (git.Top), so
// recorded as the top's absolute path whatever subfolder check runs in
// (check-here): a dot would name the render itself. A failure before any
// combination is checked (the template unreachable, its manifest refused,
// an answer missing, a release q.Ref names that the template lacks or that
// is incomplete) is returned, nothing given to opened or each, and so is an
// error each returns. Without q.Ref, check proves the branch heads, what
// itos-template release will tag, and says nothing of releases (decision
// 36).
func Handle(q Query, opened func(*manifest.Manifest), each func(template.Result) error) error {
	tmp, err := tempdir.Make("itos-template-template-")
	if err != nil {
		return err
	}
	defer tempdir.Discard(tmp)
	// from is the folder name is relative to.
	name, into := q.Template, filepath.Join(tmp, "template.git")
	var repo *git.Repo
	var from string
	if name == "" {
		name = git.Here
		if from, err = git.Top(); err == nil {
			repo, err = git.CloneHere(from, into)
		}
	} else if from, err = os.Getwd(); err == nil {
		repo, err = git.Clone(name, into)
	}
	if err != nil {
		return err
	}
	t, err := template.Open(name, repo)
	if err != nil {
		return err
	}
	t.Name, _ = template.Recorded(name, from, template.SystemOf(runtime.GOOS))
	if q.Ref != "" {
		if err := t.UseRelease(q.Ref); err != nil {
			return err
		}
	}
	answers, err := answer.Resolve(t.Manifest, q.Answers, q.Defaults)
	if err != nil {
		return err
	}
	opened(t.Manifest)
	w := project.Writer{Disk: disk.Disk{}, Git: git.Committer{}}
	return t.Check(answers, tempdir.Renders{}, w, program.Runner{Env: git.Environ()}, each)
}
