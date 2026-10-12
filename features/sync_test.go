// The steps of sync.feature: a template's clone, every branch local or only
// its default one, a branch given a commit so another lacks it, a branch
// checked out or a new one made; and what sync leaves: the merge git has in
// progress, read from git (MERGE_HEAD and the index's unmerged entries),
// never from what sync says, and its --json object, parsed, never searched.
// The clones, the snapshots, the staged paths and the conflict markers are
// the steps release.feature, adopt.feature and update.feature take them
// with.
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

func (w *world) syncSteps(sc *godog.ScenarioContext) {
	sc.Step(`^in "([^"]*)" the branch "([^"]*)" gains a commit setting "([^"]*)" to "([^"]*)"$`, w.branchGainsCommit)
	sc.Step(`^in "([^"]*)" the branch "([^"]*)" is checked out$`, w.branchCheckedOut)
	sc.Step(`^in "([^"]*)" a new branch "([^"]*)" is checked out$`, func(dir, branch string) error {
		return w.gitIn(w.path(dir), "checkout", "-q", "-b", branch)
	})

	sc.Step(`^in "([^"]*)" a merge of "([^"]*)" is in progress$`, func(dir, branch string) error {
		return w.mergeInProgress(dir, branch, nil)
	})
	sc.Step(`^in "([^"]*)" a merge of "([^"]*)" is in progress, (".*") unmerged$`, func(dir, branch, list string) error {
		return w.mergeInProgress(dir, branch, quotedList(list))
	})
	sc.Step(`^in "([^"]*)" no merge is in progress$`, w.noMergeInProgress)
	sc.Step(`^its sync JSON is ok, branch "([^"]*)", merged "([^"]*)", staged (".*") and no conflict$`, w.syncJSONMerged)
	sc.Step(`^its sync JSON is not ok, the problem "([^"]*)", conflicts (".*")$`, w.syncJSONConflicted)
}

// branchGainsCommit commits, in the clone dir, file holding text and a
// line ending on the branch branch, with git's own commit: the branch is
// checked out for it, then the branch checked out before is again, so the
// clone is left on the branch it was on, its working tree clean.
func (w *world) branchGainsCommit(dir, branch, file, text string) error {
	repo := w.path(dir)
	before, err := w.gitOut(repo, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return err
	}
	if err := w.gitIn(repo, "checkout", "-q", branch); err != nil {
		return err
	}
	p := filepath.Join(repo, filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(text+"\n"), 0o644); err != nil {
		return err
	}
	if err := w.gitIn(repo, "add", "--", file); err != nil {
		return err
	}
	if err := w.commit(dir, "Set "+file+" on "+branch); err != nil {
		return err
	}
	return w.gitIn(repo, "checkout", "-q", before)
}

// mergeHead is the commit the clone dir's MERGE_HEAD names, "" when git has
// no merge in progress there.
func (w *world) mergeHead(dir string) (string, error) {
	p, err := w.gitOut(w.path(dir), "rev-parse", "--git-path", "MERGE_HEAD")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(w.path(dir), p)
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return strings.TrimSpace(string(data)), err
}

