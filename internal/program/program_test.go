package program

import (
	"strings"
	"testing"
)

// Decision 18 leaves infra to the scenarios, and they hold a check that
// passes and one that fails; none yet names a program that cannot be
// started, so that stays here until the idea scenario-gaps gives it one.
func TestRunFailsAProgramThatCannotBeStartedSayingWhy(t *testing.T) {
	out, ok := Runner{}.Run(t.TempDir(), []string{"itos-template-no-such-program", "x"})
	if ok || !strings.HasPrefix(string(out), "cannot run itos-template-no-such-program: ") {
		t.Errorf("Run = %q, %v", out, ok)
	}
}
