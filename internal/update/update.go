// Package update is the slice of itos-template update (decisions 3, 17, 37
// and 38), in two thin layers over the domain, which does the work:
// package template renders the release the project's record names and the
// target release, each as new renders one, and package updating checks the
// project, chooses the target and merges the difference into it.
//
// Its UI is CLI, its flags as kong reads them, and Run, which assembles the
// Command from them, hands it to Handle and shows what comes back: what
// changed on stdout, or with --json the one object, and the problems
// through internal/cli. Its application is Command, a CQRS command of
// domain objects, and Handle, which only wires infra (git, tempdir) to the
// domain (template, updating) and calls it in order. It imports no other
// slice.
//
// The project is looked at first, its record read from HEAD, before the
// template is cloned, so a folder that is not a clean project with a record
// is refused at once. The template, named by the record, is cloned bare
// into a temporary folder removed when update ends; no check of the
// manifest's runs, and no setup step. Nothing of the project is written
// before the merge, which leaves it staged and commits nothing.
package update

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/cli"
	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/tempdir"
	"github.com/donvargax/itos-template/internal/template"
	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/updating"
)

// ── UI ──

// CLI is itos-template update as kong reads it (decision 4): its flags.
// --to has no environment variable: it is a choice about one run, never
// one to set once for a shell (docs/CLI.md, rule 34), as new's --ref.
type CLI struct {
	To       string   `help:"The template's release to update to, by its version (v1.2.0), a pre-release too. Without it, the newest complete stable release." placeholder:"VERSION"`
	Answer   []string `help:"An answer to a question the target release adds, as name=answer; once for each. A question the record answers keeps its answer. An answer not given is asked on a terminal." placeholder:"NAME=ANSWER" sep:"none"`
	Defaults bool     `help:"Take a missing answer's default instead of asking or refusing." negatable:"" env:"ITOS_TEMPLATE_DEFAULTS"`
	JSON     bool     `name:"json" help:"Print the result as one JSON object on stdout." negatable:"" env:"ITOS_TEMPLATE_JSON"`
}

// Help is update's detail in its --help: its JSON and its exit codes
// (docs/CLI.md, rules 10 and 14).
func (c *CLI) Help() string {
	return `Brings the project update runs in, made by itos-template new or linked by adopt, to a newer release of its template: it renders the release the project's record (.itos-template.yaml) names, from the branch commits the record names, never from a tag, which may have moved since; it renders the target release with the record's stack, features and answers, each render as new makes one, leaving out the template-only paths; and git's own merge takes the difference into the project, the old render the base, following renames as git's merge does. No check and no setup step of the template's runs.

Each path the record lists under declined, a file or a folder and everything under it, as adopt --decline lists it or as you add it by hand, is left out of both renders: no release changes, adds or removes it, the project's own file there, or its absence, stays as it is, and the new record keeps the list.

The target is the template's newest release, the highest stable version whose every branch carries its tag, or the release --to names, a pre-release too. A project at the target already is told so, and nothing is written; an older target is refused, since update only goes forward.

update writes the merged files and the new record to the working tree and the index, staged, and commits nothing: HEAD never moves. It prints a message naming the old and new releases for you to commit through the project's own gates. A conflict is left as git merge leaves one: marked in the file, unmerged in the index (the old render, the project and the new render), everything else merged and staged, the record naming the new release. update keeps no state of its own, so there is no --abort and no --continue: resolve each conflict and stage it, or undo the whole update with git reset --hard.

Only a question the target release adds is answered, as new answers one: with --answer, on a terminal, or with --defaults, refused without one. The record's answers, stack and features stay as they are; an --answer to a question the record answers is refused.

update needs a git repository with a working tree, a commit, a record at HEAD, and a clean index and working tree, untracked files git does not ignore included. It merges with git merge-tree, which takes the two renders as trees from git 2.45 on: an older git is refused.

--json prints {"schema":1,"ok","from":{"release","commits":{"<branch>":"<sha>"}},"to":{…},"changed":[{"path","kind":"added|modified|removed|renamed","from" (a rename's old path)}],"conflicts":[{"path"}],"problems":[{"rule","message","fix"}]}; from and to are null when update stopped before choosing them. A conflicted update is ok false with the problem merge-conflict. A script reads it and the exit code, never the plain output, nor a problem's message or fix.

Exit codes: 0 the update staged, or the project at the target release already; 1 refused: the merge left conflicts, the target release is older than the project's, the project's index or working tree is not clean, it has no commit, its HEAD holds no record (adopt links a project made before its template), the template has no release, the release --to names is incomplete, branches that do not merge cleanly; 2 a usage or manifest error: a --to no release has, an --answer to a question the record answers, an answer missing (without a terminal) or not one its question takes, a record that cannot be read, a template named in it with a control character or one that prints as nothing; 3 the folder is in no git repository or has no working tree, git cannot reach the template, cannot run, or is older than 2.45; 70 an internal error.

Examples:
  itos-template update
  itos-template update --to v1.2.0 --answer owner=ops

Report issues at https://github.com/donvargax/itos-template/issues.`
}

