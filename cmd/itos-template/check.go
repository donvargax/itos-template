package main

import (
	"errors"
	"io"

	"github.com/donvargax/itos-template/internal/check"
	"github.com/donvargax/itos-template/internal/newproject"
)

// checkCmd is itos-template check: every combination a template's manifest
// allows rendered and checked (PLAN.md, principle 5). It never asks: it is
// a template's CI step.
type checkCmd struct {
	Template string   `arg:"" optional:"" help:"The template: anything git clone takes, a path or a URL. The folder check runs in when not given."`
	Answer   []string `help:"An answer to one of the template's questions, as name=answer; once for each. check never asks: every answer is given, or taken with --defaults." placeholder:"NAME=ANSWER" sep:"none"`
	Defaults bool     `help:"Take a missing answer's default." negatable:"" env:"ITOS_TEMPLATE_DEFAULTS"`
}

// Help is check's detail in its --help: its report and its exit codes
// (docs/CLI.md, rules 10 and 14).
func (c *checkCmd) Help() string {
	return `Renders every combination the template's manifest (itos-template.yaml) allows, each stack alone and each stack with every set of its features whose needs are chosen too, less those it lists as unsupported; each as new renders it, from the template's branch heads, in a temporary folder removed after its checks. In each render it runs the checks the manifest names, the root's, then the stack's, then the features' in the manifest's order, with no shell, the answers in place of the literals in their words. A combination's checks stop at the first that fails; every combination is checked whatever failed before it.

The report, on stdout, gives each combination a line, "go + cli: passed" or "go + cli: failed", then a line for each check as it ran, indented two spaces: "passed: <words>", "failed: <words>" followed by its output, each line of it indented four spaces after a "|", or "skipped: <words>" after a failure; a render that failed shows "not rendered" and why. An empty line and the count of the combinations that passed end it. docs/manifest.md describes it whole.

Exit codes: 0 every combination rendered and every check passed; 1 a check or a render failed; 2 a usage or manifest error: an answer missing or not one its question takes, a manifest refused; 3 git cannot reach the template, or cannot run, or a render could not be written; 70 an internal error.

Examples:
  itos-template check --answer name=blue-fox --defaults
  itos-template check https://github.com/you/template.git --answer name=blue-fox --answer module=example.com/blue/fox

Report issues at https://github.com/donvargax/itos-template/issues.`
}

func (c *checkCmd) run(stdout, stderr io.Writer) int {
	template := c.Template
	if template == "" {
		template = "."
	}
	code, err := check.Run(check.Options{Template: template, Answers: c.Answer, Defaults: c.Defaults}, stdout)
	if err != nil {
		var f *newproject.Failure
		if !errors.As(err, &f) {
			f = &newproject.Failure{Code: exitInternal, Problems: []newproject.Problem{{Rule: "internal", Message: err.Error()}}}
		}
		for _, p := range f.Problems {
			fail(stderr, errors.New(p.Message))
		}
		return f.Code
	}
	return code
}
