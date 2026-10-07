package answer

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"go.yaml.in/yaml/v3"
	"pgregory.net/rapid"

	"github.com/donvargax/itos-template/internal/caseform"
	"github.com/donvargax/itos-template/internal/manifest"
)

// patterns are patterns a question may give its answers; "" is none.
var patterns = []string{"", `[a-z]+(-[a-z]+)*`, `[0-9]{1,3}`, `v[0-9]+\.[0-9]+`, `[A-Z][a-z]*( [A-Z][a-z]*)*`, `[a-z0-9-]+`}

// question is the i-th question of a manifest, with or without case forms,
// a pattern and a default: a default its pattern makes or a kebab answer
// most often, any text otherwise, which its question may refuse.
func question(t *rapid.T, i int) manifest.Question {
	q := manifest.Question{
		Name:      fmt.Sprintf("q%d", i),
		Literal:   fmt.Sprintf("Literal %d", i),
		Question:  "Which?",
		Pattern:   rapid.SampledFrom(patterns).Draw(t, "pattern"),
		CaseForms: rapid.Bool().Draw(t, "case forms"),
	}
	if q.CaseForms {
		q.Literal = fmt.Sprintf("q%d-literal", i)
	}
	answers := []*rapid.Generator[string]{rapid.StringMatching(`[a-z0-9]{1,4}(-[a-z0-9]{1,4}){0,2}`), rapid.StringMatching(`[ -~]{0,8}`)}
	if q.Pattern != "" {
		answers = append(answers, rapid.StringMatching(q.Pattern))
	}
	q.Default = rapid.Ptr(rapid.OneOf(answers...), true).Draw(t, "default")
	return q
}

// takes is whether q takes answer, worked out here from what
// docs/manifest.md says, apart from Question.Check: never empty, the
// pattern matched whole, and with case forms lowercase words joined by
// dashes.
func takes(q manifest.Question, answer string) bool {
	if answer == "" || q.Pattern != "" && !regexp.MustCompile(`^(?:`+q.Pattern+`)$`).MatchString(answer) {
		return false
	}
	_, err := caseform.Parse(answer)
	return !q.CaseForms || err == nil
}

// A default is always an answer its question takes: the manifest takes
// one exactly when its question does, and with --defaults every question
// with a default is answered by it, and nothing else is asked or refused.
func TestADefaultIsAlwaysAnAnswerItsQuestionTakes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := manifest.Manifest{Version: 2, Stacks: []manifest.Stack{{Name: "go"}}}
		for i := range rapid.IntRange(1, 3).Draw(t, "questions") {
			m.Questions = append(m.Questions, question(t, i))
		}
		taken := true
		for _, q := range m.Questions {
			taken = taken && (q.Default == nil || takes(q, *q.Default))
		}
		data, err := yaml.Marshal(&m)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := manifest.Parse(data)
		if taken != (err == nil) {
			t.Fatalf("every default taken is %v, and Parse = %v", taken, err)
		}
		if err != nil {
			return
		}
		answers, missing, problems := Read(parsed, nil, true, false)
		if len(missing) != 0 {
			t.Fatalf("asked %v", missing)
		}
		for i := range parsed.Questions {
			q := &parsed.Questions[i]
			if q.Default == nil {
				continue
			}
			if err := q.Check(*q.Default); err != nil {
				t.Fatalf("%s does not take its default %q: %v", q.Name, *q.Default, err)
			}
			if answers[q.Name] != *q.Default {
				t.Fatalf("%s is answered %q, not its default %q", q.Name, answers[q.Name], *q.Default)
			}
		}
		for _, p := range problems {
			var none *Missing
			if !errors.As(p, &none) || none.Question.Default != nil {
				t.Fatalf("with every default taken, a problem: %v", p)
			}
		}
	})
}
