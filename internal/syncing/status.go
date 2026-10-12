package syncing

import (
	"errors"

	"github.com/donvargax/itos-template/internal/template/port"
)

// sync --status (q-58) is where the whole tree stands, read and never
// written: every branch the manifest names, with the branches it descends
// from whose commits it lacks, so a template's author, or its CI, sees which
// branches wait for a sync. It reads the local branches alone, as the merge
// does, and nothing of the checkout but where its repository is, so it
// reports from any checkout: a detached HEAD, an unclean one, one with a
// merge in progress.

// Standing is where a branch the manifest names stands: the branches it
// descends from (From), those whose commits it lacks (Lacks), those it
// cannot be judged against because one side is not local (Unknown), each
// in the manifest's order, and whether it is not local itself (Missing),
// when Unknown is all of From.
type Standing struct {
	Branch  string
	From    []string
	Lacks   []string
	Unknown []string
	Missing bool
}

// Tree is where every branch the manifest names stands, in the manifest's
// order, the root first.
type Tree struct {
	Branches []Standing
}

// Status is where every branch of the template the clone c holds stands,
// read from the local branches, writing nothing.
//
// It refuses, before reading a branch, a folder in no repository
// (*NotRepository), one with no working tree (*NoWorkTree), a repository
// with no origin (*NoOrigin) or one whose origin names no default branch
// (*NoDefaultBranch), the root not local (*NotLocal), and the manifest the
// root's head holds missing or invalid (template's *NoManifest and
// *ManifestInvalid). Then it gives the Tree, and with it, joined, a
// *Lagging for each branch that lacks commits of a branch it descends from
// and a *Missing for each not local, in the manifest's order: nil when
// every branch is local and in step.
func Status(c port.Clone) (*Tree, error) {
	state, err := look(c)
	if err != nil {
		return nil, err
	}
	m, root, err := manifestOf(c, state.Top)
	if err != nil {
		return nil, err
	}
	branches := m.Branches(root)
	heads := map[string]string{}
	for _, b := range branches {
		head, ok, err := c.Head(b)
		if err != nil {
			return nil, err
		}
		if ok {
			heads[b] = head
		}
	}
	t := &Tree{}
	var out []error
	for _, b := range branches {
		from, _ := m.Parents(root, b)
		s := Standing{Branch: b, From: from}
		if _, ok := heads[b]; ok {
			s.Lacks, s.Unknown, err = lacking(c, from, heads, b)
			if err != nil {
				return nil, err
			}
		} else {
			s.Missing, s.Unknown = true, from
			out = append(out, &Missing{Branch: b})
		}
		if len(s.Lacks) > 0 {
			out = append(out, &Lagging{Branch: b, Lacks: s.Lacks})
		}
		t.Branches = append(t.Branches, s)
	}
	return t, errors.Join(out...)
}

// Next is the branch to sync next: the first, in the manifest's order,
// that lacks commits of a branch it descends from while every branch above
// it is local and lacks none, so sync merges into it rather than refuse it;
// "" when there is none.
func (t *Tree) Next() string {
	byName := map[string]Standing{}
	for _, s := range t.Branches {
		byName[s.Branch] = s
	}
	for _, s := range t.Branches {
		if len(s.Lacks) > 0 && clearAbove(byName, s.Branch) {
			return s.Branch
		}
	}
	return ""
}

// clearAbove is whether every branch above b, of byName, is local and
// lacks no commit of a branch it descends from.
func clearAbove(byName map[string]Standing, b string) bool {
	for a := range above(b, func(b string) []string { return byName[b].From }) {
		if s := byName[a]; a != b && (s.Missing || len(s.Lacks) > 0) {
			return false
		}
	}
	return true
}
