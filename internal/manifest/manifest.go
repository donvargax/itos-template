// Package manifest reads a template's manifest, itos-template.yaml at the
// top of its root branch and merged down into every branch (decision 8):
// its stacks, its features, the questions whose answers replace its
// literals, the paths only the template keeps, from version 2 the checks
// check runs in each render and the combinations the template cannot
// support, and from version 3 a check's long form, saying what it scans the
// render for. docs/manifest.md is its format, for template authors.
//
// A stack's branch is stack/<stack> and a feature's <stack>/<feature>, so the
// manifest names branches by convention alone. Unknown keys are refused, so
// a typo never passes for an option, and the keys later items add come with
// a version that names them: version 1 refuses checks and unsupported, so a
// manifest written for version 2 is never misread as one with no checks, and
// version 2 refuses a check's long form.
package manifest

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos-template/internal/caseform"
)

// File is the manifest's name at the top of a template's branches.
const File = "itos-template.yaml"

// Version is the newest manifest version this itos-template reads; it
// reads every one from 1.
const Version = 3

// Manifest is a template's itos-template.yaml.
type Manifest struct {
	Version      int           `yaml:"version"`
	Stacks       []Stack       `yaml:"stacks"`
	Features     []Feature     `yaml:"features"`
	Questions    []Question    `yaml:"questions"`
	TemplateOnly []string      `yaml:"template_only"`
	Checks       []Check       `yaml:"checks"`      // the root's, version 2
	Unsupported  []Unsupported `yaml:"unsupported"` // version 2
}

// Check is a command check runs in a render: its words, the program first,
// run with no shell, so it means the same on every system, and from
// version 3 what it scans the render for. It is written as its words, a
// list, or from version 3 in its long form, {run: [words…], scans: [what
// it scans]}.
type Check struct {
	Run   Words
	Scans []string

	long bool // written in the long form
}

// Credentials is what a check scanning a render for leaked credentials is
// marked as scanning: scans: [credentials]. It is the one value scans
// takes.
const Credentials = "credentials"

