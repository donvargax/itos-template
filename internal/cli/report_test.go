package cli

import (
	"errors"
	"testing"

	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/template"
)

// No scenario's render fails for want of git or for a defect of ours, and
// none checks a template of one combination.
func TestEachResultsExitCode(t *testing.T) {
	for _, c := range []struct {
		r    template.Result
		code int
	}{
		{template.Result{}, 0},
		{template.Result{Checks: []template.Ran{{Status: template.Failed}}}, 1},
		{template.Result{Err: &template.MergeConflict{Branch: "go/cli"}}, 1},
		{template.Result{Err: &git.Missing{Err: errors.New("no git")}}, 3},
		{template.Result{Err: errors.New("a defect")}, 70},
	} {
		if got := ResultCode(c.r); got != c.code {
			t.Errorf("ResultCode(%+v) = %d, not %d", c.r, got, c.code)
		}
	}
}

func TestTheSummaryCountsTheCombinationsThatPassed(t *testing.T) {
	if got := Summary(1, 1); got != "\n1 of 1 combination passed.\n" {
		t.Errorf("Summary(1, 1) = %q", got)
	}
	if got := Summary(1, 2); got != "\n1 of 2 combinations passed.\n" {
		t.Errorf("Summary(1, 2) = %q", got)
	}
}
