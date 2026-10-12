package syncing

import (
	"errors"
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/template"
	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

// inStep is acme's tree in step, in the manifest's order: each branch and
// the branches it descends from.
var inStep = []Standing{
	{Branch: "main", From: []string{}},
	{Branch: "stack/go", From: []string{"main"}},
	{Branch: "stack/python", From: []string{"main"}},
	{Branch: "go/cli", From: []string{"stack/go"}},
	{Branch: "go/web", From: []string{"go/cli"}},
	{Branch: "python/cli", From: []string{"stack/python"}},
}

// standing is the standing of the branch b in t, and false when t lists
// none.
func standing(t *Tree, b string) (Standing, bool) {
	for _, s := range t.Branches {
		if s.Branch == b {
			return s, true
		}
	}
	return Standing{}, false
}

// problems are the branches err's *Lagging and *Missing name, each
// "<branch> lacks <branches>" or "<branch> missing", in err's order.
func problems(err error) []string {
	var all []string
	for _, e := range multiple(err) {
		var lagging *Lagging
		var missing *Missing
		switch {
		case errors.As(e, &lagging):
			all = append(all, lagging.Error())
		case errors.As(e, &missing):
			all = append(all, missing.Branch+" missing")
		}
	}
	return all
}

// A template in step lists every branch the manifest names, in its order,
// none lacking a branch or missing, from any checkout: one whose HEAD is
// detached, unborn or on a branch of none, unclean or merging. Nothing is
// written: no branch moves, no merge starts.
func TestStatusListsEveryBranchInStepFromAnyCheckout(t *testing.T) {
	for name, change := range map[string]func(*porttest.Clone){
		"clean":    func(*porttest.Clone) {},
		"detached": func(c *porttest.Clone) { c.HEAD = "" },
		"unborn":   func(c *porttest.Clone) { c.HEAD, c.State.Committed = "orphan", false },
		"unclean":  func(c *porttest.Clone) { c.State.Changed = []string{"README.md", "notes.txt"} },
		"merging":  func(c *porttest.Clone) { c.MergeHead = c.Local["main"] },
	} {
		c := acme()
		change(c)
		local, merging := maps.Clone(c.Local), c.MergeHead
		tree, err := Status(c)
		if err != nil || !reflect.DeepEqual(tree.Branches, inStep) || tree.Next() != "" {
			t.Errorf("a %s checkout in step gives %+v, %v", name, tree, err)
		}
		if !maps.Equal(c.Local, local) || c.MergeHead != merging {
			t.Errorf("a %s checkout's status leaves branches %v, a merge of %q", name, c.Local, c.MergeHead)
		}
	}
}

// main's new commit is lacked by the stacks alone: the branches below them
// lack nothing of the branch they descend from until a stack takes it.
func TestStatusNamesEachBranchThatLacksABranchItDescendsFrom(t *testing.T) {
	c := acme()
	gain(c, "main", "shared.txt", "shared")
	tree, err := Status(c)
	if got := problems(err); !slices.Equal(got, []string{"stack/go lacks main", "stack/python lacks main"}) {
		t.Errorf("main's new commit gives the problems %q", got)
	}
	for b, lacks := range map[string][]string{"main": nil, "stack/go": {"main"}, "stack/python": {"main"}, "go/cli": nil, "go/web": nil} {
		if s, ok := standing(tree, b); !ok || !slices.Equal(s.Lacks, lacks) || s.Missing || s.Unknown != nil {
			t.Errorf("main's new commit leaves %s %+v", b, s)
		}
	}
	if tree.Next() != "stack/go" {
		t.Errorf("main's new commit is synced next into %q", tree.Next())
	}
	// A feature that needs another lacks that feature's commit, not its
	// stack's.
	c = acme()
	gain(c, "go/cli", "cli.txt", "more")
	tree, err = Status(c)
	if got := problems(err); !slices.Equal(got, []string{"go/web lacks go/cli"}) || tree.Next() != "go/web" {
		t.Errorf("go/cli's new commit gives %q, %q next", got, tree.Next())
	}
}

// A branch missing locally is reported as missing, and so is a lag that
// cannot be judged because one side is missing; neither stops the report.
func TestStatusNamesABranchMissingLocally(t *testing.T) {
	c := acme()
	for _, b := range []string{"stack/go", "go/cli", "go/web", "stack/python", "python/cli"} {
		delete(c.Local, b)
	}
	c.HEAD = ""
	tree, err := Status(c)
	if got := problems(err); !slices.Equal(got, []string{"stack/go missing", "stack/python missing", "go/cli missing", "go/web missing", "python/cli missing"}) {
		t.Errorf("only the root local gives %q", got)
	}
	if s, _ := standing(tree, "go/web"); !s.Missing || !slices.Equal(s.Unknown, []string{"go/cli"}) || s.Lacks != nil {
		t.Errorf("go/web missing stands %+v", s)
	}
	if s, _ := standing(tree, "main"); s.Missing || s.Unknown != nil {
		t.Errorf("the root stands %+v", s)
	}
	c = acme()
	delete(c.Local, "stack/go")
	gain(c, "main", "shared.txt", "shared")
	tree, err = Status(c)
	if got := problems(err); !slices.Equal(got, []string{"stack/go missing", "stack/python lacks main"}) {
		t.Errorf("stack/go missing and main's new commit give %q", got)
	}
	if s, _ := standing(tree, "go/cli"); s.Missing || !slices.Equal(s.Unknown, []string{"stack/go"}) || s.Lacks != nil {
		t.Errorf("go/cli under a missing stack stands %+v", s)
	}
	if tree.Next() != "stack/python" {
		t.Errorf("stack/go missing, %q is synced next", tree.Next())
	}
}

// The next branch to sync is the first, in the manifest's order, that branch
// sync would merge into rather than refuse: every branch above it local and in
// step.
func TestNextIsTheFirstLaggingBranchWithEveryBranchAboveInStep(t *testing.T) {
	for _, tc := range []struct {
		name     string
		branches []Standing
		want     string
	}{
		{"in step", inStep, ""},
		{"a feature listed before the feature it needs, which lags", []Standing{
			{Branch: "main", From: []string{}},
			{Branch: "stack/go", From: []string{"main"}},
			{Branch: "go/api", From: []string{"go/cli"}, Lacks: []string{"go/cli"}},
			{Branch: "go/cli", From: []string{"stack/go"}, Lacks: []string{"stack/go"}},
		}, "go/cli"},
		{"a branch above missing", []Standing{
			{Branch: "main", From: []string{}},
			{Branch: "stack/go", From: []string{"main"}, Missing: true, Unknown: []string{"main"}},
			{Branch: "go/cli", From: []string{"stack/go"}, Unknown: []string{"stack/go"}},
			{Branch: "go/web", From: []string{"go/cli"}, Lacks: []string{"go/cli"}},
		}, ""},
		{"two lagging, the first synced first", []Standing{
			{Branch: "main", From: []string{}},
			{Branch: "stack/go", From: []string{"main"}, Lacks: []string{"main"}},
			{Branch: "stack/python", From: []string{"main"}, Lacks: []string{"main"}},
		}, "stack/go"},
	} {
		if got := (&Tree{Branches: tc.branches}).Next(); got != tc.want {
			t.Errorf("%s: %q is synced next, not %q", tc.name, got, tc.want)
		}
	}
}

func TestStatusRefusesARepositoryItCannotRead(t *testing.T) {
	for name, tc := range map[string]struct {
		change func(*porttest.Clone)
		want   any
	}{
		"no repository":     {func(c *porttest.Clone) { c.State = port.CheckoutState{Why: "fatal: not a git repository"} }, new(*NotRepository)},
		"no working tree":   {func(c *porttest.Clone) { c.State = port.CheckoutState{Repository: true} }, new(*NoWorkTree)},
		"no origin":         {func(c *porttest.Clone) { c.Remote = false }, new(*NoOrigin)},
		"no default branch": {func(c *porttest.Clone) { c.Default = "" }, new(*NoDefaultBranch)},
		"the root missing":  {func(c *porttest.Clone) { delete(c.Local, "main") }, new(*NotLocal)},
		"no manifest": {func(c *porttest.Clone) {
			delete(c.Trees[c.Local["main"]], manifest.File)
		}, new(*template.NoManifest)},
	} {
		c := acme()
		tc.change(c)
		if tree, err := Status(c); !errors.As(err, tc.want) || tree != nil {
			t.Errorf("%s gives %+v, %v", name, tree, err)
		}
	}
}

// A port's failure that means nothing to the domain passes through it.
func TestStatusPassesThePortsFailuresThrough(t *testing.T) {
	broken := errors.New("git broke")
	for _, method := range []string{"Look", "HasOrigin", "Head", "Head stack/python", "File", "IsAncestor"} {
		c := acme()
		c.Fails = map[string]error{method: broken}
		if tree, err := Status(c); !errors.Is(err, broken) || tree != nil {
			t.Errorf("%s failing gives %+v, %v", method, tree, err)
		}
	}
}
