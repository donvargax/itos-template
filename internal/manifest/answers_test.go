package manifest

import (
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
