package syncing

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/template"
	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

// manifestText is acme's tree: two stacks, go's cli and web, web needing
// cli, and python's cli.
const manifestText = `version: 1
stacks:
  - name: go
  - name: python
features:
  - name: cli
    stack: go
  - name: web
    stack: go
    needs: [cli]
  - name: cli
    stack: python
questions:
  - name: name
    literal: acme-widget
    question: Name?
`

// tree is each branch and the branch it starts from, in the order they are
// made, as features/testdata/acme/branches.txt makes the fixture's.
var tree = [][2]string{
	{"main", ""},
	{"stack/go", "main"},
	{"go/cli", "stack/go"},
	{"go/web", "go/cli"},
	{"stack/python", "main"},
	{"python/cli", "stack/python"},
}

// into are the branches branch sync can merge into, in the manifest's order.
var into = []string{"stack/go", "stack/python", "go/cli", "go/web", "python/cli"}

// acme is a clone of the template in step, every branch local, each
// branch's commit "<branch> 0" on the one it starts from, its tree its
// parent's and a file of its own, with stack/go checked out, clean.
func acme() *porttest.Clone {
	c := &porttest.Clone{
		State:   port.CheckoutState{Repository: true, WorkTree: true, Top: "/home/you/acme", Committed: true},
		Remote:  true,
		Default: "main",
		HEAD:    "stack/go",
		Local:   map[string]string{},
		Parents: map[string][]string{},
		Trees:   map[string]fstest.MapFS{},
	}
	for _, b := range tree {
		commit := b[0] + " 0"
		files := fstest.MapFS{manifest.File: {Data: []byte(manifestText)}, "README.md": {Data: []byte("# acme-widget\n")}}
		if b[1] != "" {
			files = maps.Clone(c.Trees[c.Local[b[1]]])
			c.Parents[commit] = []string{c.Local[b[1]]}
		}
		files[b[0]+".txt"] = &fstest.MapFile{Data: []byte(b[0] + "\n")}
		c.Local[b[0]], c.Trees[commit] = commit, files
	}
	return c
}

// gain gives the branch b of c a commit on its head setting the file path
// to text.
func gain(c *porttest.Clone, b, path, text string) {
	head := c.Local[b]
	commit := head + "+" + path
	files := maps.Clone(c.Trees[head])
	files[path] = &fstest.MapFile{Data: []byte(text + "\n")}
	c.Local[b], c.Trees[commit], c.Parents[commit] = commit, files, []string{head}
}

func TestRunMergesTheFirstBranchItDescendsFromThatItLacks(t *testing.T) {
	for _, tc := range []struct{ gains, on, from string }{
		{"main", "stack/go", "main"},
		{"stack/go", "go/cli", "stack/go"},
		{"go/cli", "go/web", "go/cli"},
		{"stack/python", "python/cli", "stack/python"},
	} {
		c := acme()
		gain(c, tc.gains, "shared.txt", "shared")
		c.HEAD = tc.on
		s, err := Run(c)
		if err != nil {
			t.Fatalf("syncing %s: %v", tc.on, err)
		}
		if s.Branch != tc.on || !slices.Equal(s.From, []string{tc.from}) || s.Merged != tc.from ||
			!slices.Equal(s.Staged, []string{"shared.txt"}) || len(s.Conflicts) != 0 {
			t.Errorf("syncing %s gives %+v", tc.on, s)
		}
		if c.MergeHead != c.Local[tc.from] {
			t.Errorf("syncing %s leaves a merge of %q in progress", tc.on, c.MergeHead)
		}
	}
}

