package manifest

import (
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
		"another version":           "version: 2\nstacks: [{name: go}]\n",
		"no stack":                  "version: 1\n",
		"a feature of no stack":     "version: 1\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: rust}]\n",
		"a need of another stack":   "version: 1\nstacks: [{name: go}, {name: py}]\nfeatures: [{name: cli, stack: py}, {name: web, stack: go, needs: [cli]}]\n",
		"a one-word case literal":   "version: 1\nstacks: [{name: go}]\nquestions: [{name: n, literal: acme, question: Q, case_forms: true}]\n",
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