// Run updates the project and returns update's exit code: what changed on
// stdout, or with --json its object, then each problem on stderr.
func (c *CLI) Run(ui *cli.UI) int {
	u, err := Handle(Command{To: c.To, Answers: answer.Given(c.Answer), Defaults: c.Defaults}, ui.Asker())
	var problems []cli.Problem
	code := 0
	if err != nil {
		problems, code = cli.Report(err)
	}
	if c.JSON {
		if u == nil {
			u = &updating.Update{Changed: []port.Change{}, Conflicts: []port.Unmerged{}}
		}
		if problems == nil {
			problems = []cli.Problem{}
		}
		ui.JSON(struct {
			Schema int  `json:"schema"`
			OK     bool `json:"ok"`
			*updating.Update
			Problems []cli.Problem `json:"problems"`
		}{1, err == nil, u, problems})
	} else if u != nil {
		show(ui, u)
		ui.Line("what update prints is for people: a script reads update's --json")
	}
	for _, p := range problems {
		ui.Line(p.Message)
	}
	return code
}

// show writes u on stdout for people: the releases, each change a line,
// its kind and its path, and the message to commit it with. The error
// output names a conflict, with what to do about it.
func show(ui *cli.UI, u *updating.Update) {
	out := ui.Stdout
	if u.To.Version == u.From.Version {
		_, _ = fmt.Fprintf(out, "The project is at the template's release %s already: nothing to update.\n", u.To.Version)
		return
	}
	_, _ = fmt.Fprintf(out, "Merged the template's release %s into the project, from %s, staged and uncommitted:\n", u.To.Version, releaseOrHeads(u.From.Version))
	for _, c := range u.Changed {
		_, _ = fmt.Fprintf(out, "  %s %s\n", c.Kind, c.Path)
	}
	_, _ = fmt.Fprintf(out, "Commit it through the project's own gates once it is as you want it, or undo it with git reset --hard. A message for the commit:\n  chore: update the template from %s to %s\n",
		releaseOrHeads(u.From.Version), u.To.Version)
}

// releaseOrHeads is version, or "its branch heads" for a render of none.
func releaseOrHeads(version string) string {
	if version == "" {
		return "its branch heads"
	}
	return version
}

// ── Application ──

// Command is update's command: bring the project update runs in to the
// release To names, else the newest, a question that release adds answered
// by Answers, Defaults taking a missing answer's default.
type Command struct {
	To       string
	Answers  answer.Given
	Defaults bool
}

// Handle updates the project the working folder is in as c says, asking
// ask, when there is one, for an answer c leaves out, and gives what it
// found and did, with the refusal too when the merge left conflicts.
//
// The project is looked at and its record read first (updating.Open); then
// the template the record names is judged (template.CheckName) and git's
// version (git.Supports), before anything is cloned. The template, cloned
// bare into a temporary folder removed when update ends, keeps the name the
// record gives it.
func Handle(c Command, ask port.Asker) (*updating.Update, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	tmp, err := tempdir.Make("itos-template-update-")
	if err != nil {
		return nil, err
	}
	defer tempdir.Discard(tmp)
	p := git.OpenProject(dir, tmp)
	start, err := updating.Open(p, c.Answers)
	if err != nil {
		return nil, err
	}
	r := start.Record
	if err := template.CheckName(r.Template); err != nil {
		return nil, err
	}
	if err := git.Supports(); err != nil {
		return nil, err
	}
	repo, err := git.Clone(r.Template, filepath.Join(tmp, "template.git"))
	if err != nil {
		return nil, err
	}
	t, err := template.Open(r.Template, repo)
	if err != nil {
		return nil, err
	}
	newer, err := start.Target(t, c.To)
	if err != nil {
		return nil, err
	}
	if !newer {
		return start.Current(), nil
	}
	combination, answers, err := template.Choose(t.Manifest, manifest.Choice{Stack: r.Stack, Features: r.Features}, start.Given(t.Manifest, c.Answers), c.Defaults, ask)
	if err != nil {
		return nil, err
	}
	target, next, err := t.Baseline(combination, answers)
	if err != nil {
		return nil, err
	}
	base, err := t.Rendered(r)
	if err != nil {
		return nil, err
	}
	return start.Merge(p, base, target, next, t.Root())
}
