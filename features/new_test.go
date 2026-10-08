// The steps of new.feature: the fixture templates a scenario names (built in
// templates_test.go), what a made project holds, and --json's failure
// object, parsed, never searched. A made project is judged by
// its files, its git history and its record, .itos-template.yaml, read as
// YAML: the files itos-template writes are part of its contract
// (docs/CLI.md).
package features

import (
	"bytes"
	"encoding/json"
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

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

func (w *world) newSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the template "([^"]*)"$`, w.theTemplate)
	sc.Step(`^the template "([^"]*)" whose question "([^"]*)" has the literal "([^"]*)"$`, w.templateWithLiteral)
	sc.Step(`^the template "([^"]*)" whose branch "([^"]*)" holds the files (".*")$`, w.templateWithFiles)
	sc.Step(`^the template "([^"]*)" whose manifest is acme's (.+)$`, w.templateWithManifest)
	sc.Step(`^the template "([^"]*)" whose branch "([^"]*)" holds a submodule at "([^"]*)"$`, w.templateWithSubmodule)
	sc.Step(`^the template "([^"]*)" whose root branch holds no itos-template\.yaml$`, w.templateWithoutManifest)
	sc.Step(`^the template "([^"]*)" whose manifest has the key "([^"]*)"$`, w.templateWithKey)
	sc.Step(`^the template "([^"]*)" whose manifest gives the first commit the message "([^"]*)" with the footer "([^"]*)"$`, w.templateWithFirstCommit)
	sc.Step(`^the template "([^"]*)" whose manifest gives the first commit the message "([^"]*)" with the body line "([^"]*)" and the footer "([^"]*)"$`, w.templateWithFirstCommitBody)
	sc.Step(`^the template "([^"]*)" whose branch "([^"]*)" is missing$`, w.templateWithoutBranch)
	sc.Step(`^the template "([^"]*)" whose branch "([^"]*)" holds the file "([^"]*)"$`, func(name, branch, file string) error {
		return w.templateWithFiles(name, branch, `"`+file+`"`)
	})
	sc.Step(`^the template "([^"]*)" whose branch "([^"]*)" holds (".*")$`, w.templateWithFiles)
	sc.Step(`^an empty git repository "([^"]*)"$`, func(dir string) error {
		if err := os.MkdirAll(w.path(dir), 0o755); err != nil {
			return err
		}
		return w.gitIn(w.path(dir), "init", "-q", "-b", "main")
	})
	sc.Step(`^a file "([^"]*)"$`, func(file string) error {
		return os.WriteFile(w.path(file), []byte("kept\n"), 0o644)
	})

	sc.Step(`^itos-template runs with no git on the PATH with "([^"]*)"$`, func(args string) error {
		return w.runWith(w.noGit(), args)
	})
	sc.Step(`^itos-template runs with git knowing no one with "([^"]*)"$`, func(args string) error {
		return w.runWith(w.knowingNoOne(), args)
	})
	sc.Step(`^itos-template runs with git's commit\.cleanup set to ([a-z-]+) with "([^"]*)"$`, func(mode, args string) error {
		return w.runWith(w.withGitConfig("commit.cleanup", mode), args)
	})
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
	sc.Step(`^its error output does not say "([^"]*)"$`, func(text string) error {
		if strings.Contains(w.stderr, w.expand(text)) {
			return fmt.Errorf("the error output says %q\n%s", text, w.report())
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
	sc.Step(`^its JSON output names the problems? (".*")$`, w.jsonNamesProblems)
	sc.Step(`^the first commit of "([^"]*)" records "([^"]*)" as (executable|not executable)$`, w.firstCommitRecordsMode)
	sc.Step(`^the first commit of "([^"]*)" is by "([^"]*)"$`, w.firstCommitIsBy)
	sc.Step(`^the first commit of "([^"]*)" has the header "([^"]*)"$`, w.firstCommitHasHeader)
	sc.Step(`^the first commit of "([^"]*)" has the footer "([^"]*)"$`, w.firstCommitHasFooter)
	sc.Step(`^the first commit of "([^"]*)" has the body line "([^"]*)"$`, w.firstCommitHasBodyLine)
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

// jsonNamesProblems reads standard output as --json's failure object
// (docs/CLI.md, rule 29), parsed, never searched: schema 1, ok false, and
// a problem of its own for each rule ID listed, a rule listed twice two
// problems. Other problems may come with them: an answer with no = is
// also an answer missing.
func (w *world) jsonNamesProblems(list string) error {
	var failure struct {
		Schema   int   `json:"schema"`
		OK       *bool `json:"ok"`
		Problems []struct {
			Rule    string `json:"rule"`
			Message string `json:"message"`
		} `json:"problems"`
	}
	dec := json.NewDecoder(strings.NewReader(w.stdout))
	if err := dec.Decode(&failure); err != nil {
		return fmt.Errorf("standard output is no JSON object: %v\n%s", err, w.report())
	}
	if dec.More() {
		return fmt.Errorf("standard output holds more than one JSON value\n%s", w.report())
	}
	if failure.Schema != 1 || failure.OK == nil || *failure.OK {
		return fmt.Errorf("the JSON output is not a failure of schema 1\n%s", w.report())
	}
	var rules []string
	for _, p := range failure.Problems {
		if p.Message == "" {
			return fmt.Errorf("the problem %s has no message\n%s", p.Rule, w.report())
		}
		rules = append(rules, p.Rule)
	}
	left := slices.Clone(rules)
	for _, want := range quotedList(list) {
		i := slices.Index(left, want)
		if i < 0 {
			return fmt.Errorf("the JSON output names the problems %q, not %q\n%s", rules, list, w.report())
		}
		left = slices.Delete(left, i, i+1)
	}
	return nil
}

// firstCommit is the first commit of the repository dir, the one with no
// parent.
func (w *world) firstCommit(dir string) (string, error) {
	out, err := w.gitOut(w.path(dir), "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return "", fmt.Errorf("%v\n%s", err, w.report())
	}
	if first := strings.Fields(out); len(first) == 1 {
		return first[0], nil
	}
	return "", fmt.Errorf("%s has no one first commit: %q", dir, out)
}

// firstCommitRecordsMode reads the mode the first commit of dir records for
// file (with /), as git's tree holds it, whatever the file system says:
// 100755 is executable, 100644 not.
func (w *world) firstCommitRecordsMode(dir, file, mode string) error {
	first, err := w.firstCommit(dir)
	if err != nil {
		return err
	}
	out, err := w.gitOut(w.path(dir), "ls-tree", "-r", first, "--", file)
	if err != nil {
		return err
	}
	got, _, _ := strings.Cut(out, " ")
	want := map[string]string{"executable": "100755", "not executable": "100644"}[mode]
	if got != want {
		return fmt.Errorf("the first commit of %s records %s as %q, not %s\n%s", dir, file, out, want, w.report())
	}
	return nil
}

// firstCommitIsBy is whether the first commit of dir names who as its author
// and its committer.
func (w *world) firstCommitIsBy(dir, who string) error {
	first, err := w.firstCommit(dir)
	if err != nil {
		return err
	}
	out, err := w.gitOut(w.path(dir), "log", "-1", "--format=%an%n%cn", first)
	if err != nil {
		return err
	}
	if names := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n"); !slices.Equal(names, []string{who, who}) {
		return fmt.Errorf("the first commit of %s is by %q, not %s", dir, names, who)
	}
	return nil
}

// firstCommitHasHeader is whether the first line of the first commit of
// dir's message is header, as a commit lint reads a commit's header.
func (w *world) firstCommitHasHeader(dir, header string) error {
	first, err := w.firstCommit(dir)
	if err != nil {
		return err
	}
	out, err := w.gitOut(w.path(dir), "log", "-1", "--format=%B", first)
	if err != nil {
		return err
	}
	if got, _, _ := strings.Cut(strings.ReplaceAll(out, "\r\n", "\n"), "\n"); got != header {
		return fmt.Errorf("the first commit of %s has the header %q, not %q\n%s", dir, got, header, out)
	}
	return nil
}

// firstCommitHasBodyLine is whether line is a line of the first commit of
// dir's message after its header, exactly as written there.
func (w *world) firstCommitHasBodyLine(dir, line string) error {
	first, err := w.firstCommit(dir)
	if err != nil {
		return err
	}
	out, err := w.gitOut(w.path(dir), "log", "-1", "--format=%B", first)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	if !slices.Contains(lines[1:], line) {
		return fmt.Errorf("the first commit of %s has no body line %q: its message is\n%s", dir, line, out)
	}
	return nil
}

// firstCommitHasFooter is whether footer is one of the first commit of dir's
// footers, as git reads a message's trailers: the lines of its last
// paragraph, each a token, a colon and a value.
func (w *world) firstCommitHasFooter(dir, footer string) error {
	first, err := w.firstCommit(dir)
	if err != nil {
		return err
	}
	out, err := w.gitOut(w.path(dir), "log", "-1", "--format=%(trailers:only,unfold)", first)
	if err != nil {
		return err
	}
	if !slices.Contains(strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n"), footer) {
		body, _ := w.gitOut(w.path(dir), "log", "-1", "--format=%B", first)
		return fmt.Errorf("the first commit of %s has no footer %q: its footers are %q\n%s", dir, footer, out, body)
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
