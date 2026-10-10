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
	_, err := Open(co, true)
	var none *NotRepository
	if !errors.As(err, &none) || none.Why != "not a git repository" {
		t.Errorf("Open in no repository gave %v", err)
	}
	co.State = port.CheckoutState{Repository: true}
	if _, err := Open(co, true); !errors.As(err, new(*NoWorkTree)) {
		t.Errorf("Open in a bare repository gave %v", err)
	}
}

func TestOpenRefusesToWriteInAProjectNotReadyNamingEveryReason(t *testing.T) {
	co := checkout(file(project.RecordFile, "kept"))
	co.State = port.CheckoutState{Repository: true, WorkTree: true, Top: "/p", Changed: []string{"README.md"}}
	_, err := Open(co, false)
	var commit *NoCommit
	var dirty *Dirty
	var held *HasRecord
	if !errors.As(err, &commit) || !errors.As(err, &dirty) || !errors.As(err, &held) {
		t.Fatalf("Open gave %v, not every reason", err)
	}
	if commit.Top != "/p" || !slices.Equal(dirty.Paths, []string{"README.md"}) || held.File != project.RecordFile {
		t.Errorf("the reasons name %q, %q and %q", commit.Top, dirty.Paths, held.File)
	}
	if _, err := Open(co, true); err != nil {
		t.Errorf("a dry run refused a project it only previews: %v", err)
	}
	if state, err := Open(checkout(), false); err != nil || state.Top != "/p" {
		t.Errorf("Open of a clean project gave %v, %v", state, err)
	}
}

func TestOpenPassesOnWhyItCannotLookForARecord(t *testing.T) {
	co := checkout()
	co.Fail = errors.New("unreadable")
	if _, err := Open(co, false); !errors.Is(err, co.Fail) {
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
	a, err := Adopt(co, clean, []port.File{file("README.md", "r")}, record, How{}, porttest.Differ{})
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
	a, err := Adopt(co, clean, baseline, record, How{}, porttest.Differ{})
	var differs *Differs
	if !errors.As(err, &differs) || differs.Count != 3 || a == nil || len(a.Differences) != 3 || a.Written {
		t.Fatalf("Adopt without force gave %+v, %v", a, err)
	}
	if _, ok := co.Tree[project.RecordFile]; ok {
		t.Error("a refusal wrote the record")
	}
	a, err = Adopt(co, clean, baseline, record, How{Force: true}, porttest.Differ{})
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
	a, err := Adopt(co, dirty, []port.File{file("README.md", "r")}, record, How{DryRun: true, Force: true}, porttest.Differ{})
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
	a, err := Adopt(co, clean, []port.File{file("build.out", "theirs"), file("README.md", "r")}, record, How{DryRun: true}, porttest.Differ{})
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
	a, err := Adopt(co, clean, baseline, record, How{DryRun: true, Diff: true}, porttest.Differ{})
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
	if a, err := Adopt(co, clean, baseline, record, How{DryRun: true, Diff: true}, porttest.Differ{}); err != nil || a.Differences[5].Path != "sub" || a.Differences[5].Diff != "" {
		t.Errorf("a submodule of the project's was diffed: %+v, %v", a, err)
	}
	if a, _ := Adopt(co, clean, baseline, record, How{DryRun: true}, porttest.Differ{}); a.Differences[0].Diff != "" {
		t.Errorf("a diff not asked for was made: %q", a.Differences[0].Diff)
	}
}

func TestAdoptPassesOnWhatItCannotRead(t *testing.T) {
	fail := errors.New("broken")
	co := checkout(file("README.md", "R"))
	if _, err := Adopt(co, clean, []port.File{file("README.md", "r")}, record, How{Diff: true}, porttest.Differ{Fail: fail}); !errors.Is(err, fail) {
		t.Errorf("a diff that fails gave %v", err)
	}
	co.Fail = fail
	if a, err := Adopt(co, clean, nil, record, How{}, porttest.Differ{}); !errors.Is(err, fail) || a != nil {
		t.Errorf("files that cannot be read gave %+v, %v", a, err)
	}
}

// A record that appeared since Open looked is never replaced; a write
// that fails is passed on, nothing written.
func TestAdoptNeverReplacesARecordAndPassesOnAFailedWrite(t *testing.T) {
	co := checkout(file(project.RecordFile, "kept"))
	a, err := Adopt(co, clean, nil, record, How{Force: true}, porttest.Differ{})
	var held *HasRecord
	if !errors.As(err, &held) || a == nil || a.Written || string(co.Tree[project.RecordFile].Data) != "kept" {
		t.Errorf("Adopt over a record gave %+v, %v", a, err)
	}
	co = checkout()
	co.Unwritable = fs.ErrPermission
	if a, err := Adopt(co, clean, nil, record, How{}, porttest.Differ{}); !errors.Is(err, fs.ErrPermission) || a != nil {
		t.Errorf("a failed write gave %+v, %v", a, err)
	}
}

func TestEachErrorSaysWhatItIs(t *testing.T) {
	for _, e := range []Error{&NotRepository{Why: "w"}, &NoWorkTree{}, &NoCommit{Top: "/p"}, &Dirty{}, &HasRecord{File: "f"}, &Differs{Count: 2}} {
		if e.Error() == "" {
			t.Errorf("%T says nothing", e)
		}
	}
}
