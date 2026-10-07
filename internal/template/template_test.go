package template

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/render"
	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

const manifestText = `version: 2
checks: [[has, README.md]]
stacks:
  - name: sh
    checks: [[has, bin/acme-widget]]
  - name: py
features:
  - name: extra
    stack: sh
    checks: [[has, extra.txt]]
  - name: more
    stack: sh
    needs: [extra]
  - name: tool
    stack: py
questions:
  - name: name
    literal: acme-widget
    question: Name?
    pattern: "[a-z]+(-[a-z]+)*"
    case_forms: true
  - name: owner
    literal: Acme Corp
    question: Owner?
    default: Nobody
template_only: [ci.yml]
`

// over is base with files laid over it, as a branch is its parent with its
// own changes.
func over(base fstest.MapFS, files fstest.MapFS) fstest.MapFS {
	tree := maps.Clone(base)
	maps.Copy(tree, files)
	return tree
}

// acme is the fixture template: main with the manifest, a CRLF NOTICE, a
// README and its own CI; stack/sh adding an executable script; sh/extra a
// file, and sh/more, needing extra, another; stack/py and py/tool.
func acme() *porttest.Repository {
	main := fstest.MapFS{
		manifest.File: {Data: []byte(manifestText)},
		"NOTICE":      {Data: []byte("acme-widget by Acme Corp\r\nACME_WIDGET\r\n")},
		"README.md":   {Data: []byte("# acme-widget\n")},
		"ci.yml":      {Data: []byte("render every combination\n")},
	}
	sh := over(main, fstest.MapFS{"bin/acme-widget": {Data: []byte("#!/bin/sh\necho acme_widget\n"), Mode: porttest.Executable}})
	extra := over(sh, fstest.MapFS{"extra.txt": {Data: []byte("extra\n")}})
	py := over(main, fstest.MapFS{"pyproject.toml": {Data: []byte("[project]\nname = \"acme-widget\"\n")}})
	return &porttest.Repository{Root: "main", Branches: map[string]fstest.MapFS{
		"main":     main,
		"stack/sh": sh,
		"sh/extra": extra,
		"sh/more":  over(extra, fstest.MapFS{"more.txt": {Data: []byte("more\n")}}),
		"stack/py": py,
		"py/tool":  over(py, fstest.MapFS{"tool.txt": {Data: []byte("tool\n")}}),
	}}
}

// withManifest is repo with main's manifest text, merged down into every
// branch, as the root's files are.
func withManifest(repo *porttest.Repository, text string) *porttest.Repository {
	for _, tree := range repo.Branches {
		tree[manifest.File] = &fstest.MapFile{Data: []byte(text)}
	}
	return repo
}