// mergeInProgress is whether git has, in the clone dir, a merge of the
// local branch branch in progress, uncommitted: MERGE_HEAD names its head,
// and it alone, and the index holds unmerged exactly the paths unmerged
// lists, none for a merge that left no conflict.
func (w *world) mergeInProgress(dir, branch string, unmerged []string) error {
	head, err := w.mergeHead(dir)
	if err != nil {
		return err
	}
	want, err := w.gitOut(w.path(dir), "rev-parse", "--verify", "refs/heads/"+branch+"^{commit}")
	if err != nil {
		return err
	}
	if head != want {
		return fmt.Errorf("the merge in progress in %s is of %q, not of %s's head %s\n%s", dir, head, branch, want, w.report())
	}
	out, err := w.gitOut(w.path(dir), "ls-files", "-u", "-z")
	if err != nil {
		return err
	}
	var paths []string
	for _, e := range strings.Split(out, "\x00") {
		if _, p, ok := strings.Cut(e, "\t"); ok && !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	slices.Sort(unmerged)
	if !slices.Equal(paths, unmerged) {
		return fmt.Errorf("the index of %s holds %q unmerged, not %q\n%s", dir, paths, unmerged, w.report())
	}
	return nil
}

// noMergeInProgress is whether git has no merge in progress in the clone
// dir: no MERGE_HEAD.
func (w *world) noMergeInProgress(dir string) error {
	head, err := w.mergeHead(dir)
	if err != nil {
		return err
	}
	if head != "" {
		return fmt.Errorf("%s has a merge of %s in progress\n%s", dir, head, w.report())
	}
	return nil
}

// syncObject is sync's --json object, as a script reads it.
type syncObject struct {
	Schema    int         `json:"schema"`
	OK        *bool       `json:"ok"`
	Branch    *string     `json:"branch"`
	Merged    *string     `json:"merged"`
	Staged    *[]syncPath `json:"staged"`
	Conflicts *[]syncPath `json:"conflicts"`
	Problems  *[]struct {
		Rule string `json:"rule"`
	} `json:"problems"`
}

// syncPath is a path of the sync object's staged or conflicts.
type syncPath struct {
	Path string `json:"path"`
}

// pathsOf are the paths of list, in its order.
func pathsOf(list []syncPath) []string {
	var paths []string
	for _, p := range list {
		paths = append(paths, p.Path)
	}
	return paths
}

// syncJSON is standard output read as one sync object, every key a script
// reads of it there, of schema 1.
func (w *world) syncJSON() (*syncObject, error) {
	var o syncObject
	dec := json.NewDecoder(strings.NewReader(w.stdout))
	if err := dec.Decode(&o); err != nil {
		return nil, fmt.Errorf("standard output is no sync object: %v\n%s", err, w.report())
	}
	if dec.More() {
		return nil, fmt.Errorf("standard output holds more than one JSON value\n%s", w.report())
	}
	if o.Schema != 1 || o.OK == nil || o.Staged == nil || o.Conflicts == nil || o.Problems == nil {
		return nil, fmt.Errorf("the sync object is not of schema 1 with ok, staged, conflicts and problems\n%s", w.report())
	}
	return &o, nil
}

// syncJSONMerged is whether sync's object is ok, its branch and the branch
// it merged as given, the paths it staged exactly those listed, in any
// order, and no conflict or problem.
func (w *world) syncJSONMerged(branch, merged, list string) error {
	o, err := w.syncJSON()
	if err != nil {
		return err
	}
	staged := slices.Sorted(slices.Values(pathsOf(*o.Staged)))
	want := slices.Sorted(slices.Values(quotedList(list)))
	switch {
	case !*o.OK:
		return fmt.Errorf("the sync object is not ok\n%s", w.report())
	case o.Branch == nil || *o.Branch != branch:
		return fmt.Errorf("the sync object's branch is not %q\n%s", branch, w.report())
	case o.Merged == nil || *o.Merged != merged:
		return fmt.Errorf("the sync object's merged is not %q\n%s", merged, w.report())
	case !slices.Equal(staged, want):
		return fmt.Errorf("the sync object stages %q, not %q\n%s", staged, want, w.report())
	case len(*o.Conflicts) > 0 || len(*o.Problems) > 0:
		return fmt.Errorf("the sync object names a conflict or a problem\n%s", w.report())
	}
	return nil
}

// syncJSONConflicted is whether sync's object is not ok, a problem of it
// the rule given, and its conflicts exactly the paths listed, in that
// order.
func (w *world) syncJSONConflicted(rule, list string) error {
	o, err := w.syncJSON()
	if err != nil {
		return err
	}
	var rules []string
	for _, p := range *o.Problems {
		rules = append(rules, p.Rule)
	}
	switch {
	case *o.OK:
		return fmt.Errorf("the sync object is ok\n%s", w.report())
	case !slices.Contains(rules, rule):
		return fmt.Errorf("the sync object's problems are %q, none %q\n%s", rules, rule, w.report())
	case !slices.Equal(pathsOf(*o.Conflicts), quotedList(list)):
		return fmt.Errorf("the sync object's conflicts are %q, not %q\n%s", pathsOf(*o.Conflicts), quotedList(list), w.report())
	}
	return nil
}
