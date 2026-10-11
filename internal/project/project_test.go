package project

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

// A record of a release names it after the template; one of the branch
// heads has no release key at all.
func TestARecordNamesItsReleaseAfterTheTemplateOrNone(t *testing.T) {
	r := Record{Version: RecordVersion, Template: "../acme", Release: "v1.2.0", Stack: "go"}
	data, err := r.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), recordHeader+"version: 1\ntemplate: ../acme\nrelease: v1.2.0\nstack: go\n") {
		t.Errorf("the record is\n%s", data)
	}
	r.Release = ""
	if data, err = r.Marshal(); err != nil || strings.Contains(string(data), "release:") {
		t.Errorf("a record of the heads is\n%s", data)
	}
}

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

// The record's file is .itos-template.yaml, its format's version set
// whatever the record says, as new writes it and adopt does.
func TestARecordsFileIsItsMarshalledFormat(t *testing.T) {
	r := Record{Template: "../acme", Stack: "go"}
	f, err := r.File()
	if err != nil {
		t.Fatal(err)
	}
	r.Version = RecordVersion
	want, _ := r.Marshal()
	if f.Path != RecordFile || f.Mode != 0o644 || string(f.Data) != string(want) {
		t.Errorf("the record's file is %s %v\n%s", f.Path, f.Mode, f.Data)
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
	p, err := w.Write(Folder{Path: "made", New: true}, files, record, "", nil)
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

func TestWriteNamesTheReleaseInTheFirstCommit(t *testing.T) {
	w, _, g := fakes()
	r := record
	r.Release = "v1.2.0"
	if _, err := w.Write(Folder{Path: "made", New: true}, files, r, "", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.Commits["made"].Message, "new from ../acme at its release v1.2.0: the stack go") {
		t.Errorf("the commit message is %q", g.Commits["made"].Message)
	}
}

func TestWriteSaysNoFeaturesWhenThereAreNone(t *testing.T) {
	w, _, g := fakes()
	r := record
	r.Features = []string{}
	if _, err := w.Write(Folder{Path: "made", New: true}, files, r, "", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.Commits["made"].Message, "the stack go, no features.") {
		t.Errorf("the commit message is %q", g.Commits["made"].Message)
	}
}

// A template that holds its projects to commit rules gives the first
// commit's message itself, so the project's rules pass it; Write commits
// it as it is given.
func TestWriteCommitsWithTheMessageGiven(t *testing.T) {
	w, _, g := fakes()
	message := "chore: start blue-fox\n\nTask: T-1\n"
	if _, err := w.Write(Folder{Path: "made", New: true}, files, record, message, nil); err != nil {
		t.Fatal(err)
	}
	if got := g.Commits["made"].Message; got != message {
		t.Errorf("the commit message is %q", got)
	}
}

func TestWriteCommitsAsTheIdentityGivenWhenGitKnowsNoOne(t *testing.T) {
	w, _, g := fakes()
	g.Who = nil
	by := port.Identity{Name: "check", Email: "check@localhost"}
	if _, err := w.Write(Folder{Path: "tmp"}, files, record, "", &by); err != nil {
		t.Fatal(err)
	}
	if g.Commits["tmp"].By != by {
		t.Errorf("the commit is by %+v", g.Commits["tmp"].By)
	}
}

func TestWriteRefusesBeforeWritingWhenGitKnowsNoOne(t *testing.T) {
	w, d, g := fakes()
	g.Who = nil
	if _, err := w.Write(Folder{Path: "made", New: true}, files, record, "", nil); !errors.Is(err, porttest.ErrNoIdentity) {
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
		if _, err := w.Write(Folder{Path: "made", New: made}, files, record, "", nil); !errors.Is(err, porttest.ErrFull) {
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
	_, err := w.Write(Folder{Path: "made", New: true}, files, record, "", nil)
	var refused *CommitRefused
	if !errors.As(err, &refused) || refused.Err != g.Refuse {
		t.Fatalf("Write = %v", err)
	}
	if _, ok := d.Folders["made"]; ok {
		t.Error("the folder is left")
	}
}

// ReadRecord reads the record new writes, and refuses one it cannot read,
// every problem named, sorted.
func TestReadRecordReadsWhatNewWritesAndRefusesTheRest(t *testing.T) {
	r := Record{
		Template: "/home/you/acme",
		Release:  "v1.0.0",
		Stack:    "go",
		Features: []string{"cli"},
		Answers:  answer.Set{"name": "blue-fox"},
		Commits:  map[string]string{"main": "9a2e"},
	}
	f, err := r.File()
	if err != nil {
		t.Fatal(err)
	}
	read, err := ReadRecord(f.Data)
	r.Version = RecordVersion
	if err != nil || !reflect.DeepEqual(read, r) {
		t.Errorf("ReadRecord = %+v, %v, not %+v", read, err, r)
	}
	for text, want := range map[string][]string{
		"version: 2\ntemplate: a\nstack: go\ncommits: {main: x}\n": {"version is 2: this itos-template reads version 1"},
		"version: 1\n": {"it names no commits", "it names no stack", "it names no template"},
		"version: 1\ntemplate: a\nstack: go\ncommits: {main: x}\nextra: 1\n": nil,
		"": {"it is empty"},
	} {
		_, err := ReadRecord([]byte(text))
		var bad *BadRecord
		if !errors.As(err, &bad) || bad.File != RecordFile {
			t.Errorf("ReadRecord(%q) = %v", text, err)
			continue
		}
		if want != nil && !reflect.DeepEqual(bad.Problems, want) {
			t.Errorf("ReadRecord(%q) names %q, not %q", text, bad.Problems, want)
		}
		if !strings.HasPrefix(bad.Error(), RecordFile+": "+bad.Problems[0]) {
			t.Errorf("BadRecord says %q", bad.Error())
		}
	}
}

// A record declining paths lists them last, in their order, in the file
// and in new's and adopt's --json; one declining none has no such key.
func TestARecordListsItsDeclinedPathsOrNone(t *testing.T) {
	r := Record{Template: "/t", Stack: "go", Commits: map[string]string{"main": "1"}, Declined: []string{"docs", "cli.txt"}}
	f, err := r.File()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(f.Data), "commits:\n  main: \"1\"\ndeclined:\n  - docs\n  - cli.txt\n") {
		t.Errorf("the record is\n%s", f.Data)
	}
	read, err := ReadRecord(f.Data)
	if err != nil || !slices.Equal(read.Declined, r.Declined) {
		t.Errorf("ReadRecord = %+v, %v", read, err)
	}
	if data, _ := json.Marshal(r); !strings.Contains(string(data), `"declined":["docs","cli.txt"]`) {
		t.Errorf("--json gives %s", data)
	}
	r.Declined = nil
	f, _ = r.File()
	if data, _ := json.Marshal(r); strings.Contains(string(f.Data), "declined:") || strings.Contains(string(data), "declined") {
		t.Errorf("a record declining nothing is\n%s\n%s", f.Data, data)
	}
}

// A record's declined path is judged as a template_only path is, and
// listed once: ReadRecord names every one otherwise.
func TestReadRecordRefusesADeclinedPathNotInTheProject(t *testing.T) {
	text := "version: 1\ntemplate: a\nstack: go\ncommits: {main: x}\ndeclined: [docs, docs/, ../x, docs, a.txt]\n"
	_, err := ReadRecord([]byte(text))
	var bad *BadRecord
	want := []string{
		`the declined path "../x" is not a path in the project: write it relative to its top, with /`,
		`the declined path "docs" is listed twice: list it once`,
		`the declined path "docs/" is not a path in the project: write it relative to its top, with /`,
	}
	if !errors.As(err, &bad) || !slices.Equal(bad.Problems, want) {
		t.Errorf("ReadRecord = %v", err)
	}
}

// Decline lists the paths a command line gives as a record does: in their
// order, each once, nil for none; a path no path in a project is written
// as is refused, every one named.
func TestDeclineListsEachPathOnceInItsOrder(t *testing.T) {
	got, err := Decline([]string{"docs", "cli.txt", "docs", "a/b"})
	if err != nil || !slices.Equal(got, []string{"docs", "cli.txt", "a/b"}) {
		t.Errorf("Decline = %q, %v", got, err)
	}
	if got, err := Decline(nil); got != nil || err != nil {
		t.Errorf("Decline(nil) = %q, %v", got, err)
	}
	_, err = Decline([]string{"docs", "", "/abs", "a/./b", `a\b`})
	var bad *BadDeclined
	if !errors.As(err, &bad) || !slices.Equal(bad.Paths, []string{"", "/abs", "a/./b", `a\b`}) {
		t.Errorf("Decline of paths in no project = %v", err)
	}
	if err.Error() != `declined paths not in the project: , /abs, a/./b, a\b` {
		t.Errorf("BadDeclined says %q", err.Error())
	}
}

// A record declines a declined path and everything under it, nothing else;
// Keep is a render less what it declines, in its order.
func TestARecordDeclinesAPathAndWhatIsUnderIt(t *testing.T) {
	r := Record{Declined: []string{"docs", "cli.txt"}}
	for p, want := range map[string]bool{"docs": true, "docs/a.md": true, "docs/b/c": true, "cli.txt": true, "docsx": false, "doc": false, "cli.txt.bak": false, "a/docs": false} {
		if got := r.Declines(p); got != want {
			t.Errorf("Declines(%q) = %v, not %v", p, got, want)
		}
	}
	files := []port.File{{Path: "z"}, {Path: "docs/a.md"}, {Path: "a"}, {Path: "cli.txt"}}
	kept := r.Keep(files)
	if len(kept) != 2 || kept[0].Path != "z" || kept[1].Path != "a" {
		t.Errorf("Keep = %v", kept)
	}
	if kept := (&Record{}).Keep(files); len(kept) != len(files) {
		t.Errorf("a record declining nothing kept %v", kept)
	}
}
