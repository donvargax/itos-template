package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The acme fixture's manifest, the one the scenarios render.
func acme(t *testing.T) *Manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "features", "testdata", "acme", "main", File))
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestParseReadsTheFixture(t *testing.T) {
	m := acme(t)
	if !slices.Equal(m.StackNames(), []string{"go", "python"}) {
		t.Errorf("stacks %q", m.StackNames())
	}
	if !slices.Equal(m.FeatureNames("go"), []string{"cli", "web"}) {
		t.Errorf("go's features %q", m.FeatureNames("go"))
	}
	q, ok := m.Question("module")
	if !ok || q.Default == nil || *q.Default != "example.com/you/project" || q.CaseForms {
		t.Errorf("module is %+v", q)
	}
	if !m.IsTemplateOnly(File) || !m.IsTemplateOnly(".github/workflows/template.yml") || m.IsTemplateOnly(".github/workflows/ci.yml") {
		t.Error("IsTemplateOnly does not keep the manifest and the listed path apart from the rest")
	}
}

func TestFeaturesNamed(t *testing.T) {
	m := acme(t)
	branches := func(fs []Feature) []string {
		var b []string
		for _, f := range fs {
			b = append(b, f.Branch())
		}
		return b
	}
	if got := branches(m.FeaturesNamed("cli", "python")); !slices.Equal(got, []string{"python/cli", "go/cli"}) {
		t.Errorf("cli for python is %q", got)
	}
	if got := branches(m.FeaturesNamed("python/cli", "go")); !slices.Equal(got, []string{"python/cli"}) {
		t.Errorf("python/cli is %q", got)
	}
	if got := m.FeaturesNamed("go/nosuch", "go"); len(got) != 0 {
		t.Errorf("go/nosuch is %v", got)
	}
}

func TestCheckTakesAnAnswerMatchingThePatternWhole(t *testing.T) {
	m := acme(t)
	name, _ := m.Question("name")
	for _, ok := range []string{"blue-fox", "fox", "a1-b2"} {
		if err := name.Check(ok); err != nil {
			t.Errorf("name refuses %q: %v", ok, err)
		}
	}
	for _, bad := range []string{"Blue_Fox", "blue fox", "-fox", "fox-", "x blue-fox"} {
		if err := name.Check(bad); err == nil {
			t.Errorf("name takes %q", bad)
		}
	}
}

func TestCheckTakesAnyAnswerButAnEmptyOneWithoutAPattern(t *testing.T) {
	m, err := Parse([]byte("version: 1\nstacks: [{name: go}]\nquestions: [{name: owner, literal: Acme Corp, question: \"Owner?\"}]\n"))
	if err != nil {
		t.Fatal(err)
	}
	q, _ := m.Question("owner")
	if err := q.Check("Blue Fox & Co"); err != nil {
		t.Errorf("Check refuses an answer: %v", err)
	}
	if err := q.Check(""); err == nil {
		t.Error("Check takes an empty answer")
	}
}

func TestReplacementsPutTheLongestLiteralFirst(t *testing.T) {
	m := acme(t)
	got := m.Replacements(map[string]string{"name": "blue-fox", "module": "example.com/blue/fox"})
	// The longest first; equal lengths keep the manifest's order (kebab,
	// snake, camel, Pascal, upper snake), so the 11-byte forms precede the
	// 10-byte ones.
	want := []string{
		"example.com/acme/widget", "example.com/blue/fox",
		"acme-widget", "blue-fox",
		"acme_widget", "blue_fox",
		"ACME_WIDGET", "BLUE_FOX",
		"acmeWidget", "blueFox",
		"AcmeWidget", "BlueFox",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Replacements = %q, want %q", got, want)
	}
}

