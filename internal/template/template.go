// Package template is a template opened to render from: anything git clone
// takes, cloned into a temporary folder (decision 9), and its manifest, read
// from its root branch (decision 8). new renders one combination of it into
// a project; check renders every combination.
//
// Render runs every check before anything is written to the project's
// folder, so a refusal leaves no folder behind (decision 12): the
// combination's branches, the merge, the names the answers make, the
// record's file left free and a git that knows who commits. Only then is the
// project written (package project).
//
// A failure is a *problem.Failure, its exit code chosen here from what
// failed: git's errors are read from internal/git and render's typed
// errors, never passed through.
package template

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/problem"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/render"
	"github.com/donvargax/itos-template/internal/tempdir"
)

// Template is a template cloned into a temporary folder and its manifest,
// read from its root branch.
type Template struct {
	Name     string // the template as it was named
	Manifest *manifest.Manifest

	tmp        string
	clone      *render.Template
	root       string // the root branch
	rootCommit string
}

// Open clones the template name, anything git clone takes, and reads its
// manifest.
func Open(name string) (*Template, error) {
	tmp, err := tempdir.Make("itos-template-template-")
	if err != nil {
		return nil, problem.New(problem.CodeEnvironment, "temporary-folder", "cannot make a temporary folder: %v", err)
	}
	t := &Template{Name: name, tmp: tmp}
	t.clone, err = render.Clone(name, filepath.Join(tmp, "template.git"))
	if err != nil {
		t.Close()
		var unreachable *render.UnreachableError
		if errors.As(err, &unreachable) {
			return nil, problem.New(problem.CodeEnvironment, "template-unreachable", "git cannot reach the template %s: %v. Name a path or a URL git clone takes.", name, unreachable.Err)
		}
		return nil, gitMissing(err)
	}
	if t.Manifest, t.root, t.rootCommit, err = readManifest(t.clone, name); err != nil {
		t.Close()
		return nil, err
	}
	return t, nil
}

// Close removes the clone. A clone left behind is only a temporary folder,
// so a failure to remove it is logged, never returned.
func (t *Template) Close() {
	if err := tempdir.Remove(t.tmp); err != nil {
		slog.Warn("cannot remove a temporary folder", "folder", t.tmp, "error", err)
	}
}

// Render renders the combination c with answers, each one its question
// takes, into folder, which is missing or empty (created says whether
// Render makes it), as a git repository whose first commit is the render
// and its record. Every check runs before folder is touched, and a failure
// while writing leaves it as it was. commitEnv, when given, is added to the
// environment of the commit's git, an identity for a render no one keeps;
// without it the commit is the person's and git must know who they are.
func (t *Template) Render(c manifest.Combination, answers map[string]string, folder string, created bool, commitEnv []string) (*project.Project, error) {
	m := t.Manifest
	commits := map[string]string{t.root: t.rootCommit}
	var branches []string
	for _, b := range append([]string{c.Stack.Branch()}, featureBranches(c.Features)...) {
		sha, ok, err := t.clone.Commit(b)
		if err != nil {
			return nil, problem.Internal(err)
		}
		if !ok {
			return nil, problem.New(problem.CodeUsage, "manifest-branch-missing", "the template's manifest lists %s, but the template has no branch %s", strings.TrimPrefix(b, "stack/"), b)
		}
		commits[b] = sha
		branches = append(branches, b)
	}
	tree, err := t.clone.Merge(commits[branches[0]], branches[0], branches[1:])
	if err != nil {
		var conflict *render.ConflictError
		if errors.As(err, &conflict) {
			return nil, problem.New(problem.CodeRefused, "merge-conflict", "%v", err)
		}
		return nil, problem.Internal(err)
	}
	r := render.NewReplacer(m.Replacements(answers))
	plan, err := t.clone.Plan(tree, func(p string) bool { return !m.IsTemplateOnly(p) }, r)
	if err != nil {
		var name *render.NameError
		if errors.As(err, &name) {
			return nil, problem.New(problem.CodeUsage, "answer-name", "%v", err)
		}
		if render.IsDefect(err) {
			return nil, problem.New(problem.CodeRefused, "template-defect", "%v", err)
		}
		return nil, problem.Internal(err)
	}
	if slices.ContainsFunc(plan.Files, func(f render.File) bool { return f.To == project.RecordFile }) {
		return nil, problem.New(problem.CodeRefused, "template-defect", "the template holds %s, the file a made project records its render in: leave it out of the template", project.RecordFile)
	}
	if commitEnv == nil {
		if err := checkIdentity(t.tmp); err != nil {
			return nil, err
		}
	}
	return project.Write(folder, created, plan, project.Record{
		Template: t.Name,
		Stack:    c.Stack.Name,
		Features: featureNames(c.Features),
		Answers:  answers,
		Commits:  commits,
	}, commitEnv)
}

func readManifest(t *render.Template, name string) (*manifest.Manifest, string, string, error) {
	root, err := t.DefaultBranch()
	if err != nil {
		return nil, "", "", problem.New(problem.CodeUsage, "manifest-missing", "the template %s has no default branch to read its manifest, %s, from", name, manifest.File)
	}
	commit, ok, err := t.Commit(root)
	if err != nil || !ok {
		return nil, "", "", problem.New(problem.CodeUsage, "manifest-missing", "the template %s's default branch, %s, has no commit to read its manifest, %s, from", name, root, manifest.File)
	}
	data, ok, err := t.File(commit, manifest.File)
	if err != nil {
		return nil, "", "", problem.Internal(err)
	}
	if !ok {
		return nil, "", "", problem.New(problem.CodeUsage, "manifest-missing", "the template %s has no %s on its root branch, %s: a template names its stacks, features and questions there (docs/manifest.md)", name, manifest.File, root)
	}
	m, err := manifest.Parse(data)
	if err != nil {
		var e *manifest.Error
		if !errors.As(err, &e) {
			return nil, "", "", problem.Internal(err)
		}
		f := &problem.Failure{Code: problem.CodeUsage}
		for _, p := range e.Problems {
			f.Problems = append(f.Problems, problem.Problem{Rule: "manifest-invalid", Message: fmt.Sprintf("the template's %s on %s: %s", manifest.File, root, p)})
		}
		return nil, "", "", f
	}
	return m, root, commit, nil
}

func featureBranches(features []manifest.Feature) []string {
	var b []string
	for _, f := range features {
		b = append(b, f.Branch())
	}
	return b
}

func featureNames(features []manifest.Feature) []string {
	names := []string{}
	for _, f := range features {
		names = append(names, f.Name)
	}
	return names
}

// checkIdentity refuses, before anything is written, a git that does not
// know who commits: the project's first commit would fail.
func checkIdentity(dir string) error {
	for _, v := range []string{"GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT"} {
		if _, err := git.Run(dir, "var", v); err != nil {
			var e *git.Error
			if errors.As(err, &e) && e.Missing {
				return gitMissing(err)
			}
			return problem.New(problem.CodeEnvironment, "git-identity", "git does not know who you are, so it cannot make the project's first commit: set user.name and user.email in git's config (%v)", err)
		}
	}
	return nil
}

func gitMissing(err error) *problem.Failure {
	return problem.New(problem.CodeEnvironment, "git-missing", "cannot run git, which new clones, merges and commits with: install git, or put it on the PATH (%v)", err)
}
