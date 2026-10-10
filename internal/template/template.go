// Package template is a template opened to render from (decision 9): its
// git repository, cloned, and its manifest, read from its root branch
// (decision 8), or from a release's root tag once one is used (UseRelease,
// UseNewest, decision 36; release.go says what a release is). new chooses
// one combination of it (Choose) and renders it into a project (Render);
// check renders every combination and runs their checks (Check).
//
// It is domain (decision 17): it does the work and reaches git, the folders
// and the programs only through the interfaces of package port, which infra
// implements, so its tests run it against the fakes of port/porttest
// (decision 18). Its failures are a sealed set (Error), plain domain types
// carrying no exit code, rule ID or wording: internal/cli gives each its
// code and its line. Where a port's failure means something here (a merge
// that leaves conflicts) it is wrapped in an Error; any other is infra's,
// passed through untouched.
//
// Render runs every check before anything is written to the project's
// folder, so a refusal leaves no folder behind (decision 12): the
// combination's branches, the merge, the names the answers make, the
// record's file left free and a git that knows who commits. Only then is
// the project written (package project).
package template

import (
	"errors"
	"maps"
	"slices"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/render"
	"github.com/donvargax/itos-template/internal/template/port"
)

// Template is a template's repository and its manifest, read from its root
// branch.
type Template struct {
	// Name is the template as its made project's record names it: as it
	// was named, which git cloned, unless the caller sets it to the name
	// Recorded gives (new and check do).
	Name     string
	Manifest *manifest.Manifest

	repo       port.Repository
	root       string // the root branch
	rootCommit string
	release    *Release // the release rendered; nil for the branch heads
}

// Open reads the manifest of the template name, whose repository is repo.
func Open(name string, repo port.Repository) (*Template, error) {
	t := &Template{Name: name, repo: repo}
	var err error
	if t.root, err = repo.DefaultBranch(); err != nil {
		return nil, &NoRoot{Template: name}
	}
	commit, ok, err := repo.Commit(t.root)
	if err != nil || !ok {
		return nil, &EmptyRoot{Template: name, Root: t.root}
	}
	t.rootCommit = commit
	if t.Manifest, err = t.manifestAt(commit, t.root); err != nil {
		return nil, err
	}
	return t, nil
}

// Render renders the combination c with answers, each one its question
// takes, into the folder into, missing or empty, as a git repository whose
// first commit is the render and its record, written by w, its message the
// manifest's first_commit with the literals replaced, when it gives one, as a
// text file's contents are. Every check runs
// before into is touched, and a failure while writing leaves it as it was.
// The commit is by by when it is given, an identity for a render no one
// keeps; without it the commit is the person's and git must know who they
// are.
func (t *Template) Render(c manifest.Combination, answers answer.Set, into project.Folder, by *port.Identity, w project.Writer) (*project.Project, error) {
	p, _, err := t.render(c, answers, into, by, w)
	return p, err
}

// render is Render, giving the files it wrote as well.
func (t *Template) render(c manifest.Combination, answers answer.Set, into project.Folder, by *port.Identity, w project.Writer) (*project.Project, []port.File, error) {
	files, record, message, err := t.plan(c, answers)
	if err != nil {
		return nil, nil, err
	}
	p, err := w.Write(into, files, record, message, by)
	return p, files, err
}

// Baseline is the render of the combination c with answers, each one its
// question takes, as Render would write it, and the record Render would
// write beside it, with nothing written: what adopt compares an existing
// project to and records. Every check Render runs before it writes runs
// here, and no check of the manifest's.
func (t *Template) Baseline(c manifest.Combination, answers answer.Set) ([]port.File, project.Record, error) {
	files, record, _, err := t.plan(c, answers)
	return files, record, err
}

// Rendered is the render the record r names, as new rendered it and with
// nothing written: from r's branch commits, never a tag's, which may have
// moved since (decision 10), the manifest the root's commit holds, with
// r's stack, features and answers, each judged as new judges them. A
// branch the render needs that r names no commit of is an *Unrecorded. t
// itself is left rendering what it rendered before.
func (t *Template) Rendered(r project.Record) ([]port.File, error) {
	root, ok := r.Commits[t.root]
	if !ok {
		return nil, &Unrecorded{Branch: t.root}
	}
	m, err := t.manifestAt(root, t.root+" at "+root)
	if err != nil {
		return nil, err
	}
	var given answer.Given
	for _, name := range slices.Sorted(maps.Keys(r.Answers)) {
		given = append(given, name+"="+r.Answers[name])
	}
	c, answers, err := Choose(m, manifest.Choice{Stack: r.Stack, Features: r.Features}, given, false, nil)
	if err != nil {
		return nil, err
	}
	for _, b := range append([]string{c.Stack.Branch()}, featureBranches(c)...) {
		if _, ok := r.Commits[b]; !ok {
			return nil, &Unrecorded{Branch: b}
		}
	}
	old := *t
	old.Manifest, old.rootCommit = m, root
	old.release = &Release{Version: r.Release, Commits: r.Commits, manifest: m}
	files, _, _, err := old.plan(c, answers)
	return files, err
}

// featureBranches are the branches of c's features, in its order.
func featureBranches(c manifest.Combination) []string {
	var list []string
	for _, f := range c.Features {
		list = append(list, f.Branch())
	}
	return list
}

// plan is what a render of c with answers writes: its files, its record
// and its first commit's message, the manifest's first_commit rendered, ""
// when it gives none.
func (t *Template) plan(c manifest.Combination, answers answer.Set) ([]port.File, project.Record, string, error) {
	m := t.Manifest
	commits := map[string]string{t.root: t.rootCommit}
	branches := append([]string{c.Stack.Branch()}, featureBranches(c)...)
	merged := make([]string, len(branches))
	for i, b := range branches {
		sha, ok, err := t.commitOf(b)
		if err != nil {
			return nil, project.Record{}, "", err
		}
		if !ok {
			return nil, project.Record{}, "", &NoBranch{Branch: b}
		}
		commits[b], merged[i] = sha, sha
	}
	tree, err := t.repo.Merge(merged)
	if err != nil {
		var conflict *port.Conflict
		if errors.As(err, &conflict) {
			return nil, project.Record{}, "", &MergeConflict{Into: branches[:conflict.At], Branch: branches[conflict.At], Paths: conflict.Paths}
		}
		return nil, project.Record{}, "", err
	}
	replacer := render.NewReplacer(m.Replacements(answers))
	files, err := render.Plan(tree, func(p string) bool { return !m.IsTemplateOnly(p) }, replacer)
	if err != nil {
		return nil, project.Record{}, "", err
	}
	if slices.ContainsFunc(files, func(f port.File) bool { return f.Path == project.RecordFile }) {
		return nil, project.Record{}, "", &HoldsRecord{File: project.RecordFile}
	}
	var message string
	if m.FirstCommit != nil {
		message = replacer.Text(*m.FirstCommit)
	}
	features := []string{}
	for _, f := range c.Features {
		features = append(features, f.Name)
	}
	return files, project.Record{
		Template: t.Name,
		Release:  t.Release(),
		Stack:    c.Stack.Name,
		Features: features,
		Answers:  answers,
		Commits:  commits,
	}, message, nil
}

// commitOf is the commit the branch b is rendered from: its release's tag's,
// or its head, and false when it has neither.
func (t *Template) commitOf(b string) (string, bool, error) {
	if t.release != nil {
		sha, ok := t.release.Commits[b]
		return sha, ok, nil
	}
	return t.repo.Commit(b)
}
