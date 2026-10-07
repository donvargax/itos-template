// Package newproject is the slice of itos-template new (decisions 16 and
// 17), in two thin layers over the domain, which does the work.
//
// Its UI is CLI, its flags as kong reads them, and Run, which assembles the
// Command from them (the one copy left, kong's cost), hands it to Handle and
// shows what comes back, the problems through internal/cli. Its application
// is Command, a CQRS command of domain objects, and Handle, which only wires
// infra (git, disk, tempdir) to the domain (project, template) and calls it
// in order. It imports no other slice. The package is not named new, which
// Go predeclares.
//
// Every check runs before anything is written to the project's folder, so a
// refusal leaves no folder behind (decision 12): the folder, then the
// template (cloned into a temporary folder), its manifest, the stack and
// the features, the answers, and what Render checks. Only then is the
// folder made and the render written, recorded (.itos-template.yaml,
// decision 10) and committed as the project's first commit; a failure
// while writing removes what was written.
package newproject

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/cli"
	"github.com/donvargax/itos-template/internal/disk"
	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/tempdir"
	"github.com/donvargax/itos-template/internal/template"
	"github.com/donvargax/itos-template/internal/template/port"
)

// ── UI ──

// CLI is itos-template new as kong reads it (decision 4): its flags.
type CLI struct {
	Template string   `arg:"" help:"The template: anything git clone takes, a path or a URL."`
	Folder   string   `arg:"" help:"The project's folder: one that does not exist, or an empty one."`
	Stack    string   `help:"The stack to render, by the name the template's manifest gives it. Asked on a terminal when not given." placeholder:"STACK"`
	Feature  []string `help:"A feature to merge onto the stack, by its name (cli) or its branch (go/cli); once for each. The features a feature needs are never added unasked." placeholder:"FEATURE" sep:"none"`
	Answer   []string `help:"An answer to one of the template's questions, as name=answer; once for each. An answer not given is asked on a terminal." placeholder:"NAME=ANSWER" sep:"none"`
	Defaults bool     `help:"Take a missing answer's default instead of asking or refusing." negatable:"" env:"ITOS_TEMPLATE_DEFAULTS"`
	JSON     bool     `name:"json" help:"Print the result as one JSON object on stdout." negatable:"" env:"ITOS_TEMPLATE_JSON"`
}

// Help is new's detail in its --help: its JSON and its exit codes
// (docs/CLI.md, rules 10 and 14).
func (c *CLI) Help() string {
	return `Renders the template's stack branch merged with the chosen features' branches, replaces each literal its manifest (itos-template.yaml) lists by its answer, in file contents and names and in every case form, and commits the render, with its record (.itos-template.yaml), as the new git repository's first commit. Nothing is written to the folder until every check has passed.

--json prints {"schema":1,"ok":true,"folder","template","stack","features":[…],"answers":{…},"commits":{"<branch>":"<sha>"},"commit":"<sha>"}, or {"schema":1,"ok":false,"problems":[{"rule","message"}]}.

Exit codes: 0 made; 1 refused: the folder has files in it, a feature of another stack, a feature whose needed feature is not chosen, a combination the manifest lists as unsupported, branches that do not merge cleanly; 2 a usage or manifest error: an unknown stack or feature, an answer missing (without a terminal) or not one its question takes; 3 git cannot reach the template, or cannot run; 70 an internal error.

Examples:
  itos-template new ../acme made --stack go --feature cli --answer name=blue-fox
  itos-template new https://github.com/you/template.git made --stack go --defaults

Report issues at https://github.com/donvargax/itos-template/issues.`
}

// Run makes the project and returns new's exit code: what it made on
// stdout, a line or with --json its object, or each problem on stderr. It
// asks on a terminal for what is missing, and only there.
func (c *CLI) Run(ui *cli.UI) int {
	p, err := Handle(Command{
		Template: c.Template,
		Folder:   c.Folder,
		Choice:   manifest.Choice{Stack: c.Stack, Features: c.Feature},
		Answers:  answer.Given(c.Answer),
		Defaults: c.Defaults,
	}, ui.Asker())
	if err != nil {
		return ui.Fail(err, c.JSON)
	}
	if c.JSON {
		ui.JSON(struct {
			Schema int  `json:"schema"`
			OK     bool `json:"ok"`
			*project.Project
		}{1, true, p})
		return 0
	}
	features := "no features"
	if len(p.Features) > 0 {
		features = "the features " + strings.Join(p.Features, ", ")
	}
	_, _ = fmt.Fprintf(ui.Stdout, "Made %s from %s: the stack %s, %s.\n", p.Folder, p.Template, p.Stack, features)
	return 0
}

// ── Application ──

// Command is new's command: make a project in Folder from the template
// Template, the combination Choice names, with Answers, Defaults taking a
// missing answer's default.
type Command struct {
	Template string
	Folder   string
	Choice   manifest.Choice
	Answers  answer.Given
	Defaults bool
}

// Handle makes the project c asks for, asking ask, when there is one, for
// what c leaves out.
func Handle(c Command, ask port.Asker) (*project.Project, error) {
	w := project.Writer{Disk: disk.Disk{}, Git: git.Committer{}}
	folder, err := w.Look(c.Folder)
	if err != nil {
		return nil, err
	}
	tmp, err := tempdir.Make("itos-template-template-")
	if err != nil {
		return nil, err
	}
	defer tempdir.Discard(tmp)
	repo, err := git.Clone(c.Template, filepath.Join(tmp, "template.git"))
	if err != nil {
		return nil, err
	}
	t, err := template.Open(c.Template, repo)
	if err != nil {
		return nil, err
	}
	combination, answers, err := template.Choose(t.Manifest, c.Choice, c.Answers, c.Defaults, ask)
	if err != nil {
		return nil, err
	}
	return t.Render(combination, answers, folder, nil, w)
}
