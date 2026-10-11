package adoption

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

// clean is a project ready for a record: a repository with a working tree
// and a commit, nothing changed.
var clean = port.CheckoutState{Repository: true, WorkTree: true, Top: "/p", Committed: true}

func file(p, data string) port.File { return port.File{Path: p, Mode: 0o644, Data: []byte(data)} }

// checkout is a project holding files, clean.
func checkout(files ...port.File) *porttest.Checkout {
	tree := map[string]port.File{}
	for _, f := range files {
		tree[f.Path] = f
	}
	return &porttest.Checkout{State: clean, Tree: tree, Ignored: map[string]bool{}}
}

var record = project.Record{
	Template: "/t", Release: "v1.0.0", Stack: "go", Features: []string{"cli"},
	Answers: answer.Set{"name": "blue-fox"}, Commits: map[string]string{"main": "1", "stack/go": "2"},
}

func TestOpenRefusesAFolderInNoRepositoryNamingWhyAndOneWithNoWorkTree(t *testing.T) {
	co := checkout()
	co.State = port.CheckoutState{Why: "not a git repository"}
	_, err := Open(co, How{DryRun: true}, nil)
	var none *NotRepository
	if !errors.As(err, &none) || none.Why != "not a git repository" {
		t.Errorf("Open in no repository gave %v", err)
	}
	co.State = port.CheckoutState{Repository: true}
	if _, err := Open(co, How{DryRun: true}, nil); !errors.As(err, new(*NoWorkTree)) {
		t.Errorf("Open in a bare repository gave %v", err)
	}
}

func TestOpenRefusesToWriteInAProjectNotReadyNamingEveryReason(t *testing.T) {
	co := checkout(file(project.RecordFile, "kept"))
	co.State = port.CheckoutState{Repository: true, WorkTree: true, Top: "/p", Changed: []string{"README.md"}}
	_, err := Open(co, How{}, nil)
	var commit *NoCommit
	var dirty *Dirty
	var held *HasRecord
	if !errors.As(err, &commit) || !errors.As(err, &dirty) || !errors.As(err, &held) {
		t.Fatalf("Open gave %v, not every reason", err)
	}
	if commit.Top != "/p" || !slices.Equal(dirty.Paths, []string{"README.md"}) || held.File != project.RecordFile {
		t.Errorf("the reasons name %q, %q and %q", commit.Top, dirty.Paths, held.File)
	}
	if _, err := Open(co, How{DryRun: true}, nil); err != nil {
		t.Errorf("a dry run refused a project it only previews: %v", err)
	}
	if state, err := Open(checkout(), How{}, nil); err != nil || state.Top != "/p" {
		t.Errorf("Open of a clean project gave %v, %v", state, err)
	}
}

func TestOpenPassesOnWhyItCannotLookForARecord(t *testing.T) {
	co := checkout()
	co.Fail = errors.New("unreadable")
	if _, err := Open(co, How{}, nil); !errors.Is(err, co.Fail) {
		t.Errorf("Open gave %v", err)
	}
}

func TestCompareNamesEveryDifferenceSortedByPath(t *testing.T) {
	baseline := []port.File{file("b.txt", "b"), file("a.txt", "a"), file("same.txt", "s"), file("gone.txt", "g"), file("run.sh", "x")}
	ours := []port.File{file("same.txt", "s"), file("a.txt", "A"), file("b.txt", "b"), file("own.txt", "o"),
		{Path: "run.sh", Mode: 0o755, Data: []byte("x")}}
	ours[2].Mode = 0o755
	ours[2].Data = []byte("B")
	got := Compare(baseline, ours)
	want := []Difference{
		{Path: "a.txt", Kind: Modified, From: 0o644, To: 0o644},
		{Path: "b.txt", Kind: Modified, ModeChanged: true, From: 0o644, To: 0o755},
		{Path: "gone.txt", Kind: Missing, From: 0o644},
		{Path: "own.txt", Kind: ProjectOnly, To: 0o644},
		{Path: "run.sh", Kind: Modified, ModeChanged: true, From: 0o644, To: 0o755},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Compare gave\n%v, not\n%v", got, want)
	}
	if d := Compare(baseline, baseline); d == nil || len(d) != 0 {
		t.Errorf("a project holding the baseline differs from it in %v", d)
	}
}