// A branch already holding every commit of the branches it descends from,
// and the root, which descends from none, are synced already: nothing is
// merged.
func TestRunMergesNothingIntoABranchThatLacksNothing(t *testing.T) {
	for _, b := range []string{"main", "stack/go", "go/cli", "go/web", "python/cli"} {
		c := acme()
		c.HEAD = b
		s, err := Run(c)
		if err != nil || s.Branch != b || s.Merged != "" || s.Staged != nil || c.MergeHead != "" {
			t.Errorf("syncing %s gives %+v, %v, a merge of %q", b, s, err, c.MergeHead)
		}
	}
	c := acme()
	c.HEAD = "main"
	if s, _ := Run(c); s.From == nil || len(s.From) != 0 {
		t.Errorf("the root descends from %q", s.From)
	}
	// stack/go merged main's new commit, and go/cli stack/go's merge.
	c = acme()
	gain(c, "main", "shared.txt", "shared")
	gain(c, "stack/go", "shared.txt", "shared")
	c.Parents[c.Local["stack/go"]] = append(c.Parents[c.Local["stack/go"]], c.Local["main"])
	gain(c, "go/cli", "shared.txt", "shared")
	c.Parents[c.Local["go/cli"]] = append(c.Parents[c.Local["go/cli"]], c.Local["stack/go"])
	c.HEAD = "go/cli"
	if s, err := Run(c); err != nil || s.Merged != "" {
		t.Errorf("a branch that merged its parent gives %+v, %v", s, err)
	}
}

// A feature that needs two takes the first of them it lacks, in the
// manifest's order, whatever order its needs list them in.
func TestRunTakesTheNeededFeaturesInTheManifestsOrder(t *testing.T) {
	c := acme()
	api := manifestText[:strings.Index(manifestText, "questions:")] + "  - name: api\n    stack: go\n    needs: [web, cli]\n" + manifestText[strings.Index(manifestText, "questions:"):]
	for _, files := range c.Trees {
		files[manifest.File] = &fstest.MapFile{Data: []byte(api)}
	}
	c.Local["go/api"], c.Trees["go/api 0"] = "go/api 0", c.Trees[c.Local["go/web"]]
	c.Parents["go/api 0"] = []string{c.Local["go/web"]}
	gain(c, "go/web", "b.txt", "b")
	c.HEAD = "go/api"
	s, err := Run(c)
	if err != nil || !slices.Equal(s.From, []string{"go/cli", "go/web"}) || s.Merged != "go/web" {
		t.Errorf("syncing go/api lacking web gives %+v, %v", s, err)
	}
	c.MergeHead, c.State.Changed = "", nil
	gain(c, "go/cli", "a.txt", "a")
	gain(c, "go/web", "a.txt", "a")
	c.Parents[c.Local["go/web"]] = append(c.Parents[c.Local["go/web"]], c.Local["go/cli"])
	if s, err := Run(c); err != nil || s.Merged != "go/cli" {
		t.Errorf("syncing go/api lacking cli and web gives %+v, %v", s, err)
	}
}

func TestRunLeavesAConflictAsTheMergeLeftIt(t *testing.T) {
	c := acme()
	gain(c, "main", "README.md", "from main")
	gain(c, "main", "shared.txt", "shared")
	gain(c, "stack/go", "README.md", "from stack/go")
	c.Conflicting = []string{"README.md"}
	s, err := Run(c)
	var conflicted *Conflicted
	if !errors.As(err, &conflicted) || conflicted.Branch != "main" || conflicted.Into != "stack/go" || !slices.Equal(conflicted.Paths, []string{"README.md"}) {
		t.Fatalf("a conflicted merge gives %v", err)
	}
	if s.Merged != "main" || !slices.Equal(s.Staged, []string{"shared.txt"}) || !slices.Equal(s.Conflicts, []string{"README.md"}) || c.MergeHead != c.Local["main"] {
		t.Errorf("a conflicted merge gives %+v, a merge of %q", s, c.MergeHead)
	}
}

