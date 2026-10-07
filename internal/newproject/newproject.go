// Package newproject is the slice of itos-template new (decision 16): its
// command as kong reads it, the slice's input with no copy of its flags, the
// project made from it, and its output. It imports no other slice; what it
// shares with check is the domain's (template, answer, project, problem).
// The package is not named new, which Go predeclares.
//
// Every check runs before anything is written to the project's folder, so a
// refusal leaves no folder behind (decision 12): the folder, then the
// template (cloned into a temporary folder), its manifest, the stack and
// the features, the answers, and what package template's Render checks.
// Only then is the folder made and the render written, recorded
// (.itos-template.yaml, decision 10) and committed as the project's first
// commit; a failure while writing removes what was written.
//
// A failure is a *problem.Failure, its exit code chosen where it is made;
// Run, the slice's edge, only reports it.
package newproject

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// Command is itos-template new: its flags, the slice's input as kong reads
// it (decision 4), and the project made from them.
type Command struct {
	Template string   `arg:"" help:"The template: anything git clone takes, a path or a URL."`
	Folder   string   `arg:"" help:"The project's folder: one that does not exist, or an empty one."`
	Stack    string   `help:"The stack to render, by the name the template's manifest gives it. Asked on a terminal when not given." placeholder:"STACK"`
	Feature  []string `help:"A feature to merge onto the stack, by its name (cli) or its branch (go/cli); once for each. The features a feature needs are never added unasked." placeholder:"FEATURE" sep:"none"`
	Answer   []string `help:"An answer to one of the template's questions, as name=answer; once for each. An answer not given is asked on a terminal." placeholder:"NAME=ANSWER" sep:"none"`
	Defaults bool     `help:"Take a missing answer's default instead of asking or refusing." negatable:"" env:"ITOS_TEMPLATE_DEFAULTS"`
	JSON     bool     `name:"json" help:"Print the result as one JSON object on stdout." negatable:"" env:"ITOS_TEMPLATE_JSON"`
}

// Help is new's detail in its --help: its JSON and its exit codes
// (docs/CLI.md, rules 10 and 14).
func (c *Command) Help() string {
	return `Renders the template's stack branch merged with the chosen features' branches, replaces each literal its manifest (itos-template.yaml) lists by its answer, in file contents and names and in every case form, and commits the render, with its record (.itos-template.yaml), as the new git repository's first commit. Nothing is written to the folder until every check has passed.

--json prints {"schema":1,"ok":true,"folder","template","stack","features":[…],"answers":{…},"commits":{"<branch>":"<sha>"},"commit":"<sha>"}, or {"schema":1,"ok":false,"problems":[{"rule","message"}]}.

Exit codes: 0 made; 1 refused: the folder has files in it, a feature of another stack, a feature whose needed feature is not chosen, a combination the manifest lists as unsupported, branches that do not merge cleanly; 2 a usage or manifest error: an unknown stack or feature, an answer missing (without a terminal) or not one its question takes; 3 git cannot reach the template, or cannot run; 70 an internal error.

Examples:
  itos-template new ../acme made --stack go --feature cli --answer name=blue-fox
  itos-template new https://github.com/you/template.git made --stack go --defaults

Report issues at https://github.com/donvargax/itos-template/issues.`
}

// Run makes the project and returns new's exit code: what it made on
// stdout, a line or with --json its object, or each problem on stderr. It
// asks on in for what is missing only when terminal says stdin and stdout
// are one.
func (c *Command) Run(in io.Reader, stdout, stderr io.Writer, terminal bool) int {
	var asker prompt.Asker
	if terminal {
		asker = prompt.NewLines(in, stderr)
	}
	p, err := c.makeProject(asker)
	if err != nil {
		return problem.As(err).Report(stdout, stderr, c.JSON)
	}
	if c.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(struct {
			Schema int  `json:"schema"`
			OK     bool `json:"ok"`
			*project.Project
		}{1, true, p}); err != nil {
			problem.Line(stderr, err.Error())
		}
		return 0
	}
	features := "no features"
	if len(p.Features) > 0 {
		features = "the features " + strings.Join(p.Features, ", ")
	}
	_, _ = fmt.Fprintf(stdout, "Made %s from %s: the stack %s, %s.\n", p.Folder, p.Template, p.Stack, features)
	return 0
}

// makeProject makes the project c asks for, asking asker, when there is
// one, for what c leaves out.
func (c *Command) makeProject(asker prompt.Asker) (*project.Project, error) {
	created, err := checkFolder(c.Folder)
	if err != nil {
		return nil, err
	}
	t, err := template.Open(c.Template)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	s, err := c.choose(t.Manifest, asker)
	if err != nil {
		return nil, err
	}
	return t.Render(manifest.Combination{Stack: s.stack, Features: s.features}, s.answers, c.Folder, created, nil)
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

// choose checks the stack, the features and the answers c names against the
// manifest, asking on a terminal for what is missing. Usage problems (a name
// the manifest does not list, an answer missing or malformed) are reported
// together, exit 2; then a combination the template refuses, exit 1; only
// then is anything asked, so nobody answers questions for a refusal.
func (c *Command) choose(m *manifest.Manifest, asker prompt.Asker) (*selection, error) {
	usage := &problem.Failure{Code: problem.CodeUsage}
	refused := &problem.Failure{Code: problem.CodeRefused}
	add := func(f *problem.Failure, rule, format string, args ...any) {
		f.Problems = append(f.Problems, problem.Problem{Rule: rule, Message: fmt.Sprintf(format, args...)})
	}
	s := &selection{}

	stackName := c.Stack
	if stackName == "" && asker != nil && len(m.Stacks) > 0 {
		var err error
		stackName, err = asker.Ask(prompt.Question{
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
	for _, name := range c.Feature {
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
			if !slices.ContainsFunc(s.features, func(other manifest.Feature) bool { return other.Name == need }) {
				add(refused, "feature-needs", "the feature %s needs the feature %s: choose it too, with --feature %s", f.Name, need, need)
			}
		}
	}
	if combination := (manifest.Combination{Stack: stack, Features: s.features}); stackOK && len(refused.Problems) == 0 {
		if _, ok := m.IsUnsupported(combination); ok {
			add(refused, "combination-unsupported", "the template does not support %s: its manifest lists that combination as unsupported", combination.Name())
		}
	}

	var ask []*manifest.Question
	var problems []problem.Problem
	s.answers, ask, problems = answer.Read(m, c.Answer, c.Defaults, asker != nil)
	usage.Problems = append(usage.Problems, problems...)
	if len(usage.Problems) > 0 {
		return nil, usage
	}
	if len(refused.Problems) > 0 {
		return nil, refused
	}
	for _, q := range ask {
		given, err := asker.Ask(prompt.Question{
			Text:       q.Question,
			Default:    deref(q.Default),
			HasDefault: q.Default != nil,
			Check:      q.Check,
		})
		if err != nil {
			return nil, problem.New(problem.CodeUsage, "answer-missing", "no answer to %s (%s): %v", q.Name, q.Question, err)
		}
		s.answers[q.Name] = given
	}
	return s, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
