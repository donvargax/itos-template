package newproject

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/problem"
	"github.com/donvargax/itos-template/internal/prompt"
)

func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "newproject tests")
	t.Setenv("GIT_COMMITTER_NAME", "newproject tests")
	t.Setenv("GIT_AUTHOR_EMAIL", "newproject@localhost")
	t.Setenv("GIT_COMMITTER_EMAIL", "newproject@localhost")
}

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

const manifestText = `version: 1
stacks:
  - name: sh
features:
  - name: extra
    stack: sh
questions:
  - name: name
    literal: acme-widget
    question: Name?
    pattern: "[a-z]+(-[a-z]+)*"
    case_forms: true
  - name: owner
    literal: Acme Corp
    question: Owner?
    default: Nobody
`

// newTemplate makes a template: main with the manifest and a CRLF file,
// stack/sh adding an executable script, sh/extra a file.
func newTemplate(t *testing.T) string {
	t.Helper()
	isolate(t)
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	run(t, dir, "config", "core.autocrlf", "false")
	writeFile(t, dir, "itos-template.yaml", manifestText)
	writeFile(t, dir, "NOTICE", "acme-widget by Acme Corp\r\nACME_WIDGET\r\n")
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "main")
	run(t, dir, "checkout", "-q", "-b", "stack/sh")
	writeFile(t, dir, "bin/acme-widget", "#!/bin/sh\necho acme_widget\n")
	run(t, dir, "add", "-A")
	run(t, dir, "update-index", "--chmod=+x", "bin/acme-widget")
	// The bit on disk too, so a later git add -A keeps it.
	if err := os.Chmod(filepath.Join(dir, "bin", "acme-widget"), 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "commit", "-q", "-m", "stack/sh")
	run(t, dir, "checkout", "-q", "-b", "sh/extra")
	writeFile(t, dir, "extra.txt", "extra\n")
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "sh/extra")
	run(t, dir, "checkout", "-q", "main")
	return dir
}

// answers is an Asker answering from a list, recording what it was asked.
type answers struct {
	given []string
	asked []string
}

func (a *answers) Ask(q prompt.Question) (string, error) {
	a.asked = append(a.asked, q.Text)
	if len(a.given) == 0 {
		return "", prompt.ErrNoAnswer
	}
	answer := a.given[0]
	a.given = a.given[1:]
	if answer == "" && q.HasDefault {
		answer = q.Default
	}
	if q.Check != nil {
		if err := q.Check(answer); err != nil {
			return "", err
		}
	}
	return answer, nil
}

func TestMakeAsksOnATerminalForWhatTheCommandLineLeftOut(t *testing.T) {
	tpl := newTemplate(t)
	folder := filepath.Join(t.TempDir(), "made")
	asker := &answers{given: []string{"sh", "blue-fox", ""}}
	result, err := Make(Options{Template: tpl, Folder: folder, Asker: asker})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(asker.asked, []string{"Which stack (sh)?", "Name?", "Owner?"}) {
		t.Errorf("asked %q", asker.asked)
	}
	if result.Answers["name"] != "blue-fox" || result.Answers["owner"] != "Nobody" {
		t.Errorf("answers %v", result.Answers)
	}
	data, err := os.ReadFile(filepath.Join(folder, "NOTICE"))
	if err != nil || string(data) != "blue-fox by Nobody\r\nBLUE_FOX\r\n" {
		t.Errorf("NOTICE is %q (%v): CRLF kept and the literals replaced, as written without case forms", data, err)
	}
}

// The commit records an executable as the template does, on every system:
// on windows, where the file system has no execute bit, too.
func TestMakeCommitsAnExecutableAsExecutable(t *testing.T) {
	tpl := newTemplate(t)
	folder := filepath.Join(t.TempDir(), "made")
	_, err := Make(Options{Template: tpl, Folder: folder, Stack: "sh", Features: []string{"extra"}, Answers: []string{"name=blue-fox"}, Defaults: true})
	if err != nil {
		t.Fatal(err)
	}
	modes := run(t, folder, "ls-tree", "-r", "HEAD")
	if !strings.Contains(modes, "100755 blob") || !strings.Contains(modes, "\tbin/blue-fox") {
		t.Errorf("the first commit's files:\n%s", modes)
	}
	for _, line := range strings.Split(modes, "\n") {
		if strings.HasSuffix(line, "\tbin/blue-fox") && !strings.HasPrefix(line, "100755") {
			t.Errorf("bin/blue-fox is not executable: %s", line)
		}
	}
	if status := run(t, folder, "status", "--porcelain"); status != "" {
		t.Errorf("the working tree has changes:\n%s", status)
	}
}

