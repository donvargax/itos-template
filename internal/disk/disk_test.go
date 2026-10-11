package disk

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Decision 19 leaves infra to the scenarios, and slice-3 gave them what
// Look tells (a missing, an empty and a full folder, and a file:
// ID-NEW-01, ID-NEW-09, ID-NEW-10 and ID-NEW-29). What stays, no scenario
// reaches: what Clear removes after a write that fails half way, which no
// scenario can make fail; and a symbolic link, which waits for the idea
// template-special-files. A written file's execute bit and a CRLF line
// ending written as it is a scenario could read but none reads yet:
// ID-NEW-22 reads the mode the commit records, not the file's (the idea
// outside-test-gaps).

func TestWriteWritesEachFileWithItsMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "made")
	err := Disk{}.Write(dir, []port.File{
		{Path: "bin/run.sh", Mode: 0o755, Data: []byte("#!/bin/sh\n")},
		{Path: "README.md", Mode: 0o644, Data: []byte("x\r\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "README.md")); err != nil || string(data) != "x\r\n" {
		t.Errorf("README.md is %q, %v", data, err)
	}
	info, err := os.Stat(filepath.Join(dir, "bin", "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Errorf("run.sh is not executable: %v", info.Mode())
	}
}

func TestWriteMakesASymbolicLinkOrSaysWhichItCouldNot(t *testing.T) {
	dir := t.TempDir()
	err := Disk{}.Write(dir, []port.File{{Path: "run", Mode: fs.ModeSymlink | 0o777, Data: []byte("bin/run.sh")}})
	var refused *Unwritable
	if errors.As(err, &refused) && runtime.GOOS == "windows" {
		if refused.Link != "run" {
			t.Errorf("the link refused is %q", refused.Link)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	// windows writes the target with its own separator, as os.Symlink does.
	if target, err := os.Readlink(filepath.Join(dir, "run")); err != nil || target != filepath.FromSlash("bin/run.sh") {
		t.Errorf("run links to %q, %v", target, err)
	}
}

// Write refuses to write a file whose parent folder is a symbolic link, so a
// link a render wrote cannot carry a later file out of the project folder
// (bug-8). render.Plan refuses the names that would do it, so this second
// layer runs under no scenario; a direct test holds it. Symbolic links need a
// privilege on windows, which template-special-files will face.
func TestWriteRefusesToWriteThroughASymbolicLinkParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need a privilege on windows")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "made")
	outside := filepath.Join(root, "outside")
	for _, d := range []string{dir, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	err := Disk{}.Write(dir, []port.File{
		{Path: "link", Mode: fs.ModeSymlink | 0o777, Data: []byte(outside)},
		{Path: "link/escaped.txt", Mode: 0o644, Data: []byte("x")},
	})
	var through *Through
	if !errors.As(err, &through) || through.Link != "link" || through.Path != "link/escaped.txt" {
		t.Fatalf("Write = %v, not a Through naming the link and the file", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "escaped.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a file was written outside the folder, through the link: %v", err)
	}
}

// Clear runs only after a write fails half way, which no scenario can make
// happen.
func TestClearRemovesWhatAFailedWriteLeft(t *testing.T) {
	dir := t.TempDir()
	if err := (Disk{}).Write(dir, []port.File{{Path: "a/b.txt", Mode: 0o644}}); err != nil {
		t.Fatal(err)
	}
	Disk{}.Clear(dir, false)
	if got, _ := (Disk{}).Look(dir); got != port.Empty {
		t.Errorf("a folder cleared is %v", got)
	}
	Disk{}.Clear(dir, true)
	if got, _ := (Disk{}).Look(dir); got != port.Missing {
		t.Errorf("a folder made and cleared is %v", got)
	}
}