// Binary is git's rule, a NUL in the first 8000 bytes, of either side's
// contents, and never a link's target.
func TestCompareTellsABinaryFileAsGitDoes(t *testing.T) {
	nul := func(at int) string { return strings.Repeat("x", at) + "\x00" }
	baseline := []port.File{file("early", nul(7999)), file("late", nul(8000)), file("ours", "text"), file("gone", nul(0)),
		{Path: "link", Mode: porttest.Link, Data: []byte("a\x00")}}
	ours := []port.File{file("early", "text"), file("late", "text"), file("ours", nul(10)), file("own", nul(1)),
		{Path: "link", Mode: porttest.Link, Data: []byte("b\x00")}}
	binary := map[string]bool{}
	for _, d := range Compare(baseline, ours) {
		binary[d.Path] = d.Binary
	}
	want := map[string]bool{"early": true, "late": false, "ours": true, "gone": true, "own": true, "link": false}
	for p, b := range want {
		if binary[p] != b {
			t.Errorf("%s is binary %v, not %v", p, binary[p], b)
		}
	}
}

func TestAdoptWritesTheRecordNewWritesBesideAMatchingProject(t *testing.T) {
	co := checkout(file("README.md", "r"))
	a, err := Adopt(co, clean, []port.File{file("README.md", "r")}, record, How{}, nil, porttest.Differ{})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := record.File()
	if got := co.Tree[project.RecordFile]; !a.Written || string(got.Data) != string(want.Data) {
		t.Errorf("Adopt wrote %q, written %v, not\n%s", got.Data, a.Written, want.Data)
	}
	if a.Baseline == nil || a.Baseline.Release != "v1.0.0" || len(a.Differences) != 0 || a.Dirty {
		t.Errorf("the adoption is %+v", a)
	}
}

func TestAdoptRefusesDifferencesUnlessForcedAndNeverAppliesThem(t *testing.T) {
	baseline := []port.File{file("README.md", "r"), file("cli.txt", "c")}
	co := checkout(file("README.md", "changed"), file("own.txt", "o"))
	a, err := Adopt(co, clean, baseline, record, How{}, nil, porttest.Differ{})
	var differs *Differs
	if !errors.As(err, &differs) || differs.Count != 3 || a == nil || len(a.Differences) != 3 || a.Written {
		t.Fatalf("Adopt without force gave %+v, %v", a, err)
	}
	if _, ok := co.Tree[project.RecordFile]; ok {
		t.Error("a refusal wrote the record")
	}
	a, err = Adopt(co, clean, baseline, record, How{Force: true}, nil, porttest.Differ{})
	if err != nil || !a.Written {
		t.Fatalf("Adopt with force gave %+v, %v", a, err)
	}
	if _, ok := co.Tree["cli.txt"]; ok || string(co.Tree["README.md"].Data) != "changed" || len(co.Tree) != 3 {
		t.Errorf("force changed the project's files: %v", co.Tree)
	}
}

func TestADryRunPreviewsADirtyProjectAndWritesNothing(t *testing.T) {
	co := checkout(file("README.md", "changed"))
	dirty := clean
	dirty.Changed = []string{"README.md"}
	a, err := Adopt(co, dirty, []port.File{file("README.md", "r")}, record, How{DryRun: true, Force: true}, nil, porttest.Differ{})
	if err != nil || a.Written || !a.Dirty || len(a.Differences) != 1 {
		t.Fatalf("a dry run gave %+v, %v", a, err)
	}
	if _, ok := co.Tree[project.RecordFile]; ok {
		t.Error("a dry run wrote the record")
	}
}

// The baseline's files are compared where the project ignores them too;
// a project-only file it ignores is no difference.
func TestAdoptComparesTheBaselinesFilesTheProjectIgnores(t *testing.T) {
	co := checkout(file("build.out", "ours"), file("local.txt", "secret"), file("README.md", "r"))
	co.Ignored = map[string]bool{"build.out": true, "local.txt": true}
	a, err := Adopt(co, clean, []port.File{file("build.out", "theirs"), file("README.md", "r")}, record, How{DryRun: true}, nil, porttest.Differ{})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Differences) != 1 || a.Differences[0].Path != "build.out" || a.Differences[0].Kind != Modified {
		t.Errorf("the differences are %+v", a.Differences)
	}
}

