// Package newproject makes a project from a template: itos-template new.
//
// Every check runs before anything is written to the project's folder, so a
// refusal leaves no folder behind (decision 12): the folder, then the
// template (cloned into a temporary folder), its manifest, the stack and
// the features, the answers, the merge and the names the answers make. Only
// then is the folder made and the render written, recorded
// (.itos-template.yaml, decision 10) and committed as the project's first
// commit; a failure while writing removes what was written.
//
// A failure is a *Failure: its exit code (docs/CLI.md) and every problem,
// each with its rule ID for --json.
package newproject

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/prompt"
	"github.com/donvargax/itos-template/internal/render"
)

// The exit codes a failure carries (docs/CLI.md, "Exit codes").
const (
	CodeRefused     = 1
	CodeUsage       = 2
	CodeEnvironment = 3
	CodeInternal    = 70
)

// RecordFile is the made project's record of its render.
const RecordFile = ".itos-template.yaml"

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

// Result is what new made.
type Result struct {
	Folder   string            `json:"folder"`
	Template string            `json:"template"`
	Stack    string            `json:"stack"`
	Features []string          `json:"features"`
	Answers  map[string]string `json:"answers"`
	Commits  map[string]string `json:"commits"`
	Commit   string            `json:"commit"`
}

// Problem is one thing wrong, its rule ID and a sentence for people.
type Problem struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// Failure is new refusing or failing: its exit code and every problem.
type Failure struct {
	Code     int
	Problems []Problem
}

func (f *Failure) Error() string {
	var lines []string
	for _, p := range f.Problems {
		lines = append(lines, p.Message)
	}
	return strings.Join(lines, "\n")
}

func fail(code int, rule, format string, args ...any) *Failure {
	return &Failure{Code: code, Problems: []Problem{{Rule: rule, Message: fmt.Sprintf(format, args...)}}}
}

// Make makes the project o asks for.
func Make(o Options) (*Result, error) {
	created, err := checkFolder(o.Folder)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "itos-template-new-")
	if err != nil {
		return nil, fail(CodeEnvironment, "temporary-folder", "cannot make a temporary folder: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	t, err := render.Clone(o.Template, filepath.Join(tmp, "template.git"))
	if err != nil {
		var unreachable *render.UnreachableError
		if errors.As(err, &unreachable) {
			return nil, fail(CodeEnvironment, "template-unreachable", "git cannot reach the template %s: %v. Name a path or a URL git clone takes.", o.Template, unreachable.Err)
		}
		return nil, gitMissing(err)
	}
	m, root, rootCommit, err := readManifest(t, o.Template)
	if err != nil {
		return nil, err
	}
	s, err := choose(m, o)
	if err != nil {
		return nil, err
	}

	commits := map[string]string{root: rootCommit}
	var branches []string
	for _, b := range append([]string{s.stack.Branch()}, s.featureBranches()...) {
		sha, ok, err := t.Commit(b)
		if err != nil {
			return nil, internal(err)
		}
		if !ok {
			return nil, fail(CodeUsage, "manifest-branch-missing", "the template's manifest lists %s, but the template has no branch %s", strings.TrimPrefix(b, "stack/"), b)
		}
		commits[b] = sha
		branches = append(branches, b)
	}
	tree, err := t.Merge(commits[branches[0]], branches[0], branches[1:])
	if err != nil {
		var conflict *render.ConflictError
		if errors.As(err, &conflict) {
			return nil, fail(CodeRefused, "merge-conflict", "%v", err)
		}
		return nil, internal(err)
	}
	r := render.NewReplacer(m.Replacements(s.answers))
	plan, err := t.Plan(tree, func(p string) bool { return !m.IsTemplateOnly(p) }, r)
	if err != nil {
		var name *render.NameError
		if errors.As(err, &name) {
			return nil, fail(CodeUsage, "answer-name", "%v", err)
		}
		if render.IsDefect(err) {
			return nil, fail(CodeRefused, "template-defect", "%v", err)
		}
		return nil, internal(err)
	}
	if slices.ContainsFunc(plan.Files, func(f render.File) bool { return f.To == RecordFile }) {
		return nil, fail(CodeRefused, "template-defect", "the template holds %s, the file a made project records its render in: leave it out of the template", RecordFile)
	}
	if err := checkIdentity(tmp); err != nil {
		return nil, err
	}

	result := &Result{
		Folder:   o.Folder,
		Template: o.Template,
		Stack:    s.stack.Name,
		Features: s.featureNames(),
		Answers:  s.answers,
		Commits:  commits,
	}
	if result.Commit, err = write(o.Folder, plan, result); err != nil {
		undo(o.Folder, created)
		return nil, err
	}
	return result, nil
}

// checkFolder refuses a folder with files in it, or a path that is a file,
// and says whether new makes the folder.
func checkFolder(folder string) (bool, error) {
	info, err := os.Stat(folder)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, fail(CodeEnvironment, "folder-unreadable", "cannot read the folder %s: %v", folder, err)
	}
	if !info.IsDir() {
		return false, fail(CodeRefused, "folder-not-empty", "%s is a file: new writes a project into a missing or empty folder", folder)
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return false, fail(CodeEnvironment, "folder-unreadable", "cannot read the folder %s: %v", folder, err)
	}
	if len(entries) > 0 {
		return false, fail(CodeRefused, "folder-not-empty", "the folder %s has files in it: new writes a project into a missing or empty folder", folder)
	}
	return false, nil
}

