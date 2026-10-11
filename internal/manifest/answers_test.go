package manifest

import (
	"maps"
	"slices"
	"testing"
)

// taken are texts as m's questions take them, by name.
func taken(t *testing.T, m *Manifest, texts map[string]string) Answers {
	t.Helper()
	var answers Answers
	for name, text := range texts {
		q, _ := m.Question(name)
		answer, err := q.Take(text)
		if err != nil {
			t.Fatalf("%s does not take %q: %v", name, text, err)
		}
		answers.Add(answer)
	}
	return answers
}

func TestReplacementsPutTheLongestLiteralFirst(t *testing.T) {
	m := acme(t)
	got := m.Replacements(taken(t, m, map[string]string{"name": "blue-fox", "module": "example.com/blue/fox"}))
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

// Answers keep each text its question took, by question, the one added last
// for a question answered twice: the texts the record keeps. No answer at
// all is an empty map, never nil, and replaces each literal with nothing.
func TestAnswersKeepTheTextsTheirQuestionsTook(t *testing.T) {
	m := acme(t)
	answers := taken(t, m, map[string]string{"name": "red-fox", "module": "example.com/blue/fox"})
	q, _ := m.Question("name")
	blueFox, err := q.Take("blue-fox")
	if err != nil {
		t.Fatal(err)
	}
	answers.Add(blueFox)
	if got, want := answers.Texts(), map[string]string{"name": "blue-fox", "module": "example.com/blue/fox"}; !maps.Equal(got, want) {
		t.Errorf("Texts = %q, want %q", got, want)
	}
	var none Answers
	if got := none.Texts(); got == nil || len(got) != 0 {
		t.Errorf("no answers' Texts = %#v", got)
	}
	replaced := m.Replacements(none)
	for i := 1; i < len(replaced); i += 2 {
		if replaced[i] != "" {
			t.Errorf("with no answer, %q is replaced by %q", replaced[i-1], replaced[i])
		}
	}
}

// A default is taken once, when the manifest is read, and kept as its
// question took it; a question with none has no default answer.
func TestADefaultIsTakenWhenTheManifestIsRead(t *testing.T) {
	m, err := Parse([]byte("version: 1\nstacks: [{name: go}]\nquestions: [{name: owner, literal: Acme Corp, question: Q, default: Nobody}, {name: name, literal: acme-widget, question: Q, case_forms: true}]\n"))
	if err != nil {
		t.Fatal(err)
	}
	owner, _ := m.Question("owner")
	def, ok := owner.DefaultAnswer()
	var answers Answers
	answers.Add(def)
	if !ok || answers.Texts()["owner"] != "Nobody" {
		t.Errorf("owner's default answer is %v, %q", ok, answers.Texts())
	}
	name, _ := m.Question("name")
	if def, ok := name.DefaultAnswer(); ok {
		t.Errorf("name, with no default, has the default answer %+v", def)
	}
}