func TestAdoptDiffsEachTextChangeAndNoOther(t *testing.T) {
	baseline := []port.File{file("README.md", "r"), file("gone.txt", "g"), file("logo.bin", "\x00a"), file("mode.sh", "m"),
		{Path: "sub", Mode: porttest.Submodule}}
	co := checkout(file("README.md", "R"), file("own.txt", "o"), file("logo.bin", "\x00b"),
		port.File{Path: "mode.sh", Mode: 0o755, Data: []byte("m")}, file("sub", "now a file"))
	a, err := Adopt(co, clean, baseline, record, How{DryRun: true, Diff: true}, nil, porttest.Differ{})
	if err != nil {
		t.Fatal(err)
	}
	diffs := map[string]string{}
	for _, d := range a.Differences {
		diffs[d.Path] = d.Diff
	}
	want := map[string]string{
		"README.md": "a/README.md: rb/README.md: R",
		"gone.txt":  "a/gone.txt: gb: none\n",
		"own.txt":   "a: none\nb/own.txt: o",
		"logo.bin":  "", "mode.sh": "", "sub": "",
	}
	for p, d := range want {
		if diffs[p] != d {
			t.Errorf("the diff of %s is %q, not %q", p, diffs[p], d)
		}
	}
	co.Tree["sub"] = port.File{Path: "sub", Mode: porttest.Submodule}
	baseline[4] = file("sub", "a file")
	if a, err := Adopt(co, clean, baseline, record, How{DryRun: true, Diff: true}, nil, porttest.Differ{}); err != nil || a.Differences[5].Path != "sub" || a.Differences[5].Diff != "" {
		t.Errorf("a submodule of the project's was diffed: %+v, %v", a, err)
	}
	if a, _ := Adopt(co, clean, baseline, record, How{DryRun: true}, nil, porttest.Differ{}); a.Differences[0].Diff != "" {
		t.Errorf("a diff not asked for was made: %q", a.Differences[0].Diff)
	}
}

func TestAdoptPassesOnWhatItCannotRead(t *testing.T) {
	fail := errors.New("broken")
	co := checkout(file("README.md", "R"))
	if _, err := Adopt(co, clean, []port.File{file("README.md", "r")}, record, How{Diff: true}, nil, porttest.Differ{Fail: fail}); !errors.Is(err, fail) {
		t.Errorf("a diff that fails gave %v", err)
	}
	co.Fail = fail
	if a, err := Adopt(co, clean, nil, record, How{}, nil, porttest.Differ{}); !errors.Is(err, fail) || a != nil {
		t.Errorf("files that cannot be read gave %+v, %v", a, err)
	}
}

// A record that appeared since Open looked is never replaced; a write
// that fails is passed on, nothing written.
func TestAdoptNeverReplacesARecordAndPassesOnAFailedWrite(t *testing.T) {
	co := checkout(file(project.RecordFile, "kept"))
	a, err := Adopt(co, clean, nil, record, How{Force: true}, nil, porttest.Differ{})
	var held *HasRecord
	if !errors.As(err, &held) || a == nil || a.Written || string(co.Tree[project.RecordFile].Data) != "kept" {
		t.Errorf("Adopt over a record gave %+v, %v", a, err)
	}
	co = checkout()
	co.Unwritable = fs.ErrPermission
	if a, err := Adopt(co, clean, nil, record, How{}, nil, porttest.Differ{}); !errors.Is(err, fs.ErrPermission) || a != nil {
		t.Errorf("a failed write gave %+v, %v", a, err)
	}
}

func TestEachErrorSaysWhatItIs(t *testing.T) {
	for _, e := range []Error{&NotRepository{Why: "w"}, &NoWorkTree{}, &NoCommit{Top: "/p"}, &Dirty{}, &HasRecord{File: "f"}, &Differs{Count: 2}, &Conflicted{Paths: []string{"a"}}} {
		if e.Error() == "" {
			t.Errorf("%T says nothing", e)
		}
	}
}

// merging is a project holding files at HEAD and in its checkout alike,
// clean, and the merger over that HEAD.
func merging(files ...port.File) (*porttest.Checkout, *porttest.Project) {
	co := checkout(files...)
	return co, &porttest.Project{State: clean, Head: co.Tree}
}

