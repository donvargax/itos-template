package project

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

// The record is read as it is written: update and adopt read the type new
// writes.
func TestARecordReadsAsItIsWritten(t *testing.T) {
	r := Record{
		Version:  RecordVersion,
		Template: "../acme",
		Stack:    "go",
		Features: []string{"cli"},
		Answers:  answer.Set{"name": "blue-fox", "module": "example.com/blue/fox"},
		Commits:  map[string]string{"main": "9a2e", "stack/go": "7b44", "go/cli": "3f1c"},
	}
	data, err := r.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, recordHeader+"version: 1\ntemplate: ../acme\nstack: go\nfeatures:\n  - cli\nanswers:\n") {
		t.Errorf("the record is\n%s", text)
	}
	var read Record
	if err := yaml.Unmarshal(data, &read); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read, r) {
		t.Errorf("read %+v, wrote %+v", read, r)
	}
}

func fakes() (Writer, *porttest.Disk, *porttest.Git) {
	d := porttest.NewDisk()
	g := porttest.NewGit(d)
	return Writer{Disk: d, Git: g}, d, g
}

func TestLookTakesAMissingOrEmptyFolderOnly(t *testing.T) {
	w, d, _ := fakes()
	d.Folders["empty"] = map[string]port.File{}
	d.Folders["full"] = map[string]port.File{"keep.txt": {Path: "keep.txt"}}
	d.Files["file"] = true
	if f, err := w.Look("made"); err != nil || f != (Folder{Path: "made", New: true}) {
		t.Errorf("a missing folder: %+v, %v", f, err)
	}
	if f, err := w.Look("empty"); err != nil || f != (Folder{Path: "empty"}) {
		t.Errorf("an empty folder: %+v, %v", f, err)
	}
	var full *NotEmpty
	if _, err := w.Look("full"); !errors.As(err, &full) || full.Folder != "full" {
		t.Errorf("a folder with files in it: %v", err)
	}
	var file *NotFolder
	if _, err := w.Look("file"); !errors.As(err, &file) || file.Folder != "file" {
		t.Errorf("a file: %v", err)
	}
}

var files = []port.File{
	{Path: "README.md", Mode: 0o644, Data: []byte("# blue-fox\n")},
	{Path: "bin/blue-fox", Mode: porttest.Executable, Data: []byte("#!/bin/sh\n")},
}

var record = Record{Template: "../acme", Stack: "go", Features: []string{"cli"}, Answers: answer.Set{"name": "blue-fox"}, Commits: map[string]string{"main": "1"}}

func TestWriteCommitsTheFilesAndTheRecordAsTheFirstCommit(t *testing.T) {
	w, d, g := fakes()
	p, err := w.Write(Folder{Path: "made", New: true}, files, record, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Folder != "made" || p.Commit != "commit of made" || p.Version != RecordVersion || p.Stack != "go" {
		t.Errorf("the project is %+v", p)
	}
	commit := g.Commits["made"]
	if len(commit.Files) != 3 || string(commit.Files["README.md"].Data) != "# blue-fox\n" || commit.By.Name != "Someone" {
		t.Errorf("the commit is %+v", commit)
	}
	// Recorded executable whatever the file system says: the commit's mode.
	if commit.Files["bin/blue-fox"].Mode != 0o755 || commit.Files["README.md"].Mode != 0o644 {
		t.Errorf("the commit's modes: %v", commit.Files)
	}
	written, err := (&p.Record).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Folders["made"][RecordFile]; string(got.Data) != string(written) {
		t.Errorf("the record written is\n%s", got.Data)
	}
	want := "chore: make the project from its template\n\nMade by itos-template new from ../acme: the stack go, the features cli. .itos-template.yaml records the render.\n"
	if commit.Message != want {
		t.Errorf("the commit message is %q", commit.Message)
	}
}

func TestWriteSaysNoFeaturesWhenThereAreNone(t *testing.T) {
	w, _, g := fakes()
	r := record
	r.Features = []string{}
	if _, err := w.Write(Folder{Path: "made", New: true}, files, r, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.Commits["made"].Message, "the stack go, no features.") {
		t.Errorf("the commit message is %q", g.Commits["made"].Message)
	}
}

func TestWriteCommitsAsTheIdentityGivenWhenGitKnowsNoOne(t *testing.T) {
	w, _, g := fakes()
	g.Who = nil
	by := port.Identity{Name: "check", Email: "check@localhost"}
	if _, err := w.Write(Folder{Path: "tmp"}, files, record, &by); err != nil {
		t.Fatal(err)
	}
	if g.Commits["tmp"].By != by {
		t.Errorf("the commit is by %+v", g.Commits["tmp"].By)
	}
}

func TestWriteRefusesBeforeWritingWhenGitKnowsNoOne(t *testing.T) {
	w, d, g := fakes()
	g.Who = nil
	if _, err := w.Write(Folder{Path: "made", New: true}, files, record, nil); !errors.Is(err, porttest.ErrNoIdentity) {
		t.Fatalf("Write = %v", err)
	}
	if _, ok := d.Folders["made"]; ok {
		t.Error("the folder was made")
	}
}

// A failed write leaves the folder as it was: a folder Write made removed,
// one that was empty emptied.
func TestWriteRemovesWhatItWroteWhenItFails(t *testing.T) {
	for _, made := range []bool{true, false} {
		w, d, _ := fakes()
		if !made {
			d.Folders["made"] = map[string]port.File{}
		}
		d.Full = "bin/blue-fox"
		if _, err := w.Write(Folder{Path: "made", New: made}, files, record, nil); !errors.Is(err, porttest.ErrFull) {
			t.Fatalf("Write = %v", err)
		}
		if files, ok := d.Folders["made"]; ok == made || len(files) != 0 {
			t.Errorf("made %v: the folder is left %v, %v", made, ok, files)
		}
	}
}

func TestWriteRemovesWhatItWroteWhenGitRefusesTheCommit(t *testing.T) {
	w, d, g := fakes()
	g.Refuse = errors.New("a hook said no")
	_, err := w.Write(Folder{Path: "made", New: true}, files, record, nil)
	var refused *CommitRefused
	if !errors.As(err, &refused) || refused.Err != g.Refuse {
		t.Fatalf("Write = %v", err)
	}
	if _, ok := d.Folders["made"]; ok {
		t.Error("the folder is left")
	}
}