// branch sync goes down the tree a branch at a time: a branch above the one
// checked out that lacks its own parents' commits is refused, every one
// named, and nothing is merged.
func TestRunRefusesABranchAboveThatLacksItsParentsCommits(t *testing.T) {
	c := acme()
	gain(c, "main", "shared.txt", "shared")
	c.HEAD = "go/web"
	s, err := Run(c)
	var behind *Behind
	if !errors.As(err, &behind) || behind.Branch != "stack/go" || !slices.Equal(behind.Lacks, []string{"main"}) {
		t.Fatalf("a parent behind its own gives %v", err)
	}
	if s == nil || s.Branch != "go/web" || c.MergeHead != "" {
		t.Errorf("a parent behind its own gives %+v, a merge of %q", s, c.MergeHead)
	}
	gain(c, "stack/go", "go.txt", "go")
	if _, err := Run(c); !errors.As(err, &behind) || len(multiple(err)) != 2 {
		t.Errorf("two branches behind give %v", err)
	}
}

// multiple are the errors err joins.
func multiple(err error) []error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}
	return []error{err}
}

func TestRunRefusesAMergeInProgressThenAnUncleanCheckout(t *testing.T) {
	c := acme()
	gain(c, "main", "shared.txt", "shared")
	if _, err := Run(c); err != nil {
		t.Fatal(err)
	}
	var merging *Merging
	if _, err := Run(c); !errors.As(err, &merging) {
		t.Errorf("a second sync gives %v", err)
	}
	c = acme()
	gain(c, "main", "shared.txt", "shared")
	c.State.Changed = []string{"README.md", "notes.txt"}
	var dirty *Dirty
	if s, err := Run(c); !errors.As(err, &dirty) || !slices.Equal(dirty.Paths, c.State.Changed) || s.Merged != "" || c.MergeHead != "" {
		t.Errorf("an unclean checkout gives %+v, %v", s, err)
	}
	c = acme()
	c.State.Changed = []string{"README.md"}
	if s, err := Run(c); err != nil || s.Merged != "" {
		t.Errorf("an unclean checkout with nothing to merge gives %+v, %v", s, err)
	}
}

func TestRunRefusesAHeadNamingNoBranchTheManifestNames(t *testing.T) {
	c := acme()
	c.HEAD = ""
	var detached *Detached
	if s, err := Run(c); !errors.As(err, &detached) || !slices.Equal(detached.Branches, into) || s != nil {
		t.Errorf("a detached HEAD gives %+v, %v", s, err)
	}
	c.HEAD = "topic"
	var unnamed *Unnamed
	if s, err := Run(c); !errors.As(err, &unnamed) || unnamed.Branch != "topic" || !slices.Equal(unnamed.Branches, into) || s.Branch != "topic" {
		t.Errorf("a branch the manifest does not name gives %+v, %v", s, err)
	}
}

func TestRunRefusesABranchItReadsThatIsNotLocal(t *testing.T) {
	c := acme()
	delete(c.Local, "stack/go")
	delete(c.Local, "main")
	var notLocal *NotLocal
	if _, err := Run(c); !errors.As(err, &notLocal) || notLocal.Branch != "main" {
		t.Errorf("the root not local gives %v", err)
	}
	c = acme()
	delete(c.Local, "stack/go")
	delete(c.Local, "go/cli")
	delete(c.Local, "stack/python")
	c.HEAD = "go/web"
	s, err := Run(c)
	var named []string
	for _, e := range multiple(err) {
		if errors.As(e, &notLocal) {
			named = append(named, notLocal.Branch)
		}
	}
	if !slices.Equal(named, []string{"stack/go", "go/cli"}) || s.Branch != "go/web" {
		t.Errorf("branches above not local give %+v, %v", s, err)
	}
}

