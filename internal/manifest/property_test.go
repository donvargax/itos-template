package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
	"pgregory.net/rapid"

	"github.com/donvargax/itos-template/internal/caseform"
)

// branchWord is a stack's or a feature's name.
var branchWord = rapid.StringMatching(`[a-z][a-z0-9]{0,3}`)

// manifests are manifests a template author may write, every one valid: one
// to three stacks, each with up to four features, each feature needing some
// of its stack's others (two may need each other); some of the
// combinations they allow listed as unsupported, their features in any
// order; and a question with case forms and one without.
var manifests = rapid.Custom(func(t *rapid.T) *Manifest {
	m := &Manifest{Version: 2}
	for _, s := range rapid.SliceOfNDistinct(branchWord, 1, 3, rapid.ID[string]).Draw(t, "stacks") {
		m.Stacks = append(m.Stacks, Stack{Name: s})
	}
	for _, s := range m.Stacks {
		names := rapid.SliceOfNDistinct(branchWord, 0, 4, rapid.ID[string]).Draw(t, "features of "+s.Name)
		for _, name := range names {
			var needs []string
			for _, other := range names {
				if other != name && rapid.IntRange(0, 3).Draw(t, name+" needs "+other) == 0 {
					needs = append(needs, other)
				}
			}
			m.Features = append(m.Features, Feature{Name: name, Stack: s.Name, Needs: needs})
		}
	}
	for _, s := range m.Stacks {
		for _, set := range allowedSets(m, s.Name) {
			if rapid.IntRange(0, 3).Draw(t, "unsupported") == 0 {
				m.Unsupported = append(m.Unsupported, Unsupported{Stack: s.Name, Features: rapid.Permutation(set).Draw(t, "listed as")})
			}
		}
	}
	m.Questions = []Question{
		{Name: "name", Literal: "acme-widget", Question: "Name?", CaseForms: true},
		{Name: "owner", Literal: "Acme Corp", Question: "Owner?"},
	}
	return m
})

// allowedSets are the sets of the stack's features in which every
// feature's needs are in the set too, the empty set first, each as its
// features' names, sorted: worked out here, by trying every set, apart from
// how Combinations derives them.
func allowedSets(m *Manifest, stack string) [][]string {
	var features []Feature
	for _, f := range m.Features {
		if f.Stack == stack {
			features = append(features, f)
		}
	}
	var sets [][]string
	for bits := range 1 << len(features) {
		var set []string
		for i, f := range features {
			if bits&(1<<i) != 0 {
				set = append(set, f.Name)
			}
		}
		allowed := true
		for i, f := range features {
			for _, need := range f.Needs {
				allowed = allowed && (bits&(1<<i) == 0 || slices.Contains(set, need))
			}
		}
		if allowed {
			slices.Sort(set)
			sets = append(sets, set)
		}
	}
	return sets
}

// yamlOf is m as an author writes it in itos-template.yaml.
func yamlOf(t *rapid.T, m *Manifest) []byte {
	data, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// isUnsupported is whether m lists the stack's features set as
// unsupported, in any order.
func isUnsupported(m *Manifest, stack string, set []string) bool {
	return slices.ContainsFunc(m.Unsupported, func(u Unsupported) bool {
		return u.Stack == stack && slices.Equal(slices.Sorted(slices.Values(u.Features)), set)
	})
}

// The combinations check renders are exactly those the manifest allows:
// each one's
// features of its stack with their needs chosen, none listed as
// unsupported, and every other set of a stack's features whose needs are in
// it derived once.
func TestEveryCombinationHasItsNeedsChosenAndNoneIsUnsupported(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m, err := Parse(yamlOf(t, manifests.Draw(t, "manifest")))
		if err != nil {
			t.Fatalf("a valid manifest refused: %v", err)
		}
		derived := map[string]bool{}
		for _, c := range m.Combinations() {
			var set []string
			for _, f := range c.Features {
				if f.Stack != c.Stack.Name {
					t.Fatalf("%s holds %s, a feature of another stack", c.Name(), f.Branch())
				}
				for _, need := range f.Needs {
					if !slices.ContainsFunc(c.Features, func(o Feature) bool { return o.Name == need }) {
						t.Fatalf("%s holds %s without %s, which it needs", c.Name(), f.Name, need)
					}
				}
				set = append(set, f.Name)
			}
			slices.Sort(set)
			if isUnsupported(m, c.Stack.Name, set) {
				t.Fatalf("%s is listed as unsupported", c.Name())
			}
			key := c.Stack.Name + ": " + strings.Join(set, ", ")
			if derived[key] {
				t.Fatalf("%s is derived twice", c.Name())
			}
			derived[key] = true
		}
		for _, s := range m.Stacks {
			for _, set := range allowedSets(m, s.Name) {
				if !derived[s.Name+": "+strings.Join(set, ", ")] && !isUnsupported(m, s.Name, set) {
					t.Fatalf("the stack %s with %q is allowed and not derived", s.Name, set)
				}
			}
		}
	})
}

