package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
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
		"an unknown key":                "version: 1\nstacks: [{name: go}]\nsetup: []\n",
		"a later version":               "version: 5\nstacks: [{name: go}]\n",
		"first_commit in version 3":     "version: 3\nstacks: [{name: go}]\nfirst_commit: 'chore: start'\n",
		"first_commit empty":            "version: 4\nstacks: [{name: go}]\nfirst_commit: ''\n",
		"first_commit with no header":   "version: 4\nstacks: [{name: go}]\nfirst_commit: \"\\n\\nTask: T-1\\n\"\n",
		"first_commit, a blank header":  "version: 4\nstacks: [{name: go}]\nfirst_commit: \" \\r\\n\\nTask: T-1\"\n",
		"first_commit, a NUL":           "version: 4\nstacks: [{name: go}]\nfirst_commit: \"chore: start\\0\"\n",
		"first_commit not a string":     "version: 4\nstacks: [{name: go}]\nfirst_commit: [chore]\n",
		"a long check in version 2":     "version: 2\nstacks: [{name: go}]\nchecks: [{run: [gitleaks], scans: [credentials]}]\n",
		"a scan not known":              "version: 3\nstacks: [{name: go}]\nchecks: [{run: [x], scans: [licences]}]\n",
		"a long check, a key not known": "version: 3\nstacks: [{name: go}]\nchecks: [{run: [x], with: [y]}]\n",
		"a long check, run a string":    "version: 3\nstacks: [{name: go}]\nchecks: [{run: go test}]\n",
		"a long check, scans a string":  "version: 3\nstacks: [{name: go}]\nchecks: [{run: [x], scans: credentials}]\n",
		"a long check with no run":      "version: 3\nstacks: [{name: go}]\nchecks: [{scans: [credentials]}]\n",
		"no version":                    "stacks: [{name: go}]\n",
		"checks in version 1":           "version: 1\nstacks: [{name: go}]\nchecks: [[go, test]]\n",
		"empty checks in version 1":     "version: 1\nstacks: [{name: go, checks: []}]\n",
		"unsupported in version 1":      "version: 1\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: go}]\nunsupported: [{stack: go, features: [cli]}]\n",
		"a check as one string":         "version: 2\nstacks: [{name: go}]\nchecks: [go test ./...]\n",
		"a check with no program":       "version: 2\nstacks: [{name: go, checks: [[]]}]\n",
		"a check's empty program":       "version: 2\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: go, checks: [['', x]]}]\n",
		"unsupported, no stack":         "version: 2\nstacks: [{name: go}]\nunsupported: [{stack: rust}]\n",
		"unsupported, no feature":       "version: 2\nstacks: [{name: go}]\nunsupported: [{stack: go, features: [cli]}]\n",
		"unsupported, a need left":      "version: 2\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: go}, {name: web, stack: go, needs: [cli]}]\nunsupported: [{stack: go, features: [web]}]\n",
		"unsupported, one twice":        "version: 2\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: go}]\nunsupported: [{stack: go, features: [cli, cli]}]\n",
		"no stack":                      "version: 1\n",
		"a feature of no stack":         "version: 1\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: rust}]\n",
		"a need of another stack":       "version: 1\nstacks: [{name: go}, {name: py}]\nfeatures: [{name: cli, stack: py}, {name: web, stack: go, needs: [cli]}]\n",
		"a one-word case literal":       "version: 1\nstacks: [{name: go}]\nquestions: [{name: n, literal: acme, question: Q, case_forms: true}]\n",
		"a case literal, 2fa-code":      "version: 1\nstacks: [{name: go}]\nquestions: [{name: n, literal: 2fa-code, question: Q, case_forms: true}]\n",
		"a case literal, 1-2":           "version: 1\nstacks: [{name: go}]\nquestions: [{name: n, literal: 1-2, question: Q, case_forms: true}]\n",
		"a literal not in kebab":        "version: 1\nstacks: [{name: go}]\nquestions: [{name: n, literal: AcmeWidget, question: Q, case_forms: true}]\n",
		"two questions, one form":       "version: 1\nstacks: [{name: go}]\nquestions: [{name: a, literal: acme-widget, question: Q, case_forms: true}, {name: b, literal: acmeWidget, question: Q}]\n",
		"a default it refuses":          "version: 1\nstacks: [{name: go}]\nquestions: [{name: a, literal: x-y, question: Q, pattern: '[a-z]+', default: '1'}]\n",
		"a pattern that is no RE2":      "version: 1\nstacks: [{name: go}]\nquestions: [{name: a, literal: x, question: Q, pattern: '(a'}]\n",
		"an absolute template path":     "version: 1\nstacks: [{name: go}]\ntemplate_only: [/ci.yml]\n",
		"a path out of the tree":        "version: 1\nstacks: [{name: go}]\ntemplate_only: [../ci.yml]\n",
		"no question text":              "version: 1\nstacks: [{name: go}]\nquestions: [{name: a, literal: x}]\n",
		"an empty default":              "version: 1\nstacks: [{name: go}]\nquestions: [{name: a, literal: x, question: Q, default: ''}]\n",
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
		got = append(got, strings.Join(c.Run, " "))
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
	got := Words{"sh", "-c", "go test ./...", "", `a"b`, "tab\there", "it's", `back\slash`, "bell\a", "plain/path.go"}.String()
	want := `sh -c "go test ./..." "" "a\"b" "tab\there" "it's" "back\\slash" "bell\a" plain/path.go`
	if got != want {
		t.Errorf("the check is written\n%s, not\n%s", got, want)
	}
}

