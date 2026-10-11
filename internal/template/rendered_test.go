package template

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/template/port"
)

// headsRecord is the record of a project made from acme's branch heads, the
// stack sh and the feature extra.
func headsRecord() project.Record {
	return project.Record{
		Template: "../acme",
		Stack:    "sh",
		Features: []string{"extra"},
		Answers:  answer.Set{"owner": "Blue Corp", "name": "blue-fox"},
		Commits:  map[string]string{"main": "commit of main", "stack/sh": "commit of stack/sh", "sh/extra": "commit of sh/extra"},
	}
}

func byPath(files []port.File) map[string]port.File {
	m := map[string]port.File{}
	for _, f := range files {
		m[f.Path] = f
	}
	return m
}

// The record's commits render, never the release t renders, which t keeps
// rendering after.
func TestRenderedIsTheRecordsCommitsAsNewRenderedThem(t *testing.T) {
	tpl := open(t, released("v1.0.0"))
	if err := tpl.UseRelease("v1.0.0"); err != nil {
		t.Fatal(err)
	}
	files, err := tpl.Rendered(headsRecord())
	if err != nil {
		t.Fatal(err)
	}
	heads := open(t, acme())
	want, _, err := heads.Baseline(combination(t, heads, "sh", "extra"), taken(t, heads.Manifest, answers))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(files, want, func(a, b port.File) bool {
		return a.Path == b.Path && a.Mode == b.Mode && string(a.Data) == string(b.Data)
	}) {
		t.Errorf("Rendered is %v, not the heads' render %v", files, want)
	}
	if _, ok := byPath(files)["release.txt"]; ok {
		t.Error("the render is the release's, not the record's commits'")
	}
	if got := string(byPath(files)["bin/blue-fox"].Data); got != "#!/bin/sh\necho blue_fox\n" {
		t.Errorf("the render's answers are not the record's: bin/blue-fox holds %q", got)
	}
	if tpl.Release() != "v1.0.0" || tpl.rootCommit != "tag main/v1.0.0" {
		t.Errorf("Rendered left t rendering %q from %s", tpl.Release(), tpl.rootCommit)
	}
}

// The manifest of the record's root commit decides the render: its
// template-only paths, its questions.
func TestRenderedReadsTheManifestTheRecordsRootCommitHolds(t *testing.T) {
	repo := released("v1.0.0")
	tag := repo.Tagged["main/v1.0.0"]
	text := strings.Replace(manifestText, "template_only: [ci.yml]\n", "  - name: extra\n    literal: Extra Thing\n    question: Extra?\n", 1)
	tag.Tree = over(tag.Tree, fstest.MapFS{manifest.File: {Data: []byte(text)}})
	repo.Tagged["main/v1.0.0"] = tag
	r := headsRecord()
	r.Commits["main"] = "tag main/v1.0.0"
	var missing *answer.Missing
	if _, err := open(t, repo).Rendered(r); !errors.As(err, &missing) || missing.Question.Name != "extra" {
		t.Errorf("a record lacking an answer the root commit's manifest asks gives %v", err)
	}
	r.Answers["extra"] = "More"
	delete(r.Answers, "owner")
	if _, err := open(t, repo).Rendered(r); !errors.As(err, &missing) || missing.Question.Name != "owner" {
		t.Errorf("a record lacking an answer with a default takes it, not the record's: %v", err)
	}
	r.Answers["owner"] = "Blue Corp"
	files, err := open(t, repo).Rendered(r)
	if err != nil {
		t.Fatalf("a record answering the root commit's manifest's questions: %v", err)
	}
	if _, ok := byPath(files)["ci.yml"]; !ok {
		t.Error("ci.yml, template-only on the heads but not in the root commit's manifest, is left out")
	}
	r.Commits["main"] = "commit of nowhere"
	if _, err := open(t, repo).Rendered(r); err == nil {
		t.Error("a root commit the template lacks renders")
	}
}

func TestRenderedRefusesABranchTheRecordNamesNoCommitOf(t *testing.T) {
	for _, b := range []string{"main", "stack/sh", "sh/extra"} {
		r := headsRecord()
		delete(r.Commits, b)
		_, err := open(t, acme()).Rendered(r)
		var unrecorded *Unrecorded
		if !errors.As(err, &unrecorded) || unrecorded.Branch != b {
			t.Errorf("a record without %s gives %v", b, err)
		}
		if unrecorded != nil && unrecorded.Error() != "the record names no commit of "+b {
			t.Errorf("Unrecorded says %q", unrecorded.Error())
		}
	}
	r := headsRecord()
	r.Stack = "go"
	var unknown *UnknownStack
	if _, err := open(t, acme()).Rendered(r); !errors.As(err, &unknown) {
		t.Errorf("a record of a stack the manifest lacks gives %v", err)
	}
}
