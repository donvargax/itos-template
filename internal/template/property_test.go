package template

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.yaml.in/yaml/v3"
	"golang.org/x/mod/semver"
	"pgregory.net/rapid"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

// pathElements are what a template's paths are made of: the literals'
// forms, the manifest's name and other names.
var pathElements = []string{"acme-widget", "AcmeWidget", "acme_widget", "ACME_WIDGET", "acmeWidget", "Acme Corp", "src", "docs", "x.txt", manifest.File}

// pieces are what a template's files hold: the literals' forms, line
// endings of every kind, and other text.
var pieces = []string{"acme-widget", "AcmeWidget", "acme_widget", "ACME_WIDGET", "acmeWidget", "Acme Corp", "\n", "\r\n", "\r", " ", "package main", "# "}

// ownFiles are the files a branch adds, under its own folder, dir: text
// files, binary ones, executable ones and links, named with the literals.
func ownFiles(t *rapid.T, dir string) fstest.MapFS {
	tree := fstest.MapFS{}
	for range rapid.IntRange(0, 4).Draw(t, "files of "+dir) {
		p := strings.Join(append([]string{dir}, rapid.SliceOfN(rapid.SampledFrom(pathElements), 1, 3).Draw(t, "path")...), "/")
		data := []byte(strings.Join(rapid.SliceOfN(rapid.SampledFrom(pieces), 0, 8).Draw(t, "contents"), ""))
		var mode = rapid.SampledFrom([]string{"text", "binary", "executable", "link"}).Draw(t, "kind")
		switch mode {
		case "binary":
			tree[p] = &fstest.MapFile{Data: slices.Insert(data, rapid.IntRange(0, len(data)).Draw(t, "NUL at"), 0)}
		case "executable":
			tree[p] = &fstest.MapFile{Data: data, Mode: porttest.Executable}
		case "link":
			tree[p] = &fstest.MapFile{Data: []byte(rapid.SampledFrom(pathElements).Draw(t, "target")), Mode: porttest.Link}
		default:
			tree[p] = &fstest.MapFile{Data: data}
		}
	}
	return tree
}

// templates are template repositories as an author makes them: a manifest
// of one or two stacks, each with up to two features, some needing
// another; the root's files with the manifest, each stack's branch its
// root's files and its own, and each feature's its stack's, the files of
// the features it needs and its own.
var templates = rapid.Custom(func(t *rapid.T) *porttest.Repository {
	m := manifest.Manifest{Version: 2, TemplateOnly: []string{"docs"}, Questions: []manifest.Question{
		{Name: "name", Literal: "acme-widget", Question: "Name?", Pattern: "[a-z]+(-[a-z]+)*", CaseForms: true},
		{Name: "owner", Literal: "Acme Corp", Question: "Owner?"},
	}}
	name := rapid.StringMatching(`[a-z]{2,3}`)
	for _, s := range rapid.SliceOfNDistinct(name, 1, 2, rapid.ID[string]).Draw(t, "stacks") {
		m.Stacks = append(m.Stacks, manifest.Stack{Name: s})
		features := rapid.SliceOfNDistinct(name, 0, 2, rapid.ID[string]).Draw(t, "features of "+s)
		for _, f := range features {
			var needs []string
			for _, other := range features {
				if other != f && rapid.Bool().Draw(t, f+" needs "+other) {
					needs = append(needs, other)
				}
			}
			m.Features = append(m.Features, manifest.Feature{Name: f, Stack: s, Needs: needs})
		}
	}
	text, err := yaml.Marshal(&m)
	if err != nil {
		t.Fatal(err)
	}
	root := ownFiles(t, "root")
	root[manifest.File] = &fstest.MapFile{Data: text}
	repo := &porttest.Repository{Root: "main", Branches: map[string]fstest.MapFS{"main": root}}
	own := map[string]fstest.MapFS{}
	for _, s := range m.Stacks {
		repo.Branches[s.Branch()] = over(root, ownFiles(t, "stack-"+s.Name))
	}
	for _, f := range m.Features {
		own[f.Branch()] = ownFiles(t, "feature-"+f.Stack+"-"+f.Name)
	}
	for _, f := range m.Features {
		tree := over(repo.Branches["stack/"+f.Stack], own[f.Branch()])
		for _, need := range f.Needs {
			tree = over(tree, own[f.Stack+"/"+need])
		}
		repo.Branches[f.Branch()] = tree
	}
	return repo
})

// rendered is what a render made: its project and its commit, or why it
// refused.
type rendered struct {
	project *project.Project
	commit  porttest.Commit
	err     error
}