// A merge takes the baseline in over an empty base: a file the project
// lacks is added, a file both hold alike and a project-only one are left
// as they are, and the record is merged in with them, never created apart.
func TestAMergeAddsWhatTheProjectLacksAndKeepsTheRest(t *testing.T) {
	co, p := merging(file("README.md", "r"), file("own.txt", "o"))
	baseline := make([]port.File, 0, 3)
	baseline = append(baseline, file("README.md", "r"), file("cli.txt", "c"))
	a, err := Adopt(co, clean, baseline, record, How{Merge: true}, p, porttest.Differ{})
	if err != nil {
		t.Fatal(err)
	}
	if !a.Written || a.Conflicts == nil || len(a.Conflicts) != 0 || len(a.Differences) != 2 {
		t.Errorf("the merge's adoption is %+v", a)
	}
	want, _ := record.File()
	if p.Merges != 1 || string(p.Tree[project.RecordFile].Data) != string(want.Data) || string(p.Tree["cli.txt"].Data) != "c" ||
		string(p.Tree["own.txt"].Data) != "o" || string(p.Tree["README.md"].Data) != "r" || len(p.Tree) != 4 {
		t.Errorf("the merge left %v", p.Tree)
	}
	if p.Labels != (port.Labels{Base: Unrelated, Target: "template v1.0.0"}) {
		t.Errorf("the merge is labelled %+v", p.Labels)
	}
	if _, ok := co.Tree[project.RecordFile]; ok {
		t.Error("a merge created the record beside the merge")
	}
	if extra := baseline[:3][2]; extra.Path != "" {
		t.Errorf("the merge wrote %s into the caller's baseline", extra.Path)
	}
}

// A file both hold otherwise is a conflict both added, which refuses the
// adoption, naming it, the record merged all the same.
func TestAMergeThatLeavesConflictsNamesThem(t *testing.T) {
	co, p := merging(file("release.txt", "project-release"), file("a.txt", "ours"))
	baseline := []port.File{file("release.txt", "v1.0.0"), file("a.txt", "theirs")}
	a, err := Adopt(co, clean, baseline, record, How{Merge: true}, p, porttest.Differ{})
	var conflicted *Conflicted
	if !errors.As(err, &conflicted) || conflicted.Release != "v1.0.0" || !slices.Equal(conflicted.Paths, []string{"a.txt", "release.txt"}) {
		t.Fatalf("the merge gave %v", err)
	}
	if a == nil || !a.Written || !slices.Equal(a.Conflicts, []port.Unmerged{{Path: "a.txt"}, {Path: "release.txt"}}) {
		t.Errorf("the conflicted adoption is %+v", a)
	}
	if stages := p.Stages["release.txt"]; stages[0] != nil || string(stages[1].Data) != "project-release" || string(stages[2].Data) != "v1.0.0" {
		t.Errorf("the conflict's stages are %v", stages)
	}
	if _, ok := p.Tree[project.RecordFile]; !ok {
		t.Error("a conflicted merge left the record out")
	}
	if err.Error() != "conflicts in a.txt, release.txt" {
		t.Errorf("the conflict says %q", err.Error())
	}
}

func TestAMergeRunsOnlyWhenItWritesAndPassesOnAFailure(t *testing.T) {
	co, p := merging(file("README.md", "changed"))
	if a, err := Adopt(co, clean, []port.File{file("README.md", "r")}, record, How{Merge: true, DryRun: true}, p, porttest.Differ{}); err != nil || a.Written || p.Merges != 0 {
		t.Errorf("a dry run merged: %+v, %v, %d merges", a, err, p.Merges)
	}
	p.Fail = errors.New("broken")
	if a, err := Adopt(co, clean, []port.File{file("README.md", "r")}, record, How{Merge: true}, p, porttest.Differ{}); !errors.Is(err, p.Fail) || a != nil {
		t.Errorf("a merge that fails gave %+v, %v", a, err)
	}
}

// A merge needs a merger that can merge, a git new enough, which Open asks
// before anything is rendered, beside every other reason, and only when it
// would merge.
func TestOpenRefusesAMergeTheMergerCannotMake(t *testing.T) {
	co := checkout()
	old := &porttest.Project{Unable: errors.New("git too old")}
	if _, err := Open(co, How{Merge: true}, old); !errors.Is(err, old.Unable) {
		t.Errorf("a merge git cannot make gave %v", err)
	}
	co.State.Changed = []string{"README.md"}
	if _, err := Open(co, How{Merge: true}, old); !errors.Is(err, old.Unable) || !errors.As(err, new(*Dirty)) {
		t.Errorf("a dirty project's merge gave %v, not both reasons", err)
	}
	co.State.Changed = nil
	for _, how := range []How{{}, {Force: true}, {Merge: true, DryRun: true}} {
		if _, err := Open(co, how, old); err != nil {
			t.Errorf("Open %+v asked the merger: %v", how, err)
		}
	}
	if _, err := Open(co, How{Merge: true}, &porttest.Project{}); err != nil {
		t.Errorf("a merge git can make gave %v", err)
	}
}