func TestParseRefuses(t *testing.T) {
	cases := map[string]string{
		"an unknown key":            "version: 1\nstacks: [{name: go}]\nsetup: []\n",
		"a later version":           "version: 3\nstacks: [{name: go}]\n",
		"no version":                "stacks: [{name: go}]\n",
		"checks in version 1":       "version: 1\nstacks: [{name: go}]\nchecks: [[go, test]]\n",
		"empty checks in version 1": "version: 1\nstacks: [{name: go, checks: []}]\n",
		"unsupported in version 1":  "version: 1\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: go}]\nunsupported: [{stack: go, features: [cli]}]\n",
		"a check as one string":     "version: 2\nstacks: [{name: go}]\nchecks: [go test ./...]\n",
		"a check with no program":   "version: 2\nstacks: [{name: go, checks: [[]]}]\n",
		"a check's empty program":   "version: 2\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: go, checks: [['', x]]}]\n",
		"unsupported, no stack":     "version: 2\nstacks: [{name: go}]\nunsupported: [{stack: rust}]\n",
		"unsupported, no feature":   "version: 2\nstacks: [{name: go}]\nunsupported: [{stack: go, features: [cli]}]\n",
		"unsupported, a need left":  "version: 2\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: go}, {name: web, stack: go, needs: [cli]}]\nunsupported: [{stack: go, features: [web]}]\n",
		"unsupported, one twice":    "version: 2\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: go}]\nunsupported: [{stack: go, features: [cli, cli]}]\n",
		"no stack":                  "version: 1\n",
		"a feature of no stack":     "version: 1\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: rust}]\n",
		"a need of another stack":   "version: 1\nstacks: [{name: go}, {name: py}]\nfeatures: [{name: cli, stack: py}, {name: web, stack: go, needs: [cli]}]\n",
		"a one-word case literal":   "version: 1\nstacks: [{name: go}]\nquestions: [{name: n, literal: acme, question: Q, case_forms: true}]\n",
		"a case literal, 2fa-code":  "version: 1\nstacks: [{name: go}]\nquestions: [{name: n, literal: 2fa-code, question: Q, case_forms: true}]\n",
		"a case literal, 1-2":       "version: 1\nstacks: [{name: go}]\nquestions: [{name: n, literal: 1-2, question: Q, case_forms: true}]\n",
		"a literal not in kebab":    "version: 1\nstacks: [{name: go}]\nquestions: [{name: n, literal: AcmeWidget, question: Q, case_forms: true}]\n",
		"two questions, one form":   "version: 1\nstacks: [{name: go}]\nquestions: [{name: a, literal: acme-widget, question: Q, case_forms: true}, {name: b, literal: acmeWidget, question: Q}]\n",
		"a default it refuses":      "version: 1\nstacks: [{name: go}]\nquestions: [{name: a, literal: x-y, question: Q, pattern: '[a-z]+', default: '1'}]\n",
		"a pattern that is no RE2":  "version: 1\nstacks: [{name: go}]\nquestions: [{name: a, literal: x, question: Q, pattern: '(a'}]\n",
		"an absolute template path": "version: 1\nstacks: [{name: go}]\ntemplate_only: [/ci.yml]\n",
		"a path out of the tree":    "version: 1\nstacks: [{name: go}]\ntemplate_only: [../ci.yml]\n",
		"no question text":          "version: 1\nstacks: [{name: go}]\nquestions: [{name: a, literal: x}]\n",
		"an empty default":          "version: 1\nstacks: [{name: go}]\nquestions: [{name: a, literal: x, question: Q, default: ''}]\n",
	}
	for name, text := range cases {
		if _, err := Parse([]byte(text)); err == nil {
			t.Errorf("%s: Parse took it", name)
		} else if strings.TrimSpace(err.Error()) == "" {
			t.Errorf("%s: no problem named", name)
		}
	}
}

// A case-forms literal whose forms are not five different strings would
// map one string to two answers; the manifest refuses it, saying which
// forms collide and how to fix the literal (bug-1).
func TestParseSaysWhichCaseFormsOfALiteralCollide(t *testing.T) {
	cases := map[string]string{
		"2fa-code":    `the question name has case forms, so the five forms of its literal "2fa-code" must be five different strings: its camel and Pascal forms are both 2faCode: start its first word with a letter`,
		"1-2":         `the question name has case forms, so the five forms of its literal "1-2" must be five different strings: its snake and upper snake forms are both 1_2; its camel and Pascal forms are both 12: start its first word with a letter`,
		"acme-2fa":    "",
		"acme-widget": "",
	}
	for literal, want := range cases {
		_, err := Parse([]byte("version: 1\nstacks: [{name: go}]\nquestions: [{name: name, literal: " + literal + ", question: Q, case_forms: true}]\n"))
		var invalid *Invalid
		switch {
		case want == "" && err != nil:
			t.Errorf("%s: Parse = %v", literal, err)
		case want == "":
		case !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, []string{want}):
			t.Errorf("%s: Parse = %v, want the problem\n%s", literal, err, want)
		}
	}
}