func TestMakeRefusesWithoutWritingWhenTheInputEnds(t *testing.T) {
	tpl := newTemplate(t)
	folder := filepath.Join(t.TempDir(), "made")
	_, err := Make(Options{Template: tpl, Folder: folder, Stack: "sh", Asker: &answers{}})
	var f *problem.Failure
	if !errors.As(err, &f) || f.Code != problem.CodeUsage || f.Problems[0].Rule != "answer-missing" {
		t.Fatalf("Make = %v", err)
	}
	if _, err := os.Stat(folder); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the folder was made: %v", err)
	}
}

func TestMakeRefusesATemplateWithNoManifest(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "README.md", "x\n")
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "main")
	_, err := Make(Options{Template: dir, Folder: filepath.Join(t.TempDir(), "made"), Stack: "sh"})
	var f *problem.Failure
	if !errors.As(err, &f) || f.Code != problem.CodeUsage || f.Problems[0].Rule != "manifest-missing" {
		t.Fatalf("Make = %v", err)
	}
}

func TestMakeRefusesAnAnswerThatMakesTwoFilesOne(t *testing.T) {
	tpl := newTemplate(t)
	writeFile(t, tpl, "acme-widget.txt", "one\n")
	writeFile(t, tpl, "blue-fox.txt", "two\n")
	run(t, tpl, "add", "-A")
	run(t, tpl, "commit", "-q", "-m", "two names")
	run(t, tpl, "checkout", "-q", "stack/sh")
	run(t, tpl, "merge", "-q", "--no-edit", "main")
	run(t, tpl, "checkout", "-q", "main")
	folder := filepath.Join(t.TempDir(), "made")
	_, err := Make(Options{Template: tpl, Folder: folder, Stack: "sh", Answers: []string{"name=blue-fox"}, Defaults: true})
	var f *problem.Failure
	if !errors.As(err, &f) || f.Code != problem.CodeUsage || f.Problems[0].Rule != "answer-name" {
		t.Fatalf("Make = %v", err)
	}
	if _, err := os.Stat(folder); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the folder was made: %v", err)
	}
}

// withManifest makes tpl's manifest on main text.
func withManifest(t *testing.T, tpl, text string) {
	t.Helper()
	writeFile(t, tpl, "itos-template.yaml", text)
	run(t, tpl, "commit", "-q", "-a", "-m", "manifest")
}

func TestMakeRefusesACombinationTheManifestListsAsUnsupported(t *testing.T) {
	tpl := newTemplate(t)
	withManifest(t, tpl, strings.Replace(manifestText, "version: 1\n", "version: 2\nunsupported:\n  - stack: sh\n    features: [extra]\n", 1))
	folder := filepath.Join(t.TempDir(), "made")
	_, err := Make(Options{Template: tpl, Folder: folder, Stack: "sh", Features: []string{"extra"}, Answers: []string{"name=blue-fox"}, Defaults: true})
	var f *problem.Failure
	if !errors.As(err, &f) || f.Code != problem.CodeRefused || f.Problems[0].Rule != "combination-unsupported" || !strings.Contains(f.Problems[0].Message, "sh + extra") {
		t.Fatalf("Make = %v", err)
	}
	if _, err := os.Stat(folder); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the folder was made: %v", err)
	}
	if _, err := Make(Options{Template: tpl, Folder: folder, Stack: "sh", Answers: []string{"name=blue-fox"}, Defaults: true}); err != nil {
		t.Errorf("the stack alone, which is supported: %v", err)
	}
}
