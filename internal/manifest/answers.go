package manifest

import "github.com/donvargax/itos-template/internal/caseform"

// Answer is an answer its question took (Question.Take): the text as it was
// given and, with case forms, the words Take split it into, kept so nothing
// after Take parses it again (decision 41). Only Take makes one, so holding
// one proves its question took it. Its zero value answers no question.
type Answer struct {
	question string
	text     string
	words    caseform.Words
}

// Answers are answers their questions took, each by its question's name:
// what a render replaces the literals with (Replacements), built where the
// answers are resolved (package answer). The zero value holds none.
type Answers struct{ taken map[string]Answer }

// Add adds a, replacing an answer to its question added before.
func (a *Answers) Add(answer Answer) {
	if a.taken == nil {
		a.taken = map[string]Answer{}
	}
	a.taken[answer.question] = answer
}

// Texts are the answers as they were given, by question, as a made
// project's record keeps them (decision 10): never nil, so the record
// writes none as an empty map.
func (a Answers) Texts() map[string]string {
	texts := make(map[string]string, len(a.taken))
	for name, answer := range a.taken {
		texts[name] = answer.text
	}
	return texts
}
