package manifest

import "slices"

// The branch tree (docs/manifest.md's "Branches"): the root, the template's
// default branch, which the manifest does not name; a stack off the root; a
// feature off its stack, or off each feature it needs. sync merges a branch
// down the tree, into each branch from the ones it descends from.

// Branches are the branches the manifest names, root its root branch: the
// root first, then each stack's, then each feature's, in the manifest's
// order.
func (m *Manifest) Branches(root string) []string {
	list := []string{root}
	for _, s := range m.Stacks {
		list = append(list, s.Branch())
	}
	for _, f := range m.Features {
		list = append(list, f.Branch())
	}
	return list
}

// Parents are the branches the branch b descends from, root its root
// branch: none for the root; the root for a stack; for a feature, each
// feature it needs, in the manifest's order, or its stack when it needs
// none, since a needed feature carries the stack already. False when the
// manifest names no branch b.
func (m *Manifest) Parents(root, b string) ([]string, bool) {
	if b == root {
		return []string{}, true
	}
	for _, s := range m.Stacks {
		if s.Branch() == b {
			return []string{root}, true
		}
	}
	for _, f := range m.Features {
		if f.Branch() != b {
			continue
		}
		if len(f.Needs) == 0 {
			return []string{Stack{Name: f.Stack}.Branch()}, true
		}
		var parents []string
		for _, need := range m.Features {
			if need.Stack == f.Stack && slices.Contains(f.Needs, need.Name) {
				parents = append(parents, need.Branch())
			}
		}
		return parents, true
	}
	return nil, false
}
