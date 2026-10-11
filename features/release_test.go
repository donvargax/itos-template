// The steps of release.feature: a template's clone with every branch local,
// as its author or its CI releases from one, its branches moved off origin's
// and its working tree changed; the tags release makes, read from git, never
// from what release says; the tags pushed to an origin of the scenario's
// own; and release's --json object.
//
// A pushed tag would reach every later scenario through the fixture
// template, which the run builds once and shares (templates_test.go), and
// change the release new renders by default there. So a push first gives
// the clone an origin of its own, a mirror of the fixture in the scratch
// folder, and the scenario's template becomes that mirror: the fixture is
// never written.
package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

func (w *world) releaseCutSteps(sc *godog.ScenarioContext) {
	sc.Step(`^a clone "([^"]*)" of the template, every branch local$`, w.cloneEveryBranchLocal)
	sc.Step(`^a copy "([^"]*)" of the template with no origin$`, w.copyWithNoOrigin)
	sc.Step(`^a folder "([^"]*)" outside any git repository$`, func(dir string) error {
		return w.projectIs(dir, "a folder outside any git repository")
	})
	sc.Step(`^in "([^"]*)" the branch "([^"]*)" carries the tag "([^"]*)"$`, w.branchCarriesTag)
	sc.Step(`^in "([^"]*)" the branch "([^"]*)" is (ahead of|behind|diverged from) origin's$`, w.branchMovedOffOrigin)
	sc.Step(`^in "([^"]*)" an uncommitted change to "([^"]*)" and an untracked file "([^"]*)"$`, w.uncommittedChange)
	sc.Step(`^itos-template ran in the folder "([^"]*)" with "([^"]*)"$`, w.ranInFolder)
	sc.Step(`^the tags of "([^"]*)" pushed to its origin$`, w.pushTags)

	sc.Step(`^in "([^"]*)" every branch the template lists carries the annotated tag "([^"]*)" at its head, its message naming "([^"]*)"$`, w.everyBranchTagged)
	sc.Step(`^in "([^"]*)" the tag "([^"]*)" is annotated and names the commit of origin's "([^"]*)"$`, w.tagNamesOriginsHead)
	sc.Step(`^the template has no tag "([^"]*)"$`, w.templateHasNoTag)
	sc.Step(`^"([^"]*)" has no tag ending "([^"]*)"$`, w.noTagEnding)
	sc.Step(`^in "([^"]*)" no branch but "([^"]*)" carries a tag ending "([^"]*)"$`, w.onlyBranchCarries)
	sc.Step(`^its report checks no combination$`, w.reportChecksNone)
	sc.Step(`^after the tags of "([^"]*)" are pushed to its origin, new renders the release "([^"]*)" by default$`, w.newRendersAfterPush)
	sc.Step(`^in "([^"]*)" the change to "([^"]*)" and the file "([^"]*)" are still there, uncommitted$`, w.changeStillThere)
	sc.Step(`^its release JSON is ok, version "([^"]*)", with a tag for each branch the template lists at its head$`, w.releaseJSONOK)
	sc.Step(`^its release JSON push begins (".*")$`, w.releaseJSONPush)
	sc.Step(`^its release JSON is not ok, with no tag, and its report says "([^"]*)" failed$`, w.releaseJSONFailed)
}

// The clones.

// cloneEveryBranchLocal clones the scenario's template into the folder name
// of the scratch repository, every branch of origin's given a local branch
// at its head, as a template's author has them or its CI makes them. A
// folder the scenario cloned before (the background's) is moved away first,
// so a scenario naming its own template clones that one.
func (w *world) cloneEveryBranchLocal(name string) error {
	if w.templateDir == "" {
		return errors.New("no template in this scenario")
	}
	dir := w.path(name)
	if _, err := os.Lstat(dir); err == nil {
		if err := w.moveAway(dir); err != nil {
			return err
		}
	}
	if err := w.git("clone", "-q", "--", w.template, name); err != nil {
		return err
	}
	current, err := w.gitOut(dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return err
	}
	branches, err := w.templateBranches()
	if err != nil {
		return err
	}
	for _, b := range branches {
		if b == current {
			continue
		}
		if err := w.gitIn(dir, "branch", "-q", "--track", b, "refs/remotes/origin/"+b); err != nil {
			return err
		}
	}
	return nil
}

// copyWithNoOrigin is a clone of the template with every branch local
// whose remote origin is removed, its remote-tracking branches with it.
func (w *world) copyWithNoOrigin(name string) error {
	if err := w.cloneEveryBranchLocal(name); err != nil {
		return err
	}
	return w.gitIn(w.path(name), "remote", "remove", "origin")
}

