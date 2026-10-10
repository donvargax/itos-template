package template

import (
	"errors"
	"slices"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/donvargax/itos-template/internal/manifest"
)

// A template release (decision 36) is one version tagged <branch>/<version>
// on every branch the manifest lists, the root included: main/v1.2.0,
// stack/go/v1.2.0, go/cli/v1.2.0. The version is the tag's last path
// segment, since a branch's name holds slashes, and a version
// golang.org/x/mod/semver refuses (semver.IsValid) is no release's. A
// release's manifest is the one its root branch's tag holds, and it lists
// the branches the release must tag; while the root's tag is missing, the
// root branch's head's does.
//
// A release is complete when every branch its manifest lists carries its
// tag at a commit on that branch: the branch's head or one of its
// ancestors, so a tag moved off its branch, or left on a branch deleted
// since, is a tag the release lacks. Releases are ordered by semver, the
// newest the highest stable (no pre-release part) complete one, never the
// one tagged last: a backport cut after a newer release never wins.
//
// A template renders its branch heads until UseRelease or UseNewest picks a
// release; then it renders that release's commits, its manifest the
// release's, and the record names the release beside them.

// Release is a complete release of a template: its version, each branch's
// tagged commit, and the manifest its root's commit holds.
type Release struct {
	Version  string
	Commits  map[string]string
	manifest *manifest.Manifest
}

// Release is the version of the release t renders, or "" when it renders
// its branch heads.
func (t *Template) Release() string {
	if t.release == nil {
		return ""
	}
	return t.release.Version
}

// UseRelease has t render the release version, a pre-release too. A version
// no branch the manifest lists carries a tag of is a *NoRelease; a release
// a branch lacks its tag of, an *Incomplete naming each such branch.
func (t *Template) UseRelease(version string) error {
	tags, err := t.tags()
	if err != nil {
		return err
	}
	r, err := t.find(version, tags)
	if err != nil {
		return err
	}
	t.use(r)
	return nil
}

// UseNewest has t render its newest release: the highest stable version its
// root branch's tags name whose release is complete. It is false, t
// rendering its branch heads, when there is none.
func (t *Template) UseNewest() (bool, error) {
	tags, err := t.tags()
	if err != nil {
		return false, err
	}
	for _, v := range slices.Backward(t.versions(tags)) {
		if semver.Prerelease(v) != "" {
			continue
		}
		r, err := t.find(v, tags)
		var incomplete *Incomplete
		if errors.As(err, &incomplete) {
			continue
		}
		if err != nil {
			return false, err
		}
		t.use(r)
		return true, nil
	}
	return false, nil
}

func (t *Template) use(r *Release) {
	t.release = r
	t.Manifest = r.manifest
	t.rootCommit = r.Commits[t.root]
}

// tags are the template's tags, each name's commit.
func (t *Template) tags() (map[string]string, error) {
	list, err := t.repo.Tags()
	if err != nil {
		return nil, err
	}
	tags := map[string]string{}
	for _, tag := range list {
		tags[tag.Name] = tag.Commit
	}
	return tags, nil
}

// versions are the versions the root branch's tags name, each a version
// semver takes, oldest first, by semver; versions semver holds equal (v1
// and v1.0.0, or two builds) by their text.
func (t *Template) versions(tags map[string]string) []string {
	var versions []string
	for name := range tags {
		v, ok := strings.CutPrefix(name, t.root+"/")
		if ok && semver.IsValid(v) {
			versions = append(versions, v)
		}
	}
	slices.SortFunc(versions, func(a, b string) int {
		if c := semver.Compare(a, b); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	return versions
}

// find is the release version, from the template's tags: a *NoRelease when
// no branch its manifest lists carries the version's tag, an *Incomplete
// when one does not.
func (t *Template) find(version string, tags map[string]string) (*Release, error) {
	none := &NoRelease{Version: version, Releases: t.versions(tags)}
	if !semver.IsValid(version) {
		return nil, none
	}
	m := t.Manifest
	if root, ok := tags[t.root+"/"+version]; ok {
		var err error
		if m, err = t.manifestAt(root, t.root+"/"+version); err != nil {
			return nil, err
		}
	}
	r := &Release{Version: version, Commits: map[string]string{}, manifest: m}
	tagged := false
	var lacking []string
	for _, b := range branches(t.root, m) {
		commit, ok := tags[b+"/"+version]
		tagged = tagged || ok
		on, err := t.onBranch(commit, ok, b)
		if err != nil {
			return nil, err
		}
		if !on {
			lacking = append(lacking, b)
			continue
		}
		r.Commits[b] = commit
	}
	switch {
	case !tagged:
		return nil, none
	case len(lacking) > 0:
		return nil, &Incomplete{Version: version, Branches: lacking}
	}
	return r, nil
}

// onBranch is whether commit, a tag's when tagged, is on the branch b: its
// head or one of its ancestors.
func (t *Template) onBranch(commit string, tagged bool, b string) (bool, error) {
	if !tagged {
		return false, nil
	}
	head, ok, err := t.repo.Commit(b)
	if err != nil || !ok {
		return false, err
	}
	return t.repo.IsAncestor(commit, head)
}

// branches are the branches the manifest m lists, the root first, then each
// stack's, then each feature's, in the manifest's order.
func branches(root string, m *manifest.Manifest) []string {
	list := []string{root}
	for _, s := range m.Stacks {
		list = append(list, s.Branch())
	}
	for _, f := range m.Features {
		list = append(list, f.Branch())
	}
	return list
}

// manifestAt is the manifest the commit holds, at, its tag or branch, saying
// where it was read from in an error.
func (t *Template) manifestAt(commit, at string) (*manifest.Manifest, error) {
	data, ok, err := t.repo.File(commit, manifest.File)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &NoManifest{Template: t.Name, Root: at}
	}
	m, err := manifest.Parse(data)
	if err != nil {
		var invalid *manifest.Invalid
		if errors.As(err, &invalid) {
			return nil, &ManifestInvalid{Root: at, Problems: invalid.Problems}
		}
		return nil, err
	}
	return m, nil
}
