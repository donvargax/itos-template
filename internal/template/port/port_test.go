package port

import "testing"

// The domain wraps a conflict in its own error, so no scenario reads this.
func TestConflictSaysWhichCommitAndWhere(t *testing.T) {
	err := &Conflict{At: 2, Paths: []string{"a.txt", "b.txt"}}
	if got := err.Error(); got != "merging commit 2 leaves conflicts in a.txt, b.txt" {
		t.Errorf("Error() = %q", got)
	}
}
