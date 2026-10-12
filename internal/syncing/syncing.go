// Package syncing is a template's branch taking the upkeep of the branches
// it descends from (decision 2, features/sync.feature): run in a clone of
// the template, on a branch the manifest names, it merges into that branch
// the first branch it descends from whose commits it lacks, with git's own
// merge, and commits nothing (decision 42): the merge is left in progress and
// staged, or conflicted as git merge leaves it, for the person to commit
// through the template's own gates, as update leaves its merge (decision
// 37). No path is special (q-59): git's merge decides each.
//
// The branch tree is the manifest's (manifest.Parents), read from the
// root's local head, the root origin's default branch, as release reads it,
// never the branch checked out, which is the one being synced. The branches
// merged are the local ones: nothing is fetched. sync goes down the tree a
// branch at a time, so a branch above the one checked out that lacks
// commits of its own parents is refused, naming it, to be synced first.
//
// It reaches the clone only through a port.Clone. It is domain (decision
// 17): its tests run it against fakes of the port. Its refusals are a
// sealed set (Error), plain domain types carrying no exit code or wording.
package syncing

import (
	"errors"

	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/template"
	"github.com/donvargax/itos-template/internal/template/port"
)

// Sync is what a sync found and did: the branch checked out, the branches
// it descends from, the branch merged into it, "" when it lacked none, and
// the paths the merge staged and those it left conflicted, each sorted.
type Sync struct {
	Branch    string
	From      []string
	Merged    string
	Staged    []string
	Conflicts []string
}

// Run syncs the branch checked out in the clone c: it merges into it the
// first branch it descends from whose commits it lacks, in the manifest's
// order, and gives what it found and did; nothing when it lacks none, the
// root included, which descends from none.
//
// Each refusal comes before anything is written: a folder in no repository
// (*NotRepository), one with no working tree (*NoWorkTree), a branch with no
// commit (*NoCommit), a repository with no origin (*NoOrigin) or one whose
// origin names no default branch (*NoDefaultBranch), the root not local
// (*NotLocal), the manifest the root's head holds missing or invalid
// (template's *NoManifest and *ManifestInvalid), a detached HEAD
// (*Detached) or a branch the manifest does not name (*Unnamed), a branch
// above the one checked out not local (*NotLocal, every one joined) or
// lacking commits of its own parents (*Behind, every one joined), and,
// before a merge, a merge in progress (*Merging) or a checkout not clean
// (*Dirty). A merge that leaves conflicts is a *Conflicted, the Sync given
// with it. Once the branch is known, a refusal gives the Sync found so far.
func Run(c port.Clone) (*Sync, error) {
	state, err := c.Look()
	switch {
	case err != nil:
		return nil, err
	case !state.Repository:
		return nil, &NotRepository{Why: state.Why}
	case !state.WorkTree:
		return nil, &NoWorkTree{}
	case !state.Committed:
		return nil, &NoCommit{Top: state.Top}
	}
	m, root, err := manifestOf(c, state.Top)
	if err != nil {
		return nil, err
	}
	into := m.Branches(root)[1:]
	branch, ok, err := c.Branch()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &Detached{Branches: into}
	}
	from, named := m.Parents(root, branch)
	if !named {
		return &Sync{Branch: branch}, &Unnamed{Branch: branch, Branches: into}
	}
	s := &Sync{Branch: branch, From: from}
	heads, err := headsAbove(c, m, root, branch)
	if err != nil {
		return s, err
	}
	if err := behind(c, m, root, branch, heads); err != nil {
		return s, err
	}
	for _, p := range from {
		has, err := c.IsAncestor(heads[p], heads[branch])
		if err != nil {
			return s, err
		}
		if !has {
			return s, s.merge(c, state, p)
		}
	}
	return s, nil
}

// manifestOf is the manifest of the clone c, at top, and its root: origin's
// default branch, read from its local head.
func manifestOf(c port.Clone, top string) (*manifest.Manifest, string, error) {
	origin, err := c.HasOrigin()
	if err != nil {
		return nil, "", err
	}
	if !origin {
		return nil, "", &NoOrigin{}
	}
	root, ok := c.DefaultBranch()
	if !ok {
		return nil, "", &NoDefaultBranch{}
	}
	head, ok, err := c.Head(root)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", &NotLocal{Branch: root}
	}
	m, err := template.ReadManifest(c.File, top, head, root)
	return m, root, err
}

// headsAbove are the local heads of branch and of every branch above it,
// those it descends from and theirs, up to the root, by name: a *NotLocal
// for each that is not local, joined, in the manifest's order.
func headsAbove(c port.Clone, m *manifest.Manifest, root, branch string) (map[string]string, error) {
	above := map[string]bool{branch: true}
	for walk := []string{branch}; len(walk) > 0; walk = walk[1:] {
		parents, _ := m.Parents(root, walk[0])
		for _, p := range parents {
			if !above[p] {
				above[p] = true
				walk = append(walk, p)
			}
		}
	}
	heads := map[string]string{}
	var missing []error
	for _, b := range m.Branches(root) {
		if !above[b] {
			continue
		}
		head, ok, err := c.Head(b)
		if err != nil {
			return nil, err
		}
		if !ok {
			missing = append(missing, &NotLocal{Branch: b})
		}
		heads[b] = head
	}
	return heads, errors.Join(missing...)
}

// behind is a *Behind for each branch of heads but branch, the one checked
// out, that lacks commits of a branch it descends from, joined, in the
// manifest's order.
func behind(c port.Clone, m *manifest.Manifest, root, branch string, heads map[string]string) error {
	var lagging []error
	for _, b := range m.Branches(root) {
		head, above := heads[b]
		if !above || b == branch {
			continue
		}
		parents, _ := m.Parents(root, b)
		var lacks []string
		for _, p := range parents {
			has, err := c.IsAncestor(heads[p], head)
			if err != nil {
				return err
			}
			if !has {
				lacks = append(lacks, p)
			}
		}
		if len(lacks) > 0 {
			lagging = append(lagging, &Behind{Branch: b, Lacks: lacks})
		}
	}
	return errors.Join(lagging...)
}

// merge merges the branch p into s's branch, refusing a merge in progress
// or a checkout state says is not clean, and keeps what it staged and left
// conflicted.
func (s *Sync) merge(c port.Clone, state port.CheckoutState, p string) error {
	merging, err := c.Merging()
	if err != nil {
		return err
	}
	if merging {
		return &Merging{}
	}
	if len(state.Changed) > 0 {
		return &Dirty{Paths: state.Changed}
	}
	staged, err := c.Merge(p, s.Branch)
	if err != nil {
		return err
	}
	s.Merged, s.Staged, s.Conflicts = p, staged.Paths, staged.Conflicts
	if len(s.Conflicts) > 0 {
		return &Conflicted{Branch: p, Into: s.Branch, Paths: s.Conflicts}
	}
	return nil
}
