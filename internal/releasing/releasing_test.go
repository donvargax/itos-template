package releasing

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
	"github.com/donvargax/itos-template/internal/template"
	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

// manifestText is a template of one stack and one feature, each checked:
// the feature's check names a file its render holds unless broken says
// otherwise.
const manifestText = `version: 2
checks:
  - [has, README.md]
stacks:
  - name: sh
    checks:
      - [has, bin/blue-fox]
features:
  - name: extra
    stack: sh
    checks:
      - [has, %s]
questions:
  - name: name
    literal: acme-widget
    question: Name?
    case_forms: true
`

// acme is the template's repository, its feature's check looking for
// checked: every branch at "commit of <branch>", as porttest names it.
func acme(checked string) *porttest.Repository {
	main := fstest.MapFS{
		manifest.File: {Data: []byte(strings.Replace(manifestText, "%s", checked, 1))},
		"README.md":   {Data: []byte("# acme-widget\n")},
	}
	sh := maps.Clone(main)
	sh["bin/acme-widget"] = &fstest.MapFile{Data: []byte("#!/bin/sh\n"), Mode: porttest.Executable}
	extra := maps.Clone(sh)
	extra["extra.txt"] = &fstest.MapFile{Data: []byte("extra\n")}
	return &porttest.Repository{Root: "main", Branches: map[string]fstest.MapFS{"main": main, "stack/sh": sh, "sh/extra": extra}, Tagged: map[string]porttest.Tag{}}
}

var listed = []string{"main", "stack/sh", "sh/extra"}

// published is a repository at /home/you/acme with an origin whose HEAD
// names main, every listed branch's local head at origin's, as the copy of
// it reads them.
func published() *porttest.Tagger {
	heads := map[string]string{}
	for _, b := range listed {
		heads[b] = "commit of " + b
	}
	return &porttest.Tagger{Folder: "/home/you/acme", Remote: true, Default: "main", Local: heads, Origin: maps.Clone(heads)}
}

