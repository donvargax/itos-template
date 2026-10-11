// Package adopt is the slice of itos-template adopt (decisions 16, 17 and
// 24), in two thin layers over the domain, which does the work: package
// template renders the baseline, the release --ref names, as new renders
// it, and package adoption compares it with the project and writes the
// record.
//
// Its UI is CLI, its flags as kong reads them, and Run, which assembles the
// Command from them, hands it to Handle and shows what comes back: the
// preview on stdout, or with --json the one object, and the problems
// through internal/cli. Its application is Command, a CQRS command of
// domain objects, and Handle, which only wires infra (git, tempdir) to the
// domain (template, adoption) and calls it in order. It imports no other
// slice.
//
// The project is looked at first, before the template is cloned, so a
// folder in no repository, or one not ready for a record, is refused at
// once. The baseline is rendered from the template's bare clone into
// memory, in a temporary folder removed when adopt ends: no check of the
// manifest's runs, and no setup step. Nothing of the project is written
// but its record, unless --merge merges the baseline in with git's merge,
// as update merges a release (GitHub issue #3), the merge and the record
// left staged. The paths --decline names are left out of the baseline and
// listed in the record, so no later update brings them.
package adopt

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/donvargax/itos-template/internal/adoption"
	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/cli"
	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/tempdir"
	"github.com/donvargax/itos-template/internal/template"
	"github.com/donvargax/itos-template/internal/template/port"
)

// ── UI ──

// CLI is itos-template adopt as kong reads it (decision 4): its flags.
// --dry-run, --force and --merge have no environment variable: each is a
// choice about one run, never one to set once for a shell (docs/CLI.md,
// rule 34).
type CLI struct {
	Template string   `arg:"" help:"The template: anything git clone takes, a path or a URL."`
	Ref      string   `required:"" help:"The template's release the project matches, by its version (v1.2.0), a pre-release too: adopt never guesses one, nor takes the newest." placeholder:"VERSION"`
	Stack    string   `help:"The stack the project was made of, by the name the template's manifest gives it. Asked on a terminal when not given." placeholder:"STACK"`
	Feature  []string `help:"A feature the project has, by its name (cli) or its branch (go/cli); once for each. The features a feature needs are never added unasked." placeholder:"FEATURE" sep:"none"`
	Answer   []string `help:"An answer to one of the template's questions, as name=answer; once for each. An answer not given is asked on a terminal, never guessed from the project's files." placeholder:"NAME=ANSWER" sep:"none"`
	Defaults bool     `help:"Take a missing answer's default instead of asking or refusing." negatable:"" env:"ITOS_TEMPLATE_DEFAULTS"`
	DryRun   bool     `name:"dry-run" help:"Preview the baseline and the differences, and write nothing." negatable:""`
	Force    bool     `help:"Accept the differences from the baseline and write the record: no file of the project is copied, deleted or merged." negatable:""`
	Merge    bool     `help:"Merge the baseline into the project with git's merge, over an empty base, and stage it with the record: a file the project lacks is added, one both hold otherwise is a conflict both added." negatable:""`
	Decline  []string `help:"A file or a folder of the baseline the project turns down, by its path from the project's top, with /; once for each. It is left out of the comparison and the merge, and the record lists it, so no later update brings it." placeholder:"PATH" sep:"none"`
	Diff     bool     `help:"Show each text file's changes from the baseline to the project, as git diff shows them." negatable:"" env:"ITOS_TEMPLATE_DIFF"`
	JSON     bool     `name:"json" help:"Print the result as one JSON object on stdout." negatable:"" env:"ITOS_TEMPLATE_JSON"`
}

// Validate refuses --merge with --force, as kong refuses a usage error:
// --force writes the record and nothing else, which --merge contradicts.
func (c *CLI) Validate() error {
	if c.Merge && c.Force {
		return errors.New("--merge and --force contradict each other: --merge merges the baseline in, --force writes the record and nothing else; give one")
	}
	return nil
}

