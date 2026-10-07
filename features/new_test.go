// The steps of new.feature: the fixture templates, built as git repositories
// from testdata, and what a made project holds. A made project is judged by
// its files, its git history and its record, .itos-template.yaml, read as
// YAML: the files itos-template writes are part of its contract
// (docs/CLI.md).
package features

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

func (w *world) newSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the template "([^"]*)"$`, w.theTemplate)
	sc.Step(`^the template "([^"]*)" whose question "([^"]*)" has the literal "([^"]*)"$`, w.templateWithLiteral)
	sc.Step(`^the template "([^"]*)" whose branch "([^"]*)" holds the files (".*")$`, w.templateWithFiles)
	sc.Step(`^an empty folder "([^"]*)"$`, func(dir string) error {
		return os.MkdirAll(w.path(dir), 0o755)
	})
	sc.Step(`^a folder "([^"]*)" holding the file "([^"]*)"$`, func(dir, file string) error {
		if err := os.MkdirAll(w.path(dir), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(w.path(dir), file), []byte("kept\n"), 0o644)
	})

	sc.Step(`^its error output says "([^"]*)"$`, func(text string) error {
		if !strings.Contains(w.stderr, w.expand(text)) {
			return fmt.Errorf("the error output does not say %q\n%s", text, w.report())
		}
		return nil
	})
	sc.Step(`^the file "([^"]*)" exists$`, func(file string) error {
		info, err := os.Lstat(w.path(file))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("no file %s\n%s", file, w.report())
		}
		return nil
	})
	sc.Step(`^the path "([^"]*)" does not exist$`, func(path string) error {
		if _, err := os.Lstat(w.path(path)); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s exists (%v)\n%s", path, err, w.report())
		}
		return nil
	})
	sc.Step(`^the file "([^"]*)" contains (".*")$`, w.fileContains)
	sc.Step(`^no text file of the project in "([^"]*)" contains (".*")$`, w.noTextFileContains)
	sc.Step(`^the file "([^"]*)" is the same as the template's "([^"]*)"$`, w.sameAsTemplates)
	sc.Step(`^the folder "([^"]*)" holds only "([^"]*)"$`, w.holdsOnly)
	sc.Step(`^the folder "([^"]*)" is a git repository with exactly (\d+) commits?$`, w.repositoryWithCommits)
	sc.Step(`^the working tree of "([^"]*)" has no changes$`, w.noChanges)

	sc.Step(`^the record in "([^"]*)" names the template as "([^"]*)"$`, w.recordNamesTemplate)
	sc.Step(`^the record in "([^"]*)" names the stack "([^"]*)" and the features (".*")$`, w.recordNamesStack)
	sc.Step(`^the record in "([^"]*)" has the answer (".*")$`, w.recordHasAnswers)
	sc.Step(`^the record in "([^"]*)" names the commit of the template's branches (".*")$`, w.recordNamesCommits)
}

// path is a step's folder or file, relative to the scenario's scratch folder
// and written with /, as a path of this system.
func (w *world) path(p string) string {
	return filepath.Join(w.dir, filepath.FromSlash(p))
}

// expand replaces {template} with the fixture template's path, written with
// / on every system, as git takes it in a path and after file://.
func (w *world) expand(s string) string {
	return strings.ReplaceAll(s, "{template}", w.template)
}

// quoted reads a step's list of quoted strings: "a", "b" and "c", or "a" or
// "b".
var quoted = regexp.MustCompile(`"([^"]*)"`)

func quotedList(list string) []string {
	var found []string
	for _, m := range quoted.FindAllStringSubmatch(list, -1) {
		found = append(found, m[1])
	}
	return found
}

// The fixture templates.

var (
	templatesMu  sync.Mutex
	templates    = map[string]string{}
	templatesDir string
)

// removeTemplates removes the fixture templates the run built.
func removeTemplates() {
	if templatesDir != "" {
		_ = os.RemoveAll(templatesDir)
	}
}

// theTemplate builds the fixture template name from testdata, once a run:
// no command a scenario runs changes it, as new and check only clone it.
func (w *world) theTemplate(name string) error {
	return w.useTemplate(name, func(dir string) error {
		return w.buildTemplate(filepath.Join(w.root, "features", "testdata", name), dir)
	})
}

// templateWithLiteral is the fixture template name, its manifest on the root
// branch giving the question named question the literal literal, the rest
// of the question as the fixture has it.
func (w *world) templateWithLiteral(name, question, literal string) error {
	key := fmt.Sprintf("%s whose question %s has the literal %q", name, question, literal)
	return w.changedTemplate(name, key, func(top *yaml.Node) error {
		questions := mappingValue(top, "questions")
		if questions == nil || questions.Kind != yaml.SequenceNode {
			return errors.New("the manifest lists no questions")
		}
		for _, q := range questions.Content {
			if scalarValue(q, "name") != question {
				continue
			}
			value := mappingValue(q, "literal")
			if value == nil {
				return fmt.Errorf("the question %s has no literal", question)
			}
			*value = *scalar(literal)
			return nil
		}
		return fmt.Errorf("the manifest has no question %s", question)
	})
}

