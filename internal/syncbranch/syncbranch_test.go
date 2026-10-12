package syncbranch

import (
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/cli"
	"github.com/donvargax/itos-template/internal/syncing"
)

// Decision 19 leaves the UI to the scenarios, which read sync's exit code,
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

// The scenarios read --status's plain output for a branch missing locally
// alone (ID-SYNC-18); the rest of what people read, each branch's line and
// the advice on what to sync next, is checked here.
func TestShowStatusSaysWhereEachBranchStandsAndWhatToSyncNext(t *testing.T) {
	for _, tc := range []struct {
		branches []syncing.Standing
		want     string
	}{
		{[]syncing.Standing{
			{Branch: "main", From: []string{}},
			{Branch: "stack/go", From: []string{"main"}},
			{Branch: "go/api", From: []string{"go/cli", "go/web"}},
		}, "main: the root, up to date\nstack/go: up to date with main\ngo/api: up to date with go/cli and go/web\nEvery branch is up to date.\n"},
		{[]syncing.Standing{
			{Branch: "main", From: []string{}},
			{Branch: "stack/go", From: []string{"main"}, Lacks: []string{"main"}},
			{Branch: "go/cli", From: []string{"stack/go"}},
		}, "main: the root, up to date\nstack/go: lacks main\ngo/cli: up to date with stack/go\nSync next: stack/go, checked out with git switch stack/go, then itos-template sync.\n"},
		{[]syncing.Standing{
			{Branch: "main", From: []string{}},
			{Branch: "stack/go", From: []string{"main"}, Missing: true, Unknown: []string{"main"}},
			{Branch: "go/api", From: []string{"go/cli", "stack/go"}, Lacks: []string{"go/cli"}, Unknown: []string{"stack/go"}},
		}, "main: the root, up to date\nstack/go: missing, not a local branch\ngo/api: lacks go/cli; cannot be judged against stack/go, missing\nNothing can be synced until each missing branch is local.\n"},
	} {
		var stdout, stderr strings.Builder
		showStatus(&cli.UI{Stdout: &stdout, Stderr: &stderr}, &syncing.Tree{Branches: tc.branches})
		if stdout.String() != tc.want || !strings.Contains(stderr.String(), "--json") {
			t.Errorf("showStatus wrote\n%s\nnot\n%s", stdout.String(), tc.want)
		}
	}
}
