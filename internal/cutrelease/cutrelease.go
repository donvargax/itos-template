// Package cutrelease is the slice of itos-template release (decision 36,
// q-50 to q-53), in two thin layers over the domain, which does the work:
// package releasing judges the version and the repository, plans a tag on
// every branch the manifest lists and makes them once package template's
// check passes over every combination.
//
// Its UI is CLI, its flags as kong reads them, and Run, which assembles the
// Command from them, hands it to Handle and shows what comes back: check's
// report as check prints it (internal/cli's Block and Summary), the tags
// made and the push that publishes them, or with --json the one object, and
// the problems through internal/cli. Its application is Command, a CQRS
// command of domain objects, and Handle, which only wires infra (git, disk,
// tempdir, program) to the domain (releasing, template, answer) and calls
// it in order. It imports no other slice.
package cutrelease

import (
	"errors"
	"fmt"
	"io"
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
	"github.com/donvargax/itos-template/internal/releasing"
	"github.com/donvargax/itos-template/internal/tempdir"
	"github.com/donvargax/itos-template/internal/template"
)

// ── UI ──

// CLI is itos-template release as kong reads it (decision 4): its version
// and its flags, check's. The version has no environment variable: it is a
// choice about one run (docs/CLI.md, rule 34).
type CLI struct {
	Version  string   `arg:"" help:"The version to release: vX.Y.Z, with an optional pre-release (v1.3.0-rc.1)." placeholder:"VERSION"`
	Answer   []string `help:"An answer to one of the template's questions, as name=answer; once for each. release never asks: every answer is given, or taken with --defaults." placeholder:"NAME=ANSWER" sep:"none"`
	Defaults bool     `help:"Take a missing answer's default." negatable:"" env:"ITOS_TEMPLATE_DEFAULTS"`
	JSON     bool     `name:"json" help:"Print the result as one JSON object on stdout." negatable:"" env:"ITOS_TEMPLATE_JSON"`
}

// Help is release's detail in its --help: its JSON and its exit codes
// (docs/CLI.md, rules 10 and 14).
func (c *CLI) Help() string {
	return `Cuts a release of the template whose repository release runs in, from its top whatever folder of it: one version tagged <branch>/<version> on every branch the manifest (itos-template.yaml) lists, the root included (main/v1.2.0, stack/go/v1.2.0, go/cli/v1.2.0).

It tags the heads check reads, each listed branch's local head, else origin's, and only once check passes over them: it runs check's own work, every combination rendered and its checks run with the answers given, its report on stdout as check prints it. A combination that fails is check's report and exit 1, no tag made. It tags commits, never the working tree, which it neither renders nor judges.

A release is for others to fetch: the repository needs a remote origin, and every listed branch it holds a local head of must be at origin's head of it. A branch ahead of, behind or diverged from origin's, or not on origin, is refused, naming it, before check runs.

The version is vX.Y.Z, with an optional pre-release (v1.3.0-rc.1). A version any listed branch carries a tag of already, a partial release's included, is refused, naming the tag. A version lower than the newest is a backport, and is cut; a pre-release too. Neither becomes the release new renders by default, the highest stable complete one.

The tags are annotated, each message naming the version, and made in this repository alone: release pushes nothing. It prints the one command that publishes them all, git push --atomic origin and each tag, for you to run. A tag git refuses midway takes back the tags made before it, so a release is made whole or not at all.

--json prints {"schema":1,"ok","version","tags":[{"name","branch","commit"}],"push":["git","push","--atomic","origin","refs/tags/<tag>",…],"report":[{"combination","passed","leftovers":[{"text","path","line"}],"checks":[{"run":[words],"status":"passed|failed|skipped"}],"problems":[{"rule","message","fix"}]}],"problems":[{"rule","message","fix"}]}; tags and push are empty unless ok, and report holds each combination checked, none when release stopped before check. A script reads it and the exit code, never the plain output, nor a problem's message or fix.

Exit codes: 0 every combination passed and every listed branch tagged; 1 refused: a render, a scan or a check failed, a listed branch carries the version's tag already, a listed branch is ahead of, behind or diverged from origin's or not on origin, the repository has no origin, git refused a tag; 2 a usage or manifest error: a version that is not vX.Y.Z with an optional pre-release, an answer missing or not one its question takes, a manifest refused; 3 the folder is in no git repository, git cannot run, or a render could not be written; 70 an internal error.

Examples:
  itos-template release v1.2.0 --answer name=blue-fox --defaults
  itos-template release v1.3.0-rc.1 --answer name=blue-fox --answer module=example.com/blue/fox --json

Report issues at https://github.com/donvargax/itos-template/issues.`
}