// UnmarshalYAML reads a check, in either form, refusing a string with a
// sentence saying why: a shell would read one, and none runs a check.
func (c *Check) UnmarshalYAML(n *yaml.Node) error {
	words := func(n *yaml.Node, what string) ([]string, error) {
		if n.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("line %d: %s is a list of words, the program first, as [go, test, ./...]: no shell runs it, so it is never one string", n.Line, what)
		}
		var w []string
		err := n.Decode(&w)
		return w, err
	}
	if n.Kind != yaml.MappingNode {
		w, err := words(n, "a check")
		c.Run = w
		return err
	}
	c.long = true
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		var err error
		switch k.Value {
		case "run":
			c.Run, err = words(v, "a check's run")
		case "scans":
			if v.Kind != yaml.SequenceNode {
				return fmt.Errorf("line %d: a check's scans is a list, as [credentials]", v.Line)
			}
			err = v.Decode(&c.Scans)
		default:
			return fmt.Errorf("line %d: a check has no key %s: its long form is {run: [words…], scans: [what it scans]}", k.Line, k.Value)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// MarshalYAML writes a check as it was written: its words, or its long
// form.
func (c Check) MarshalYAML() (any, error) {
	if !c.long && c.Scans == nil {
		return []string(c.Run), nil
	}
	return struct {
		Run   []string `yaml:"run,flow"`
		Scans []string `yaml:"scans,flow,omitempty"`
	}{c.Run, c.Scans}, nil
}

// Words are a check's words, the program first.
type Words []string

// String is the check as check's report writes it: its words joined by
// spaces, a word written as it is unless it is empty or holds a space, a
// quote, a backslash or a character that does not print, which is written
// in double quotes as Go writes a string.
func (w Words) String() string {
	shown := make([]string, len(w))
	for i, word := range w {
		shown[i] = word
		if word == "" || strings.ContainsFunc(word, func(r rune) bool {
			return unicode.IsSpace(r) || r == '"' || r == '\'' || r == '\\' || !unicode.IsPrint(r)
		}) {
			shown[i] = strconv.Quote(word)
		}
	}
	return strings.Join(shown, " ")
}

// Unsupported is a combination the template cannot support: a stack and
// exactly the features it lists, by their names in the stack.
type Unsupported struct {
	Stack    string   `yaml:"stack"`
	Features []string `yaml:"features"`
}

// Stack is a stack: the root and a working project in one language or
// framework, on the branch stack/<name>.
type Stack struct {
	Name   string  `yaml:"name"`
	Checks []Check `yaml:"checks"` // version 2
}

// Branch is the stack's branch, stack/<name>.
func (s Stack) Branch() string { return "stack/" + s.Name }

// Feature is an optional part of a stack, on the branch <stack>/<name>,
// branched off its stack or off the features it needs.
type Feature struct {
	Name   string   `yaml:"name"`
	Stack  string   `yaml:"stack"`
	Needs  []string `yaml:"needs"`
	Checks []Check  `yaml:"checks"` // version 2
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

// Invalid is a manifest that cannot be used, every problem found in it.
type Invalid struct{ Problems []string }

func (e *Invalid) Error() string { return strings.Join(e.Problems, "; ") }

// Parse reads a manifest strictly (Decode) and checks it. A manifest that
// cannot be used is an *Invalid.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if problems := Decode(data, &m); len(problems) > 0 {
		return nil, &Invalid{problems}
	}
	if problems := m.check(); len(problems) > 0 {
		return nil, &Invalid{problems}
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
	if m.Version < 1 || m.Version > Version {
		add("version is %d: this itos-template reads versions 1 to %d", m.Version, Version)
	}
	if m.Version == 1 {
		for _, key := range m.version2Keys() {
			add("%s is a key of version 2: write version: 2", key)
		}
	}
	m.checkChecks(add)
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
			if collide := collidingForms(q.words); collide != "" {
				add("the question %s has case forms, so the five forms of its literal %q must be five different strings: %s", q.Name, q.Literal, collide)
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
	for _, u := range m.Unsupported {
		if _, err := m.combination(u.Stack, u.Features); err != nil {
			add("the unsupported combination %s names no combination the manifest allows: %v", unsupportedName(u), err)
		}
	}
	for _, p := range m.TemplateOnly {
		if p == "" || path.IsAbs(p) || strings.Contains(p, `\`) || path.Clean(p) != p || p == "." || strings.HasPrefix(p, "../") || p == ".." {
			add("the template_only path %q is not a path in the template: write it relative to its top, with /", p)
		}
	}
	return problems
}

// collidingForms says which forms of a case-forms literal's words are one
// string, and how to tell them apart, or is "" when the five are five
// different strings. Two forms as one string would map it to two answers,
// the first winning, so the manifest holds every form to the promise, a
// form added later too, rather than narrowing what a literal may be.
func collidingForms(words caseform.Words) string {
	forms := words.Forms()
	var groups []string
	counted := map[int]bool{}
	for i, form := range forms {
		if counted[i] {
			continue
		}
		names := []string{caseform.FormNames[i]}
		for j := i + 1; j < len(forms); j++ {
			if forms[j] == form {
				names = append(names, caseform.FormNames[j])
				counted[j] = true
			}
		}
		switch {
		case len(names) == 2:
			groups = append(groups, fmt.Sprintf("its %s and %s forms are both %s", names[0], names[1], form))
		case len(names) > 2:
			groups = append(groups, fmt.Sprintf("its %s and %s forms are all %s", strings.Join(names[:len(names)-1], ", "), names[len(names)-1], form))
		}
	}
	if len(groups) == 0 {
		return ""
	}
	fix := "rename it so they differ"
	if first := words[0][0]; first >= '0' && first <= '9' {
		fix = "start its first word with a letter"
	}
	return strings.Join(groups, "; ") + ": " + fix
}

// version2Keys are the keys of version 2 the manifest gives, each where it
// is: a key given an empty list counts, as version 1 refused it.
func (m *Manifest) version2Keys() []string {
	var keys []string
	if m.Checks != nil {
		keys = append(keys, "checks")
	}
	if m.Unsupported != nil {
		keys = append(keys, "unsupported")
	}
	for _, s := range m.Stacks {
		if s.Checks != nil {
			keys = append(keys, "the stack "+s.Name+"'s checks")
		}
	}
	for _, f := range m.Features {
		if f.Checks != nil {
			keys = append(keys, "the feature "+f.Branch()+"'s checks")
		}
	}
	return keys
}

// checkChecks refuses a check with no program, a check in the long form
// in version 2, and a scan the format does not know.
func (m *Manifest) checkChecks(add func(string, ...any)) {
	each := func(where string, checks []Check) {
		for i, c := range checks {
			if len(c.Run) == 0 || c.Run[0] == "" {
				add("%s check %d names no program: a check is a list of words, the program first", where, i+1)
			}
			if c.long && m.Version == 2 {
				add("%s check %d is in the long form, {run, scans}, of version 3: write version: 3", where, i+1)
			}
			for _, scan := range c.Scans {
				if scan != Credentials {
					add("%s check %d scans %q, which the format does not know: scans takes %s", where, i+1, scan, Credentials)
				}
			}
		}
	}
	each("the root's", m.Checks)
	for _, s := range m.Stacks {
		each("the stack "+s.Name+"'s", s.Checks)
	}
	for _, f := range m.Features {
		each("the feature "+f.Branch()+"'s", f.Checks)
	}
}

// Words are a case-forms literal's words, none when q has no case forms.
func (q *Question) Words() caseform.Words { return q.words }

// Scans is whether a check of the manifest, the root's, a stack's or a
// feature's, is marked as scanning renders for what.
func (m *Manifest) Scans(what string) bool {
	checks := slices.Clone(m.Checks)
	for _, s := range m.Stacks {
		checks = append(checks, s.Checks...)
	}
	for _, f := range m.Features {
		checks = append(checks, f.Checks...)
	}
	return slices.ContainsFunc(checks, func(c Check) bool { return slices.Contains(c.Scans, what) })
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

// Choice is a combination as a command line names it: a stack by its name,
// and features by their names (cli) or their branches (go/cli), unchecked.
type Choice struct {
	Stack    string
	Features []string
}

// Combination is a stack and the features a render merges onto it, in the
// manifest's order.
type Combination struct {
	Stack    Stack
	Features []Feature
}

// Name is the combination as check's report and new's refusals write it:
// the stack, then each feature's name, joined by " + " (go + cli + web).
func (c Combination) Name() string {
	names := []string{c.Stack.Name}
	for _, f := range c.Features {
		names = append(names, f.Name)
	}
	return strings.Join(names, " + ")
}

func unsupportedName(u Unsupported) string {
	return strings.Join(append([]string{u.Stack}, u.Features...), " + ")
}

// combination is the combination of the stack named stack with the features
// named, by their names in the stack, or why the manifest does not allow it:
// a stack or a feature it does not list, a feature named twice, or a feature
// whose needs are not all named.
func (m *Manifest) combination(stack string, features []string) (Combination, error) {
	s, ok := m.Stack(stack)
	if !ok {
		return Combination{}, fmt.Errorf("the manifest lists no stack %s", stack)
	}
	var chosen []Feature
	for _, name := range features {
		i := slices.IndexFunc(m.Features, func(f Feature) bool { return f.Stack == stack && f.Name == name })
		switch {
		case i < 0:
			return Combination{}, fmt.Errorf("the stack %s has no feature %s", stack, name)
		case slices.ContainsFunc(chosen, func(f Feature) bool { return f.Name == name }):
			return Combination{}, fmt.Errorf("it names the feature %s twice", name)
		}
		chosen = append(chosen, m.Features[i])
	}
	c := Combination{Stack: s, Features: m.Ordered(chosen)}
	for _, f := range c.Features {
		for _, need := range f.Needs {
			if !slices.Contains(features, need) {
				return Combination{}, fmt.Errorf("the feature %s needs the feature %s, which it does not name", f.Name, need)
			}
		}
	}
	return c, nil
}

// IsUnsupported is the entry of unsupported that lists the combination c,
// and whether one does: its stack and exactly its features, in any order.
func (m *Manifest) IsUnsupported(c Combination) (Unsupported, bool) {
	for _, u := range m.Unsupported {
		if u.Stack != c.Stack.Name || len(u.Features) != len(c.Features) {
			continue
		}
		if !slices.ContainsFunc(c.Features, func(f Feature) bool { return !slices.Contains(u.Features, f.Name) }) {
			return u, true
		}
	}
	return Unsupported{}, false
}

// Combinations are every combination the manifest allows, less those it
// lists as unsupported: for each stack in the manifest's order, the stack
// alone, then each set of its features in which every feature's needs are
// in the set too, the fewer features first, sets of as many in the
// manifest's order.
func (m *Manifest) Combinations() []Combination {
	var all []Combination
	for _, s := range m.Stacks {
		var features []Feature
		for _, f := range m.Features {
			if f.Stack == s.Name {
				features = append(features, f)
			}
		}
		var sets [][]int
		var grow func(set []int, from int)
		grow = func(set []int, from int) {
			sets = append(sets, slices.Clone(set))
			for i := from; i < len(features); i++ {
				grow(append(set, i), i+1)
			}
		}
		grow(nil, 0)
		slices.SortStableFunc(sets, func(a, b []int) int {
			if len(a) != len(b) {
				return len(a) - len(b)
			}
			return slices.Compare(a, b)
		})
		for _, set := range sets {
			c := Combination{Stack: s}
			for _, i := range set {
				c.Features = append(c.Features, features[i])
			}
			if !c.needsChosen() {
				continue
			}
			if _, unsupported := m.IsUnsupported(c); !unsupported {
				all = append(all, c)
			}
		}
	}
	return all
}

// needsChosen is whether every feature of c has the features it needs in c.
func (c Combination) needsChosen() bool {
	for _, f := range c.Features {
		for _, need := range f.Needs {
			if !slices.ContainsFunc(c.Features, func(o Feature) bool { return o.Name == need }) {
				return false
			}
		}
	}
	return true
}

// ChecksOf are the checks of the combination c, in the order check runs
// them: the root's, then its stack's, then each of its features' in the
// manifest's order.
func (m *Manifest) ChecksOf(c Combination) []Check {
	checks := slices.Clone(m.Checks)
	checks = append(checks, c.Stack.Checks...)
	for _, f := range m.Ordered(c.Features) {
		checks = append(checks, f.Checks...)
	}
	return checks
}
