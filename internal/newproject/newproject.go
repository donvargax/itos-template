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
//
// The setup steps the template lists for the combination are printed after,
// for the person to run, and never run here (decision 7, the running left
// to the idea new-trust): a step is the template's code, and new does not
// decide for the person to run it.
package newproject

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

git clones the template by the name given; the record, the line new prints, --json and the first commit name it by one update can reach: a relative path as the absolute path it names from the current folder, a URL with the credential it holds left out, which the error output then says, update reaching the template through git's credential helper (git help credentials).

The setup steps the manifest lists for the chosen branches, the root's, then the stack's, then the features', are printed after, one to a line and quoted for a POSIX shell, for you to run in the folder: new runs none of them. A template with no step prints none.

--json prints {"schema":1,"ok":true,"folder","template","stack","features":[…],"answers":{…},"commits":{"<branch>":"<sha>"},"commit":"<sha>"}, or {"schema":1,"ok":false,"problems":[{"rule","message"}]}; the setup steps then go to the error output, so the object stays alone.

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
	p, steps, err := Handle(Command{
		Template: c.Template,
		Folder:   c.Folder,
		Choice:   manifest.Choice{Stack: c.Stack, Features: c.Feature},
		Answers:  answer.Given(c.Answer),
		Defaults: c.Defaults,
	}, ui.Asker(), credentialLeftOut(ui))
	if err != nil {
		return ui.Fail(err, c.JSON)
	}
	return show(ui, p, steps, c.JSON)
}

// credentialLeftOut says, for a template whose URL held a credential the
// record leaves out, naming it as recorded, that update will reach it
// through git's credential helper: once, on the error output, with --json
// too, so stdout keeps the object alone (docs/CLI.md, rule 29).
func credentialLeftOut(ui *cli.UI) func(recorded string) {
	return func(recorded string) {
		ui.Line("the record names the template " + recorded + ", the credential its URL held left out: itos-template update will reach the template through git's credential helper (git help credentials)")
	}
}

// show shows what new made, p, and the setup steps it leaves the person to
// run: a line or with --json its object on stdout, then the steps.
func show(ui *cli.UI, p *project.Project, steps []manifest.Words, withJSON bool) int {
	if withJSON {
		ui.JSON(struct {
			Schema int  `json:"schema"`
			OK     bool `json:"ok"`
			*project.Project
		}{1, true, p})
		printSetup(ui.Stderr, p.Folder, steps)
		return 0
	}
	features := "no features"
	if len(p.Features) > 0 {
		features = "the features " + strings.Join(p.Features, ", ")
	}
	_, _ = fmt.Fprintf(ui.Stdout, "Made %s from %s: the stack %s, %s.\n", p.Folder, p.Template, p.Stack, features)
	printSetup(ui.Stdout, p.Folder, steps)
	return 0
}

// printSetup prints the template's setup steps to out, for the person to
// run in folder: a line heading them, then each step on a line of its own,
// its words quoted for a POSIX shell (manifest.Words.Shell) and nothing
// before them, so each line is what a shell should run. A template with no
// step prints nothing. On stdout beside what new made, or on stderr under
// --json, which keeps the object alone on stdout (docs/CLI.md, rule 29).
func printSetup(out io.Writer, folder string, steps []manifest.Words) {
	if len(steps) == 0 {
		return
	}
	_, _ = fmt.Fprintf(out, "itos-template ran none of the template's setup steps; run them in %s:\n", folder)
	for _, step := range steps {
		_, _ = fmt.Fprintf(out, "%s\n", step.Shell())
	}
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
// what c leaves out, and gives the setup steps its template lists for it,
// the answers in place, which it does not run.
//
// git clones the template by its name as given; the project records it by
// the name template.Recorded gives, from the folder new runs in, on this
// system, and when that left a credential out of its URL, leftOut is told
// the name, once the project is made. The clone, its origin naming the
// template as given, is a bare repository in a temporary folder, removed
// when new ends; the project is a repository of its own (git init), with no
// remote and nothing of the clone's.
func Handle(c Command, ask port.Asker, leftOut func(recorded string)) (*project.Project, []manifest.Words, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	w := project.Writer{Disk: disk.Disk{}, Git: git.Committer{}}
	folder, err := w.Look(c.Folder)
	if err != nil {
		return nil, nil, err
	}
	tmp, err := tempdir.Make("itos-template-template-")
	if err != nil {
		return nil, nil, err
	}
	defer tempdir.Discard(tmp)
	repo, err := git.Clone(c.Template, filepath.Join(tmp, "template.git"))
	if err != nil {
		return nil, nil, err
	}
	t, err := template.Open(c.Template, repo)
	if err != nil {
		return nil, nil, err
	}
	name, cut := template.Recorded(c.Template, dir, template.SystemOf(runtime.GOOS))
	t.Name = name
	combination, answers, err := template.Choose(t.Manifest, c.Choice, c.Answers, c.Defaults, ask)
	if err != nil {
		return nil, nil, err
	}
	p, err := t.Render(combination, answers, folder, nil, w)
	if err != nil {
		return nil, nil, err
	}
	if cut {
		leftOut(name)
	}
	return p, t.Setup(combination, answers), nil
}
