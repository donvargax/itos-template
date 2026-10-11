package cli

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos-template/internal/adoption"
	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/disk"
	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/releasing"
	"github.com/donvargax/itos-template/internal/render"
	"github.com/donvargax/itos-template/internal/template"
	"github.com/donvargax/itos-template/internal/updating"
)

// Each switch below gives every kind of its sealed set a code, a rule ID
// and a sentence, and has no default: gochecksumtype refuses one that
// leaves a kind out. Each starts from an internal error only so a kind
// with no case, which the lint refuses, would still exit 70, as a bug.

func templateProblem(err template.Error) (code int, problems []Problem) {
	code, problems = CodeInternal, internal(err)
	switch e := err.(type) {
	case *template.BadName:
		code, problems = CodeUsage, one("template-name", "the template %+q holds %U, a control character, or one that prints as nothing or moves the text around it where no script spells with it: name it by a path or a URL without it", e.Name, e.Char)
	case *template.NoStack:
		code, problems = CodeUsage, one("stack-missing", "no stack chosen: name one with --stack (%s)", strings.Join(e.Stacks, ", "))
	case *template.UnknownStack:
		code, problems = CodeUsage, one("stack-unknown", "the template has no stack %+q: its stacks are %s", e.Name, strings.Join(e.Stacks, ", "))
	case *template.UnknownFeature:
		if len(e.Known) == 0 {
			code, problems = CodeUsage, one("feature-unknown", "the template has no feature %+q", e.Name)
		} else {
			code, problems = CodeUsage, one("feature-unknown", "the template has no feature %+q: the stack %s's features are %s", e.Name, e.Stack, strings.Join(e.Known, ", "))
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
	case *template.NoRelease:
		releases := "it has none"
		if len(e.Releases) > 0 {
			releases = "its releases are " + strings.Join(e.Releases, ", ")
		}
		code, problems = CodeUsage, one("release-unknown", "the template has no release %+q: %s; a release is one version tagged <branch>/<version> on every branch its manifest lists", e.Version, releases)
	case *template.Incomplete:
		lacking := make([]string, len(e.Branches))
		for i, b := range e.Branches {
			lacking[i] = "no tag " + b + "/" + e.Version + " on the branch " + b
		}
		code, problems = CodeRefused, one("release-incomplete", "the template's release %s is incomplete: %s; a release tags every branch its manifest lists <branch>/<version>: tag each in the template, or name another release with --ref",
			e.Version, strings.Join(lacking, ", "))
	case *template.HoldsRecord:
		code, problems = CodeRefused, one("template-defect", "the template holds %s, the file a made project records its render in: leave it out of the template", e.File)
	case *template.Unrecorded:
		code, problems = CodeUsage, fixed("record-invalid", "adopt the project again, or name the commit in the record",
			"the project's record names no commit of the template's branch %s, which its render needs", e.Branch)
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
		code, problems = CodeUsage, one("answer-malformed", "--answer takes name=answer, and %+q has no =", e.Given)
	case *answer.Unknown:
		code, problems = CodeUsage, one("answer-unknown", "the template asks no question %+q: its questions are %s", e.Name, strings.Join(e.Questions, ", "))
	case *answer.Twice:
		code, problems = CodeUsage, one("answer-twice", "the answer to %s is given twice", e.Name)
	case *answer.NotTaken:
		code, problems = CodeUsage, one("answer-malformed", "the answer to %s, %+q, is not one it takes: %v", e.Name, e.Answer, e.Reason)
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
	case *project.BadRecord:
		code, problems = CodeUsage, nil
		for _, p := range e.Problems {
			problems = append(problems, one("record-invalid", "the project's record, %s, cannot be read: %s", e.File, p)...)
		}
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
	case *git.Unreadable:
		code, problems = CodeEnvironment, one("file-unreadable", "cannot read the project's %s: %v", e.Path, e.Err)
	case *git.Unwritable:
		code, problems = CodeEnvironment, one("file-unwritable", "cannot write the project's %s: %v", e.Path, e.Err)
	case *git.TooOld:
		code, problems = CodeEnvironment, fixed("git-too-old", "install git "+e.Need+" or a newer one",
			"update and adopt --merge merge with git merge-tree, which takes a template's renders as trees from git %s on, and this git is older (%s)", e.Need, e.Version)
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

func adoptionProblem(err adoption.Error) (code int, problems []Problem) {
	code, problems = CodeInternal, internal(err)
	switch e := err.(type) {
	case *adoption.NotRepository:
		code, problems = CodeEnvironment, fixed("git-repository-missing", "make the project a git repository and commit it, or run adopt in its folder",
			"adopt links an existing project's git repository, and git finds none here (%s)", e.Why)
	case *adoption.NoWorkTree:
		code, problems = CodeEnvironment, fixed("git-work-tree-missing", "run adopt in the project's checkout, a clone of the repository",
			"this is a bare git repository, or a .git folder, with no working tree to compare with the template's baseline")
	case *adoption.NoCommit:
		code, problems = CodeRefused, fixed("git-commit-missing", "commit the project first, then adopt it",
			"the repository at %s has no commit: adopt writes a record only beside a committed HEAD", e.Top)
	case *adoption.Dirty:
		code, problems = CodeRefused, fixed("working-tree-dirty", "commit or stash the changes, or remove the untracked files, then adopt again; --dry-run previews a dirty project",
			"the project's index and working tree are not clean (%s): adopt writes a record only in a clean project, --force or not", strings.Join(e.Paths, ", "))
	case *adoption.HasRecord:
		code, problems = CodeRefused, fixed("record-exists", "itos-template update takes the project from its record; to link it to another baseline, remove the record first",
			"the project holds %s already: adopt never replaces a record, --force or not", e.File)
	case *adoption.Differs:
		code, problems = CodeRefused, fixed("baseline-differs", "rerun with --merge to merge the baseline in, with --force to accept them and write the record alone, or with --dry-run --diff to see the text changes",
			"the project differs from the baseline in %d %s, listed on the standard output: adopt writes the record only when --merge or --force accepts them", e.Count, paths(e.Count))
	case *adoption.Conflicted:
		code, problems = CodeRefused, fixed("merge-conflict", "resolve each, stage it with git add and commit; or undo the whole adoption with git reset --hard",
			"merging the template's release %s into the project left conflicts in %s, each marked in its file and unmerged in the index as git merge leaves one, the rest of the merge and the record staged",
			e.Release, strings.Join(e.Paths, ", "))
	}
	return code, problems
}

// paths is "path" or "paths", as count says.
func paths(count int) string {
	if count == 1 {
		return "path"
	}
	return "paths"
}

// fixed is a problem of its rule, its sentence, format with args, then
// what to do, fix, which it gives apart too.
func fixed(rule, fix, format string, args ...any) []Problem {
	return []Problem{{Rule: rule, Message: fmt.Sprintf(format, args...) + "; " + fix, Fix: fix}}
}

func updatingProblem(err updating.Error) (code int, problems []Problem) {
	code, problems = CodeInternal, internal(err)
	switch e := err.(type) {
	case *updating.NotRepository:
		code, problems = CodeEnvironment, fixed("git-repository-missing", "run update in a made or adopted project's folder",
			"update merges into a project's git repository, and git finds none here (%s)", e.Why)
	case *updating.NoWorkTree:
		code, problems = CodeEnvironment, fixed("git-work-tree-missing", "run update in the project's checkout, a clone of the repository",
			"this is a bare git repository, or a .git folder, with no working tree to merge into")
	case *updating.NoCommit:
		code, problems = CodeRefused, fixed("git-commit-missing", "make the project with itos-template new, or commit it and adopt it",
			"the repository at %s has no commit, so no record of a render to update from", e.Top)
	case *updating.Dirty:
		code, problems = CodeRefused, fixed("working-tree-dirty", "commit or stash the changes, or remove the untracked files, then update again",
			"the project's index and working tree are not clean (%s): update merges only into a clean project, so git reset --hard undoes it", strings.Join(e.Paths, ", "))
	case *updating.NoRecord:
		code, problems = CodeRefused, fixed("record-missing", "a project made before the template takes its updates once itos-template adopt links it and its record is committed",
			"the project's HEAD holds no %s, the record of the template render update starts from", e.File)
	case *updating.Answered:
		code, problems = CodeUsage, fixed("answer-recorded", "leave --answer "+e.Name+" out: update answers only the questions a newer release adds",
			"the record answers %s already, as %+q, and update keeps the record's answers", e.Name, e.Answer)
	case *updating.NoRelease:
		code, problems = CodeRefused, fixed("release-missing", "release the template (itos-template release), or name a pre-release with --to",
			"the template has no complete stable release to update to: a release is one version tagged <branch>/<version> on every branch its manifest lists")
	case *updating.Older:
		code, problems = CodeRefused, fixed("release-older", "name a release newer than "+e.Recorded+" with --to, or leave --to out for the newest",
			"the project is at the template's release %s, and %s is older: update only goes forward", e.Recorded, e.Target)
	case *updating.Conflicted:
		code, problems = CodeRefused, fixed("merge-conflict", "resolve each, stage it with git add and commit; or undo the whole update with git reset --hard",
			"merging the template's release %s into the project left conflicts in %s, each marked in its file and unmerged in the index as git merge leaves one, the rest of the update and the new record staged",
			e.Release, strings.Join(e.Paths, ", "))
	}
	return code, problems
}

func releasingProblem(err releasing.Error) (code int, problems []Problem) {
	code, problems = CodeInternal, internal(err)
	switch e := err.(type) {
	case *releasing.BadVersion:
		code, problems = CodeUsage, fixed("release-version", "give vX.Y.Z, three numbers, with an optional pre-release: v1.2.0, v1.3.0-rc.1",
			"the version %+q is not one a release is cut as", e.Version)
	case *releasing.NotRepository:
		code, problems = CodeEnvironment, fixed("git-repository-missing", "run release in the template's checkout or clone",
			"release tags the branches of the template's git repository it runs in, and git finds none here (%s)", e.Why)
	case *releasing.NoOrigin:
		code, problems = CodeRefused, fixed("origin-missing", "add the remote people fetch the template from as origin (git remote add origin <url>), fetch it, then release",
			"the template's repository has no remote origin: a release is for others to fetch, so release tags only commits origin has and prints the push that publishes the tags there")
	case *releasing.NoDefaultBranch:
		code, problems = CodeUsage, fixed("default-branch-missing", "point origin's HEAD at the template's default branch, or record it here with git remote set-head origin <branch>, then release",
			"origin names no default branch, neither as git clone recorded it nor when asked: release tags the template's root, its default branch as origin names it, whatever branch is checked out")
	case *releasing.Used:
		code, problems = CodeRefused, fixed("release-exists", "cut another version; a tag no one has fetched yet can be deleted with git tag -d",
			"the template carries %s already: a version is released once, a partial release's tags included", strings.Join(e.Tags, ", "))
	case *releasing.Unsynced:
		code, problems = CodeRefused, unsynced(e)
	case *releasing.Failed:
		code, problems = CodeRefused, fixed("check-failed", "fix the template, then release again",
			"%d of %d combinations failed, as the report says, so release made no tag", e.Failed, e.All)
	case *releasing.Refused:
		if len(e.Left) == 0 {
			code, problems = CodeRefused, fixed("tag-refused", "fix what git says, then release again",
				"git refused the tag %s (%v), and release deleted the tags it made before it, so it made none", e.Tag, e.Err)
		} else {
			code, problems = CodeRefused, fixed("tag-refused", "delete each with git tag -d, fix what git says, then release again",
				"git refused the tag %s (%v), and release could not delete %s, which it made before it", e.Tag, e.Err, strings.Join(e.Left, ", "))
		}
	}
	return code, problems
}

// unsynced is the problem of a branch whose local head is not origin's:
// how it differs, and what makes it origin's.
func unsynced(e *releasing.Unsynced) []Problem {
	fix := map[releasing.How]string{
		releasing.Ahead:       "push it, then release",
		releasing.Behind:      "fast-forward it to origin's, then release",
		releasing.Diverged:    "merge origin's into it and push it, then release",
		releasing.Unpublished: "push it to origin, then release",
	}[e.How]
	where := string(e.How) + " origin's"
	if e.How == releasing.Unpublished {
		where = "not on origin"
	}
	return fixed("branch-unpublished", fix,
		"the template's branch %s is %s: a release tags only the heads others can fetch, so every branch the manifest lists must be at origin's head of it", e.Branch, where)
}