func open(t *testing.T, repo port.Repository) *Template {
	t.Helper()
	tpl, err := Open("../acme", repo)
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

func writer() (project.Writer, *porttest.Disk, *porttest.Git) {
	d := porttest.NewDisk()
	g := porttest.NewGit(d)
	return project.Writer{Disk: d, Git: g}, d, g
}

// combination is the combination of the stack and the features named.
func combination(t *testing.T, tpl *Template, stack string, features ...string) manifest.Combination {
	t.Helper()
	c, _, err := Choose(tpl.Manifest, manifest.Choice{Stack: stack, Features: features}, answer.Given{"name=blue-fox"}, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var answers = answer.Set{"name": "blue-fox", "owner": "Blue Corp"}

func TestOpenReadsTheManifestOnTheRootBranch(t *testing.T) {
	tpl := open(t, acme())
	if tpl.Name != "../acme" || !slices.Equal(tpl.Manifest.StackNames(), []string{"sh", "py"}) {
		t.Errorf("the template is %+v", tpl)
	}
}

func TestOpenRefusesATemplateWithNoManifestToRead(t *testing.T) {
	noRoot := acme()
	noRoot.Root = ""
	var root *NoRoot
	if _, err := Open("../acme", noRoot); !errors.As(err, &root) || root.Template != "../acme" {
		t.Errorf("a HEAD naming no branch: %v", err)
	}

	empty := acme()
	delete(empty.Branches, "main")
	var emptyRoot *EmptyRoot
	if _, err := Open("../acme", empty); !errors.As(err, &emptyRoot) || emptyRoot.Root != "main" {
		t.Errorf("a root branch with no commit: %v", err)
	}

	none := acme()
	delete(none.Branches["main"], manifest.File)
	var noManifest *NoManifest
	if _, err := Open("../acme", none); !errors.As(err, &noManifest) || noManifest.Root != "main" || noManifest.Template != "../acme" {
		t.Errorf("no manifest: %v", err)
	}
}

func TestOpenRefusesAManifestNamingEveryProblemAndTheRootBranch(t *testing.T) {
	_, err := Open("../acme", withManifest(acme(), "version: 5\nstacks: []\n"))
	var invalid *ManifestInvalid
	if !errors.As(err, &invalid) || invalid.Root != "main" || len(invalid.Problems) != 2 {
		t.Fatalf("Open = %v", err)
	}
}

func TestRenderMergesTheBranchesReplacesTheLiteralsAndRecordsEveryCommit(t *testing.T) {
	tpl := open(t, acme())
	w, d, g := writer()
	p, err := tpl.Render(combination(t, tpl, "sh", "extra"), answers, project.Folder{Path: "made", New: true}, nil, w)
	if err != nil {
		t.Fatal(err)
	}
	files := d.Folders["made"]
	if got := slices.Sorted(maps.Keys(files)); !slices.Equal(got, []string{project.RecordFile, "NOTICE", "README.md", "bin/blue-fox", "extra.txt"}) {
		t.Errorf("the project's files are %q: the manifest and the template's own files left out", got)
	}
	if got := string(files["NOTICE"].Data); got != "blue-fox by Blue Corp\r\nBLUE_FOX\r\n" {
		t.Errorf("NOTICE is %q: CRLF kept, the literals replaced, as written without case forms", got)
	}
	if g.Commits["made"].Files["bin/blue-fox"].Mode != 0o755 {
		t.Error("the script is not committed executable")
	}
	want := project.Record{
		Version:  project.RecordVersion,
		Template: "../acme",
		Stack:    "sh",
		Features: []string{"extra"},
		Answers:  answers,
		Commits:  map[string]string{"main": "commit of main", "stack/sh": "commit of stack/sh", "sh/extra": "commit of sh/extra"},
	}
	if !equalRecords(p.Record, want) || p.Commit != "commit of made" || p.Folder != "made" {
		t.Errorf("the project is %+v", p)
	}
}

func equalRecords(a, b project.Record) bool {
	return a.Version == b.Version && a.Template == b.Template && a.Stack == b.Stack &&
		slices.Equal(a.Features, b.Features) && maps.Equal(a.Answers, b.Answers) && maps.Equal(a.Commits, b.Commits)
}

// A render no one keeps is committed as the identity given, so a git that
// knows no one can still render it; its features are a list, empty.
func TestRenderCommitsAsTheIdentityGiven(t *testing.T) {
	tpl := open(t, acme())
	w, _, g := writer()
	g.Who = nil
	by := port.Identity{Name: "check", Email: "check@localhost"}
	p, err := tpl.Render(combination(t, tpl, "sh"), answers, project.Folder{Path: "tmp"}, &by, w)
	if err != nil {
		t.Fatal(err)
	}
	if g.Commits["tmp"].By != by {
		t.Errorf("the render's commit is by %+v", g.Commits["tmp"].By)
	}
	if p.Features == nil || len(p.Features) != 0 || len(p.Commits) != 2 {
		t.Errorf("the project is %+v", p)
	}
}

// firstCommit is the manifest's first_commit, of version 4: the whole
// message, literals in its header, body and footer, its body's lines ending
// in CRLF.
const firstCommit = "version: 4\nfirst_commit: \"chore: start acme-widget\\n\\nMade for Acme Corp, ACME_WIDGET.\\r\\n\\nTask: T-1\\n\"\n"

// A template whose rules judge its projects' first commit gives the
// message, its literals replaced as a text file's contents are, line
// endings kept; without one the commit keeps project's own.
func TestRenderCommitsTheManifestsFirstCommitItsLiteralsReplaced(t *testing.T) {
	tpl := open(t, withManifest(acme(), strings.Replace(manifestText, "version: 2\n", firstCommit, 1)))
	w, _, g := writer()
	if _, err := tpl.Render(combination(t, tpl, "sh"), answers, project.Folder{Path: "made", New: true}, nil, w); err != nil {
		t.Fatal(err)
	}
	want := "chore: start blue-fox\n\nMade for Blue Corp, BLUE_FOX.\r\n\nTask: T-1\n"
	if got := g.Commits["made"].Message; got != want {
		t.Errorf("the first commit's message is %q, not %q", got, want)
	}
	tpl = open(t, acme())
	if _, err := tpl.Render(combination(t, tpl, "sh"), answers, project.Folder{Path: "other", New: true}, nil, w); err != nil {
		t.Fatal(err)
	}
	if got := g.Commits["other"].Message; !strings.HasPrefix(got, "chore: make the project from its template\n") {
		t.Errorf("without first_commit the first commit's message is %q", got)
	}
}

func TestRenderRefusesABranchTheManifestImpliesAndTheTemplateLacks(t *testing.T) {
	repo := acme()
	tpl := open(t, repo)
	c := combination(t, tpl, "sh", "extra")
	delete(repo.Branches, "sh/extra")
	w, d, _ := writer()
	_, err := tpl.Render(c, answers, project.Folder{Path: "made", New: true}, nil, w)
	var none *NoBranch
	if !errors.As(err, &none) || none.Branch != "sh/extra" {
		t.Fatalf("Render = %v", err)
	}
	if len(d.Folders) != 0 {
		t.Errorf("something was written: %v", d.Folders)
	}
}

func TestRenderRefusesBranchesThatDoNotMergeCleanlyNamingThem(t *testing.T) {
	repo := acme()
	repo.Branches["sh/more"]["extra.txt"] = &fstest.MapFile{Data: []byte("changed\n")}
	tpl := open(t, repo)
	w, d, _ := writer()
	_, err := tpl.Render(combination(t, tpl, "sh", "extra", "more"), answers, project.Folder{Path: "made", New: true}, nil, w)
	var conflict *MergeConflict
	if !errors.As(err, &conflict) || conflict.Branch != "sh/more" || !slices.Equal(conflict.Into, []string{"stack/sh", "sh/extra"}) || !slices.Equal(conflict.Paths, []string{"extra.txt"}) {
		t.Fatalf("Render = %v", err)
	}
	if len(d.Folders) != 0 {
		t.Errorf("something was written: %v", d.Folders)
	}
}

func TestRenderRefusesATemplateHoldingTheRecordsFile(t *testing.T) {
	repo := acme()
	repo.Branches["stack/sh"][project.RecordFile] = &fstest.MapFile{Data: []byte("x\n")}
	tpl := open(t, repo)
	w, d, _ := writer()
	_, err := tpl.Render(combination(t, tpl, "sh"), answers, project.Folder{Path: "made", New: true}, nil, w)
	var holds *HoldsRecord
	if !errors.As(err, &holds) || holds.File != project.RecordFile {
		t.Fatalf("Render = %v", err)
	}
	if len(d.Folders) != 0 {
		t.Errorf("something was written: %v", d.Folders)
	}
}

func TestRenderRefusesAnAnswerThatMakesTwoFilesOneWithoutWriting(t *testing.T) {
	repo := acme()
	repo.Branches["stack/sh"]["acme-widget.txt"] = &fstest.MapFile{Data: []byte("one\n")}
	repo.Branches["stack/sh"]["blue-fox.txt"] = &fstest.MapFile{Data: []byte("two\n")}
	tpl := open(t, repo)
	w, d, _ := writer()
	_, err := tpl.Render(combination(t, tpl, "sh"), answers, project.Folder{Path: "made", New: true}, nil, w)
	var same *render.SameName
	if !errors.As(err, &same) || same.To != "blue-fox.txt" {
		t.Fatalf("Render = %v", err)
	}
	if len(d.Folders) != 0 {
		t.Errorf("something was written: %v", d.Folders)
	}
}

func TestRenderRefusesBeforeWritingWhenGitKnowsNoOne(t *testing.T) {
	tpl := open(t, acme())
	w, d, g := writer()
	g.Who = nil
	if _, err := tpl.Render(combination(t, tpl, "sh"), answers, project.Folder{Path: "made", New: true}, nil, w); !errors.Is(err, porttest.ErrNoIdentity) {
		t.Fatalf("Render = %v", err)
	}
	if len(d.Folders) != 0 {
		t.Errorf("something was written: %v", d.Folders)
	}
}
