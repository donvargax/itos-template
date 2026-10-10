package template

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

// acmeBranches are the branches acme's manifest lists, in its order, the
// root first.
var acmeBranches = []string{"main", "stack/sh", "stack/py", "sh/extra", "sh/more", "py/tool"}

// release tags every branch of acme's manifest <branch>/<version> on that
// branch, each tag's tree its branch's with release.txt holding version.
func release(repo *porttest.Repository, version string) *porttest.Repository {
	if repo.Tagged == nil {
		repo.Tagged = map[string]porttest.Tag{}
	}
	for _, b := range acmeBranches {
		repo.Tagged[b+"/"+version] = porttest.Tag{Tree: over(repo.Branches[b], fstest.MapFS{"release.txt": {Data: []byte(version + "\n")}}), On: b}
	}
	return repo
}

// released is acme released as each of versions.
func released(versions ...string) *porttest.Repository {
	repo := acme()
	for _, v := range versions {
		release(repo, v)
	}
	return repo
}

func TestNewestIsTheHighestCompleteStableReleaseAndTheRenderIsIts(t *testing.T) {
	tpl := open(t, released("v1.0.0", "v1.10.0", "v1.9.0", "v2.0.0-rc.1"))
	if ok, err := tpl.UseNewest(); err != nil || !ok {
		t.Fatalf("UseNewest = %v, %v", ok, err)
	}
	if tpl.Release() != "v1.10.0" {
		t.Errorf("the newest release is %q, not v1.10.0: semver orders them, a pre-release is never the newest", tpl.Release())
	}
	w, d, _ := writer()
	p, err := tpl.Render(combination(t, tpl, "sh", "extra"), answers, project.Folder{Path: "made", New: true}, nil, w)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(d.Folders["made"]["release.txt"].Data); got != "v1.10.0\n" {
		t.Errorf("the render holds release.txt %q", got)
	}
	want := map[string]string{"main": "tag main/v1.10.0", "stack/sh": "tag stack/sh/v1.10.0", "sh/extra": "tag sh/extra/v1.10.0"}
	if p.Release != "v1.10.0" || !maps.Equal(p.Commits, want) {
		t.Errorf("the record names the release %q and the commits %v", p.Release, p.Commits)
	}
}

func TestAnIncompleteReleaseIsSkippedByDefault(t *testing.T) {
	for name, lack := range map[string]func(*porttest.Repository){
		"a tag missing":          func(r *porttest.Repository) { delete(r.Tagged, "sh/more/v1.1.0") },
		"the root's tag missing": func(r *porttest.Repository) { delete(r.Tagged, "main/v1.1.0") },
		"a tag on no branch":     func(r *porttest.Repository) { r.Tagged["py/tool/v1.1.0"] = porttest.Tag{Tree: r.Branches["py/tool"]} },
		"a tag on another branch": func(r *porttest.Repository) {
			r.Tagged["py/tool/v1.1.0"] = porttest.Tag{Tree: r.Branches["py/tool"], On: "stack/py"}
		},
	} {
		repo := released("v1.0.0", "v1.1.0")
		lack(repo)
		tpl := open(t, repo)
		if ok, err := tpl.UseNewest(); err != nil || !ok || tpl.Release() != "v1.0.0" {
			t.Errorf("with %s, UseNewest = %v, %v, the release %q, not v1.0.0", name, ok, err, tpl.Release())
		}
	}
}

