package git

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsItosFollowsLinksToAFileNamedItos(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need a privilege on windows")
	}
	dir := t.TempDir()
	itos := filepath.Join(dir, "itos")
	if err := os.WriteFile(itos, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(dir, "git")
	if err := os.Symlink(itos, shim); err != nil {
		t.Fatal(err)
	}
	again := filepath.Join(dir, "git2")
	if err := os.Symlink("git", again); err != nil {
		t.Fatal(err)
	}
	if !IsItos(shim) || !IsItos(again) {
		t.Error("a link to itos, directly or through another link, is not read as an itos")
	}
	if IsItos(itos) {
		t.Error("a script named itos, not a Go build of itos, is read as one")
	}
}

func TestErrorSaysGitsLastLine(t *testing.T) {
	e := &Error{Args: []string{"clone"}, Code: 128, Stderr: "Cloning into 'x'...\nfatal: repository 'x' does not exist\n\n"}
	if got := e.Error(); got != "fatal: repository 'x' does not exist" {
		t.Errorf("Error() = %q", got)
	}
	if Code(e) != 128 || Code(os.ErrNotExist) != -1 {
		t.Error("Code does not read the exit code")
	}
}

func TestOutputRunsGitInItsFolder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_DIR", filepath.Join(dir, "elsewhere"))
	if _, err := Run(dir, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Errorf("git ran on the caller's GIT_DIR, not in its folder: %v", err)
	}
	_, err := Run(dir, "rev-parse", "--verify", "nosuch")
	if Code(err) <= 0 {
		t.Errorf("a failing git command gave %v", err)
	}
}
