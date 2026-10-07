// Package manifest reads a template's manifest, itos-template.yaml at the
// top of its root branch and merged down into every branch (decision 8):
// its stacks, its features, the questions whose answers replace its
// literals, and the paths only the template keeps. docs/manifest.md is its
// format, for template authors.
//
// A stack's branch is stack/<stack> and a feature's <stack>/<feature>, so the
// manifest names branches by convention alone. Unknown keys are refused, so
// a typo never passes for an option, and the keys later items add (setup
// steps, checks) come with a version that names them.
package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos-template/internal/caseform"
)

// File is the manifest's name at the top of a template's branches.
const File = "itos-template.yaml"

// Version is the manifest version this itos-template reads.
const Version = 1

// Manifest is a template's itos-template.yaml.
type Manifest struct {
	Version      int        `yaml:"version"`
	Stacks       []Stack    `yaml:"stacks"`
	Features     []Feature  `yaml:"features"`
	Questions    []Question `yaml:"questions"`
	TemplateOnly []string   `yaml:"template_only"`
}

// Stack is a stack: the root and a working project in one language or
// framework, on the branch stack/<name>.
type Stack struct {
	Name string `yaml:"name"`
}

// Branch is the stack's branch, stack/<name>.
func (s Stack) Branch() string { return "stack/" + s.Name }

// Feature is an optional part of a stack, on the branch <stack>/<name>,
// branched off its stack or off the features it needs.
type Feature struct {
	Name  string   `yaml:"name"`
	Stack string   `yaml:"stack"`
	Needs []string `yaml:"needs"`
}

// Branch is the feature's branch, <stack>/<name>.
func (f Feature) Branch() string { return f.Stack + "/" + f.Name }

// Question is a literal of the template and the question whose answer
// replaces it.
type Question struct {
	Name      string  `yaml:"name"`
	Literal   string  `yaml:"literal"`
	Question  string  `yaml:"question"`
	Pattern   string  `yaml:"pattern"`
	Default   *string `yaml:"default"`
	CaseForms bool    `yaml:"case_forms"`

	pattern *regexp.Regexp
	words   caseform.Words
}

// Error is a manifest that cannot be used, every problem found in it.
type Error struct{ Problems []string }

func (e *Error) Error() string { return strings.Join(e.Problems, "; ") }

// Parse reads and checks a manifest.
func Parse(data []byte) (*Manifest, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, &Error{[]string{"it is empty"}}
		}
		return nil, &Error{[]string{err.Error()}}
	}
	if problems := m.check(); len(problems) > 0 {
		return nil, &Error{problems}
	}
	return &m, nil
}

