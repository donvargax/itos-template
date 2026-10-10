// The steps that read what a made project keeps of where it came from: its
// record, .itos-template.yaml, read as YAML, and its history, searched for
// a text it must never hold (record-name).
package features

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

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

// recordNamesTemplate is whether the record names the template exactly as
// template, expanded. {template} there is the fixture's path written with /,
// as a scenario gives it to new, or as the system writes it: a relative path
// new resolves is written with the system's own separators, \ on windows.
func (w *world) recordNamesTemplate(dir, template string) error {
	r, err := w.record(dir)
	if err != nil {
		return err
	}
	want := w.expand(template)
	native := w.expand(strings.ReplaceAll(template, "{template}", w.templateDir))
	if r.Template != want && r.Template != native {
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

// noFileOrCommitHolds is whether nothing the repository dir keeps holds
// text: no file tracked at HEAD, no commit's message, on any ref, and not
// its .git/config, where a clone would keep its origin's URL. It names each
// place it finds text.
func (w *world) noFileOrCommitHolds(dir, text string) error {
	repo := w.path(dir)
	files, err := w.gitOut(repo, "ls-tree", "-r", "--name-only", "-z", "HEAD")
	if err != nil {
		return fmt.Errorf("%v\n%s", err, w.report())
	}
	var found []string
	for _, file := range strings.Split(files, "\x00") {
		if file == "" {
			continue
		}
		data, err := w.gitOut(repo, "cat-file", "blob", "HEAD:"+file)
		if err != nil {
			return err
		}
		if strings.Contains(data, text) {
			found = append(found, "the file "+file)
		}
	}
	commits, err := w.gitOut(repo, "rev-list", "--all")
	if err != nil {
		return err
	}
	for _, commit := range strings.Fields(commits) {
		message, err := w.gitOut(repo, "log", "-1", "--format=%B", commit)
		if err != nil {
			return err
		}
		if strings.Contains(message, text) {
			found = append(found, "the message of the commit "+commit)
		}
	}
	config, err := os.ReadFile(filepath.Join(repo, ".git", "config"))
	if err != nil {
		return err
	}
	if strings.Contains(string(config), text) {
		found = append(found, ".git/config")
	}
	if len(found) > 0 {
		return fmt.Errorf("%s holds %q in %s", dir, text, strings.Join(found, ", "))
	}
	return nil
}
