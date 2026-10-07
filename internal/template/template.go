// Package template is a template opened to render from (decision 9): its
// git repository, cloned, and its manifest, read from its root branch
// (decision 8). new chooses one combination of it (Choose) and renders it
// into a project (Render); check renders every combination and runs their
// checks (Check).
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
	Name     string // the template as it was named
	Manifest *manifest.Manifest

	repo       port.Repository
	root       string // the root branch
	rootCommit string
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
	data, ok, err := repo.File(commit, manifest.File)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &NoManifest{Template: name, Root: t.root}
	}
	if t.Manifest, err = manifest.Parse(data); err != nil {
		var invalid *manifest.Invalid
		if errors.As(err, &invalid) {
			return nil, &ManifestInvalid{Root: t.root, Problems: invalid.Problems}
		}
		return nil, err
	}
	return t, nil
}

// Render renders the combination c with answers, each one its question
// takes, into the folder into, missing or empty, as a git repository whose
// first commit is the render and its record, written by w. Every check runs
// before into is touched, and a failure while writing leaves it as it was.
// The commit is by by when it is given, an identity for a render no one
// keeps; without it the commit is the person's and git must know who they
// are.
func (t *Template) Render(c manifest.Combination, answers answer.Set, into project.Folder, by *port.Identity, w project.Writer) (*project.Project, error) {
	m := t.Manifest
	commits := map[string]string{t.root: t.rootCommit}
	branches := []string{c.Stack.Branch()}
	for _, f := range c.Features {
		branches = append(branches, f.Branch())
	}
	for _, b := range branches {
		sha, ok, err := t.repo.Commit(b)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, &NoBranch{Branch: b}
		}
		commits[b] = sha
	}
	tree, err := t.repo.Merge(branches)
	if err != nil {
		var conflict *port.Conflict
		if errors.As(err, &conflict) {
			into := branches[:max(slices.Index(branches, conflict.Branch), 1)]
			return nil, &MergeConflict{Into: into, Branch: conflict.Branch, Paths: conflict.Paths}
		}
		return nil, err
	}
	files, err := render.Plan(tree, func(p string) bool { return !m.IsTemplateOnly(p) }, render.NewReplacer(m.Replacements(answers)))
	if err != nil {
		return nil, err
	}
	if slices.ContainsFunc(files, func(f port.File) bool { return f.Path == project.RecordFile }) {
		return nil, &HoldsRecord{File: project.RecordFile}
	}
	features := []string{}
	for _, f := range c.Features {
		features = append(features, f.Name)
	}
	return w.Write(into, files, project.Record{
		Template: t.Name,
		Stack:    c.Stack.Name,
		Features: features,
		Answers:  answers,
		Commits:  commits,
	}, by)
}