func opened(t *testing.T, repo *porttest.Repository) *template.Template {
	t.Helper()
	tpl, err := template.Open("/home/you/acme", repo)
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

func here(t *testing.T, r port.Tagger, version string) *Here {
	t.Helper()
	h, err := Open(r, version)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// cut plans and cuts the release version of tpl in r with the fakes, its
// feature's check passing, and gives the release, each combination's name
// and whether it passed, and the error.
func cut(t *testing.T, tpl *template.Template, r *porttest.Tagger, version string) (*Release, []string, error) {
	t.Helper()
	p, err := here(t, r, version).Plan(tpl)
	if err != nil {
		t.Fatal(err)
	}
	d := porttest.NewDisk()
	w := project.Writer{Disk: d, Git: porttest.NewGit(d)}
	var results []string
	rel, err := p.Cut(tpl, answer.Set{"name": "blue-fox"}, &porttest.Folders{Disk: d}, w, porttest.Programs{"has": porttest.Has(d)}, func(r template.Result) error {
		results = append(results, r.Combination.Name()+map[bool]string{true: " passed", false: " failed"}[r.Passed()])
		return nil
	})
	return rel, results, err
}

func TestOpenJudgesTheVersionBeforeTheRepository(t *testing.T) {
	for _, v := range []string{"1.0.0", "v1.0", "v1", "latest", "v1.0.0+build", "v01.0.0", ""} {
		var bad *BadVersion
		if _, err := Open(&porttest.Tagger{}, v); !errors.As(err, &bad) || bad.Version != v {
			t.Errorf("Open(%q) = %v, not a *BadVersion", v, err)
		}
	}
	for _, v := range []string{"v1.0.0", "v0.0.1", "v1.3.0-rc.1", "v2.0.0-alpha"} {
		if h, err := Open(published(), v); err != nil || h.Top != "/home/you/acme" || h.Root != "main" {
			t.Errorf("Open(%q) = %+v, %v", v, h, err)
		}
	}
}

func TestOpenRefusesNoRepositoryAndNoOrigin(t *testing.T) {
	var none *NotRepository
	if _, err := Open(&porttest.Tagger{Remote: true}, "v1.0.0"); !errors.As(err, &none) || none.Why != "not a git repository" {
		t.Errorf("a folder in no repository gives %v", err)
	}
	var noOrigin *NoOrigin
	if _, err := Open(&porttest.Tagger{Folder: "/home/you/acme"}, "v1.0.0"); !errors.As(err, &noOrigin) {
		t.Errorf("a repository with no origin gives %v", err)
	}
	broken := errors.New("git broke")
	for _, method := range []string{"Top", "HasOrigin"} {
		r := published()
		r.Fails = map[string]error{method: broken}
		if h, err := Open(r, "v1.0.0"); h != nil || !errors.Is(err, broken) {
			t.Errorf("%s failing gives %+v, %v", method, h, err)
		}
	}
}

// The root is the branch origin's HEAD names, whatever is checked out
// (bug-7): an origin naming none is refused before anything is read.
func TestOpenTakesTheRootFromOrigin(t *testing.T) {
	r := published()
	r.Default = "trunk"
	if h, err := Open(r, "v1.0.0"); err != nil || h.Root != "trunk" {
		t.Errorf("an origin whose HEAD names trunk gives %+v, %v", h, err)
	}
	r.Default = ""
	var none *NoDefaultBranch
	if h, err := Open(r, "v1.0.0"); h != nil || !errors.As(err, &none) {
		t.Errorf("an origin naming no default branch gives %+v, %v", h, err)
	}
}

func TestCutTagsEveryListedBranchAtTheHeadCheckReadsAnnotated(t *testing.T) {
	r := published()
	r.Tags = map[string]porttest.Tagged{"main/v0.9.0": {Commit: "commit of main"}}
	rel, results, err := cut(t, opened(t, acme("extra.txt")), r, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"sh passed", "sh + extra passed"}; !slices.Equal(results, want) {
		t.Errorf("the check gave %q", results)
	}
	want := []Tag{
		{Name: "main/v1.0.0", Branch: "main", Commit: "commit of main"},
		{Name: "stack/sh/v1.0.0", Branch: "stack/sh", Commit: "commit of stack/sh"},
		{Name: "sh/extra/v1.0.0", Branch: "sh/extra", Commit: "commit of sh/extra"},
	}
	if rel.Version != "v1.0.0" || !slices.Equal(rel.Tags, want) {
		t.Errorf("the release is %+v", rel)
	}
	for _, tag := range want {
		if got := r.Tags[tag.Name]; got.Commit != tag.Commit || got.Message != "Template release v1.0.0" {
			t.Errorf("the repository holds %s as %+v", tag.Name, got)
		}
	}
	if len(r.Tags) != 4 {
		t.Errorf("the repository holds %d tags", len(r.Tags))
	}
	push := []string{"git", "push", "--atomic", "origin", "refs/tags/main/v1.0.0", "refs/tags/stack/sh/v1.0.0", "refs/tags/sh/extra/v1.0.0"}
	if !slices.Equal(rel.Push(), push) {
		t.Errorf("the push is %q", rel.Push())
	}
}

func TestCutTagsABranchHeldOnlyAsOriginsAtTheHeadCheckReads(t *testing.T) {
	r := published()
	delete(r.Local, "stack/sh")
	rel, _, err := cut(t, opened(t, acme("extra.txt")), r, "v1.3.0-rc.1")
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Tags["stack/sh/v1.3.0-rc.1"]; got.Commit != "commit of stack/sh" || len(rel.Tags) != 3 {
		t.Errorf("the repository holds stack/sh/v1.3.0-rc.1 as %+v, the release %+v", got, rel)
	}
}

func TestPlanRefusesAVersionAListedBranchCarriesAlready(t *testing.T) {
	repo := acme("extra.txt")
	repo.Tagged["sh/extra/v1.0.0"] = porttest.Tag{Tree: repo.Branches["sh/extra"], On: "sh/extra"}
	repo.Tagged["main/v1.0.0"] = porttest.Tag{Tree: repo.Branches["main"]}
	repo.Tagged["other/v2.0.0"] = porttest.Tag{Tree: repo.Branches["main"]}
	tpl := opened(t, repo)
	var used *Used
	if _, err := here(t, published(), "v1.0.0").Plan(tpl); !errors.As(err, &used) || !slices.Equal(used.Tags, []string{"main/v1.0.0", "sh/extra/v1.0.0"}) {
		t.Errorf("a version two branches carry gives %v", err)
	}
	if _, err := here(t, published(), "v1.0.0").Plan(opened(t, oneTagged())); !errors.As(err, &used) || !slices.Equal(used.Tags, []string{"stack/sh/v1.0.0"}) {
		t.Errorf("a version one branch carries gives %v", err)
	}
	if _, err := here(t, published(), "v2.0.0").Plan(tpl); err != nil {
		t.Errorf("a version only a branch the manifest does not list carries gives %v", err)
	}
	repo.NoTags = errors.New("cannot read the tags")
	if p, err := here(t, published(), "v1.0.0").Plan(tpl); p != nil || !errors.Is(err, repo.NoTags) {
		t.Errorf("tags that cannot be read give %+v, %v", p, err)
	}
}

// oneTagged is acme with one listed branch, stack/sh, carrying v1.0.0.
func oneTagged() *porttest.Repository {
	repo := acme("extra.txt")
	repo.Tagged["stack/sh/v1.0.0"] = porttest.Tag{Tree: repo.Branches["stack/sh"], On: "stack/sh"}
	return repo
}

// The history of every branch head of published(), and the commits a
// branch moved off origin's is at: past origin's (ahead), before it
// (behind), past its parent (diverged).
var history = map[string]string{
	"ahead":              "commit of stack/sh",
	"commit of stack/sh": "parent",
	"diverged":           "parent",
	"commit of sh/extra": "commit of stack/sh",
}

func TestPlanRefusesEveryListedBranchWhoseLocalHeadIsNotOrigins(t *testing.T) {
	tpl := opened(t, acme("extra.txt"))
	for _, c := range []struct {
		local, origin string
		how           How
	}{
		{"ahead", "commit of stack/sh", Ahead},
		{"parent", "commit of stack/sh", Behind},
		{"diverged", "commit of stack/sh", Diverged},
		{"commit of stack/sh", "", Unpublished},
	} {
		r := published()
		r.Parents = history
		r.Local["stack/sh"], r.Origin["stack/sh"] = c.local, c.origin
		r.Local["main"] = "ahead"
		_, err := here(t, r, "v1.0.0").Plan(tpl)
		var all []*Unsynced
		for _, e := range err.(interface{ Unwrap() []error }).Unwrap() {
			var u *Unsynced
			if errors.As(e, &u) {
				all = append(all, u)
			}
		}
		if len(all) != 2 || *all[0] != (Unsynced{Branch: "main", How: Diverged}) || *all[1] != (Unsynced{Branch: "stack/sh", How: c.how}) {
			t.Errorf("stack/sh at %s, origin's at %q, gives %v", c.local, c.origin, err)
		}
		if len(r.Tags) != 0 {
			t.Errorf("a refused plan made the tags %v", r.Tags)
		}
	}
}

func TestPlanPassesOnWhatTheRepositoryAndTheTemplateFailWith(t *testing.T) {
	broken := errors.New("git broke")
	for _, method := range []string{"Heads", "IsAncestor"} {
		r := published()
		r.Local["main"] = "ahead"
		r.Fails = map[string]error{method: broken}
		if p, err := here(t, r, "v1.0.0").Plan(opened(t, acme("extra.txt"))); p != nil || !errors.Is(err, broken) {
			t.Errorf("%s failing gives %+v, %v", method, p, err)
		}
	}
	repo := acme("extra.txt")
	delete(repo.Branches, "sh/extra")
	var noBranch *template.NoBranch
	if _, err := here(t, published(), "v1.0.0").Plan(opened(t, repo)); !errors.As(err, &noBranch) || noBranch.Branch != "sh/extra" {
		t.Errorf("a listed branch the template lacks gives %v", err)
	}
}

// IsAncestor failing on the second question, whether the branch is behind,
// after it answered it is not ahead.
func TestHowPassesOnAFailureTellingBehind(t *testing.T) {
	broken := errors.New("git broke")
	r := &askedOnce{Tagger: published(), fail: broken}
	h := here(t, r, "v1.0.0")
	if how, err := h.how("local", "origin"); !errors.Is(err, broken) || how != Behind {
		t.Errorf("a failure telling behind gives %q, %v", how, err)
	}
}

// askedOnce is a repository whose IsAncestor answers false once, then
// fails.
type askedOnce struct {
	*porttest.Tagger
	fail  error
	asked int
}

func (a *askedOnce) IsAncestor(string, string) (bool, error) {
	a.asked++
	if a.asked > 1 {
		return false, a.fail
	}
	return false, nil
}

func TestCutMakesNoTagWhenACombinationFails(t *testing.T) {
	r := published()
	rel, results, err := cut(t, opened(t, acme("missing.txt")), r, "v1.0.0")
	var failed *Failed
	if rel != nil || !errors.As(err, &failed) || *failed != (Failed{Failed: 1, All: 2}) {
		t.Errorf("a failing combination gives %+v, %v", rel, err)
	}
	if want := []string{"sh passed", "sh + extra failed"}; !slices.Equal(results, want) {
		t.Errorf("the check gave %q", results)
	}
	if len(r.Tags) != 0 {
		t.Errorf("a failed check made the tags %v", r.Tags)
	}
}

func TestCutStopsWhereEachDoes(t *testing.T) {
	r := published()
	tpl := opened(t, acme("extra.txt"))
	p, err := here(t, r, "v1.0.0").Plan(tpl)
	if err != nil {
		t.Fatal(err)
	}
	stop := errors.New("cannot write the report")
	d := porttest.NewDisk()
	w := project.Writer{Disk: d, Git: porttest.NewGit(d)}
	rel, err := p.Cut(tpl, answer.Set{"name": "blue-fox"}, &porttest.Folders{Disk: d}, w, porttest.Programs{"has": porttest.Has(d)}, func(template.Result) error { return stop })
	if rel != nil || !errors.Is(err, stop) || len(r.Tags) != 0 {
		t.Errorf("each stopping gives %+v, %v, the tags %v", rel, err, r.Tags)
	}
}

// Nothing half-made: no scenario can make git refuse a tag midway, so the
// repository refusing the last tag is held here.
func TestCutTakesBackTheTagsItMadeWhenOneIsRefused(t *testing.T) {
	r := published()
	r.Tags = map[string]porttest.Tagged{"main/v0.9.0": {Commit: "commit of main"}}
	refusal := errors.New("cannot lock ref")
	r.Refused = map[string]error{"sh/extra/v1.0.0": refusal}
	rel, _, err := cut(t, opened(t, acme("extra.txt")), r, "v1.0.0")
	var refused *Refused
	if rel != nil || !errors.As(err, &refused) || refused.Tag != "sh/extra/v1.0.0" || refused.Left != nil || !errors.Is(err, refusal) {
		t.Errorf("a refused tag gives %+v, %v", rel, err)
	}
	if !slices.Equal(slices.Sorted(maps.Keys(r.Tags)), []string{"main/v0.9.0"}) {
		t.Errorf("the repository holds %v after the refusal", slices.Sorted(maps.Keys(r.Tags)))
	}
}

func TestCutNamesTheTagsItCouldNotTakeBack(t *testing.T) {
	r := published()
	r.Refused = map[string]error{"sh/extra/v1.0.0": errors.New("cannot lock ref")}
	r.Stuck = map[string]error{"main/v1.0.0": errors.New("cannot delete")}
	_, _, err := cut(t, opened(t, acme("extra.txt")), r, "v1.0.0")
	var refused *Refused
	if !errors.As(err, &refused) || !slices.Equal(refused.Left, []string{"main/v1.0.0"}) {
		t.Errorf("a tag left behind gives %v", err)
	}
	if !slices.Equal(slices.Sorted(maps.Keys(r.Tags)), []string{"main/v1.0.0"}) {
		t.Errorf("the repository holds %v", slices.Sorted(maps.Keys(r.Tags)))
	}
}

func TestEachRefusalSaysWhatItIs(t *testing.T) {
	for _, c := range []struct {
		err  Error
		want string
	}{
		{&BadVersion{Version: "v1.0"}, `"v1.0"`},
		{&NotRepository{Why: "fatal"}, "fatal"},
		{&NoOrigin{}, "origin"},
		{&Used{Tags: []string{"main/v1.0.0", "go/cli/v1.0.0"}}, "main/v1.0.0, go/cli/v1.0.0"},
		{&Unsynced{Branch: "stack/go", How: Behind}, "stack/go is behind origin's"},
		{&Failed{Failed: 1, All: 5}, "1 of 5"},
		{&Refused{Tag: "main/v1.0.0", Err: errors.New("locked")}, "main/v1.0.0: locked"},
	} {
		if !strings.Contains(c.err.Error(), c.want) {
			t.Errorf("%T says %q", c.err, c.err.Error())
		}
	}
}