func renderOf(t *rapid.T, repo *porttest.Repository, c manifest.Combination, answers answer.Set) rendered {
	tpl, err := Open("../acme", repo)
	if err != nil {
		t.Fatalf("Open = %v", err)
	}
	w, _, g := writer()
	p, err := tpl.Render(c, answers, project.Folder{Path: "made", New: true}, nil, w)
	return rendered{p, g.Commits["made"], err}
}

// The same template, combination and answers render the same bytes every
// time: the same files with the same modes and contents, the same commit
// message and the same record, or the same refusal, as an update needs to
// reproduce a render (decision 3).
func TestRenderIsDeterministic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		repo := templates.Draw(t, "template")
		tpl, err := Open("../acme", repo)
		if err != nil {
			t.Fatalf("Open = %v", err)
		}
		c := rapid.SampledFrom(tpl.Manifest.Combinations()).Draw(t, "combination")
		answers := answer.Set{
			"name":  rapid.StringMatching(`[a-z]{1,4}(-[a-z]{1,4}){0,2}`).Draw(t, "name"),
			"owner": rapid.StringMatching(`[ -~]{1,10}`).Draw(t, "owner"),
		}
		first, again := renderOf(t, repo, c, answers), renderOf(t, repo, c, answers)
		switch {
		case fmt.Sprint(first.err) != fmt.Sprint(again.err):
			t.Fatalf("one render gives %v, the same again %v", first.err, again.err)
		case first.err != nil:
			return
		case !maps.EqualFunc(first.commit.Files, again.commit.Files, func(a, b port.File) bool {
			return a.Path == b.Path && a.Mode == b.Mode && string(a.Data) == string(b.Data)
		}):
			t.Fatalf("one render commits %v, the same again %v", first.commit.Files, again.commit.Files)
		case first.commit.Message != again.commit.Message || !reflect.DeepEqual(first.project, again.project):
			t.Fatalf("one render makes %+v, the same again %+v", first.project, again.project)
		}
	})
}

// versionText draws what a tag's last segment can hold: versions semver
// takes, stable or pre-releases, whole or short (v1, v1.2), with a build
// or not, and texts it refuses.
var versionText = rapid.OneOf(
	rapid.StringMatching(`v[0-2](\.[0-2](\.[0-2])?)?`),
	rapid.StringMatching(`v[0-2]\.[0-2]\.[0-2]-(rc|beta)\.[0-2]`),
	rapid.StringMatching(`v[0-2]\.[0-2]\.[0-2]\+b[0-2]`),
	rapid.SampledFrom([]string{"1.0.0", "v01.0.0", "latest", "v1.0.0.0"}),
)

// The newest release is the semver maximum of the complete versions with
// no pre-release part, whatever the order they were tagged in and however
// the others are incomplete; with none of them the heads are rendered.
func TestTheNewestReleaseIsTheSemverMaximumOfTheCompleteStableOnes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		repo := acme()
		var stable []string // the complete versions with no pre-release part
		for _, v := range rapid.SliceOfNDistinct(versionText, 0, 6, rapid.ID[string]).Draw(t, "versions") {
			release(repo, v)
			switch rapid.SampledFrom([]string{"complete", "a tag missing", "a tag off its branch"}).Draw(t, "how "+v) {
			case "a tag missing":
				delete(repo.Tagged, rapid.SampledFrom(acmeBranches).Draw(t, "branch")+"/"+v)
			case "a tag off its branch":
				name := rapid.SampledFrom(acmeBranches).Draw(t, "branch") + "/" + v
				repo.Tagged[name] = porttest.Tag{Tree: repo.Tagged[name].Tree}
			default:
				if semver.IsValid(v) && semver.Prerelease(v) == "" {
					stable = append(stable, v)
				}
			}
		}
		tpl, err := Open("../acme", repo)
		if err != nil {
			t.Fatalf("Open = %v", err)
		}
		ok, err := tpl.UseNewest()
		switch {
		case err != nil:
			t.Fatalf("UseNewest = %v", err)
		case ok != (len(stable) > 0):
			t.Fatalf("UseNewest = %v, the release %q, of the complete stable versions %q", ok, tpl.Release(), stable)
		case ok && !slices.Contains(stable, tpl.Release()):
			t.Fatalf("the newest release %q is none of the complete stable versions %q", tpl.Release(), stable)
		}
		for _, v := range stable {
			if semver.Compare(tpl.Release(), v) < 0 {
				t.Fatalf("the newest release %q is older than %q", tpl.Release(), v)
			}
		}
	})
}