// templateWithFiles is the fixture template name with one more commit on its
// branch branch, adding the files listed (with /), each holding its own path
// and a line ending. The root branch is checked out again after, so it stays
// the template's default branch.
func (w *world) templateWithFiles(name, branch, files string) error {
	list := quotedList(files)
	key := fmt.Sprintf("%s whose branch %s holds the files %q", name, branch, list)
	return w.useTemplate(key, func(dir string) error {
		if err := w.buildTemplate(filepath.Join(w.root, "features", "testdata", name), dir); err != nil {
			return err
		}
		root, err := w.gitOut(dir, "symbolic-ref", "--short", "HEAD")
		if err != nil {
			return err
		}
		if err := w.gitIn(dir, "checkout", "-q", branch); err != nil {
			return err
		}
		for _, file := range list {
			p := filepath.Join(dir, filepath.FromSlash(file))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(p, []byte(file+"\n"), 0o644); err != nil {
				return err
			}
			if err := w.gitIn(dir, "add", "--", file); err != nil {
				return err
			}
		}
		if err := w.gitIn(dir, "commit", "-q", "-m", "Add the files: "+key); err != nil {
			return err
		}
		return w.gitIn(dir, "checkout", "-q", root)
	})
}

// useTemplate makes the template the scenario's: the one built for key, or
// one build makes in a new folder, once a run for each key.
func (w *world) useTemplate(key string, build func(dir string) error) error {
	templatesMu.Lock()
	defer templatesMu.Unlock()
	dir, ok := templates[key]
	if !ok {
		if templatesDir == "" {
			var err error
			if templatesDir, err = os.MkdirTemp("", "itos-template-features-templates-"); err != nil {
				return err
			}
		}
		dir = filepath.Join(templatesDir, strconv.Itoa(len(templates)))
		if err := build(dir); err != nil {
			return fmt.Errorf("building the template %s: %w", key, err)
		}
		templates[key] = dir
	}
	w.templateDir = dir
	w.template = filepath.ToSlash(dir)
	return nil
}

