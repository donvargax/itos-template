package manifest

import (
	"slices"
	"testing"
)

func TestBranchesAreTheRootThenTheStacksThenTheFeatures(t *testing.T) {
	want := []string{"trunk", "stack/go", "stack/python", "go/cli", "go/web", "python/cli"}
	if got := acme(t).Branches("trunk"); !slices.Equal(got, want) {
		t.Errorf("Branches = %q", got)
	}
}

// docs/manifest.md's "Branches": a stack descends from the root, a feature
// from its stack, and one that needs others from each of them, in the
// manifest's order, not from its stack.
func TestParentsFollowTheBranchTree(t *testing.T) {
	m := acme(t)
	m.Features = append(m.Features, Feature{Name: "api", Stack: "go", Needs: []string{"web", "cli"}})
	for b, want := range map[string][]string{
		"main":         {},
		"stack/go":     {"main"},
		"stack/python": {"main"},
		"go/cli":       {"stack/go"},
		"go/web":       {"go/cli"},
		"python/cli":   {"stack/python"},
		"go/api":       {"go/cli", "go/web"},
	} {
		got, ok := m.Parents("main", b)
		if !ok || got == nil || !slices.Equal(got, want) {
			t.Errorf("Parents(%s) = %q, %v", b, got, ok)
		}
	}
	for _, b := range []string{"topic", "stack/rust", "python/web", "cli", "trunk"} {
		if got, ok := m.Parents("main", b); ok || got != nil {
			t.Errorf("Parents(%s), a branch the manifest does not name, = %q, %v", b, got, ok)
		}
	}
}