var (
	branchName   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	questionName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

func (m *Manifest) check() []string {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	if m.Version != Version {
		add("version is %d: this itos-template reads version %d", m.Version, Version)
	}
	if len(m.Stacks) == 0 {
		add("it lists no stack")
	}
	stacks := map[string]bool{}
	for _, s := range m.Stacks {
		switch {
		case !branchName.MatchString(s.Name):
			add("the stack %q is not a name: lowercase letters, digits, dots, dashes and underscores", s.Name)
		case stacks[s.Name]:
			add("the stack %s is listed twice", s.Name)
		}
		stacks[s.Name] = true
	}
	features := map[string]bool{}
	for _, f := range m.Features {
		switch {
		case !branchName.MatchString(f.Name):
			add("the feature %q is not a name: lowercase letters, digits, dots, dashes and underscores", f.Name)
		case !stacks[f.Stack]:
			add("the feature %s names the stack %q, which it does not list", f.Name, f.Stack)
		case features[f.Branch()]:
			add("the feature %s is listed twice", f.Branch())
		}
		features[f.Branch()] = true
	}
	for _, f := range m.Features {
		for _, need := range f.Needs {
			if need == f.Name || !features[f.Stack+"/"+need] {
				add("the feature %s needs %q, which is no other feature of the stack %s", f.Branch(), need, f.Stack)
			}
		}
	}
	names := map[string]bool{}
	literals := map[string]string{}
	for i := range m.Questions {
		q := &m.Questions[i]
		switch {
		case !questionName.MatchString(q.Name):
			add("the question %q is not a name: a lowercase letter, then lowercase letters, digits, dashes and underscores", q.Name)
		case names[q.Name]:
			add("the question %s is listed twice", q.Name)
		}
		names[q.Name] = true
		if q.Question == "" {
			add("the question %s has no question text", q.Name)
		}
		if q.Literal == "" {
			add("the question %s has no literal", q.Name)
			continue
		}
		var err error
		if q.Pattern != "" {
			if q.pattern, err = regexp.Compile(`^(?:` + q.Pattern + `)$`); err != nil {
				add("the question %s's pattern does not compile: %v", q.Name, err)
			}
		}
		forms := []string{q.Literal}
		if q.CaseForms {
			if q.words, err = caseform.Parse(q.Literal); err != nil {
				add("the question %s has case forms, so its literal is lowercase words joined by dashes: %v", q.Name, err)
				continue
			}
			if len(q.words) < 2 {
				add("the question %s has case forms, so its literal needs two words or more, to make five different forms", q.Name)
				continue
			}
			forms = q.words.Forms()
		}
		for _, form := range forms {
			if other, ok := literals[form]; ok && other != q.Name {
				add("the questions %s and %s both replace %q", other, q.Name, form)
			}
			literals[form] = q.Name
		}
		if q.Default != nil && (q.pattern != nil || q.Pattern == "") {
			if err := q.Check(*q.Default); err != nil {
				add("the question %s's default %q is not an answer it takes: %v", q.Name, *q.Default, err)
			}
		}
	}
	for _, p := range m.TemplateOnly {
		if p == "" || path.IsAbs(p) || strings.Contains(p, `\`) || path.Clean(p) != p || p == "." || strings.HasPrefix(p, "../") || p == ".." {
			add("the template_only path %q is not a path in the template: write it relative to its top, with /", p)
		}
	}
	return problems
}

// Check is whether answer is an answer q takes: never empty, the pattern
// matched whole when there is one, and with case forms, lowercase words
// joined by dashes.
func (q *Question) Check(answer string) error {
	if answer == "" {
		return fmt.Errorf("it is empty")
	}
	if q.pattern != nil && !q.pattern.MatchString(answer) {
		return fmt.Errorf("it does not match the pattern %s", q.Pattern)
	}
	if q.CaseForms {
		if _, err := caseform.Parse(answer); err != nil {
			return err
		}
	}
	return nil
}

// Question is the question named name.
func (m *Manifest) Question(name string) (*Question, bool) {
	for i := range m.Questions {
		if m.Questions[i].Name == name {
			return &m.Questions[i], true
		}
	}
	return nil, false
}

// QuestionNames are the questions' names, in the manifest's order.
func (m *Manifest) QuestionNames() []string {
	var names []string
	for _, q := range m.Questions {
		names = append(names, q.Name)
	}
	return names
}

// Stack is the stack named name.
func (m *Manifest) Stack(name string) (Stack, bool) {
	for _, s := range m.Stacks {
		if s.Name == name {
			return s, true
		}
	}
	return Stack{}, false
}

// StackNames are the stacks' names, in the manifest's order.
func (m *Manifest) StackNames() []string {
	var names []string
	for _, s := range m.Stacks {
		names = append(names, s.Name)
	}
	return names
}

// FeaturesNamed are the features a command line's name can mean: the
// feature whose branch it is (python/cli), or every feature of that name in
// any stack (cli), the stack's own first.
func (m *Manifest) FeaturesNamed(name, stack string) []Feature {
	var found []Feature
	for _, f := range m.Features {
		if f.Branch() == name {
			return []Feature{f}
		}
		if f.Name == name && !strings.Contains(name, "/") {
			found = append(found, f)
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].Stack == stack && found[j].Stack != stack })
	return found
}

// FeatureNames are the names of stack's features, in the manifest's order.
func (m *Manifest) FeatureNames(stack string) []string {
	var names []string
	for _, f := range m.Features {
		if f.Stack == stack {
			names = append(names, f.Name)
		}
	}
	return names
}

// Ordered are the features chosen, each once, in the manifest's order: the
// order they are merged in, so the order of the command line never changes a
// render.
func (m *Manifest) Ordered(chosen []Feature) []Feature {
	var ordered []Feature
	for _, f := range m.Features {
		if slices.ContainsFunc(chosen, func(c Feature) bool { return c.Branch() == f.Branch() }) {
			ordered = append(ordered, f)
		}
	}
	return ordered
}

// Replacements are the pairs a render replaces, each literal's form and the
// answer's form in it, the longest literal first, so where one literal
// holds another the longer is replaced whole (strings.NewReplacer takes the
// first pair that matches at a position). answers must hold an answer to
// every question, each one Check took.
func (m *Manifest) Replacements(answers map[string]string) []string {
	type pair struct{ old, new string }
	var pairs []pair
	for _, q := range m.Questions {
		answer := answers[q.Name]
		if !q.CaseForms {
			pairs = append(pairs, pair{q.Literal, answer})
			continue
		}
		words, err := caseform.Parse(answer)
		if err != nil {
			panic("manifest: an answer Check did not take: " + err.Error())
		}
		for i, form := range q.words.Forms() {
			pairs = append(pairs, pair{form, words.Forms()[i]})
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return len(pairs[i].old) > len(pairs[j].old) })
	var flat []string
	for _, p := range pairs {
		flat = append(flat, p.old, p.new)
	}
	return flat
}

// IsTemplateOnly is whether the path p of the template (with /) is the
// manifest or under a path only the template keeps.
func (m *Manifest) IsTemplateOnly(p string) bool {
	if p == File {
		return true
	}
	for _, only := range m.TemplateOnly {
		if p == only || strings.HasPrefix(p, only+"/") {
			return true
		}
	}
	return false
}
