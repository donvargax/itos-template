// Package answer reads the answers to a template's questions given on a
// command line, each name=answer, against its manifest: each taken by its
// question into a manifest.Answer, checked once there and trusted after
// (decision 41), a missing one's default taken when asked to (--defaults),
// and what is still missing either returned to be asked on a terminal or
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

// Set is the answers as text by their questions' names, as a made
// project's record keeps them (decision 10). Read takes them into a
// manifest.Answers, which a render replaces the literals with.
type Set map[string]string

// Resolve is the answers to m's questions, never asked: those given, and
// with defaults a missing one's default. Every problem is returned, joined
// (errors.Join): a malformed or unknown answer, one given twice, one its
// question does not take, and each missing one.
func Resolve(m *manifest.Manifest, given Given, defaults bool) (manifest.Answers, error) {
	answers, _, problems := Read(m, given, defaults, false)
	if len(problems) > 0 {
		return manifest.Answers{}, errors.Join(problems...)
	}
	return answers, nil
}

// Read takes the answers given by m's questions, takes a missing one's
// default with defaults, and returns the questions still missing to ask
// them when ask; without ask each missing one is a problem. Every problem
// is an Error. An answer its question does not take is a problem, and its
// question is answered all the same: it is neither missing nor asked.
func Read(m *manifest.Manifest, given Given, defaults, ask bool) (manifest.Answers, []*manifest.Question, []error) {
	var problems []error
	var answers manifest.Answers
	answered := map[string]bool{}
	for _, kv := range given {
		name, text, ok := strings.Cut(kv, "=")
		q, known := m.Question(name)
		switch {
		case !ok:
			problems = append(problems, &Malformed{Given: kv})
		case !known:
			problems = append(problems, &Unknown{Name: name, Questions: m.QuestionNames()})
		case answered[name]:
			problems = append(problems, &Twice{Name: name})
		default:
			answered[name] = true
			taken, err := q.Take(text)
			if err != nil {
				problems = append(problems, &NotTaken{Name: name, Answer: text, Reason: err})
				continue
			}
			answers.Add(taken)
		}
	}
	var missing []*manifest.Question
	for i := range m.Questions {
		q := &m.Questions[i]
		def, hasDefault := q.DefaultAnswer()
		switch {
		case answered[q.Name]:
		case defaults && hasDefault:
			answers.Add(def)
		case ask:
			missing = append(missing, q)
		default:
			problems = append(problems, &Missing{Question: q})
		}
	}
	return answers, missing, problems
}

// Error is an answer that cannot be taken, a sealed set (decision 17):
// internal/cli gives each kind its exit code. Each quotes what the person
// gave and the template does not take, an answer or a question's name, with
// %+q, every character outside ASCII escaped: Go counts a Hangul filler as
// printable, so %q would print the very character refused (bug-5). A name
// given twice or missing is one the template asks, and stays as it is.
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

func (e *Malformed) Error() string { return fmt.Sprintf("the answer %+q has no =", e.Given) }
func (e *Unknown) Error() string   { return fmt.Sprintf("no question %+q", e.Name) }
func (e *Twice) Error() string     { return "the answer to " + e.Name + " twice" }
func (e *NotTaken) Error() string {
	return fmt.Sprintf("the answer to %s, %+q: %v", e.Name, e.Answer, e.Reason)
}
func (e *Missing) Error() string { return "no answer to " + e.Question.Name }
func (e *NotAnswered) Error() string {
	return fmt.Sprintf("no answer to %s: %v", e.Question.Name, e.Err)
}
func (e *NotAnswered) Unwrap() error { return e.Err }
