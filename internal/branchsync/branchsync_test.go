package branchsync

import (
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/cli"
	"github.com/donvargax/itos-template/internal/syncing"
)

// Decision 19 leaves the UI to the scenarios, which read branch sync's exit code,
// its --json object and the branches its plain output names. What none
// reads yet is the plain output's advice, which a conflict changes (the
// idea outside-test-gaps): a merge left clean is to be committed, while a
// conflicted one's advice is its problem's, on stderr.
func TestShowAdvisesCommittingOnlyAMergeLeftClean(t *testing.T) {
	const commit = "Commit it through the template's own gates"
	for _, tc := range []struct {
		s    syncing.Sync
		want string
		adv  bool
	}{
		{syncing.Sync{Branch: "stack/go", From: []string{"main"}, Merged: "main", Staged: []string{"shared.txt"}}, "Merged main into stack/go with git merge, uncommitted, 1 path staged:\n  shared.txt\n", true},
		{syncing.Sync{Branch: "stack/go", From: []string{"main"}, Merged: "main", Staged: []string{"a", "b"}, Conflicts: []string{"README.md"}}, "Merged main into stack/go with git merge, uncommitted, 2 paths staged:\n  a\n  b\n", false},
		{syncing.Sync{Branch: "main", From: []string{}}, "main is the template's root, which descends from no branch: up to date.\n", false},
		{syncing.Sync{Branch: "go/api", From: []string{"go/cli", "go/web"}}, "go/api is up to date: it lacks no commit of go/cli or go/web.\n", false},
	} {
		var stdout, stderr strings.Builder
		show(&cli.UI{Stdout: &stdout, Stderr: &stderr}, &tc.s)
		if !strings.HasPrefix(stdout.String(), tc.want) || strings.Contains(stdout.String(), commit) != tc.adv {
			t.Errorf("show(%+v) wrote\n%s", tc.s, stdout.String())
		}
	}
}