// Decode refuses what JSON's data model cannot say, each problem naming
// what it found and its line (docs/CONFIG.md rule 1).
func TestDecodeRefusesWhatJSONCannotSay(t *testing.T) {
	cases := map[string]struct {
		file string
		want []string
	}{
		"a custom tag": {"a: !foo x\n", []string{
			"line 1: the tag !foo: JSON has no tags, so write the value without it"}},
		"a core tag written out": {"a: 1\nb: !!str 2\n", []string{
			"line 2: the tag !!str: JSON has no tags, so write the value without it"}},
		"an anchor and an alias": {"a: &shared [1]\nb: *shared\n", []string{
			"line 1: the anchor &shared: JSON has no anchors or aliases, so write the value out where it is used",
			"line 2: the alias *shared: JSON has no anchors or aliases, so write the value out where it is used"}},
		"a merge key": {"a: {x: 1}\nb:\n  <<: {x: 1}\n  y: 2\n", []string{
			"line 3: the merge key <<: JSON has no merge keys, so write the keys out in the mapping"}},
		"a second document": {"a: 1\n---\nb: 2\n", []string{
			"line 2: a second document follows the first: a file is one document, so remove the --- and what follows it"}},
		"an empty second document": {"a: 1\n---\n", []string{
			"line 2: a second document follows the first: a file is one document, so remove the --- and what follows it"}},
		"keys that are not strings": {"1: a\ntrue: b\n~: c\n? [k]\n: d\n", []string{
			"line 1: the key 1 is not a string: JSON's keys are strings, so quote it",
			"line 2: the key true is not a string: JSON's keys are strings, so quote it",
			"line 3: the key ~ is not a string: JSON's keys are strings, so quote it",
			"line 4: a key is a list or a mapping: JSON's keys are strings"}},
		"a key given twice": {"a: 1\nb:\n  a: 2\na: 3\n", []string{
			"line 4: the key a is given twice, first at line 1: give it once"}},
		"a key given twice, quoted once": {"a: 1\n'a': 2\n", []string{
			"line 2: the key a is given twice, first at line 1: give it once"}},
		"every problem at once, in the file's order": {"a: &x !foo 1\nb: *x\n---\n", []string{
			"line 1: the tag !foo: JSON has no tags, so write the value without it",
			"line 1: the anchor &x: JSON has no anchors or aliases, so write the value out where it is used",
			"line 2: the alias *x: JSON has no anchors or aliases, so write the value out where it is used",
			"line 3: a second document follows the first: a file is one document, so remove the --- and what follows it"}},
	}
	for name, c := range cases {
		var v any
		if got := Decode([]byte(c.file), &v); !slices.Equal(got, c.want) {
			t.Errorf("%s: Decode names\n%s\nnot\n%s", name, strings.Join(got, "\n"), strings.Join(c.want, "\n"))
		}
	}
}

// What is only spelling reads as the data it spells: quotes, comments,
// flow style, a quoted "<<", which is a key like any other, and the
// non-specific tag !, which yaml.v3 reads as if it were not there.
func TestDecodeTakesWhatIsOnlySpelling(t *testing.T) {
	var got, want any
	if problems := Decode([]byte("a: ! x\nb: {'<<': [x, \"y\"]}\n# a comment\n"), &got); problems != nil {
		t.Fatalf("Decode refuses: %q", problems)
	}
	if problems := Decode([]byte(`{"a": "x", "b": {"<<": ["x", "y"]}}`), &want); problems != nil {
		t.Fatalf("Decode refuses JSON: %q", problems)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Decode reads %#v, not %#v", got, want)
	}
}

// The manifest is read by Decode: what JSON cannot say is a manifest
// problem, and so is a key the manifest does not have.
func TestParseReadsStrictly(t *testing.T) {
	_, err := Parse([]byte("version: 2\nstacks: [{name: !foo go}]\n"))
	var invalid *Invalid
	if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, []string{"line 2: the tag !foo: JSON has no tags, so write the value without it"}) {
		t.Errorf("Parse = %v", err)
	}
	if _, err := Parse([]byte("version: 2\nstacks: [{name: go}]\nsetup: []\n")); !errors.As(err, &invalid) || !strings.Contains(err.Error(), "setup") {
		t.Errorf("Parse takes an unknown key: %v", err)
	}
}