// Run cuts the release and returns release's exit code: check's report and
// the tags with the push on stdout, or with --json its object, then each
// problem on stderr.
func (c *CLI) Run(ui *cli.UI) int {
	worst, passed, all := 0, 0, 0
	report := []combination{}
	var werr error
	rel, err := Handle(Command{Version: c.Version, Answers: answer.Given(c.Answer), Defaults: c.Defaults}, ui.WarnUnscanned, func(r template.Result) error {
		all++
		if r.Passed() {
			passed++
		}
		worst = max(worst, cli.ResultCode(r))
		if c.JSON {
			report = append(report, combinationOf(r))
			return nil
		}
		if _, werr = io.WriteString(ui.Stdout, cli.Block(r)); werr != nil {
			return fmt.Errorf("cannot write the report: %w", werr)
		}
		return nil
	})
	// The report's end, once check ran and every block of it was written.
	if !c.JSON && werr == nil && all != 0 {
		if _, werr = io.WriteString(ui.Stdout, cli.Summary(passed, all)); werr != nil {
			err = errors.Join(err, fmt.Errorf("cannot write the report: %w", werr))
		}
	}
	var problems []cli.Problem
	code := 0
	if err != nil {
		problems, code = cli.Report(err)
	}
	if c.JSON {
		tags, push := []releasing.Tag{}, []string{}
		if rel != nil {
			tags, push = rel.Tags, rel.Push()
		}
		if problems == nil {
			problems = []cli.Problem{}
		}
		ui.JSON(struct {
			Schema   int             `json:"schema"`
			OK       bool            `json:"ok"`
			Version  string          `json:"version"`
			Tags     []releasing.Tag `json:"tags"`
			Push     []string        `json:"push"`
			Report   []combination   `json:"report"`
			Problems []cli.Problem   `json:"problems"`
		}{1, err == nil, c.Version, tags, push, report, problems})
	} else if rel != nil {
		show(ui, rel)
	}
	for _, p := range problems {
		ui.Line(p.Message)
	}
	return max(worst, code)
}

// show writes the release on stdout for people: each tag made and its
// commit, and the command that publishes them.
func show(ui *cli.UI, rel *releasing.Release) {
	out := ui.Stdout
	_, _ = fmt.Fprintf(out, "\nTagged %s on every branch the manifest lists, annotated, in this repository alone:\n", rel.Version)
	for _, t := range rel.Tags {
		_, _ = fmt.Fprintf(out, "  %s %s\n", t.Name, t.Commit)
	}
	_, _ = fmt.Fprintf(out, "Nothing is pushed. Publish the release, every tag at once, with:\n  %s\n", strings.Join(rel.Push(), " "))
	ui.Line("what release prints is for people: a script reads release's --json")
}

// combination is a combination of the report in --json: its name, whether
// it passed, each literal its render kept, each of its checks as it ran,
// and why it was not rendered.
type combination struct {
	Combination string        `json:"combination"`
	Passed      bool          `json:"passed"`
	Leftovers   []leftover    `json:"leftovers"`
	Checks      []check       `json:"checks"`
	Problems    []cli.Problem `json:"problems"`
}

// leftover is a literal a render kept: the text found, its path, with /,
// and its line, 0 for one in the path.
type leftover struct {
	Text string `json:"text"`
	Path string `json:"path"`
	Line int    `json:"line"`
}

// check is a check as it ran, the answers in place, and how it ended.
type check struct {
	Run    manifest.Words `json:"run"`
	Status string         `json:"status"`
}

func combinationOf(r template.Result) combination {
	c := combination{Combination: r.Combination.Name(), Passed: r.Passed(), Leftovers: []leftover{}, Checks: []check{}, Problems: []cli.Problem{}}
	if r.Err != nil {
		c.Problems, _ = cli.Report(r.Err)
	}
	for _, l := range r.Leftovers {
		c.Leftovers = append(c.Leftovers, leftover{Text: l.Text, Path: l.Path, Line: l.Line})
	}
	for _, ran := range r.Checks {
		c.Checks = append(c.Checks, check{Run: ran.Check, Status: [...]string{"passed", "failed", "skipped"}[ran.Status]})
	}
	return c
}

// ── Application ──

// Command is release's command: cut the release Version of the template
// the working folder's repository holds, checked with Answers, Defaults
// taking a missing answer's default.
type Command struct {
	Version  string
	Answers  answer.Given
	Defaults bool
}

// Handle cuts the release c names in the repository the working folder is
// in, giving the manifest to opened once the release is planned and its
// answers resolved, then each combination's result to each as check finds
// it, and the release once every listed branch is tagged.
//
// The version and the repository are judged first (releasing.Open), before
// anything of it is read. The template is the repository copied bare from
// its top into a temporary folder removed when release ends, each branch
// its local one or else origin's, as check copies it (git.CloneHere), and
// recorded in each render by the top's absolute path, as check records it.
// The release is planned on that copy (releasing.Here.Plan), refusing a
// version used or a branch off origin's, before the answers are resolved
// and check runs; the tags are made in the repository itself.
func Handle(c Command, opened func(*manifest.Manifest), each func(template.Result) error) (*releasing.Release, error) {
	repo := git.OpenTagger("")
	here, err := releasing.Open(repo, c.Version)
	if err != nil {
		return nil, err
	}
	tmp, err := tempdir.Make("itos-template-release-")
	if err != nil {
		return nil, err
	}
	defer tempdir.Discard(tmp)
	copied, err := git.CloneHere(here.Top, filepath.Join(tmp, "template.git"))
	if err != nil {
		return nil, err
	}
	t, err := template.Open(git.Here, copied)
	if err != nil {
		return nil, err
	}
	t.Name, _ = template.Recorded(git.Here, here.Top, template.SystemOf(runtime.GOOS))
	plan, err := here.Plan(t)
	if err != nil {
		return nil, err
	}
	answers, err := answer.Resolve(t.Manifest, c.Answers, c.Defaults)
	if err != nil {
		return nil, err
	}
	opened(t.Manifest)
	w := project.Writer{Disk: disk.Disk{}, Git: git.Committer{}}
	return plan.Cut(t, answers, tempdir.Renders{}, w, program.Runner{Env: git.Environ()}, each)
}
