package branchlist

import (
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/cli"
	"github.com/donvargax/itos-template/internal/syncing"
)

// The scenarios read branch list's plain output for a branch missing
// locally alone (ID-SYNC-18); the rest of what people read, each branch's
// line and the advice on what to sync next, is checked here.
func TestShowSaysWhereEachBranchStandsAndWhatToSyncNext(t *testing.T) {
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
		}, "main: the root, up to date\nstack/go: lacks main\ngo/cli: up to date with stack/go\nSync next: stack/go, checked out with git switch stack/go, then itos-template branch sync.\n"},
		{[]syncing.Standing{
			{Branch: "main", From: []string{}},
			{Branch: "stack/go", From: []string{"main"}, Missing: true, Unknown: []string{"main"}},
			{Branch: "go/api", From: []string{"go/cli", "stack/go"}, Lacks: []string{"go/cli"}, Unknown: []string{"stack/go"}},
		}, "main: the root, up to date\nstack/go: missing, not a local branch\ngo/api: lacks go/cli; cannot be judged against stack/go, missing\nNothing can be synced until each missing branch is local.\n"},
	} {
		var stdout, stderr strings.Builder
		show(&cli.UI{Stdout: &stdout, Stderr: &stderr}, &syncing.Tree{Branches: tc.branches})
		if stdout.String() != tc.want || !strings.Contains(stderr.String(), "--json") {
			t.Errorf("show wrote\n%s\nnot\n%s", stdout.String(), tc.want)
		}
	}
}