// templateBranches are the branches of the scenario's template: those its
// manifest lists, as every fixture holds exactly those.
func (w *world) templateBranches() ([]string, error) {
	out, err := w.gitOut(w.templateDir, "for-each-ref", "--format=%(refname:strip=2)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// branchCarriesTag tags the branch's head in the clone dir with tag, a
// lightweight tag, as a release cut by hand may have left one.
func (w *world) branchCarriesTag(dir, branch, tag string) error {
	return w.gitIn(w.path(dir), "tag", tag, "refs/heads/"+branch)
}

// branchMovedOffOrigin moves the clone dir's local branch off origin's head
// of it, origin's remote-tracking branch left where it is: one commit past
// it (ahead of), at its parent (behind), or one commit past its parent
// (diverged from). Each commit is made with git commit-tree, so nothing is
// checked out and the working tree stays as it is.
func (w *world) branchMovedOffOrigin(dir, branch, how string) error {
	repo := w.path(dir)
	ref := "refs/heads/" + branch
	from := ref
	if how != "ahead of" {
		from = ref + "~1"
	}
	to, err := w.gitOut(repo, "rev-parse", "--verify", from+"^{commit}")
	if err != nil {
		return err
	}
	if how != "behind" {
		tree, err := w.gitOut(repo, "rev-parse", "--verify", to+"^{tree}")
		if err != nil {
			return err
		}
		if to, err = w.gitOut(repo, "commit-tree", tree, "-p", to, "-m", "A commit origin does not have"); err != nil {
			return err
		}
	}
	return w.gitIn(repo, "update-ref", ref, to)
}

// The change to a file and the untracked file uncommittedChange makes.
const (
	uncommittedLine = "a change no release tags\n"
	untrackedText   = "notes no release tags\n"
)

// uncommittedChange adds a line to the clone dir's tracked file, left
// uncommitted, and writes the untracked file untracked.
func (w *world) uncommittedChange(dir, file, untracked string) error {
	p := filepath.Join(w.path(dir), filepath.FromSlash(file))
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, append(data, uncommittedLine...), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(w.path(dir), filepath.FromSlash(untracked)), []byte(untrackedText), 0o644)
}

// changeStillThere is whether the clone dir still holds uncommittedChange's
// change to file, unstaged and uncommitted, and the file untracked,
// untracked, as git says, each holding what it was given.
func (w *world) changeStillThere(dir, file, untracked string) error {
	repo := w.path(dir)
	for _, c := range []struct {
		args []string
		want []string
	}{
		{[]string{"diff", "--name-only"}, []string{file}},
		{[]string{"diff", "--cached", "--name-only"}, nil},
		{[]string{"ls-files", "--others", "--exclude-standard"}, []string{untracked}},
	} {
		out, err := w.gitOut(repo, c.args...)
		if err != nil {
			return err
		}
		if got := strings.Fields(out); !slices.Equal(got, c.want) {
			return fmt.Errorf("git %s in %s lists %q, not %q\n%s", strings.Join(c.args, " "), dir, got, c.want, w.report())
		}
	}
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(file)))
	if err != nil {
		return err
	}
	if !strings.HasSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), uncommittedLine) {
		return fmt.Errorf("%s in %s no longer ends with the uncommitted line %q", file, dir, uncommittedLine)
	}
	notes, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(untracked)))
	if err != nil {
		return err
	}
	if string(notes) != untrackedText {
		return fmt.Errorf("%s in %s holds %q, not %q", untracked, dir, notes, untrackedText)
	}
	return nil
}

// ranInFolder runs itos-template in the folder dir with args, as the
// runs-in-the-folder step does, and fails unless it exits 0: what a Given
// sets up must have happened.
func (w *world) ranInFolder(dir, args string) error {
	fields := strings.Fields(args)
	for i, f := range fields {
		fields[i] = w.expand(f)
	}
	if err := w.run(w.path(dir), w.bin, fields...); err != nil {
		return err
	}
	return w.exitsWith(0)
}

// reportChecksNone is whether release wrote no report at all: check never
// ran, so not even the count of the combinations that passed ends it.
func (w *world) reportChecksNone() error {
	if w.stdout != "" {
		return fmt.Errorf("standard output is not empty: check ran\n%s", w.report())
	}
	return nil
}

// The push.

// pushTags gives the clone dir an origin of its own, a mirror of the
// scenario's template in the scratch folder, and pushes every tag of the
// clone there in one atomic push; the mirror is the scenario's template
// from then on, so {template} names it.
func (w *world) pushTags(dir string) error {
	if w.templateDir == "" {
		return errors.New("no template in this scenario")
	}
	origin := w.path(dir + "-origin.git")
	if err := w.git("clone", "-q", "--mirror", "--", w.template, origin); err != nil {
		return err
	}
	repo := w.path(dir)
	if err := w.gitIn(repo, "remote", "set-url", "origin", filepath.ToSlash(origin)); err != nil {
		return err
	}
	if err := w.gitIn(repo, "push", "-q", "--atomic", "origin", "--tags"); err != nil {
		return err
	}
	w.templateDir, w.template = origin, filepath.ToSlash(origin)
	return nil
}

