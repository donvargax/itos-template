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

// problems are the problems a refusal of Plan names, one each, in order.
func problems(t *testing.T, err error) []string {
	t.Helper()
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("Plan = %v, not the problems joined", err)
	}
	var lines []string
	for _, e := range joined.Unwrap() {
		var re Error
		if !errors.As(e, &re) {
			t.Fatalf("%v is no render.Error", e)
		}
		lines = append(lines, e.Error())
	}
	return lines
}

// refusesInOrder is whether Plan refuses files, and the same files in the
// reverse order, run after run, naming the problems want in want's order:
// a render, a refusal included, is the same every time (decision 3; bug-2).
func refusesInOrder(t *testing.T, files []port.File, r *Replacer, want []string) {
	t.Helper()
	reversed := slices.Clone(files)
	slices.Reverse(reversed)
	for range 20 {
		for _, tree := range [][]port.File{files, reversed} {
			_, err := Plan(tree, all, r)
			if got := problems(t, err); !slices.Equal(got, want) {
				t.Fatalf("Plan names\n%s\nnot\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		}
	}
}

func TestPlanNamesEveryFileWhereAFolderIsNeededInPathOrder(t *testing.T) {
	refusesInOrder(t, []port.File{
		{Path: "acme-widget", Mode: 0o644},
		{Path: "blue-fox/b.txt", Mode: 0o644},
		{Path: "blue-fox/a.txt", Mode: 0o644},
		{Path: "acme-widget/c/d.txt", Mode: 0o644},
	}, acme, []string{
		"the answers name the file acme-widget blue-fox, a folder of blue-fox/a.txt",
		"the answers name the file acme-widget blue-fox, a folder of blue-fox/b.txt",
		"the answers name the file acme-widget blue-fox, a folder of acme-widget/c/d.txt",
	})
}

func TestPlanNamesEveryTwoFilesOfOneNameInPathOrder(t *testing.T) {
	refusesInOrder(t, []port.File{
		{Path: "b/acme-widget", Mode: 0o644},
		{Path: "b/blue-fox", Mode: 0o644},
		{Path: "a/blue-fox", Mode: 0o644},
		{Path: "a/acme-widget", Mode: 0o644},
	}, acme, []string{
		"the answers name both a/acme-widget and a/blue-fox a/blue-fox",
		"the answers name both b/acme-widget and b/blue-fox b/blue-fox",
	})
}

func TestPlanNamesEveryNameNoFileCanHaveInPathOrder(t *testing.T) {
	refusesInOrder(t, []port.File{
		{Path: "z/acme/x", Mode: 0o644},
		{Path: "a/acme", Mode: 0o644},
		{Path: "m/ok", Mode: 0o644},
	}, NewReplacer([]string{"acme", ".."}), []string{
		`the answers name a/acme "a/.."`,
		`the answers name z/acme/x "z/.."`,
	})
}

func TestPlanNamesEverySubmoduleInPathOrder(t *testing.T) {
	refusesInOrder(t, append(tree(),
		port.File{Path: "vendor/z", Mode: porttest.Submodule},
		port.File{Path: "vendor/a", Mode: porttest.Submodule},
	), acme, []string{"a submodule at vendor/a", "a submodule at vendor/z"})
}

// A disk that folds case files acme-widget (the answer, blue-fox) and
// Blue-Fox as one, though a case-sensitive one tells them apart; Plan refuses
// them on every disk (bug-8), naming both, in the order of the names.
func TestPlanRefusesTwoNamesOneFileWhereCaseIsFolded(t *testing.T) {
	refusesInOrder(t, []port.File{
		{Path: "acme-widget", Mode: 0o644},
		{Path: "Blue-Fox", Mode: 0o644},
	}, acme, []string{"the answers name Blue-Fox Blue-Fox and acme-widget blue-fox, one file where case or accents do not count"})
}

// cafe precomposed (NFC) and decomposed (NFD) are different byte strings but
// one file where Unicode normalization is folded (HFS+); Plan refuses them.
func TestPlanRefusesTwoNamesOneFileWhereNormalizationIsFolded(t *testing.T) {
	nfd := "cafe" + string(rune(0x0301)) // e then a combining acute accent
	nfc := "caf" + string(rune(0x00e9))  // the precomposed e-acute
	_, err := Plan([]port.File{{Path: nfd, Mode: 0o644}, {Path: nfc, Mode: 0o644}}, all, acme)
	var clash *CaseClash
	if !errors.As(err, &clash) {
		t.Fatalf("Plan = %v, not a CaseClash", err)
	}
	if fold(clash.As) != fold(clash.AndAs) || clash.As == clash.AndAs {
		t.Errorf("CaseClash names %q and %q, not two spellings of one folded name", clash.As, clash.AndAs)
	}
}

// Two names that fold apart are two files: Plan keeps them both.
func TestPlanKeepsNamesThatFoldApart(t *testing.T) {
	if _, err := Plan([]port.File{{Path: "README.md", Mode: 0o644}, {Path: "readme.txt", Mode: 0o644}}, all, acme); err != nil {
		t.Errorf("Plan refused two names one disk cannot fold into one: %v", err)
	}
}

// Path refuses every spelling a file system resolves to the .git folder, as
// git does (core.protectNTFS and core.protectHFS), and keeps the names that
// merely look like one (bug-8).
func TestPathRefusesEveryDotGitSpellingAndKeepsTheRest(t *testing.T) {
	none := NewReplacer(nil)
	zwnj, zwj, lrm, bom := string(rune(0x200c)), string(rune(0x200d)), string(rune(0x200e)), string(rune(0xfeff))
	refused := []string{
		".git", ".GIT", ".Git", ".git ", ".git.", ".git. .", "git~1", "GIT~1",
		".git::$INDEX_ALLOCATION", ".git:x",
		".gi" + zwnj + "t", bom + ".git", ".g" + zwj + "i" + lrm + "t",
	}
	for _, e := range refused {
		_, err := none.Path("a/" + e + "/b")
		var bad *BadName
		if !errors.As(err, &bad) || bad.Element != e {
			t.Errorf("Path kept %q, a spelling of .git: %v", e, err)
		}
	}
	for _, e := range []string{"git", ".gitignore", ".gitmodules", ".github", "digit", "mygit~1", "git~", "git~1x"} {
		if _, err := none.Path("a/" + e + "/b"); err != nil {
			t.Errorf("Path refused %q, a name a file can have: %v", e, err)
		}
	}
}
