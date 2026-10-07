package cli

import (
	"strings"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/disk"
	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/render"
	"github.com/donvargax/itos-template/internal/template"
)

// Each switch below gives every kind of its sealed set a code, a rule ID
// and a sentence, and has no default: gochecksumtype refuses one that
// leaves a kind out. Each starts from an internal error only so a kind
// with no case, which the lint refuses, would still exit 70, as a bug.

func templateProblem(err template.Error) (code int, problems []Problem) {
	code, problems = CodeInternal, internal(err)
	switch e := err.(type) {
	case *template.NoStack:
		code, problems = CodeUsage, one("stack-missing", "no stack chosen: name one with --stack (%s)", strings.Join(e.Stacks, ", "))
	case *template.UnknownStack:
		code, problems = CodeUsage, one("stack-unknown", "the template has no stack %s: its stacks are %s", e.Name, strings.Join(e.Stacks, ", "))
	case *template.UnknownFeature:
		if len(e.Known) == 0 {
			code, problems = CodeUsage, one("feature-unknown", "the template has no feature %s", e.Name)
		} else {
			code, problems = CodeUsage, one("feature-unknown", "the template has no feature %s: the stack %s's features are %s", e.Name, e.Stack, strings.Join(e.Known, ", "))
		}
	case *template.OtherStack:
		code, problems = CodeRefused, one("feature-other-stack", "the feature %s is the stack %s's, not the stack %s's: a project has one stack's features", e.Feature.Branch(), e.Feature.Stack, e.Stack)
	case *template.Needs:
		code, problems = CodeRefused, one("feature-needs", "the feature %s needs the feature %s: choose it too, with --feature %s", e.Feature, e.Need, e.Need)
	case *template.Unsupported:
		code, problems = CodeRefused, one("combination-unsupported", "the template does not support %s: its manifest lists that combination as unsupported", e.Combination.Name())
	case *template.NoRoot:
		code, problems = CodeUsage, one("manifest-missing", "the template %s has no default branch to read its manifest, %s, from", e.Template, manifest.File)
	case *template.EmptyRoot:
		code, problems = CodeUsage, one("manifest-missing", "the template %s's default branch, %s, has no commit to read its manifest, %s, from", e.Template, e.Root, manifest.File)
	case *template.NoManifest:
		code, problems = CodeUsage, one("manifest-missing", "the template %s has no %s on its root branch, %s: a template names its stacks, features and questions there (docs/manifest.md)", e.Template, manifest.File, e.Root)
	case *template.ManifestInvalid:
		code, problems = CodeUsage, nil
		for _, p := range e.Problems {
			problems = append(problems, one("manifest-invalid", "the template's %s on %s: %s", manifest.File, e.Root, p)...)
		}
	case *template.NoBranch:
		code, problems = CodeUsage, one("manifest-branch-missing", "the template's manifest lists %s, but the template has no branch %s", strings.TrimPrefix(e.Branch, "stack/"), e.Branch)
	case *template.MergeConflict:
		code, problems = CodeRefused, one("merge-conflict", "merging the template's branch %s into %s leaves conflicts in %s: the template's branches must merge cleanly; merge them in the template and resolve them there",
			e.Branch, strings.Join(e.Into, " + "), strings.Join(e.Paths, ", "))
	case *template.HoldsRecord:
		code, problems = CodeRefused, one("template-defect", "the template holds %s, the file a made project records its render in: leave it out of the template", e.File)
	}
	return code, problems
}

func renderProblem(err render.Error) (code int, problems []Problem) {
	code, problems = CodeInternal, internal(err)
	switch e := err.(type) {
	case *render.BadName:
		code, problems = CodeUsage, one("answer-name", "the template's %s would be named %q, which no file can be: the answers make it %q", e.Path, e.Would, e.Element)
	case *render.SameName:
		code, problems = CodeUsage, one("answer-name", "the template's %s and %s would both be %s: give answers that tell them apart", e.First, e.Second, e.To)
	case *render.FileIsFolder:
		code, problems = CodeUsage, one("answer-name", "the template's file %s would be %s, a folder of %s: give answers that tell them apart", e.File, e.Folder, e.Of)
	case *render.Submodule:
		code, problems = CodeRefused, one("template-defect", "the template holds the submodule %s, which new cannot render", e.Path)
	}
	return code, problems
}

