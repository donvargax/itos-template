// Package caseform writes a literal's words in the five case forms a
// template's literal is replaced in: kebab (acme-widget), snake
// (acme_widget), camel (acmeWidget), Pascal (AcmeWidget) and upper snake
// (ACME_WIDGET).
//
// The words are always given, never guessed: a literal and an answer are
// written in kebab case, lowercase words joined by dashes, and the dashes
// are the only word breaks. A library splitting "HTTPServer" by its own
// acronym rules could change a render between its releases, and an update
// needs renders reproduced byte for byte (decision 3), so this is our own
// (decision 13; the item slice-1's why).
package caseform

import (
	"fmt"
	"strings"
)

// Words are a literal's words, each lowercase letters and digits.
type Words []string

// Parse reads kebab, words of lowercase letters and digits joined by single
// dashes.
func Parse(kebab string) (Words, error) {
	if kebab == "" {
		return nil, fmt.Errorf("no words")
	}
	words := strings.Split(kebab, "-")
	for _, w := range words {
		if w == "" {
			return nil, fmt.Errorf("%q has an empty word: write lowercase words joined by single dashes", kebab)
		}
		for _, r := range w {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				return nil, fmt.Errorf("%q holds %q: write lowercase words of letters and digits joined by dashes", kebab, r)
			}
		}
	}
	return words, nil
}

// Kebab is the words joined by dashes: acme-widget.
func (w Words) Kebab() string { return strings.Join(w, "-") }

// Snake is the words joined by underscores: acme_widget.
func (w Words) Snake() string { return strings.Join(w, "_") }

// Camel is the first word as it is and each other capitalized: acmeWidget.
func (w Words) Camel() string {
	if len(w) == 0 {
		return ""
	}
	return w[0] + Words(w[1:]).Pascal()
}

// Pascal is every word capitalized: AcmeWidget.
func (w Words) Pascal() string {
	var b strings.Builder
	for _, word := range w {
		b.WriteString(strings.ToUpper(word[:1]))
		b.WriteString(word[1:])
	}
	return b.String()
}

// UpperSnake is the words in capitals joined by underscores: ACME_WIDGET.
func (w Words) UpperSnake() string { return strings.ToUpper(w.Snake()) }

// Forms are the five forms, in the order kebab, snake, camel, Pascal and
// upper snake. Two words or more make five different strings; one word
// makes its kebab, snake and camel forms the same.
func (w Words) Forms() []string {
	return []string{w.Kebab(), w.Snake(), w.Camel(), w.Pascal(), w.UpperSnake()}
}