// Help is adopt's detail in its --help: its JSON and its exit codes
// (docs/CLI.md, rules 10 and 14).
func (c *CLI) Help() string {
	return `Links the existing project adopt runs in, at its git repository's top, to the template's release --ref names, so itos-template update can take it to a newer one: it writes .itos-template.yaml, the record new writes, and nothing else. The project keeps its files, its history and its index; the record is left unstaged and uncommitted, for you to commit through the project's own gates before you update. With --merge it takes the release in too.

The baseline is that release rendered as new renders it, with the stack, the features and the answers given (asked on a terminal, refused without one unless --defaults), in a temporary folder: no check and no setup step of the template's runs. adopt compares it with the project's working tree, the files git tracks and those it neither tracks nor ignores, and the baseline's files even where the project ignores them: each path is project-only, modified or missing from the project, a binary file and a mode git records otherwise told apart. The preview names the template, the release, the stack, the features, the answers and each branch's commit, then each difference, sorted by path, with no file's contents; --diff adds git's diff of each text change, a/ the baseline and b/ the project, never a binary file's.

A project that differs from the baseline is refused unless --force or --merge accepts the differences. --force writes the record alone: a file missing from the project is a deletion of the project's, never a file adopt installs, and a later update keeps it only while the template leaves the file unchanged, a release that changes it leaving a modify/delete conflict. --merge merges the baseline into the project with git's own merge, as update merges, over an empty base: a file the project lacks is added, a file both hold the same and a file only the project holds are left as they are, and a file both hold differently is left as git merge leaves a conflict both sides added, marked in the file and unmerged in the index (the project's side and the release's, no base). The merge and the record are staged, HEAD never moves and nothing is committed: commit it through the project's own gates once it is as you want it, or undo it with git reset --hard. --merge needs git 2.45 or later, as update does; --merge and --force together are refused.

--decline turns down a file or a folder of the baseline (a folder covering everything under it), named by its path from the project's top, with /, as a template's template_only paths are, never a glob; give it once for each, beside --merge, which leaves it out of the merge, or --force, which writes the record alone. A declined path is left out of the baseline, so the project's file there, or its absence, is no difference and is never merged; the record lists it, and every later update leaves it out of both its renders, so no release changes, adds or removes it. A path the baseline holds neither as a file nor as a folder is refused, so a misspelt one is caught; one a later release adds is declined by listing it under declined in the record.

To take the template's changes since the project was made, adopt the release it was made from, commit the record, then update; adopting the newest release takes none of them. --dry-run previews, differences and a dirty project included, and writes nothing, --merge or not. Writing asks for a git repository with a working tree, a commit, a clean index and working tree (no untracked file git does not ignore) and no record, --force, --merge or not.

--json prints {"schema":1,"ok","written","dirty","baseline":{"template","release","stack","features":[…],"answers":{…},"commits":{"<branch>":"<sha>"},"declined":[…] (when it declines paths)},"differences":[{"path","kind":"project-only|modified|missing","binary","mode_changed","diff" (with --diff, for a text change)}],"conflicts":[{"path"}],"problems":[{"rule","message","fix"}]}; baseline is null when adopt stopped before rendering it, and conflicts are the paths --merge left conflicted, as update names them. A conflicted merge is ok false, written true, with the problem merge-conflict. A script reads it and the exit code, never the preview, nor a problem's message or fix.

Exit codes: 0 the record written, with --merge the merge staged, or with --dry-run the preview shown; 1 refused: the merge left conflicts, the project differs from the baseline without --force or --merge, its index or working tree is not clean, it has no commit, it holds a record already, the release --ref names is incomplete, a feature of another stack, a feature whose needed feature is not chosen, a combination the manifest lists as unsupported, branches that do not merge cleanly; 2 a usage or manifest error: --merge with --force, a --decline that is not a path in the project or one the baseline holds neither as a file nor as a folder, a template named with a control character or one that prints as nothing, no --ref, a --ref no release has, an unknown stack or feature, an answer missing (without a terminal) or not one its question takes; 3 the folder is in no git repository or has no working tree, git cannot reach the template, cannot run, or, for --merge, is older than 2.45; 70 an internal error.

Examples:
  itos-template adopt ../acme --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --defaults --dry-run
  itos-template adopt ../acme --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --defaults --force
  itos-template adopt ../acme --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --defaults --merge --decline docs/sample

Report issues at https://github.com/donvargax/itos-template/issues.`
}