// newRendersAfterPush pushes the clone dir's tags to an origin of its own
// (pushTags), then has new render the template into a folder of its own
// with no --ref, and reads the release its record names.
func (w *world) newRendersAfterPush(dir, version string) error {
	if err := w.pushTags(dir); err != nil {
		return err
	}
	made := dir + "-made"
	if err := w.runWith(w.env(), "new {template} "+made+" --stack go --feature cli --answer name=blue-fox --defaults"); err != nil {
		return err
	}
	if err := w.exitsWith(0); err != nil {
		return err
	}
	return w.recordNamesRelease(made, version)
}

// The tags.

// annotatedTag is the commit the annotated tag tag names in the repository
// repo, and its message; an error when it is no annotated tag.
func (w *world) annotatedTag(repo, tag string) (commit, message string, err error) {
	kind, err := w.gitOut(repo, "cat-file", "-t", "refs/tags/"+tag)
	if err != nil {
		return "", "", fmt.Errorf("no tag %s: %v\n%s", tag, err, w.report())
	}
	if kind != "tag" {
		return "", "", fmt.Errorf("the tag %s is a lightweight one, naming a %s, not an annotated one\n%s", tag, kind, w.report())
	}
	if commit, err = w.gitOut(repo, "rev-parse", "--verify", "refs/tags/"+tag+"^{commit}"); err != nil {
		return "", "", err
	}
	message, err = w.gitOut(repo, "for-each-ref", "--format=%(contents)", "refs/tags/"+tag)
	return commit, message, err
}

// everyBranchTagged is whether, in the clone dir, every branch of the
// template carries the annotated tag pattern names, <branch> replaced by the
// branch, at the clone's head of that branch, its message naming version.
func (w *world) everyBranchTagged(dir, pattern, version string) error {
	repo := w.path(dir)
	branches, err := w.templateBranches()
	if err != nil {
		return err
	}
	for _, b := range branches {
		tag := strings.ReplaceAll(pattern, "<branch>", b)
		commit, message, err := w.annotatedTag(repo, tag)
		if err != nil {
			return err
		}
		head, err := w.gitOut(repo, "rev-parse", "--verify", "refs/heads/"+b+"^{commit}")
		if err != nil {
			return err
		}
		if commit != head {
			return fmt.Errorf("the tag %s names %s, not the head of %s, %s\n%s", tag, commit, b, head, w.report())
		}
		if !strings.Contains(message, version) {
			return fmt.Errorf("the tag %s's message, %q, does not name %s\n%s", tag, message, version, w.report())
		}
	}
	return nil
}

// tagNamesOriginsHead is whether, in the clone dir, tag is an annotated tag
// naming the head of origin's branch.
func (w *world) tagNamesOriginsHead(dir, tag, branch string) error {
	repo := w.path(dir)
	commit, _, err := w.annotatedTag(repo, tag)
	if err != nil {
		return err
	}
	head, err := w.gitOut(repo, "rev-parse", "--verify", "refs/remotes/origin/"+branch+"^{commit}")
	if err != nil {
		return err
	}
	if commit != head {
		return fmt.Errorf("the tag %s names %s, not origin's head of %s, %s\n%s", tag, commit, branch, head, w.report())
	}
	return nil
}