func TestATemplateWithNoCompleteStableReleaseRendersItsHeads(t *testing.T) {
	for name, repo := range map[string]*porttest.Repository{
		"no tag":                  acme(),
		"only a pre-release":      released("v1.0.0-rc.1"),
		"no version semver takes": released("1.0.0", "latest", "v1.0.0.0"),
		"only incomplete ones": func() *porttest.Repository {
			r := released("v1.0.0")
			delete(r.Tagged, "stack/py/v1.0.0")
			return r
		}(),
		"tags of other branches only": func() *porttest.Repository {
			r := released("v1.0.0")
			delete(r.Tagged, "main/v1.0.0")
			return r
		}(),
	} {
		tpl := open(t, repo)
		if ok, err := tpl.UseNewest(); err != nil || ok || tpl.Release() != "" {
			t.Errorf("with %s, UseNewest = %v, %v, the release %q", name, ok, err, tpl.Release())
			continue
		}
		w, d, _ := writer()
		p, err := tpl.Render(combination(t, tpl, "sh"), answers, project.Folder{Path: "made", New: true}, nil, w)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := d.Folders["made"]["release.txt"]; ok || p.Release != "" || p.Commits["stack/sh"] != "commit of stack/sh" {
			t.Errorf("with %s, the render of the heads is %+v", name, p.Record)
		}
	}
}

func TestAVersionSemverRefusesIsNoRelease(t *testing.T) {
	repo := acme()
	release(repo, "1.0.0")
	release(repo, "latest")
	release(repo, "v1.2")
	tpl := open(t, repo)
	if ok, err := tpl.UseNewest(); err != nil || !ok || tpl.Release() != "v1.2" {
		t.Errorf("UseNewest = %v, %v, the release %q: v1.2 is semver's, 1.0.0 and latest are not", ok, err, tpl.Release())
	}
	var none *NoRelease
	if err := open(t, repo).UseRelease("latest"); !errors.As(err, &none) || none.Version != "latest" || !slices.Equal(none.Releases, []string{"v1.2"}) {
		t.Errorf("UseRelease(latest) = %v", err)
	}
}

// Versions semver holds equal are ordered by their text, so the newest is
// the same on every run.
func TestVersionsSemverHoldsEqualAreOrderedByTheirText(t *testing.T) {
	tpl := open(t, released("v1.0.0", "v1", "v1.0.0+b", "v0.9.0"))
	if ok, err := tpl.UseNewest(); err != nil || !ok || tpl.Release() != "v1.0.0+b" {
		t.Errorf("UseNewest = %v, %v, the release %q", ok, err, tpl.Release())
	}
	var none *NoRelease
	err := open(t, released("v1.0.0", "v1", "v1.0.0+b", "v0.9.0")).UseRelease("v3.0.0")
	if !errors.As(err, &none) || !slices.Equal(none.Releases, []string{"v0.9.0", "v1", "v1.0.0", "v1.0.0+b"}) {
		t.Errorf("UseRelease = %v", err)
	}
}

func TestUseReleaseRendersTheReleaseNamedAPreReleaseToo(t *testing.T) {
	for _, v := range []string{"v1.0.0", "v2.0.0-rc.1"} {
		tpl := open(t, released("v1.0.0", "v1.1.0", "v2.0.0-rc.1"))
		if err := tpl.UseRelease(v); err != nil {
			t.Fatal(err)
		}
		w, d, _ := writer()
		p, err := tpl.Render(combination(t, tpl, "py", "tool"), answers, project.Folder{Path: "made", New: true}, nil, w)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(d.Folders["made"]["release.txt"].Data); got != v+"\n" || p.Release != v || p.Commits["main"] != "tag main/"+v {
			t.Errorf("--ref %s rendered release.txt %q, the record %+v", v, got, p.Record)
		}
	}
}

func TestUseReleaseRefusesAVersionNoReleaseHas(t *testing.T) {
	for name, repo := range map[string]*porttest.Repository{
		"released": released("v1.1.0", "v1.0.0"),
		"tagged elsewhere": func() *porttest.Repository {
			r := released("v1.1.0", "v1.0.0")
			r.Tagged["other/v9.9.9"] = porttest.Tag{Tree: r.Branches["main"], On: "main"}
			return r
		}(),
	} {
		var none *NoRelease
		err := open(t, repo).UseRelease("v9.9.9")
		if !errors.As(err, &none) || none.Version != "v9.9.9" || !slices.Equal(none.Releases, []string{"v1.0.0", "v1.1.0"}) {
			t.Errorf("%s: UseRelease = %v", name, err)
		}
	}
}

