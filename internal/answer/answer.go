// Package answer reads the answers to a template's questions given on a
// command line, each name=answer, against its manifest: each checked by its
// question, a missing one's default taken when asked to (--defaults), and
// what is still missing either returned to be asked on a terminal or
// refused (decision 11). Every problem is a usage problem, all reported
// together, so one run names every answer to fix.
package answer

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/problem"
)

// Resolve is the answers to m's questions, never asked: those given, and
// with defaults a missing one's default. Every problem is a usage problem,
// all reported together: a malformed or unknown answer, one given twice,
// one its question does not take, and each missing one.
func Resolve(m *manifest.Manifest, given []string, defaults bool) (map[string]string, error) {
	answers, _, problems := Read(m, given, defaults, false)
	if len(problems) > 0 {
		return nil, &problem.Failure{Code: problem.CodeUsage, Problems: problems}
	}
	return answers, nil
}

// Read reads the answers given against m's questions, takes a missing one's
// default with defaults, and returns the questions still missing to ask
// them when ask; without ask each missing one is a problem.
func Read(m *manifest.Manifest, given []string, defaults, ask bool) (map[string]string, []*manifest.Question, []problem.Problem) {
	var problems []problem.Problem
	add := func(rule, format string, args ...any) {
		problems = append(problems, problem.Problem{Rule: rule, Message: fmt.Sprintf(format, args...)})
	}
	answers := map[string]string{}
	for _, kv := range given {
		name, answer, ok := strings.Cut(kv, "=")
		q, known := m.Question(name)
		switch {
		case !ok:
			add("answer-malformed", "--answer takes name=answer, and %q has no =", kv)
		case !known:
			add("answer-unknown", "the template asks no question %s: its questions are %s", name, strings.Join(m.QuestionNames(), ", "))
		case hasKey(answers, name):
			add("answer-twice", "the answer to %s is given twice", name)
		default:
			if err := q.Check(answer); err != nil {
				add("answer-malformed", "the answer to %s, %q, is not one it takes: %v", name, answer, err)
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
		case q.Default != nil:
			add("answer-missing", "no answer to %s (%s): give one with --answer %s=<answer>, or take its default, %s, with --defaults", q.Name, q.Question, q.Name, *q.Default)
		default:
			add("answer-missing", "no answer to %s (%s): give one with --answer %s=<answer>", q.Name, q.Question, q.Name)
		}
	}
	return answers, missing, problems
}

func hasKey(m map[string]string, k string) bool {
	_, ok := m[k]
	return ok
}