// tags are the tags of the repository repo.
func (w *world) tags(repo string) ([]string, error) {
	out, err := w.gitOut(repo, "for-each-ref", "--format=%(refname:strip=2)", "refs/tags/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// templateHasNoTag is whether the scenario's template holds no tag tag.
func (w *world) templateHasNoTag(tag string) error {
	tags, err := w.tags(w.templateDir)
	if err != nil {
		return err
	}
	if slices.Contains(tags, tag) {
		return fmt.Errorf("the template has the tag %s\n%s", tag, w.report())
	}
	return nil
}

// noTagEnding is whether the repository dir holds no tag ending suffix.
func (w *world) noTagEnding(dir, suffix string) error {
	tags, err := w.tags(w.path(dir))
	if err != nil {
		return err
	}
	for _, tag := range tags {
		if strings.HasSuffix(tag, suffix) {
			return fmt.Errorf("%s has the tag %s\n%s", dir, tag, w.report())
		}
	}
	return nil
}

// onlyBranchCarries is whether the only tag ending suffix the repository
// dir holds is branch's.
func (w *world) onlyBranchCarries(dir, branch, suffix string) error {
	tags, err := w.tags(w.path(dir))
	if err != nil {
		return err
	}
	for _, tag := range tags {
		if strings.HasSuffix(tag, suffix) && tag != branch+suffix {
			return fmt.Errorf("%s has the tag %s too\n%s", dir, tag, w.report())
		}
	}
	return nil
}

// The --json object.

// releaseObject is release's --json object, as far as the steps read it.
type releaseObject struct {
	Schema   int                  `json:"schema"`
	OK       *bool                `json:"ok"`
	Version  string               `json:"version"`
	Tags     []releaseTag         `json:"tags"`
	Push     []string             `json:"push"`
	Report   []releaseCombination `json:"report"`
	Problems []struct {
		Rule    string `json:"rule"`
		Message string `json:"message"`
	} `json:"problems"`
}

// releaseTag is a tag of release's object: its name, its branch and the
// commit it names.
type releaseTag struct {
	Name   string `json:"name"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

// releaseCombination is a combination of the report in release's object,
// its name and whether it passed.
type releaseCombination struct {
	Combination string `json:"combination"`
	Passed      *bool  `json:"passed"`
}

// releaseJSON reads standard output as release's one --json object, of
// schema 1, with an ok, a list of tags, a push and a report.
func (w *world) releaseJSON() (*releaseObject, error) {
	var r releaseObject
	dec := json.NewDecoder(strings.NewReader(w.stdout))
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("standard output is no JSON object: %v\n%s", err, w.report())
	}
	if dec.More() {
		return nil, fmt.Errorf("standard output holds more than one JSON value\n%s", w.report())
	}
	if r.Schema != 1 || r.OK == nil || r.Tags == nil || r.Push == nil || r.Report == nil {
		return nil, fmt.Errorf("the JSON output is no release object of schema 1, with ok, tags, push and report\n%s", w.report())
	}
	return &r, nil
}

// releaseJSONOK is whether release's object is ok, of version, its tags
// one for each branch of the template, <branch>/<version>, naming the
// template's head of that branch, and its report naming every combination
// passed.
func (w *world) releaseJSONOK(version string) error {
	r, err := w.releaseJSON()
	if err != nil {
		return err
	}
	if !*r.OK || r.Version != version || len(r.Problems) > 0 {
		return fmt.Errorf("the release object is not ok, of version %s, with no problem\n%s", version, w.report())
	}
	branches, err := w.templateBranches()
	if err != nil {
		return err
	}
	if len(r.Tags) != len(branches) {
		return fmt.Errorf("the release object names %d tags, not one for each of %q\n%s", len(r.Tags), branches, w.report())
	}
	for _, b := range branches {
		head, err := w.gitOut(w.templateDir, "rev-parse", "--verify", "refs/heads/"+b+"^{commit}")
		if err != nil {
			return err
		}
		i := slices.IndexFunc(r.Tags, func(t releaseTag) bool { return t.Branch == b })
		if i < 0 || r.Tags[i].Name != b+"/"+version || r.Tags[i].Commit != head {
			return fmt.Errorf("the release object names no tag %s/%s at %s\n%s", b, version, head, w.report())
		}
	}
	if len(r.Report) == 0 || slices.ContainsFunc(r.Report, func(c releaseCombination) bool { return c.Passed == nil || !*c.Passed }) {
		return fmt.Errorf("the release object's report does not say every combination passed\n%s", w.report())
	}
	return nil
}

// releaseJSONPush is whether release's push begins with the words listed.
func (w *world) releaseJSONPush(list string) error {
	r, err := w.releaseJSON()
	if err != nil {
		return err
	}
	want := quotedList(list)
	if len(r.Push) < len(want) || !slices.Equal(r.Push[:len(want)], want) {
		return fmt.Errorf("the release object's push is %q, not beginning %q\n%s", r.Push, want, w.report())
	}
	return nil
}

// releaseJSONFailed is whether release's object is not ok, names no tag
// and no push, and its report says the combination name failed.
func (w *world) releaseJSONFailed(name string) error {
	r, err := w.releaseJSON()
	if err != nil {
		return err
	}
	if *r.OK || len(r.Tags) > 0 || len(r.Push) > 0 || len(r.Problems) == 0 {
		return fmt.Errorf("the release object is ok, names a tag or a push, or no problem\n%s", w.report())
	}
	for _, c := range r.Report {
		if c.Combination == name {
			if c.Passed == nil || *c.Passed {
				return fmt.Errorf("the release object's report says %s passed\n%s", name, w.report())
			}
			return nil
		}
	}
	return fmt.Errorf("the release object's report does not name %s\n%s", name, w.report())
}
