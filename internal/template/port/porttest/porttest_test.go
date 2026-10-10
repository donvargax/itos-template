package porttest

import (
	"errors"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/donvargax/itos-template/internal/template/port"
)

// The fake answers as git does where the domain's tests never ask: a
// commit no branch or tag has, and a commit's ancestry of itself.
func TestRepositoryAnswersAsGitDoes(t *testing.T) {
	r := &Repository{Root: "main", Branches: map[string]fstest.MapFS{"main": {}}, Tagged: map[string]Tag{"main/v1": {Tree: fstest.MapFS{}, On: "main"}}}
	if _, ok, err := r.File("commit of nosuch", "x"); err == nil || ok {
		t.Errorf("File of no commit gave %v, %v", ok, err)
	}
	for _, c := range [][2]string{{"commit of nosuch", "commit of main"}, {"tag main/v1", "commit of nosuch"}} {
		if is, err := r.IsAncestor(c[0], c[1]); err == nil || is {
			t.Errorf("IsAncestor(%s, %s) = %v, %v", c[0], c[1], is, err)
		}
	}
	if is, err := r.IsAncestor("commit of main", "commit of main"); err != nil || !is {
		t.Errorf("a commit is not its own ancestor: %v, %v", is, err)
	}
	if is, _ := r.IsAncestor("tag main/v1", "commit of main"); !is {
		t.Error("a tag on main is not an ancestor of main's head")
	}
}

func files(pairs ...string) []port.File {
	var list []port.File
	for i := 0; i+1 < len(pairs); i += 2 {
		list = append(list, port.File{Path: pairs[i], Mode: 0o644, Data: []byte(pairs[i+1])})
	}
	return list
}

// The fake merges each path as a whole file: the side that changed it wins,
// a change both made alike is one, and changes unlike conflict, HEAD's side
// left in the tree, or the target's when HEAD removed it.
func TestProjectMergesEachPathAsAWholeFile(t *testing.T) {
	p := &Project{Head: map[string]port.File{}}
	for _, f := range files("ours", "ours\n", "theirs", "base\n", "both", "alike\n", "clash", "ours\n", "kept", "base\n", "gone", "base\n", "added", "ours\n") {
		p.Head[f.Path] = f
	}
	exec := port.File{Path: "mode", Mode: 0o755, Data: []byte("base\n")}
	p.Head["mode"] = exec
	base := files("ours", "base\n", "theirs", "base\n", "both", "base\n", "clash", "base\n", "kept", "base\n", "gone", "base\n", "deleted", "base\n", "mode", "base\n", "dropped", "base\n")
	target := files("ours", "base\n", "theirs", "theirs\n", "both", "alike\n", "clash", "theirs\n", "kept", "base\n", "new", "theirs\n", "deleted", "theirs\n", "mode", "base\n")
	m, err := p.Merge(base, target)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"ours": "ours\n", "theirs": "theirs\n", "both": "alike\n", "clash": "ours\n", "kept": "base\n", "added": "ours\n", "new": "theirs\n", "deleted": "theirs\n", "mode": "base\n"}
	if len(p.Tree) != len(want) {
		t.Errorf("the tree holds %d files, not %d: %v", len(p.Tree), len(want), p.Tree)
	}
	for path, data := range want {
		if string(p.Tree[path].Data) != data {
			t.Errorf("the tree's %s holds %q, not %q", path, p.Tree[path].Data, data)
		}
	}
	if p.Tree["mode"].Mode != 0o755 {
		t.Error("a mode only HEAD changed is not kept")
	}
	if !slices.Equal(m.Conflicts, []port.Unmerged{{Path: "clash"}, {Path: "deleted"}}) {
		t.Errorf("the conflicts are %v", m.Conflicts)
	}
	stages := p.Stages["deleted"]
	if string(stages[0].Data) != "base\n" || stages[1] != nil || string(stages[2].Data) != "theirs\n" {
		t.Errorf("deleted's stages are %v", stages)
	}
	wantChanges := []port.Change{
		{Path: "deleted", Kind: port.Added}, {Path: "gone", Kind: port.Removed},
		{Path: "new", Kind: port.Added}, {Path: "theirs", Kind: port.Modified},
	}
	if !slices.Equal(m.Changes, wantChanges) {
		t.Errorf("the changes are %v, not %v", m.Changes, wantChanges)
	}
	if p.Merges != 1 {
		t.Errorf("%d merges counted", p.Merges)
	}
}

func TestProjectReadsHeadAndFailsAsToldTo(t *testing.T) {
	state := port.CheckoutState{Repository: true, Top: "/made"}
	p := &Project{State: state, Head: map[string]port.File{"a": {Path: "a", Data: []byte("a\n")}}}
	if got, err := p.Look(); err != nil || got.Top != "/made" {
		t.Errorf("Look = %v, %v", got, err)
	}
	if data, ok, err := p.Committed("a"); err != nil || !ok || string(data) != "a\n" {
		t.Errorf("Committed(a) = %q, %v, %v", data, ok, err)
	}
	if _, ok, err := p.Committed("b"); err != nil || ok {
		t.Errorf("Committed(b) = %v, %v", ok, err)
	}
	p.Fail = errors.New("broken")
	if _, ok, err := p.Committed("a"); err != p.Fail || ok {
		t.Errorf("Committed of a broken HEAD = %v", err)
	}
	if _, err := p.Merge(nil, nil); err != p.Fail || p.Merges != 0 {
		t.Errorf("Merge into a broken HEAD = %v, %d merges", err, p.Merges)
	}
}
