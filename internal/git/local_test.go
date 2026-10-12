package git

import (
	"path/filepath"
	"slices"
	"testing"
)

// A git that fails where branch sync's scenarios run a working
// one: none can make
// git fail reading HEAD, merging or listing what a merge staged, each an
// internal error (exit 70).
func TestLocalGivesGitsFailure(t *testing.T) {
	useGit(t, filepath.Join(t.TempDir(), "no-git"))
	l := &Local{Worktree: &Worktree{top: t.TempDir()}}
	if b, ok, err := l.Branch(); err == nil || ok || b != "" {
		t.Errorf("Branch with no git = %q, %v, %v", b, ok, err)
	}
	if s, err := l.Merge("main", "stack/go"); err == nil || s.Paths != nil || s.Conflicts != nil {
		t.Errorf("Merge with no git = %+v, %v", s, err)
	}
	if s, err := l.staged(); err == nil || s.Paths != nil || s.Conflicts != nil {
		t.Errorf("staged with no git = %+v, %v", s, err)
	}
}

// What git status lists after a merge, each entry its two letters, a space
// and its path: the scenarios' paths are never one character long, nor
// do they make each kind of unmerged entry, which a stub git writes here.
func TestStagedReadsEachEntryGitStatusLists(t *testing.T) {
	stubGit(t, "M  a\x00UU b\x00A  c.txt\x00DD d\x00AA e\x00UD f\x00 M g\x00", false)
	s, err := (&Local{Worktree: &Worktree{top: t.TempDir()}}).staged()
	if err != nil || !slices.Equal(s.Paths, []string{"a", "c.txt"}) || !slices.Equal(s.Conflicts, []string{"b", "d", "e", "f"}) {
		t.Errorf("staged = %+v, %v", s, err)
	}
}
