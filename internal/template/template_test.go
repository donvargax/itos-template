package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
)

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git.Run(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, dir, p, data string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newTemplate makes a template: main with the manifest and README.md,
// stack/sh adding bin/acme-widget. Its own commits are by someone, and no
// other git identity is set, as in a CI that configures none.
func newTemplate(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	c := func(args ...string) {
		t.Helper()
		g := git.Command{Dir: dir, Env: []string{"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@localhost", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@localhost"}}
		if _, err := g.Output(args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	c("init", "-q", "-b", "main")
	writeFile(t, dir, "itos-template.yaml", `version: 1
stacks:
  - name: sh
questions:
  - name: name
    literal: acme-widget
    question: Name?
    case_forms: true
`)
	writeFile(t, dir, "README.md", "acme-widget\n")
	c("add", "-A")
	c("commit", "-q", "-m", "main")
	c("checkout", "-q", "-b", "stack/sh")
	writeFile(t, dir, "bin/acme-widget", "echo acme_widget\n")
	c("add", "-A")
	c("commit", "-q", "-m", "stack/sh")
	c("checkout", "-q", "main")
	return dir
}

// A render no one keeps is committed as the identity given, so a git that
// knows no one can still render it.
func TestRenderCommitsAsTheIdentityGiven(t *testing.T) {
	tpl, err := Open(newTemplate(t))
	if err != nil {
		t.Fatal(err)
	}
	defer tpl.Close()
	stack, _ := tpl.Manifest.Stack("sh")
	folder := t.TempDir()
	identity := []string{"GIT_AUTHOR_NAME=check", "GIT_AUTHOR_EMAIL=check@localhost", "GIT_COMMITTER_NAME=check", "GIT_COMMITTER_EMAIL=check@localhost"}
	p, err := tpl.Render(manifest.Combination{Stack: stack}, map[string]string{"name": "blue-fox"}, folder, false, identity)
	if err != nil {
		t.Fatal(err)
	}
	if author := run(t, folder, "log", "-1", "--format=%an <%ae>"); author != "check <check@localhost>" {
		t.Errorf("the render's commit is by %s", author)
	}
	if files := run(t, folder, "ls-files"); files != project.RecordFile+"\nREADME.md\nbin/blue-fox" {
		t.Errorf("the render's files:\n%s", files)
	}
	if p.Commit != run(t, folder, "rev-parse", "HEAD") || p.Features == nil || len(p.Commits) != 2 {
		t.Errorf("the project is %+v", p)
	}
}
