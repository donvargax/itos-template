package porttest

import (
	"testing"
	"testing/fstest"
)

// The fake answers as git does where the domain's tests never ask: a
// commit no branch or tag has, and a commit's ancestry of itself.
func TestRepositoryAnswersAsGitDoes(t *testing.T) {
	r := &Repository{Root: "main", Branches: map[string]fstest.MapFS{"main": {}}, Tagged: map[string]Tag{"main/v1": {Tree: fstest.MapFS{}, On: "main"}}}
	if _, _, err := r.File("commit of nosuch", "x"); err == nil {
		t.Error("File of no commit gave no error")
	}
	if is, err := r.IsAncestor("commit of main", "commit of main"); err != nil || !is {
		t.Errorf("a commit is not its own ancestor: %v, %v", is, err)
	}
	if is, _ := r.IsAncestor("tag main/v1", "commit of main"); !is {
		t.Error("a tag on main is not an ancestor of main's head")
	}
}