func answerProblem(err answer.Error) (code int, problems []Problem) {
	code, problems = CodeInternal, internal(err)
	switch e := err.(type) {
	case *answer.Malformed:
		code, problems = CodeUsage, one("answer-malformed", "--answer takes name=answer, and %q has no =", e.Given)
	case *answer.Unknown:
		code, problems = CodeUsage, one("answer-unknown", "the template asks no question %s: its questions are %s", e.Name, strings.Join(e.Questions, ", "))
	case *answer.Twice:
		code, problems = CodeUsage, one("answer-twice", "the answer to %s is given twice", e.Name)
	case *answer.NotTaken:
		code, problems = CodeUsage, one("answer-malformed", "the answer to %s, %q, is not one it takes: %v", e.Name, e.Answer, e.Reason)
	case *answer.Missing:
		q := e.Question
		if q.Default != nil {
			code, problems = CodeUsage, one("answer-missing", "no answer to %s (%s): give one with --answer %s=<answer>, or take its default, %s, with --defaults", q.Name, q.Question, q.Name, *q.Default)
		} else {
			code, problems = CodeUsage, one("answer-missing", "no answer to %s (%s): give one with --answer %s=<answer>", q.Name, q.Question, q.Name)
		}
	case *answer.NotAnswered:
		code, problems = CodeUsage, one("answer-missing", "no answer to %s (%s): %v", e.Question.Name, e.Question.Question, e.Err)
	}
	return code, problems
}

func projectProblem(err project.Error) (code int, problems []Problem) {
	code, problems = CodeInternal, internal(err)
	switch e := err.(type) {
	case *project.NotEmpty:
		code, problems = CodeRefused, one("folder-not-empty", "the folder %s has files in it: new writes a project into a missing or empty folder", e.Folder)
	case *project.NotFolder:
		code, problems = CodeRefused, one("folder-not-empty", "%s is a file: new writes a project into a missing or empty folder", e.Folder)
	case *project.CommitRefused:
		code, problems = CodeRefused, one("commit-refused", "git refused the project's first commit: %v", e.Err)
	}
	return code, problems
}

func gitProblem(err git.Error) (code int, problems []Problem) {
	code, problems = CodeInternal, internal(err)
	switch e := err.(type) {
	case *git.Missing:
		code, problems = CodeEnvironment, one("git-missing", "cannot run git, which new clones, merges and commits with: install git, or put it on the PATH (cannot run git: %v)", e.Err)
	case *git.Unreachable:
		code, problems = CodeEnvironment, one("template-unreachable", "git cannot reach the template %s: %v. Name a path or a URL git clone takes.", e.Name, e.Err)
	case *git.Failed:
		// A git command that should not fail, failing: a defect, ours or git's.
		code, problems = CodeInternal, internal(e)
	case *git.NoIdentity:
		code, problems = CodeEnvironment, one("git-identity", "git does not know who you are, so it cannot make the project's first commit: set user.name and user.email in git's config (%v)", e.Err)
	}
	return code, problems
}

func diskProblem(err disk.Error) (code int, problems []Problem) {
	code, problems = CodeInternal, internal(err)
	switch e := err.(type) {
	case *disk.Unreadable:
		code, problems = CodeEnvironment, one("folder-unreadable", "cannot read the folder %s: %v", e.Folder, e.Err)
	case *disk.Unmakeable:
		code, problems = CodeEnvironment, one("folder-unwritable", "cannot make the folder %s: %v", e.Folder, e.Err)
	case *disk.Unwritable:
		if e.Link != "" {
			code, problems = CodeEnvironment, one("folder-unwritable", "cannot write the project in %s: cannot make the symbolic link %s: %v", e.Folder, e.Link, e.Err)
		} else {
			code, problems = CodeEnvironment, one("folder-unwritable", "cannot write the project in %s: %v", e.Folder, e.Err)
		}
	}
	return code, problems
}
