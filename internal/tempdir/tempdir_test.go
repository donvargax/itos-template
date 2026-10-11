package tempdir

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

// clock is a clock of the test's own for remove: how long it was asked to
// wait in all, never waiting. Asked a hundred times, it fails the test, as
// a Remove that never gives up would wait for good.
type clock struct {
	t      *testing.T
	waited time.Duration
	naps   int
}

func (c *clock) sleep(d time.Duration) {
	if c.naps++; c.naps > 100 {
		c.t.Fatalf("still waiting after %d waits, %v in all", c.naps, c.waited)
	}
	c.waited += d
}

// lasting is a folder no try removes: os.RemoveAll refuses a path ending in
// a dot on every system, as windows refuses a folder a program holds for
// longer than Remove waits.
func lasting(t *testing.T) string {
	return t.TempDir() + string(os.PathSeparator) + "."
}

// A folder that will not go is tried again five times, waiting 50 ms, then
// twice as long each time, 1.55 s in all, before its failure is returned;
// one that goes at once is not waited for.
func TestRemoveWaitsLongerEachTryThenGivesUp(t *testing.T) {
	c := &clock{t: t}
	if err := remove(lasting(t), c.sleep); err == nil {
		t.Error("remove gave no failure for a folder that cannot be removed")
	}
	if c.waited != 1550*time.Millisecond || c.naps != 5 {
		t.Errorf("remove waited %v in %d waits, not 1.55s in 5", c.waited, c.naps)
	}
	c = &clock{t: t}
	dir := t.TempDir()
	if err := remove(dir, c.sleep); err != nil || c.naps != 0 {
		t.Errorf("remove = %v, after %d waits, %v in all", err, c.naps, c.waited)
	}
	if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s is still there: %v", dir, err)
	}
}

// logged is what slog's default logger writes while the test runs, a
// JSON object a line.
func logged(t *testing.T) *strings.Builder {
	var log strings.Builder
	was := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(was) })
	return &log
}

// Discard and Renders remove a folder saying nothing, and log a folder
// they cannot remove, naming it, never returning the failure.
func TestDiscardAndRendersLogOnlyAFolderLeftBehind(t *testing.T) {
	for name, discard := range map[string]func(string){"Discard": Discard, "Renders": Renders{}.Remove} {
		log := logged(t)
		dir := t.TempDir()
		discard(dir)
		if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) || log.Len() != 0 {
			t.Errorf("%s of a folder it removes: %v, logged %q", name, err, log.String())
		}
		left := lasting(t)
		discard(left)
		var entry struct{ Level, Folder string }
		if err := json.Unmarshal([]byte(log.String()), &entry); err != nil || entry.Level != "WARN" || entry.Folder != left {
			t.Errorf("%s of a folder left behind logged %q: %v", name, log.String(), err)
		}
	}
}
