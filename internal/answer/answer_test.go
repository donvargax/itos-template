package answer

import (
	"errors"
	"testing"

	"github.com/donvargax/itos-template/internal/manifest"
)

const manifestText = `version: 1
stacks:
  - name: sh
questions:
  - name: name
    literal: acme-widget
    question: Name?
    pattern: "[a-z]+(-[a-z]+)*"
    case_forms: true
  - name: owner
    literal: Acme Corp
    question: Owner?
    default: Nobody
`

func parse(t *testing.T) *manifest.Manifest {
	t.Helper()
	m, err := manifest.Parse([]byte(manifestText))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestResolveNamesEveryMissingOneAndNeverAsks(t *testing.T) {
	m := parse(t)
	_, err := Resolve(m, nil, false)
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok || len(joined.Unwrap()) != 2 {
		t.Fatalf("Resolve = %v", err)
	}
	for i, name := range []string{"name", "owner"} {
		var missing *Missing
		if !errors.As(joined.Unwrap()[i], &missing) || missing.Question.Name != name {
			t.Errorf("problem %d is %v, not the missing %s", i, joined.Unwrap()[i], name)
		}
	}
	got, err := Resolve(m, Given{"name=blue-fox"}, true)
	if err != nil || got["name"] != "blue-fox" || got["owner"] != "Nobody" {
		t.Errorf("Resolve = %v, %v", got, err)
	}
}

func TestReadReturnsTheMissingQuestionsToAsk(t *testing.T) {
	m := parse(t)
	answers, missing, problems := Read(m, Given{"name=blue-fox", "name=red-fox", "nope", "who=x"}, false, true)
	if len(missing) != 1 || missing[0].Name != "owner" {
		t.Errorf("missing %v", missing)
	}
	if len(problems) != 3 {
		t.Fatalf("problems %v", problems)
	}
	var twice *Twice
	var malformed *Malformed
	var unknown *Unknown
	var notTaken *NotTaken
	if !errors.As(problems[0], &twice) || twice.Name != "name" {
		t.Errorf("problem 0 is %v", problems[0])
	}
	if !errors.As(problems[1], &malformed) || malformed.Given != "nope" {
		t.Errorf("problem 1 is %v", problems[1])
	}
	if !errors.As(problems[2], &unknown) || unknown.Name != "who" || len(unknown.Questions) != 2 {
		t.Errorf("problem 2 is %v", problems[2])
	}
	if answers["name"] != "blue-fox" {
		t.Errorf("answers %v", answers)
	}
	_, _, problems = Read(m, Given{"name=Blue"}, true, false)
	if len(problems) != 1 || !errors.As(problems[0], &notTaken) || notTaken.Answer != "Blue" || notTaken.Reason == nil {
		t.Errorf("an answer its pattern refuses: %v", problems)
	}
}

func TestReadTakesADefaultOnlyWhenAskedTo(t *testing.T) {
	m := parse(t)
	answers, missing, problems := Read(m, Given{"name=blue-fox"}, true, true)
	if len(problems) != 0 || len(missing) != 0 || answers["owner"] != "Nobody" {
		t.Errorf("Read = %v, %v, %v", answers, missing, problems)
	}
	_, _, problems = Read(m, Given{"name=blue-fox"}, false, false)
	var missingOwner *Missing
	if len(problems) != 1 || !errors.As(problems[0], &missingOwner) || *missingOwner.Question.Default != "Nobody" {
		t.Errorf("problems %v", problems)
	}
}