func readManifest(t *render.Template, name string) (*manifest.Manifest, string, string, error) {
	root, err := t.DefaultBranch()
	if err != nil {
		return nil, "", "", fail(CodeUsage, "manifest-missing", "the template %s has no default branch to read its manifest, %s, from", name, manifest.File)
	}
	commit, ok, err := t.Commit(root)
	if err != nil || !ok {
		return nil, "", "", fail(CodeUsage, "manifest-missing", "the template %s's default branch, %s, has no commit to read its manifest, %s, from", name, root, manifest.File)
	}
	data, ok, err := t.File(commit, manifest.File)
	if err != nil {
		return nil, "", "", internal(err)
	}
	if !ok {
		return nil, "", "", fail(CodeUsage, "manifest-missing", "the template %s has no %s on its root branch, %s: a template names its stacks, features and questions there (docs/manifest.md)", name, manifest.File, root)
	}
	m, err := manifest.Parse(data)
	if err != nil {
		var e *manifest.Error
		if !errors.As(err, &e) {
			return nil, "", "", internal(err)
		}
		f := &Failure{Code: CodeUsage}
		for _, p := range e.Problems {
			f.Problems = append(f.Problems, Problem{Rule: "manifest-invalid", Message: fmt.Sprintf("the template's %s on %s: %s", manifest.File, root, p)})
		}
		return nil, "", "", f
	}
	return m, root, commit, nil
}

// selection is the stack, the features and the answers chosen, all checked.
type selection struct {
	stack    manifest.Stack
	features []manifest.Feature
	answers  map[string]string
}

func (s *selection) featureBranches() []string {
	var b []string
	for _, f := range s.features {
		b = append(b, f.Branch())
	}
	return b
}

func (s *selection) featureNames() []string {
	names := []string{}
	for _, f := range s.features {
		names = append(names, f.Name)
	}
	return names
}

// choose checks the stack, the features and the answers o names against the
// manifest, asking on a terminal for what is missing. Usage problems (a name
// the manifest does not list, an answer missing or malformed) are reported
// together, exit 2; then a combination the template refuses, exit 1; only
// then is anything asked, so nobody answers questions for a refusal.
func choose(m *manifest.Manifest, o Options) (*selection, error) {
	usage := &Failure{Code: CodeUsage}
	refused := &Failure{Code: CodeRefused}
	add := func(f *Failure, rule, format string, args ...any) {
		f.Problems = append(f.Problems, Problem{Rule: rule, Message: fmt.Sprintf(format, args...)})
	}
	s := &selection{answers: map[string]string{}}

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
			return nil, fail(CodeUsage, "stack-missing", "no stack chosen: name one with --stack (%s)", strings.Join(m.StackNames(), ", "))
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

	for _, kv := range o.Answers {
		name, answer, ok := strings.Cut(kv, "=")
		q, known := m.Question(name)
		switch {
		case !ok:
			add(usage, "answer-malformed", "--answer takes name=answer, and %q has no =", kv)
		case !known:
			add(usage, "answer-unknown", "the template asks no question %s: its questions are %s", name, strings.Join(m.QuestionNames(), ", "))
		case hasKey(s.answers, name):
			add(usage, "answer-twice", "the answer to %s is given twice", name)
		default:
			if err := q.Check(answer); err != nil {
				add(usage, "answer-malformed", "the answer to %s, %q, is not one it takes: %v", name, answer, err)
			}
			s.answers[name] = answer
		}
	}
	var ask []*manifest.Question
	for i := range m.Questions {
		q := &m.Questions[i]
		switch {
		case hasKey(s.answers, q.Name):
		case o.Defaults && q.Default != nil:
			s.answers[q.Name] = *q.Default
		case o.Asker != nil:
			ask = append(ask, q)
		case q.Default != nil:
			add(usage, "answer-missing", "no answer to %s (%s): give one with --answer %s=<answer>, or take its default, %s, with --defaults", q.Name, q.Question, q.Name, *q.Default)
		default:
			add(usage, "answer-missing", "no answer to %s (%s): give one with --answer %s=<answer>", q.Name, q.Question, q.Name)
		}
	}
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
			return nil, fail(CodeUsage, "answer-missing", "no answer to %s (%s): %v", q.Name, q.Question, err)
		}
		s.answers[q.Name] = answer
	}
	return s, nil
}