// Run adopts the project and returns adopt's exit code: the preview on
// stdout, or with --json its object, then each problem on stderr.
func (c *CLI) Run(ui *cli.UI) int {
	a, err := Handle(Command{
		Template: c.Template,
		Choice:   manifest.Choice{Stack: c.Stack, Features: c.Feature},
		Answers:  answer.Given(c.Answer),
		Defaults: c.Defaults,
		Ref:      c.Ref,
		How:      adoption.How{DryRun: c.DryRun, Force: c.Force, Merge: c.Merge, Diff: c.Diff, Decline: c.Decline},
	}, ui.Asker(), credentialLeftOut(ui))
	var problems []cli.Problem
	code := 0
	if err != nil {
		problems, code = cli.Report(err)
	}
	if c.JSON {
		if a == nil {
			a = &adoption.Adoption{Differences: []adoption.Difference{}, Conflicts: []port.Unmerged{}}
		}
		if problems == nil {
			problems = []cli.Problem{}
		}
		ui.JSON(struct {
			Schema int  `json:"schema"`
			OK     bool `json:"ok"`
			*adoption.Adoption
			Problems []cli.Problem `json:"problems"`
		}{1, err == nil, a, problems})
	} else if a != nil {
		preview(ui, a, c.Ref, c.DryRun, c.Merge)
		ui.Line("the preview is for people: a script reads adopt's --json")
	}
	for _, p := range problems {
		ui.Line(p.Message)
	}
	return code
}

// credentialLeftOut says, for a template whose URL held a credential the
// record leaves out, naming it as recorded, that update will reach it
// through git's credential helper, as new says it.
func credentialLeftOut(ui *cli.UI) func(recorded string) {
	return func(recorded string) {
		ui.Line("the record names the template " + recorded + ", the credential its URL held left out: itos-template update will reach the template through git's credential helper (git help credentials)")
	}
}

// preview writes a on stdout for people: the baseline, the answers and each
// branch's commit, a line each; each difference, a line each, its kind, its
// path and, in brackets, binary and the modes git records; each diff asked
// for; then what was done, merged when merge.
func preview(ui *cli.UI, a *adoption.Adoption, ref string, dryRun, merge bool) {
	out := ui.Stdout
	b := a.Baseline
	features := "no features"
	if len(b.Features) > 0 {
		features = "the features " + strings.Join(b.Features, ", ")
	}
	_, _ = fmt.Fprintf(out, "The baseline: %s%s (--ref %s), the stack %s, %s.\n", b.Template, b.AtRelease(), ref, b.Stack, features)
	answers := "none"
	if len(b.Answers) > 0 {
		var given []string
		for _, name := range slices.Sorted(maps.Keys(b.Answers)) {
			given = append(given, name+"="+b.Answers[name])
		}
		answers = strings.Join(given, ", ")
	}
	_, _ = fmt.Fprintf(out, "Its answers: %s.\n", answers)
	if len(b.Declined) > 0 {
		_, _ = fmt.Fprintf(out, "Its declined paths, left out of the comparison and the merge: %s.\n", strings.Join(b.Declined, ", "))
	}
	_, _ = fmt.Fprintln(out, "Its commits:")
	for _, branch := range slices.Sorted(maps.Keys(b.Commits)) {
		_, _ = fmt.Fprintf(out, "  %s %s\n", branch, b.Commits[branch])
	}
	_, _ = fmt.Fprintln(out, "The project's differences from it, a line each, none when the project holds its files alone:")
	for _, d := range a.Differences {
		_, _ = fmt.Fprintf(out, "  %s %s%s\n", d.Kind, d.Path, notes(d))
	}
	for _, d := range a.Differences {
		if d.Diff != "" {
			_, _ = fmt.Fprint(out, d.Diff)
		}
	}
	if a.Dirty {
		_, _ = fmt.Fprintln(out, "The project's index or working tree is dirty: the preview compares them as they are.")
	}
	switch {
	case a.Written && merge:
		_, _ = fmt.Fprintf(out, "Merged the baseline into the project with %s, staged and uncommitted: commit it through the project's own gates once it is as you want it, or undo it with git reset --hard.\n", project.RecordFile)
	case a.Written:
		_, _ = fmt.Fprintf(out, "Wrote %s, unstaged: commit it through the project's own gates, then itos-template update takes the project to a newer release.\n", project.RecordFile)
	case dryRun:
		_, _ = fmt.Fprintln(out, "A dry run: nothing written.")
	}
}

