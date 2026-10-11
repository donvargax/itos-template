package release

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/donvargax/itos-template/internal/release/releasetest"
)

// tools/bin/release-version and release-notes read what moved beneath the
// binary through BuiltAt, and no scenario runs them, so what it reads of
// each system and how it copies a commit's tree are held here.

// A module only one system's files import is linked all the same: go list
// is run for each system the release builds for, never only the one the
// test runs on, so each of the three modules below counts on any of them.
func TestBuiltAtCountsAModuleOneSystemAloneLinks(t *testing.T) {
	repo := releasetest.Example(t, false)
	gomod := filepath.Join(repo, "go.mod")
	for _, goos := range systems {
		name := "example.com/on" + goos
		releasetest.Change(t, gomod, "\texample.com/lib v1.0.0\n", "\texample.com/lib v1.0.0\n\t"+name+" v1.0.0\n")
		releasetest.Change(t, gomod, "\texample.com/lib => ./lib\n", "\texample.com/lib => ./lib\n\t"+name+" => ./on"+goos+"\n")
		releasetest.Change(t, filepath.Join(repo, "on"+goos, "go.mod"), "", "module "+name+"\n\ngo 1.24\n")
		releasetest.Change(t, filepath.Join(repo, "on"+goos, "on.go"), "", "package on"+goos+"\n\nconst Name = \"on"+goos+"\"\n")
		releasetest.Change(t, filepath.Join(repo, "cmd", "itos-template", goos+".go"), "",
			"//go:build "+goos+"\n\npackage main\n\nimport \""+name+"\"\n\nvar _ = on"+goos+".Name\n")
	}
	releasetest.Commit(t, repo, "build: link a module on each system")
	t.Chdir(repo)
	b, err := BuiltAt("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	for _, goos := range systems {
		if v, ok := b.Modules["example.com/on"+goos]; !ok || v != "v1.0.0 => ./on"+goos {
			t.Errorf("the module only %s links is %q, %v, in %v", goos, v, ok, b.Modules)
		}
	}
}

// A tree that cannot be written where it is copied to is that failure, once
// git archive has written the rest of the tree and ended: left unread, a
// tree larger than the pipe holds would keep git writing into it for good.
func TestExtractGivesTheFailureToWriteATreeAndLetsGitEnd(t *testing.T) {
	repo := releasetest.Example(t, false)
	releasetest.Change(t, filepath.Join(repo, "big.bin"), "", strings.Repeat("x", 1<<20))
	releasetest.Commit(t, repo, "build: add a large file")
	t.Chdir(repo)
	notAFolder := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notAFolder, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- extract("HEAD", notAFolder) }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("extract into a file wrote the tree")
		}
	case <-time.After(time.Minute):
		t.Fatal("extract still waits on git a minute on")
	}
}

// A file whose contents cannot all be read is that failure, never a file
// written short.
func TestWriteGivesTheFailureToReadAFilesContents(t *testing.T) {
	broken := errors.New("the archive is cut short")
	if err := write(filepath.Join(t.TempDir(), "file"), iotest.ErrReader(broken), 0o644); !errors.Is(err, broken) {
		t.Errorf("write = %v, want the read's failure", err)
	}
}
