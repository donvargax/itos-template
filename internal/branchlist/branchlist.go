// Package branchlist is the slice of itos-template branch list (decision
// 42, q-58, q-60), in two thin layers over the domain, which does the work:
// package syncing reads the branch tree from the manifest and says where
// every branch stands, against the branches it descends from, writing
// nothing. branch sync, a slice of its own, merges one into the branch
// checked out.
//
// Its UI is CLI, its flag as kong reads it, and Run, which hands the query
// to Handle and shows what comes back: a line per branch and the branch to
// sync next on stdout, or with --json the one object, and the problems
// through internal/cli. Its application is Handle, which only wires infra
// (git) to the domain (syncing). It imports no other slice.
package branchlist

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos-template/internal/cli"
	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/syncing"
)

// ── UI ──

// CLI is itos-template branch list as kong reads it (decision 4): its flag.
type CLI struct {
	JSON bool `name:"json" help:"Print the result as one JSON object on stdout." negatable:"" env:"ITOS_TEMPLATE_JSON"`
}

// Help is branch list's detail in its --help: its JSON and its exit codes
// (docs/CLI.md, rules 10 and 14).
func (c *CLI) Help() string {
	return `Lists every branch a template's manifest names, in its order, the root first, in a clone of the template, with the branches it descends from whose commits it lacks, a branch missing locally as missing, and the branch to sync next with branch sync. Each branch descends from others: a stack from the root, a feature from its stack, and a feature that needs others from each feature it needs, in the manifest's order, instead of its stack (docs/manifest.md's "Branches"). The root is the template's default branch, the one origin's HEAD names, as git clone recorded it, else as origin says it; the manifest (itos-template.yaml) is read from the root's local branch.

branch list merges nothing and writes nothing. It reads the local branches alone, fetching nothing, so it runs on any checkout, a detached HEAD, an unclean one or one with a merge in progress, and in a template's CI, to hold every branch in step.

branch list needs a git repository with a working tree and a commit, a remote origin naming its default branch and the root local.

--json prints {"schema":1,"ok","branches":[{"branch","from":[],"lacks":[],"missing"}],"problems":[{"rule","message","fix"}]}: each branch the manifest names, in its order, from the branches it descends from, lacks those whose commits it lacks, and missing whether it is not a local branch; ok is false, with the problem branch-behind or branch-missing, when one lacks any or is missing. A script reads it and the exit code, never the plain output, nor a problem's message or fix.

Exit codes: 0 every branch local and lacking no commit of a branch it descends from; 1 a branch lacks commits of a branch it descends from or is missing locally, the root is not local, or the repository has no commit or no origin; 2 a usage or manifest error: an origin naming no default branch, as release refuses one, a manifest missing or refused; 3 the folder is in no git repository or has no working tree, or git cannot run; 70 an internal error.

Examples:
  itos-template branch list
  itos-template branch list --json

Report issues at https://github.com/donvargax/itos-template/issues.`
}

// Run reports where every branch of the template stands and returns branch
// list's exit code: a line per branch and the branch to sync next on
// stdout, or with --json its object, then each problem on stderr.
func (c *CLI) Run(ui *cli.UI) int {
	t, err := Handle()
	var problems []cli.Problem
	code := 0
	if err != nil {
		problems, code = cli.Report(err)
	}
	if c.JSON {
		ui.JSON(objectOf(t, err == nil, problems))
	} else if t != nil {
		show(ui, t)
	}
	for _, p := range problems {
		ui.Line(p.Message)
	}
	return code
}

// object is branch list's --json object.
type object struct {
	Schema   int           `json:"schema"`
	OK       bool          `json:"ok"`
	Branches []standing    `json:"branches"`
	Problems []cli.Problem `json:"problems"`
}

// standing is a branch of the --json object's branches.
type standing struct {
	Branch  string   `json:"branch"`
	From    []string `json:"from"`
	Lacks   []string `json:"lacks"`
	Missing bool     `json:"missing"`
}

// objectOf is the object of t, nil when branch list stopped before it read
// the branches: each list empty, never null.
func objectOf(t *syncing.Tree, ok bool, problems []cli.Problem) object {
	o := object{Schema: 1, OK: ok, Branches: []standing{}, Problems: []cli.Problem{}}
	if t != nil {
		for _, s := range t.Branches {
			o.Branches = append(o.Branches, standing{Branch: s.Branch, From: append([]string{}, s.From...), Lacks: append([]string{}, s.Lacks...), Missing: s.Missing})
		}
	}
	o.Problems = append(o.Problems, problems...)
	return o
}

// show writes t on stdout for people: a line per branch, what it lacks,
// that it is missing or that it is up to date, then the branch to sync
// next.
func show(ui *cli.UI, t *syncing.Tree) {
	out := ui.Stdout
	inStep := true
	for _, s := range t.Branches {
		_, _ = fmt.Fprintf(out, "%s: %s\n", s.Branch, standingOf(s))
		inStep = inStep && !s.Missing && len(s.Lacks) == 0
	}
	switch next := t.Next(); {
	case next != "":
		_, _ = fmt.Fprintf(out, "Sync next: %s, checked out with git switch %s, then itos-template branch sync.\n", next, next)
	case inStep:
		_, _ = fmt.Fprintf(out, "Every branch is up to date.\n")
	default:
		_, _ = fmt.Fprintf(out, "Nothing can be synced until each missing branch is local.\n")
	}
	ui.Line("what branch list prints is for people: a script reads its --json")
}

// standingOf is where the branch s stands, in words.
func standingOf(s syncing.Standing) string {
	switch {
	case s.Missing:
		return "missing, not a local branch"
	case len(s.From) == 0:
		return "the root, up to date"
	}
	var says []string
	if len(s.Lacks) > 0 {
		says = append(says, "lacks "+strings.Join(s.Lacks, " and "))
	}
	if len(s.Unknown) > 0 {
		says = append(says, "cannot be judged against "+strings.Join(s.Unknown, " and ")+", missing")
	}
	if len(says) == 0 {
		return "up to date with " + strings.Join(s.From, " and ")
	}
	return strings.Join(says, "; ")
}

// ── Application ──

// Handle is where every branch of the template stands, in the clone the
// working folder is in (syncing.Status), read from git, writing nothing.
func Handle() (*syncing.Tree, error) {
	return syncing.Status(git.OpenLocal(""))
}
