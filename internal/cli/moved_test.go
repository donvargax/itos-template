package cli

import (
	"strings"
	"testing"
)

// An old command's flag counts only before --, as kong would have read it;
// after --, it is an argument. No scenario gives one after --.
func TestMovedGivesAFlagOnlyBeforeDashDash(t *testing.T) {
	for _, c := range []struct {
		words []string
		want  bool
	}{
		{nil, false},
		{[]string{"--json"}, false},
		{[]string{"--status"}, true},
		{[]string{"--json", "--status"}, true},
		{[]string{"--", "--status"}, false},
		{[]string{"--status", "--"}, true},
	} {
		if got := (Moved{Words: c.words}).Gives("--status"); got != c.want {
			t.Errorf("Moved%q.Gives(--status) = %v, want %v", c.words, got, c.want)
		}
	}
}

// The old name is a usage error naming the command that replaced it, with
// --json its failure object too (rule 29).
func TestMovedRunsNothingAndNamesTheCommandThatReplacedIt(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Moved{}.Run(&UI{Stdout: &stdout, Stderr: &stderr}, "sync", "branch sync", true)
	if code != CodeUsage {
		t.Errorf("exit %d, not %d", code, CodeUsage)
	}
	if want := `{"schema":1,"ok":false,"problems":[{"rule":"usage","message":"sync is now branch sync: run itos-template branch sync"}]}` + "\n"; stdout.String() != want {
		t.Errorf("stdout is\n%s", stdout.String())
	}
	if want := "itos-template: sync is now branch sync: run itos-template branch sync\n"; stderr.String() != want {
		t.Errorf("stderr is\n%s", stderr.String())
	}
}
