// Package newproject makes a project from a template: itos-template new.
//
// Every check runs before anything is written to the project's folder, so a
// refusal leaves no folder behind (decision 12): the folder, then the
// template (cloned into a temporary folder), its manifest, the stack and
// the features, the answers, and what package template's Render checks.
// Only then is the folder made and the render written, recorded
// (.itos-template.yaml, decision 10) and committed as the project's first
// commit; a failure while writing removes what was written.
//
// A failure is a *problem.Failure: its exit code (docs/CLI.md) and every
// problem, each with its rule ID for --json.
package newproject

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/problem"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/prompt"
	"github.com/donvargax/itos-template/internal/template"
)

// Options are what new was asked to make.
type Options struct {
	Template string   // anything git clone takes
	Folder   string   // the project's folder, missing or empty
	Stack    string   // the stack's name, or empty to ask
	Features []string // each a feature's name in the stack, or its branch
	Answers  []string // each name=answer
	Defaults bool     // take a missing answer's default
	Asker    prompt.Asker
}

// Make makes the project o asks for.
func Make(o Options) (*project.Project, error) {
	created, err := checkFolder(o.Folder)
	if err != nil {
		return nil, err
	}
	t, err := template.Open(o.Template)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	s, err := choose(t.Manifest, o)
	if err != nil {
		return nil, err
	}
	return t.Render(manifest.Combination{Stack: s.stack, Features: s.features}, s.answers, o.Folder, created, nil)
}

// checkFolder refuses a folder with files in it, or a path that is a file,
// and says whether new makes the folder.
func checkFolder(folder string) (bool, error) {
	info, err := os.Stat(folder)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, problem.New(problem.CodeEnvironment, "folder-unreadable", "cannot read the folder %s: %v", folder, err)
	}
	if !info.IsDir() {
		return false, problem.New(problem.CodeRefused, "folder-not-empty", "%s is a file: new writes a project into a missing or empty folder", folder)
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return false, problem.New(problem.CodeEnvironment, "folder-unreadable", "cannot read the folder %s: %v", folder, err)
	}
	if len(entries) > 0 {
		return false, problem.New(problem.CodeRefused, "folder-not-empty", "the folder %s has files in it: new writes a project into a missing or empty folder", folder)
	}
	return false, nil
}

// selection is the stack, the features and the answers chosen, all checked.
type selection struct {
	stack    manifest.Stack
	features []manifest.Feature
	answers  map[string]string
}

// choose checks the stack, the features and the answers o names against the
// manifest, asking on a terminal for what is missing. Usage problems (a name
// the manifest does not list, an answer missing or malformed) are reported
// together, exit 2; then a combination the template refuses, exit 1; only
// then is anything asked, so nobody answers questions for a refusal.
func choose(m *manifest.Manifest, o Options) (*selection, error) {
	usage := &problem.Failure{Code: problem.CodeUsage}
	refused := &problem.Failure{Code: problem.CodeRefused}
	add := func(f *problem.Failure, rule, format string, args ...any) {
		f.Problems = append(f.Problems, problem.Problem{Rule: rule, Message: fmt.Sprintf(format, args...)})
	}
	s := &selection{}

	stackName := o.Stack
	if stackName == "" && o.Asker != nil && len(m.Stacks) > 0 {
		var err error
		stackName, err = o.Asker.Ask(prompt.Question{
			Text: fmt.Sprintf("Which stack (%s)?", strings.Join(m.StackNames(), ", ")),
			Check: func(a string) error {
				if _, ok := m.Stack(a); !ok {
					return fmt.Errorf("the stacks are %s", strings.Join(m.StackNames(), ", "))
				}
				return nil
			},
		})
		if err != nil {
			return nil, problem.New(problem.CodeUsage, "stack-missing", "no stack chosen: name one with --stack (%s)", strings.Join(m.StackNames(), ", "))
		}
	}
	stack, stackOK := m.Stack(stackName)
	switch {
	case stackName == "":
		add(usage, "stack-missing", "no stack chosen: name one with --stack (%s)", strings.Join(m.StackNames(), ", "))
	case !stackOK:
		add(usage, "stack-unknown", "the template has no stack %s: its stacks are %s", stackName, strings.Join(m.StackNames(), ", "))
	default:
		s.stack = stack
	}

	var chosen []manifest.Feature
	for _, name := range o.Features {
		found := m.FeaturesNamed(name, stackName)
		switch {
		case len(found) == 0:
			known := m.FeatureNames(stackName)
			if len(known) == 0 {
				add(usage, "feature-unknown", "the template has no feature %s", name)
			} else {
				add(usage, "feature-unknown", "the template has no feature %s: the stack %s's features are %s", name, stackName, strings.Join(known, ", "))
			}
		case !stackOK:
		case found[0].Stack != stackName:
			add(refused, "feature-other-stack", "the feature %s is the stack %s's, not the stack %s's: a project has one stack's features", found[0].Branch(), found[0].Stack, stackName)
		default:
			chosen = append(chosen, found[0])
		}
	}
	s.features = m.Ordered(chosen)
	for _, f := range s.features {
		for _, need := range f.Needs {
			if !slices.ContainsFunc(s.features, func(c manifest.Feature) bool { return c.Name == need }) {
				add(refused, "feature-needs", "the feature %s needs the feature %s: choose it too, with --feature %s", f.Name, need, need)
			}
		}
	}
	if c := (manifest.Combination{Stack: stack, Features: s.features}); stackOK && len(refused.Problems) == 0 {
		if _, ok := m.IsUnsupported(c); ok {
			add(refused, "combination-unsupported", "the template does not support %s: its manifest lists that combination as unsupported", c.Name())
		}
	}

	var ask []*manifest.Question
	var problems []problem.Problem
	s.answers, ask, problems = answer.Read(m, o.Answers, o.Defaults, o.Asker != nil)
	usage.Problems = append(usage.Problems, problems...)
	if len(usage.Problems) > 0 {
		return nil, usage
	}
	if len(refused.Problems) > 0 {
		return nil, refused
	}
	for _, q := range ask {
		answer, err := o.Asker.Ask(prompt.Question{
			Text:       q.Question,
			Default:    deref(q.Default),
			HasDefault: q.Default != nil,
			Check:      q.Check,
		})
		if err != nil {
			return nil, problem.New(problem.CodeUsage, "answer-missing", "no answer to %s (%s): %v", q.Name, q.Question, err)
		}
		s.answers[q.Name] = answer
	}
	return s, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
