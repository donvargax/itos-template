package tempdir

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Removing a folder as windows refuses to is held here: the scenarios run
// on windows, but none can make a folder windows will not remove at once,
// nor see a temporary folder left behind (decision 18).

// A folder holding read-only files and folders, as git leaves its objects,
// is removed whole.
func TestRemoveRemovesReadOnlyFiles(t *testing.T) {
	dir, err := Make("itos-template-tempdir-test-")
	if err != nil {
		t.Fatal(err)
	}
	objects := filepath.Join(dir, "repo.git", "objects", "ab")
	if err := os.MkdirAll(objects, 0o755); err != nil {
		t.Fatal(err)
	}
	object := filepath.Join(objects, "cdef")
	if err := os.WriteFile(object, []byte("blob"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(objects, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s is still there: %v", dir, err)
	}
}

func TestRemoveTakesAFolderThatIsGone(t *testing.T) {
	if err := Remove(filepath.Join(t.TempDir(), "gone")); err != nil {
		t.Errorf("Remove: %v", err)
	}
}

// writable leaves every file and folder writable by its owner, read-only as
// git made them: on windows, by clearing the read-only attribute.
func TestWritableMakesEveryEntryOwnerWritable(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "objects")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(folder, "cdef")
	if err := os.WriteFile(file, []byte("blob"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	writable(dir)
	for _, p := range []string{dir, folder, file} {
		info, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o200 == 0 {
			t.Errorf("%s is %v, not writable by its owner", p, info.Mode().Perm())
		}
	}
}

// On unix, writable gives a folder its owner's every right and a file its
// owner's read and write, keeping the rest of its mode: an executable stays
// executable. Windows keeps no such bits, only the read-only attribute.
func TestWritableKeepsAFilesModeAndOpensFolders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows keeps no mode bits but the read-only attribute")
	}
	dir := t.TempDir()
	folder := filepath.Join(dir, "hooks")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(folder, "pre-commit")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	object := filepath.Join(dir, "object")
	if err := os.WriteFile(object, []byte("blob"), 0o644); err != nil {
		t.Fatal(err)
	}
	for p, mode := range map[string]os.FileMode{script: 0o555, object: 0o444, folder: 0o555, dir: 0o555} {
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
	}
	writable(dir)
	for p, want := range map[string]os.FileMode{dir: 0o700, folder: 0o700, script: 0o755, object: 0o644} {
		info, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s is %v, want %v", p, got, want)
		}
	}
}

// writable skips a symlink: a chmod through it would change what it points
// to, here a file it has made writable already.
func TestWritableSkipsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows keeps no mode bits but the read-only attribute")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "a")
	if err := os.WriteFile(target, []byte("blob"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a", filepath.Join(dir, "z")); err != nil {
		t.Fatal(err)
	}
	writable(dir)
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("%s is %v, want %v", target, got, os.FileMode(0o644))
	}
}