func TestUseReleaseRefusesAnIncompleteReleaseNamingEachBranchWithoutItsTag(t *testing.T) {
	repo := released("v1.0.0")
	delete(repo.Tagged, "main/v1.0.0")
	delete(repo.Tagged, "sh/more/v1.0.0")
	repo.Tagged["py/tool/v1.0.0"] = porttest.Tag{Tree: repo.Branches["py/tool"]}
	tpl := open(t, repo)
	delete(repo.Branches, "stack/py")
	var incomplete *Incomplete
	err := tpl.UseRelease("v1.0.0")
	if !errors.As(err, &incomplete) || incomplete.Version != "v1.0.0" || !slices.Equal(incomplete.Branches, []string{"main", "stack/py", "sh/more", "py/tool"}) {
		t.Errorf("UseRelease = %v", err)
	}
}

// A release's manifest is its root's tag's: the branches it lists are the
// ones the release must tag, and the questions it asks are the render's.
func TestAReleaseIsJudgedAndRenderedByItsOwnManifest(t *testing.T) {
	repo := released("v1.0.0")
	head := strings.Replace(manifestText, "  - name: tool\n    stack: py\n", "  - name: tool\n    stack: py\n  - name: new\n    stack: py\n", 1)
	head = strings.Replace(head, "Owner?", "Who owns it?", 1)
	repo.Branches["main"][manifest.File] = &fstest.MapFile{Data: []byte(head)}
	repo.Branches["py/new"] = repo.Branches["stack/py"]
	tpl := open(t, repo)
	if q, _ := tpl.Manifest.Question("owner"); q.Question != "Who owns it?" {
		t.Fatalf("the heads' manifest asks %q", q.Question)
	}
	if err := tpl.UseRelease("v1.0.0"); err != nil {
		t.Fatalf("UseRelease = %v: py/new is not the release's", err)
	}
	if q, _ := tpl.Manifest.Question("owner"); q.Question != "Owner?" {
		t.Errorf("the release's manifest asks %q", q.Question)
	}
}

func TestUseReleaseRefusesAReleaseWhoseRootHoldsNoManifestItReads(t *testing.T) {
	repo := released("v1.0.0")
	delete(repo.Tagged["main/v1.0.0"].Tree, manifest.File)
	var noManifest *NoManifest
	if err := open(t, repo).UseRelease("v1.0.0"); !errors.As(err, &noManifest) || noManifest.Root != "main/v1.0.0" || noManifest.Template != "../acme" {
		t.Errorf("UseRelease = %v", err)
	}
	repo = released("v1.0.0")
	repo.Tagged["main/v1.0.0"].Tree[manifest.File] = &fstest.MapFile{Data: []byte("version: 6\nstacks: []\n")}
	var invalid *ManifestInvalid
	if _, err := open(t, repo).UseNewest(); !errors.As(err, &invalid) || invalid.Root != "main/v1.0.0" {
		t.Errorf("UseNewest = %v", err)
	}
}

func TestCheckProvesTheReleaseUsed(t *testing.T) {
	tpl := open(t, released("v1.0.0", "v1.1.0"))
	if err := tpl.UseRelease("v1.0.0"); err != nil {
		t.Fatal(err)
	}
	w, d, _ := writer()
	folders := &porttest.Folders{Disk: d}
	var holds []string
	run := porttest.Programs{"has": func(dir string, _ []string) ([]byte, bool) {
		holds = append(holds, string(d.Folders[dir]["release.txt"].Data))
		return nil, true
	}}
	if err := tpl.Check(answers, folders, w, run, func(Result) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(holds) == 0 || slices.ContainsFunc(holds, func(h string) bool { return h != "v1.0.0\n" }) {
		t.Errorf("the checks ran in renders holding %q", holds)
	}
}