// Version 3 adds a check's long form, saying what the check scans the
// render for; its words run as a check written as a list's do, and the
// list stays valid.
func TestParseReadsACheckInItsLongFormFromVersion3(t *testing.T) {
	m, err := Parse([]byte("version: 3\nchecks: [[git, ls-files], {run: [gitleaks, dir, .], scans: [credentials]}]\nstacks: [{name: go}]\n"))
	if err != nil {
		t.Fatal(err)
	}
	c, _ := m.combination("go", nil)
	checks := m.ChecksOf(c)
	if len(checks) != 2 || checks[0].Run.String() != "git ls-files" || checks[0].Scans != nil ||
		checks[1].Run.String() != "gitleaks dir ." || !slices.Equal(checks[1].Scans, []string{Credentials}) {
		t.Errorf("the checks are %+v", checks)
	}
	if !m.Scans(Credentials) {
		t.Error("a check marked as scanning credentials is not found")
	}
	if acme(t).Scans(Credentials) {
		t.Error("acme, whose checks are marked as scanning nothing, scans credentials")
	}
}

// Any check of the template may be the one marked: a stack's or a
// feature's as well as the root's.
func TestScansFindsAMarkedCheckOfAStackOrAFeature(t *testing.T) {
	for _, text := range []string{
		"version: 3\nstacks: [{name: go, checks: [{run: [x], scans: [credentials]}]}]\n",
		"version: 3\nstacks: [{name: go}]\nfeatures: [{name: cli, stack: go, checks: [{run: [x], scans: [credentials]}]}]\n",
	} {
		m, err := Parse([]byte(text))
		if err != nil {
			t.Fatal(err)
		}
		if !m.Scans(Credentials) {
			t.Errorf("%s: no check scans credentials", text)
		}
	}
}

func TestParseNamesTheScanItDoesNotKnow(t *testing.T) {
	_, err := Parse([]byte("version: 3\nstacks: [{name: go}]\nchecks: [[x], {run: [y], scans: [credentials, licences]}]\n"))
	var invalid *Invalid
	want := []string{`the root's check 2 scans "licences", which the format does not know: scans takes credentials`}
	if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, want) {
		t.Errorf("Parse = %v", err)
	}
}

func TestParseSaysALongCheckIsOfVersion3(t *testing.T) {
	_, err := Parse([]byte("version: 2\nstacks: [{name: go, checks: [{run: [x], scans: [credentials]}]}]\n"))
	var invalid *Invalid
	want := []string{"the stack go's check 1 is in the long form, {run, scans}, of version 3: write version: 3"}
	if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, want) {
		t.Errorf("Parse = %v", err)
	}
}

// Version 4 adds first_commit, the whole message of a made project's first
// commit, read as it is written, its line endings and footers kept; a
// manifest without it gives none, and new keeps its own.
func TestParseReadsTheFirstCommitsMessageFromVersion4(t *testing.T) {
	m, err := Parse([]byte("version: 4\nstacks: [{name: go}]\nfirst_commit: |\n  chore: start acme-widget\n\n  Task: T-1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m.FirstCommit == nil || *m.FirstCommit != "chore: start acme-widget\n\nTask: T-1\n" {
		t.Errorf("first_commit is %v", m.FirstCommit)
	}
	if acme(t).FirstCommit != nil {
		t.Error("acme, which gives no first_commit, gives one")
	}
}

func TestParseSaysFirstCommitIsOfVersion4(t *testing.T) {
	_, err := Parse([]byte("version: 3\nstacks: [{name: go}]\nfirst_commit: 'chore: start'\n"))
	var invalid *Invalid
	want := []string{"first_commit is a key of version 4: write version: 4"}
	if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, want) {
		t.Errorf("Parse = %v", err)
	}
}

// git drops a message's empty first lines and would take the next for the
// header, so a message whose first line is empty or blank is refused, the
// manifest's problem named by its key.
func TestParseSaysAFirstCommitWithNoHeaderHasNone(t *testing.T) {
	for _, message := range []string{"", "\n", "\n\nTask: T-1\n", " \t\r\nchore: start\n"} {
		_, err := Parse([]byte("version: 4\nstacks: [{name: go}]\nfirst_commit: " + strconv.Quote(message) + "\n"))
		var invalid *Invalid
		want := []string{"first_commit has no header: its first line is the first commit's header, as chore: start the project"}
		if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, want) {
			t.Errorf("%q: Parse = %v", message, err)
		}
	}
}