// notes is what a difference's line says of it beyond its kind and path:
// binary, and the modes git records on each side when they differ.
func notes(d adoption.Difference) string {
	var n []string
	if d.Binary {
		n = append(n, "binary")
	}
	if d.ModeChanged {
		n = append(n, "mode "+gitMode(d.From)+" to "+gitMode(d.To))
	}
	if len(n) == 0 {
		return ""
	}
	return " (" + strings.Join(n, ", ") + ")"
}

// gitMode is a file's mode as git writes it: 100644, 100755 for an
// executable, 120000 for a symbolic link, 160000 for a submodule.
func gitMode(m fs.FileMode) string {
	switch {
	case m&fs.ModeSymlink != 0:
		return "120000"
	case m&fs.ModeIrregular != 0:
		return "160000"
	case m&0o111 != 0:
		return "100755"
	default:
		return "100644"
	}
}

// ── Application ──

// Command is adopt's command: link the project adopt runs in to the
// template Template's release Ref, rendered with the combination Choice
// names and Answers, Defaults taking a missing answer's default, as How
// says.
type Command struct {
	Template string
	Choice   manifest.Choice
	Answers  answer.Given
	Defaults bool
	Ref      string
	How      adoption.How
}

// Handle adopts the project the working folder is in as c says, asking
// ask, when there is one, for what c leaves out, and gives what it found
// and did, with the refusal too when it refused. leftOut is told the name
// the record gives the template when that left a credential out of its
// URL, as new tells it.
//
// The template's name is judged first (template.CheckName), before the
// project is looked at or anything cloned; for a merge, the project is
// refused, as adoption.Open refuses it, when git is too old to merge. The
// template, cloned bare into a
// temporary folder removed when adopt ends, is named in the record as new
// names it (template.Recorded), from the working folder. The project is
// opened as update opens it, so a merge goes through update's own
// (git.Project), its scratch files in that folder.
func Handle(c Command, ask port.Asker, leftOut func(recorded string)) (*adoption.Adoption, error) {
	if err := template.CheckName(c.Template); err != nil {
		return nil, err
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	tmp, err := tempdir.Make("itos-template-adopt-")
	if err != nil {
		return nil, err
	}
	defer tempdir.Discard(tmp)
	co := git.OpenProject(dir, tmp)
	state, err := adoption.Open(co, c.How, co)
	if err != nil {
		return nil, err
	}
	repo, err := git.Clone(c.Template, filepath.Join(tmp, "template.git"))
	if err != nil {
		return nil, err
	}
	t, err := template.Open(c.Template, repo)
	if err != nil {
		return nil, err
	}
	name, cut := template.Recorded(c.Template, dir, template.SystemOf(runtime.GOOS))
	t.Name = name
	if err := t.UseRelease(c.Ref); err != nil {
		return nil, err
	}
	combination, answers, err := template.Choose(t.Manifest, c.Choice, c.Answers, c.Defaults, ask)
	if err != nil {
		return nil, err
	}
	files, record, err := t.Baseline(combination, answers)
	if err != nil {
		return nil, err
	}
	if cut {
		leftOut(name)
	}
	return adoption.Adopt(co, state, files, record, c.How, co, git.Differ{Dir: tmp})
}