func names(cs []Combination) []string {
	var n []string
	for _, c := range cs {
		n = append(n, c.Name())
	}
	return n
}

func TestCombinationsAreEachStackAloneAndWithEachSetOfFeaturesWhoseNeedsAreChosen(t *testing.T) {
	m := acme(t)
	want := []string{"go", "go + cli", "go + cli + web", "python", "python + cli"}
	if got := names(m.Combinations()); !slices.Equal(got, want) {
		t.Errorf("Combinations = %q, want %q", got, want)
	}
}

func TestCombinationsPutFewerFeaturesFirstInTheManifestsOrder(t *testing.T) {
	m, err := Parse([]byte("version: 2\nstacks: [{name: go}]\nfeatures: [{name: a, stack: go}, {name: b, stack: go}, {name: c, stack: go, needs: [a]}]\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"go", "go + a", "go + b", "go + a + b", "go + a + c", "go + a + b + c"}
	if got := names(m.Combinations()); !slices.Equal(got, want) {
		t.Errorf("Combinations = %q, want %q", got, want)
	}
}

func TestCombinationsLeaveOutTheUnsupported(t *testing.T) {
	m, err := Parse([]byte("version: 2\nstacks: [{name: go}, {name: py}]\nfeatures: [{name: a, stack: go}, {name: b, stack: go}]\nunsupported: [{stack: go, features: [b, a]}, {stack: py}]\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"go", "go + a", "go + b"}
	if got := names(m.Combinations()); !slices.Equal(got, want) {
		t.Errorf("Combinations = %q, want %q", got, want)
	}
	all := m.Combinations()
	if _, ok := m.IsUnsupported(all[1]); ok {
		t.Errorf("%s is unsupported", all[1].Name())
	}
}

func TestChecksOfAreTheRootsThenTheStacksThenTheFeaturesInTheManifestsOrder(t *testing.T) {
	m, err := Parse([]byte(`version: 2
checks: [[root]]
stacks: [{name: go, checks: [[stack, one], [stack, two]]}]
features:
  - {name: a, stack: go, checks: [[a]]}
  - {name: b, stack: go, checks: [[b]]}
`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := m.combination("go", []string{"b", "a"})
	var got []string
	for _, c := range m.ChecksOf(b) {
		got = append(got, strings.Join(c, " "))
	}
	want := []string{"root", "stack one", "stack two", "a", "b"}
	if !slices.Equal(got, want) {
		t.Errorf("ChecksOf = %q, want %q", got, want)
	}
}

func TestParseReadsVersion1AsATemplateWithNoChecks(t *testing.T) {
	m, err := Parse([]byte("version: 1\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: go}]\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.Combinations() {
		if len(m.ChecksOf(c)) != 0 {
			t.Errorf("%s has checks", c.Name())
		}
	}
}

func TestParseSaysACheckIsAListOfWords(t *testing.T) {
	_, err := Parse([]byte("version: 2\nstacks: [{name: go}]\nchecks: [go test ./...]\n"))
	if err == nil || !strings.Contains(err.Error(), "list of words") {
		t.Errorf("Parse = %v", err)
	}
}

func TestACheckIsWrittenQuotingAWordOnlyWhenItMustBe(t *testing.T) {
	got := Check{"sh", "-c", "go test ./...", "", `a"b`, "tab\there", "it's", `back\slash`, "bell\a", "plain/path.go"}.String()
	want := `sh -c "go test ./..." "" "a\"b" "tab\there" "it's" "back\\slash" "bell\a" plain/path.go`
	if got != want {
		t.Errorf("the check is written\n%s, not\n%s", got, want)
	}
}
