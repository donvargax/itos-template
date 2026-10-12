// Package syncbranch is the slice of itos-template sync (decisions 2 and
// 42, q-59), in two thin layers over the domain, which does the work:
// package syncing reads the branch tree from the manifest, judges the clone
// and its branches, and merges into the branch checked out the branch it
// descends from, leaving the merge staged.
//
// Its UI is CLI, its flag as kong reads it, and Run, which hands the
// command to Handle and shows what comes back: the branch merged and what
// it staged on stdout, or with --json the one object, and the problems
// through internal/cli. Its application is Handle, which only wires infra
// (git) to the domain (syncing). It imports no other slice.
package syncbranch

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos-template/internal/cli"
	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/syncing"
)

// ── UI ──

// CLI is itos-template sync as kong reads it (decision 4): its flag.
type CLI struct {
	JSON bool `name:"json" help:"Print the result as one JSON object on stdout." negatable:"" env:"ITOS_TEMPLATE_JSON"`
}

// Help is sync's detail in its --help: its JSON and its exit codes
// (docs/CLI.md, rules 10 and 14).
func (c *CLI) Help() string {
	return `Takes into the template branch checked out, in a clone of the template, the upkeep of the branch it descends from, a branch at a time down the tree: the root into a stack, a stack into each of its features, and a feature that needs others each feature it needs, in the manifest's order, instead of its stack (docs/manifest.md's "Branches"). The root is the template's default branch, the one origin's HEAD names, as git clone recorded it, else as origin says it, whatever is checked out; the manifest (itos-template.yaml) is read from the root's local branch. sync fetches nothing: the branches it merges are the local ones.

sync merges the first branch it descends from whose commits the branch checked out lacks, one a run, with git's own merge, git merge --no-commit --no-ff, and commits nothing: the merge is left in progress, staged, for you to commit through the template's own gates, git commit taking git's message for it. HEAD never moves. A conflict is left as git merge leaves one: marked in the file, unmerged in the index, everything else merged and staged; resolve each and commit, or undo the whole merge with git merge --abort. No path is special: the manifest, the template-only paths and each branch's own files merge as git merges them. Commit, then run sync again for the next branch it descends from, or check out the next branch down the tree.

A branch that lacks nothing of the branches it descends from is up to date, and so is the root, which descends from none: nothing is written. A branch above the one checked out that lacks commits of its own parents is refused, naming it, to be synced first.

sync needs a git repository with a working tree and a commit, a remote origin naming its default branch, the root and every branch above the one checked out local, and the branch checked out one the manifest names; and, to merge, a clean index and working tree, untracked files git does not ignore included, and no merge in progress.

--json prints {"schema":1,"ok","branch","merged","staged":[{"path"}],"conflicts":[{"path"}],"problems":[{"rule","message","fix"}]}; branch is the branch checked out and merged the branch merged into it, each null when there is none: merged is null when nothing was merged. A conflicted merge is ok false with the problem merge-conflict. A script reads it and the exit code, never the plain output, nor a problem's message or fix.

Exit codes: 0 the merge staged, or the branch up to date, the root included; 1 refused: the merge left conflicts, the index or working tree is not clean, a merge is in progress, a branch above the one checked out lacks commits of its own parents or is not local, the root is not local, the repository has no commit or no origin; 2 a usage or manifest error: a detached HEAD or a branch the manifest does not name, the error naming the branches sync merges into, an origin naming no default branch, as release refuses one, a manifest missing or refused; 3 the folder is in no git repository or has no working tree, or git cannot run; 70 an internal error.

Examples:
  itos-template sync
  itos-template sync --json

Report issues at https://github.com/donvargax/itos-template/issues.`
}

// Run syncs the branch checked out and returns sync's exit code: what it
// merged or that it is up to date on stdout, or with --json its object,
// then each problem on stderr.
func (c *CLI) Run(ui *cli.UI) int {
	s, err := Handle()
	var problems []cli.Problem
	code := 0
	if err != nil {
		problems, code = cli.Report(err)
	}
	if c.JSON {
		ui.JSON(objectOf(s, err == nil, problems))
	} else if s != nil && (err == nil || s.Merged != "") {
		show(ui, s)
	}
	for _, p := range problems {
		ui.Line(p.Message)
	}
	return code
}

// object is sync's --json object.
type object struct {
	Schema    int           `json:"schema"`
	OK        bool          `json:"ok"`
	Branch    *string       `json:"branch"`
	Merged    *string       `json:"merged"`
	Staged    []path        `json:"staged"`
	Conflicts []path        `json:"conflicts"`
	Problems  []cli.Problem `json:"problems"`
}

// path is a path of the --json object's staged or conflicts, an object as
// update's conflicts are, so a later key (a kind) joins it without
// breaking a script.
type path struct {
	Path string `json:"path"`
}

// pathsOf are each of list as a path.
func pathsOf(list []string) []path {
	all := []path{}
	for _, p := range list {
		all = append(all, path{Path: p})
	}
	return all
}

// objectOf is the object of s, nil when sync stopped before it knew the
// branch checked out: each list empty, never null, and the branches null
// where there is none.
func objectOf(s *syncing.Sync, ok bool, problems []cli.Problem) object {
	o := object{Schema: 1, OK: ok, Staged: []path{}, Conflicts: []path{}, Problems: []cli.Problem{}}
	if s != nil {
		o.Branch = &s.Branch
		if s.Merged != "" {
			o.Merged = &s.Merged
		}
		o.Staged, o.Conflicts = pathsOf(s.Staged), pathsOf(s.Conflicts)
	}
	o.Problems = append(o.Problems, problems...)
	return o
}

// show writes s, a merge made or none needed, on stdout for people: the
// branch merged and each path it staged, and how to commit it or undo it;
// or that the branch is up to date. The error output names a conflict,
// with what to do about it.
func show(ui *cli.UI, s *syncing.Sync) {
	out := ui.Stdout
	switch {
	case s.Merged != "":
		_, _ = fmt.Fprintf(out, "Merged %s into %s with git merge, uncommitted, %d %s staged:\n", s.Merged, s.Branch, len(s.Staged), paths(len(s.Staged)))
		for _, p := range s.Staged {
			_, _ = fmt.Fprintf(out, "  %s\n", p)
		}
		if len(s.Conflicts) == 0 {
			_, _ = fmt.Fprintf(out, "Commit it through the template's own gates once it is as you want it (git commit takes git's message for it), or undo it with git merge --abort.\n")
		}
	case len(s.From) == 0:
		_, _ = fmt.Fprintf(out, "%s is the template's root, which descends from no branch: up to date.\n", s.Branch)
	default:
		_, _ = fmt.Fprintf(out, "%s is up to date: it lacks no commit of %s.\n", s.Branch, strings.Join(s.From, " or "))
	}
	ui.Line("what sync prints is for people: a script reads sync's --json")
}

// paths is "path" or "paths", as count says.
func paths(count int) string {
	if count == 1 {
		return "path"
	}
	return "paths"
}

// ── Application ──

// Handle syncs the branch checked out in the clone the working folder is
// in (syncing.Run), git's own merge doing the merge.
func Handle() (*syncing.Sync, error) {
	return syncing.Run(git.OpenLocal(""))
}
