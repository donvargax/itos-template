// Package answer reads the answers to a template's questions given on a
// command line, each name=answer, against its manifest: each checked by its
// question, a missing one's default taken when asked to (--defaults), and
// what is still missing either returned to be asked on a terminal or
// refused (decision 11). It is domain, and does no I/O.
//
// Every problem is an Error of its own, all returned together, so one run
// names every answer to fix; internal/cli turns each into a line and an
// exit code.
package answer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/donvargax/itos-template/internal/manifest"
)

// Given are the answers as a command line gives them, each name=answer.
type Given []string

// Set is the answers by their questions' names, each one its question
// takes.
type Set map[string]string

// Resolve is the answers to m's questions, never asked: those given, and
// with defaults a missing one's default. Every problem is returned, joined
// (errors.Join): a malformed or unknown answer, one given twice, one its
// question does not take, and each missing one.
func Resolve(m *manifest.Manifest, given Given, defaults bool) (Set, error) {
	answers, _, problems := Read(m, given, defaults, false)
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return answers, nil
}

// Read reads the answers given against m's questions, takes a missing one's
// default with defaults, and returns the questions still missing to ask
// them when ask; without ask each missing one is a problem. Every problem
// is an Error.
func Read(m *manifest.Manifest, given Given, defaults, ask bool) (Set, []*manifest.Question, []error) {
	var problems []error
	answers := Set{}
	for _, kv := range given {
		name, answer, ok := strings.Cut(kv, "=")
		q, known := m.Question(name)
		switch {
		case !ok:
			problems = append(problems, &Malformed{Given: kv})
		case !known:
			problems = append(problems, &Unknown{Name: name, Questions: m.QuestionNames()})
		case hasKey(answers, name):
			problems = append(problems, &Twice{Name: name})
		default:
			if err := q.Check(answer); err != nil {
				problems = append(problems, &NotTaken{Name: name, Answer: answer, Reason: err})
			}
			answers[name] = answer
		}
	}
	var missing []*manifest.Question
	for i := range m.Questions {
		q := &m.Questions[i]
		switch {
		case hasKey(answers, q.Name):
		case defaults && q.Default != nil:
			answers[q.Name] = *q.Default
		case ask:
			missing = append(missing, q)
		default:
			problems = append(problems, &Missing{Question: q})
		}
	}
	return answers, missing, problems
}

func hasKey(m Set, k string) bool {
	_, ok := m[k]
	return ok
}

// Error is an answer that cannot be taken, a sealed set (decision 17):
// internal/cli gives each kind its exit code.
//
//sumtype:decl
type Error interface {
	error
	answerError()
}

// Malformed is an answer given with no = between its name and itself.
type Malformed struct{ Given string }

// Unknown is an answer to a question the template does not ask, and the
// questions it does.
type Unknown struct {
	Name      string
	Questions []string
}

// Twice is an answer given twice.
type Twice struct{ Name string }

// NotTaken is an answer its question does not take, and why.
type NotTaken struct {
	Name, Answer string
	Reason       error
}

// Missing is a question with no answer, where none can be asked.
type Missing struct{ Question *manifest.Question }

// NotAnswered is a question asked on a terminal and not answered, and why.
type NotAnswered struct {
	Question *manifest.Question
	Err      error
}

func (*Malformed) answerError()   {}
func (*Unknown) answerError()     {}
func (*Twice) answerError()       {}
func (*NotTaken) answerError()    {}
func (*Missing) answerError()     {}
func (*NotAnswered) answerError() {}

func (e *Malformed) Error() string { return fmt.Sprintf("the answer %q has no =", e.Given) }
func (e *Unknown) Error() string   { return "no question " + e.Name }
func (e *Twice) Error() string     { return "the answer to " + e.Name + " twice" }
func (e *NotTaken) Error() string {
	return fmt.Sprintf("the answer to %s, %+q: %v", e.Name, e.Answer, e.Reason)
}
func (e *Missing) Error() string { return "no answer to " + e.Question.Name }
func (e *NotAnswered) Error() string {
	return fmt.Sprintf("no answer to %s: %v", e.Question.Name, e.Err)
}
func (e *NotAnswered) Unwrap() error { return e.Err }