// buildTemplate makes the git repository dir from the testdata folder src:
// its branches.txt lists each branch and the one it starts from, and each
// branch's files are the folder of its name, laid over what it starts from.
// The root branch is checked out at the end, so it is the template's
// default branch.
func (w *world) buildTemplate(src, dir string) error {
	list, err := os.ReadFile(filepath.Join(src, "branches.txt"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	root := ""
	lines := bufio.NewScanner(bytes.NewReader(list))
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		branch := fields[0]
		if root == "" {
			root = branch
			if err := w.gitIn(dir, "init", "-q", "-b", branch); err != nil {
				return err
			}
		} else if err := w.gitIn(dir, "checkout", "-q", "-b", branch, fields[1]); err != nil {
			return err
		}
		if err := os.CopyFS(dir, os.DirFS(filepath.Join(src, filepath.FromSlash(branch)))); err != nil {
			return err
		}
		if err := w.gitIn(dir, "add", "-A"); err != nil {
			return err
		}
		if err := w.gitIn(dir, "commit", "-q", "-m", "Make "+branch); err != nil {
			return err
		}
	}
	if err := lines.Err(); err != nil {
		return err
	}
	return w.gitIn(dir, "checkout", "-q", root)
}

// Then steps.

func (w *world) fileContains(file, list string) error {
	data, err := os.ReadFile(w.path(file))
	if err != nil {
		return fmt.Errorf("%v\n%s", err, w.report())
	}
	for _, text := range quotedList(list) {
		if !bytes.Contains(data, []byte(text)) {
			return fmt.Errorf("%s does not contain %q:\n%s", file, text, data)
		}
	}
	return nil
}

// isText is whether data is a text file's contents, as git tells them: no
// NUL in its first 8000 bytes.
func isText(data []byte) bool {
	return !bytes.Contains(data[:min(len(data), 8000)], []byte{0})
}

// noTextFileContains walks the project in dir, its .git left out, and fails
// on a text file holding any of the strings.
func (w *world) noTextFileContains(dir, list string) error {
	texts := quotedList(list)
	var found []string
	err := filepath.WalkDir(w.path(dir), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil || !isText(data) {
			return err
		}
		for _, text := range texts {
			if bytes.Contains(data, []byte(text)) {
				rel, _ := filepath.Rel(w.dir, p)
				found = append(found, fmt.Sprintf("%s holds %q", filepath.ToSlash(rel), text))
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%v\n%s", err, w.report())
	}
	if len(found) > 0 {
		return errors.New(strings.Join(found, "\n"))
	}
	return nil
}

func (w *world) sameAsTemplates(file, templateFile string) error {
	if w.templateDir == "" {
		return errors.New("no template in this scenario")
	}
	got, err := os.ReadFile(w.path(file))
	if err != nil {
		return fmt.Errorf("%v\n%s", err, w.report())
	}
	want, err := os.ReadFile(filepath.Join(w.templateDir, filepath.FromSlash(templateFile)))
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("%s is %q, the template's %s %q", file, got, templateFile, want)
	}
	return nil
}

func (w *world) holdsOnly(dir, list string) error {
	entries, err := os.ReadDir(w.path(dir))
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := quotedList(`"` + list + `"`)
	slices.Sort(want)
	if !slices.Equal(names, want) {
		return fmt.Errorf("%s holds %q, not %q", dir, names, want)
	}
	return nil
}

// gitOut is git's standard output, run in dir.
func (w *world) gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command(gitBin(), args...)
	cmd.Dir = dir
	cmd.Env = w.env()
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(string(out)), nil
}

// repositoryWithCommits is whether dir is the top of a git repository of
// its own, not a folder of the scratch repository around it, whose HEAD has
// count commits.
func (w *world) repositoryWithCommits(dir string, count int) error {
	top, err := w.gitOut(w.path(dir), "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("%v\n%s", err, w.report())
	}
	a, errA := os.Stat(filepath.FromSlash(top))
	b, errB := os.Stat(w.path(dir))
	if errA != nil || errB != nil || !os.SameFile(a, b) {
		return fmt.Errorf("%s is not the top of a git repository: its top is %s\n%s", dir, top, w.report())
	}
	out, err := w.gitOut(w.path(dir), "rev-list", "--count", "HEAD")
	if err != nil {
		return err
	}
	if n, _ := strconv.Atoi(out); n != count {
		return fmt.Errorf("%s has %s commits, not %d", dir, out, count)
	}
	return nil
}

func (w *world) noChanges(dir string) error {
	out, err := w.gitOut(w.path(dir), "status", "--porcelain", "--untracked-files=all", "--ignored")
	if err != nil {
		return err
	}
	if out != "" {
		return fmt.Errorf("the working tree of %s has changes:\n%s", dir, out)
	}
	return nil
}

// The record.

// record is a made project's .itos-template.yaml, as far as the steps read
// it.
type record struct {
	Template string            `yaml:"template"`
	Stack    string            `yaml:"stack"`
	Features []string          `yaml:"features"`
	Answers  map[string]string `yaml:"answers"`
	Commits  map[string]string `yaml:"commits"`
}

func (w *world) record(dir string) (*record, error) {
	f, err := os.Open(filepath.Join(w.path(dir), ".itos-template.yaml"))
	if err != nil {
		return nil, fmt.Errorf("%v\n%s", err, w.report())
	}
	defer func() { _ = f.Close() }()
	var r record
	if err := yaml.NewDecoder(f).Decode(&r); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading the record: %w", err)
	}
	return &r, nil
}

func (w *world) recordNamesTemplate(dir, template string) error {
	r, err := w.record(dir)
	if err != nil {
		return err
	}
	if want := w.expand(template); r.Template != want {
		return fmt.Errorf("the record names the template %q, not %q", r.Template, want)
	}
	return nil
}

func (w *world) recordNamesStack(dir, stack, features string) error {
	r, err := w.record(dir)
	if err != nil {
		return err
	}
	if r.Stack != stack {
		return fmt.Errorf("the record names the stack %q, not %q", r.Stack, stack)
	}
	if want := quotedList(features); !slices.Equal(r.Features, want) {
		return fmt.Errorf("the record names the features %q, not %q", r.Features, want)
	}
	return nil
}

// recordHasAnswers reads "key" as "value" pairs, joined by "and" or commas.
func (w *world) recordHasAnswers(dir, pairs string) error {
	r, err := w.record(dir)
	if err != nil {
		return err
	}
	list := quotedList(pairs)
	if len(list)%2 != 0 {
		return fmt.Errorf("the step's answers are not pairs: %s", pairs)
	}
	for i := 0; i < len(list); i += 2 {
		if got, ok := r.Answers[list[i]]; !ok || got != list[i+1] {
			return fmt.Errorf("the record has the answer %s as %q, not %q", list[i], got, list[i+1])
		}
	}
	return nil
}

// recordNamesCommits is whether the record names, for each branch, the
// commit the fixture template's branch is at, and no other branch.
func (w *world) recordNamesCommits(dir, branches string) error {
	r, err := w.record(dir)
	if err != nil {
		return err
	}
	want := map[string]string{}
	for _, branch := range quotedList(branches) {
		sha, err := w.gitOut(w.templateDir, "rev-parse", "refs/heads/"+branch)
		if err != nil {
			return err
		}
		want[branch] = sha
	}
	if len(r.Commits) != len(want) {
		return fmt.Errorf("the record names the commits %v, not %v", r.Commits, want)
	}
	for branch, sha := range want {
		if r.Commits[branch] != sha {
			return fmt.Errorf("the record names %s at %q, not %s", branch, r.Commits[branch], sha)
		}
	}
	return nil
}