// A case-forms literal as an author may write one: one to four words of
// lowercase letters and digits, joined by dashes, digits often first or
// alone, where forms collide.
var literalWords = rapid.Custom(func(t *rapid.T) caseform.Words {
	return caseform.Words(rapid.SliceOfN(rapid.StringMatching(`[a-z0-9]{1,3}`), 1, 4).Draw(t, "words"))
})

// Each form of a case-forms literal is replaced by the same form of the
// answer, so the manifest takes such a literal exactly when its five forms
// are five different strings: it refuses every literal whose forms collide,
// one word, a first word starting with a digit or words of digits alone,
// and takes every other
// (bug-1).
func TestTheManifestTakesACaseFormsLiteralExactlyWhenItsFormsDiffer(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		words := literalWords.Draw(t, "literal")
		forms := words.Forms()
		different := len(forms) == 5 && len(slices.Compact(slices.Sorted(slices.Values(forms)))) == 5
		m := &Manifest{Version: 2, Stacks: []Stack{{Name: "go"}}, Questions: []Question{
			{Name: "name", Literal: words.Kebab(), Question: "Name?", CaseForms: true},
		}}
		_, err := Parse(yamlOf(t, m))
		var invalid *Invalid
		switch {
		case different && err != nil:
			t.Fatalf("the forms %q are five different strings, and the manifest refuses them: %v", forms, err)
		case !different && err == nil:
			t.Fatalf("the forms %q collide, and the manifest takes them", forms)
		case !different && (!errors.As(err, &invalid) || len(invalid.Problems) != 1 || !strings.HasPrefix(invalid.Problems[0], "the question name has case forms")):
			t.Fatalf("the forms %q collide, and the manifest refuses them with %v, not the one problem of its case forms", forms, err)
		}
	})
}

// fragments are pieces of YAML and of a manifest's keys, that a cut
// manifest gains in the wrong place.
var fragments = rapid.SampledFrom([]string{
	"\n", "  ", "\t", "- ", ": ", "[", "]", "{", "}", ",", "'", `"`, "#", "\x00", "\xff",
	"&a ", "*a", "<<: *a", "!!str ", "!!binary ", "!!int ", "~", "null", "---\n", "...\n",
	"version: 1\n", "version: 2\n", "stacks:", "features:", "needs:", "unsupported:", "checks:",
	"questions:", "case_forms: true", "default:", "pattern: '('", "template_only:",
})

// manifestBytes are a valid manifest's bytes, cut, and with fragments and
// stray bytes put in, so Parse meets what a hand-edited manifest may hold
// rather than noise its decoder refuses on the first byte.
var manifestBytes = rapid.Custom(func(t *rapid.T) []byte {
	data := yamlOf(t, manifests.Draw(t, "manifest"))
	for range rapid.IntRange(0, 4).Draw(t, "changes") {
		at := rapid.IntRange(0, len(data)).Draw(t, "at")
		switch rapid.IntRange(0, 2).Draw(t, "change") {
		case 0:
			data = slices.Delete(data, at, rapid.IntRange(at, len(data)).Draw(t, "to"))
		case 1:
			data = slices.Insert(data, at, []byte(fragments.Draw(t, "fragment"))...)
		default:
			data = slices.Insert(data, at, rapid.SliceOfN(rapid.Byte(), 1, 4).Draw(t, "bytes")...)
		}
	}
	return data
})

// Parse never panics, whatever the bytes: it takes them or refuses them as
// an *Invalid.
func TestParseNeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		data := rapid.OneOf(manifestBytes, rapid.SliceOf(rapid.Byte())).Draw(t, "bytes")
		m, err := Parse(data)
		var invalid *Invalid
		switch {
		case err == nil && m == nil:
			t.Fatal("Parse returned no manifest and no error")
		case err != nil && (m != nil || !errors.As(err, &invalid)):
			t.Fatalf("Parse refused with %T, not an *Invalid: %v", err, err)
		}
	})
}

// jsonOf is the manifest data, YAML, written as JSON, as a tool writing JSON
// into itos-template.yaml writes it: compact or indented.
func jsonOf(t *rapid.T, data []byte) []byte {
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	var out []byte
	var err error
	if rapid.Bool().Draw(t, "indented") {
		out, err = json.MarshalIndent(v, "", "  ")
	} else {
		out, err = json.Marshal(v)
	}
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// A manifest is JSON data written as YAML, so written as JSON it reads as
// the same manifest (docs/CONFIG.md, "JSON is the data model, YAML its
// syntax"): a tool may write JSON into itos-template.yaml.
func TestAManifestWrittenAsJSONIsTheSameManifest(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		data := yamlOf(t, manifests.Draw(t, "manifest"))
		fromYAML, err := Parse(data)
		if err != nil {
			t.Fatalf("a valid manifest refused: %v", err)
		}
		js := jsonOf(t, data)
		fromJSON, err := Parse(js)
		if err != nil {
			t.Fatalf("the manifest written as JSON refused: %v\n%s", err, js)
		}
		if a, b := yamlOf(t, fromYAML), yamlOf(t, fromJSON); !bytes.Equal(a, b) {
			t.Fatalf("written as JSON, the manifest reads\n%s\nnot\n%s", b, a)
		}
	})
}
