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

// Decision 18 leaves infra to the scenarios, and they hold the files a
// project is made of; none yet reads a file's execute bit, a symbolic
// link, or a write that fails half way, so those stay here until the idea
// scenario-gaps gives them scenarios.

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

func TestLookAndClear(t *testing.T) {
	dir := t.TempDir()
	if got, err := (Disk{}).Look(filepath.Join(dir, "made")); got != port.Missing || err != nil {
		t.Errorf("a missing folder is %v, %v", got, err)
	}
	if got, err := (Disk{}).Look(dir); got != port.Empty || err != nil {
		t.Errorf("an empty folder is %v, %v", got, err)
	}
	if err := (Disk{}).Write(dir, []port.File{{Path: "a/b.txt", Mode: 0o644}}); err != nil {
		t.Fatal(err)
	}
	if got, err := (Disk{}).Look(dir); got != port.Full || err != nil {
		t.Errorf("a full folder is %v, %v", got, err)
	}
	if got, err := (Disk{}).Look(filepath.Join(dir, "a", "b.txt")); got != port.NotFolder || err != nil {
		t.Errorf("a file is %v, %v", got, err)
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
