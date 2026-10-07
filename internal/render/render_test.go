package render

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

var acme = NewReplacer([]string{
	"acme-widget", "blue-fox", "acme_widget", "blue_fox", "ACME_WIDGET", "BLUE_FOX",
	"acmeWidget", "blueFox", "AcmeWidget", "BlueFox",
})

// tree is a merged tree: a CRLF text file, a binary one, an executable
// script under a folder named after the literal, and a link to it.
func tree() []port.File {
	return []port.File{
		{Path: "README.md", Mode: 0o644, Data: []byte("# acme-widget\r\nAcmeWidget, ACME_WIDGET\r\n")},
		{Path: "acme-widget/run.sh", Mode: porttest.Executable, Data: []byte("#!/bin/sh\necho acme_widget\n")},
		{Path: "logo.bin", Mode: 0o644, Data: []byte("\x00acme-widget")},
		{Path: "run", Mode: porttest.Link, Data: []byte("acme-widget/run.sh")},
	}
}

func all(string) bool { return true }

func byPath(files []port.File) map[string]port.File {
	m := map[string]port.File{}
	for _, f := range files {
		m[f.Path] = f
	}
	return m
}

func TestPlanReplacesInContentsNamesAndLinksKeepingBytesAndModes(t *testing.T) {
	files, err := Plan(tree(), all, acme)
	if err != nil {
		t.Fatal(err)
	}
	got := byPath(files)
	if f := got["README.md"]; string(f.Data) != "# blue-fox\r\nBlueFox, BLUE_FOX\r\n" || f.Mode != 0o644 {
		t.Errorf("README.md is %q, %v: CRLF and the replacements not as they should be", f.Data, f.Mode)
	}
	if f := got["logo.bin"]; string(f.Data) != "\x00acme-widget" {
		t.Errorf("logo.bin is %q: a binary file is never replaced in", f.Data)
	}
	if f := got["blue-fox/run.sh"]; string(f.Data) != "#!/bin/sh\necho blue_fox\n" || !Executable(f) {
		t.Errorf("blue-fox/run.sh is %q, %v", f.Data, f.Mode)
	}
	if f := got["run"]; string(f.Data) != "blue-fox/run.sh" || f.Mode != porttest.Link || Executable(f) {
		t.Errorf("the link run is to %q, %v", f.Data, f.Mode)
	}
	if len(files) != 4 {
		t.Errorf("the plan is %v", files)
	}
}

func TestPlanLeavesOutWhatKeepRefuses(t *testing.T) {
	files, err := Plan(tree(), func(p string) bool { return p != "logo.bin" }, acme)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	if !slices.Equal(paths, []string{"README.md", "blue-fox/run.sh", "run"}) {
		t.Errorf("plan %q", paths)
	}
}

func TestPlanRefusesASubmoduleItKeeps(t *testing.T) {
	sub := append(tree(), port.File{Path: "vendor/lib", Mode: porttest.Submodule})
	_, err := Plan(sub, all, acme)
	var s *Submodule
	if !errors.As(err, &s) || s.Path != "vendor/lib" {
		t.Fatalf("Plan = %v", err)
	}
	if _, err := Plan(sub, func(p string) bool { return p != "vendor/lib" }, acme); err != nil {
		t.Errorf("a submodule left out is refused: %v", err)
	}
}

func TestPlanRefusesTwoFilesOfOneName(t *testing.T) {
	files := append(tree(), port.File{Path: "blue-fox/run.sh", Mode: 0o644})
	_, err := Plan(files, all, acme)
	var same *SameName
	if !errors.As(err, &same) || same.First != "acme-widget/run.sh" || same.Second != "blue-fox/run.sh" || same.To != "blue-fox/run.sh" {
		t.Fatalf("Plan = %v", err)
	}
}

func TestPlanRefusesAFileWhereAnotherNeedsAFolder(t *testing.T) {
	files := append(tree(), port.File{Path: "blue-fox", Mode: 0o644})
	_, err := Plan(files, all, acme)
	var folder *FileIsFolder
	if !errors.As(err, &folder) || folder.File != "blue-fox" || folder.Folder != "blue-fox" || folder.Of != "acme-widget/run.sh" {
		t.Fatalf("Plan = %v", err)
	}
}

func TestPathReplacesEachElementAndRefusesImpossibleNames(t *testing.T) {
	if got, err := acme.Path("cmd/acme-widget/acme_widget.go"); err != nil || got != "cmd/blue-fox/blue_fox.go" {
		t.Errorf("Path = %q, %v", got, err)
	}
	for answer, element := range map[string]string{"a/b": "a/b/x", "..": "../x", "": "/x", ".git": ".git/x", `a\b`: `a\b/x`} {
		r := NewReplacer([]string{"acme", answer})
		_, err := r.Path("acme/x")
		var bad *BadName
		if !errors.As(err, &bad) || bad.Path != "acme/x" || bad.Element != answer || bad.Would != strings.TrimSuffix(element, "/x") {
			t.Errorf("an answer %q making a name no file can have: %v", answer, err)
		}
	}
	r := NewReplacer([]string{"acme", ".."})
	if _, err := r.Path("x/y/acme"); err == nil || err.(*BadName).Would != "x/y/.." {
		t.Errorf("Would is not the path as far as the bad element: %v", err)
	}
}

func TestContentsLeavesABinaryFileAsItIs(t *testing.T) {
	data := append([]byte(strings.Repeat("x", 9000)), []byte("acme-widget")...)
	data[8500] = 0
	if got := acme.Contents(data); string(got[len(got)-8:]) != "blue-fox" {
		t.Error("a NUL past the first 8000 bytes made a text file binary")
	}
	data[10] = 0
	if got := acme.Contents(data); string(got[len(got)-11:]) != "acme-widget" {
		t.Error("a NUL in the first 8000 bytes did not make the file binary")
	}
}

func TestTextReplacesInAnyString(t *testing.T) {
	if got := acme.Text("ls cmd/acme-widget"); got != "ls cmd/blue-fox" {
		t.Errorf("Text = %q", got)
	}
}