func TestRunRefusesARepositoryItCannotSync(t *testing.T) {
	for name, tc := range map[string]struct {
		change func(*porttest.Clone)
		want   any
	}{
		"no repository":     {func(c *porttest.Clone) { c.State = port.CheckoutState{Why: "fatal: not a git repository"} }, new(*NotRepository)},
		"no working tree":   {func(c *porttest.Clone) { c.State = port.CheckoutState{Repository: true} }, new(*NoWorkTree)},
		"no commit":         {func(c *porttest.Clone) { c.State.Committed = false }, new(*NoCommit)},
		"no origin":         {func(c *porttest.Clone) { c.Remote = false }, new(*NoOrigin)},
		"no default branch": {func(c *porttest.Clone) { c.Default = "" }, new(*NoDefaultBranch)},
		"no manifest": {func(c *porttest.Clone) {
			delete(c.Trees[c.Local["main"]], manifest.File)
		}, new(*template.NoManifest)},
		"a manifest refused": {func(c *porttest.Clone) {
			c.Trees[c.Local["main"]][manifest.File] = &fstest.MapFile{Data: []byte("version: 1\nstacks: []\n")}
		}, new(*template.ManifestInvalid)},
	} {
		c := acme()
		tc.change(c)
		s, err := Run(c)
		if !errors.As(err, tc.want) || s != nil {
			t.Errorf("%s gives %+v, %v", name, s, err)
		}
	}
	c := acme()
	c.State = port.CheckoutState{Why: "fatal: not a git repository"}
	var none *NotRepository
	if _, err := Run(c); !errors.As(err, &none) || none.Why != "fatal: not a git repository" {
		t.Errorf("no repository gives %v", err)
	}
	c = acme()
	delete(c.Trees[c.Local["main"]], manifest.File)
	var noManifest *template.NoManifest
	if _, err := Run(c); !errors.As(err, &noManifest) || noManifest.Template != "/home/you/acme" || noManifest.Root != "main" {
		t.Errorf("no manifest gives %v", err)
	}
}

// A port's failure that means nothing to the domain passes through it.
func TestRunPassesThePortsFailuresThrough(t *testing.T) {
	broken := errors.New("git broke")
	for _, method := range []string{"Look", "HasOrigin", "Head", "File", "Branch", "IsAncestor", "Merging", "Merge"} {
		c := acme()
		gain(c, "main", "shared.txt", "shared")
		c.Fails = map[string]error{method: broken}
		if _, err := Run(c); !errors.Is(err, broken) {
			t.Errorf("%s failing gives %v", method, err)
		}
	}
	// Head fails for a branch above the one checked out, and IsAncestor
	// while the branches above are judged.
	for method, head := range map[string]string{"Head stack/go": "go/cli", "IsAncestor": "go/cli"} {
		c := acme()
		c.HEAD = head
		c.Fails = map[string]error{method: broken}
		if _, err := Run(c); !errors.Is(err, broken) {
			t.Errorf("%s failing on %s gives %v", method, head, err)
		}
	}
	// The root asks nothing of the history.
	c := acme()
	c.HEAD = "main"
	c.Fails = map[string]error{"IsAncestor": broken}
	if _, err := Run(c); err != nil {
		t.Errorf("the root asks nothing of its history: %v", err)
	}
}

// Each refusal says what it is, for a log or a wrapping error; the UI words
// each for people itself.
func TestErrorsSayWhatTheyAre(t *testing.T) {
	for err, want := range map[Error]string{
		&NotRepository{Why: "fatal"}:  "in no git repository: fatal",
		&NoWorkTree{}:                 "a git repository with no working tree",
		&NoCommit{Top: "/acme"}:       "no commit in /acme",
		&NoOrigin{}:                   "no remote origin",
		&NoDefaultBranch{}:            "origin names no default branch",
		&NotLocal{Branch: "stack/go"}: "no local branch stack/go",
		&Detached{}:                   "HEAD names no branch",
		&Unnamed{Branch: "topic"}:     "the manifest names no branch topic",
		&Behind{Branch: "go/web", Lacks: []string{"a", "b"}}:  "go/web lacks a, b",
		&Lagging{Branch: "go/web", Lacks: []string{"a", "b"}}: "go/web lacks a, b",
		&Missing{Branch: "go/web"}:                            "no local branch go/web",
		&Merging{}:                                            "a merge is in progress",
		&Dirty{Paths: []string{"a", "b"}}:                     "changes from HEAD in a, b",
		&Conflicted{Paths: []string{"a", "b"}}:                "conflicts in a, b",
	} {
		if got := err.Error(); got != want {
			t.Errorf("%T says %q, not %q", err, got, want)
		}
	}
}