func hasKey(m map[string]string, k string) bool {
	_, ok := m[k]
	return ok
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// checkIdentity refuses, before anything is written, a git that does not
// know who commits: the project's first commit would fail.
func checkIdentity(dir string) error {
	for _, v := range []string{"GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT"} {
		if _, err := git.Run(dir, "var", v); err != nil {
			var e *git.Error
			if errors.As(err, &e) && e.Missing {
				return gitMissing(err)
			}
			return fail(CodeEnvironment, "git-identity", "git does not know who you are, so it cannot make the project's first commit: set user.name and user.email in git's config (%v)", err)
		}
	}
	return nil
}

// write writes the render and its record into folder, makes it a git
// repository and commits it all as its first commit, whose SHA it returns.
func write(folder string, plan *render.Plan, result *Result) (string, error) {
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", fail(CodeEnvironment, "folder-unwritable", "cannot make the folder %s: %v", folder, err)
	}
	executables, err := plan.Write(folder)
	if err != nil {
		var link *render.LinkError
		if errors.As(err, &link) {
			return "", fail(CodeEnvironment, "folder-unwritable", "cannot write the project in %s: %v", folder, err)
		}
		var g *git.Error
		if errors.As(err, &g) {
			return "", internal(err)
		}
		return "", fail(CodeEnvironment, "folder-unwritable", "cannot write the project in %s: %v", folder, err)
	}
	record, err := marshalRecord(result)
	if err != nil {
		return "", internal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, RecordFile), record, 0o644); err != nil {
		return "", fail(CodeEnvironment, "folder-unwritable", "cannot write the project in %s: %v", folder, err)
	}
	steps := [][]string{
		{"init", "-q"},
		// The render is committed as written, whatever core.autocrlf says, and
		// every file of it, whatever its .gitignore leaves out: the template
		// holds them.
		{"-c", "core.autocrlf=false", "-c", "core.safecrlf=false", "add", "--all", "--force", "--", "."},
	}
	if len(executables) > 0 {
		// Where the file system has no execute bit (windows), git takes it from
		// here, as the template records it.
		steps = append(steps, append([]string{"update-index", "--chmod=+x", "--"}, executables...))
	}
	for _, args := range steps {
		if _, err := git.Run(folder, args...); err != nil {
			return "", internal(err)
		}
	}
	if _, err := git.Run(folder, "commit", "-q", "-m", commitMessage(result)); err != nil {
		return "", fail(CodeRefused, "commit-refused", "git refused the project's first commit: %v", err)
	}
	sha, err := git.Run(folder, "rev-parse", "HEAD")
	if err != nil {
		return "", internal(err)
	}
	return strings.TrimSpace(string(sha)), nil
}

func commitMessage(r *Result) string {
	features := "no features"
	if len(r.Features) > 0 {
		features = "the features " + strings.Join(r.Features, ", ")
	}
	return fmt.Sprintf("chore: make the project from its template\n\nMade by itos-template new from %s: the stack %s, %s. %s records the render.\n",
		r.Template, r.Stack, features, RecordFile)
}

// undo removes what a failed write left in folder: the folder when new made
// it, else everything in it, as it was empty.
func undo(folder string, created bool) {
	if created {
		_ = os.RemoveAll(folder)
		return
	}
	entries, _ := os.ReadDir(folder)
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(folder, e.Name()))
	}
}

// record is the made project's .itos-template.yaml (decision 10).
type record struct {
	Version  int               `yaml:"version"`
	Template string            `yaml:"template"`
	Stack    string            `yaml:"stack"`
	Features []string          `yaml:"features"`
	Answers  map[string]string `yaml:"answers"`
	Commits  map[string]string `yaml:"commits"`
}

const recordHeader = `# What itos-template new rendered this project from: the template as it was
# named, the stack, the features, the answers, and the commit each of the
# template's branches was at. itos-template update reads it; edit it only to
# change what an update renders.
`

func marshalRecord(r *Result) ([]byte, error) {
	var b strings.Builder
	b.WriteString(recordHeader)
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(record{
		Version:  1,
		Template: r.Template,
		Stack:    r.Stack,
		Features: r.Features,
		Answers:  r.Answers,
		Commits:  r.Commits,
	}); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func gitMissing(err error) *Failure {
	return fail(CodeEnvironment, "git-missing", "cannot run git, which new clones, merges and commits with: install git, or put it on the PATH (%v)", err)
}

func internal(err error) *Failure {
	return fail(CodeInternal, "internal", "%v", err)
}
