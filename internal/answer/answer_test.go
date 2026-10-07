package answer

import (
	"errors"
	"testing"

	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/problem"
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

func TestResolveNamesEveryMissingOneAndNeverAsks(t *testing.T) {
	m, err := manifest.Parse([]byte(manifestText))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Resolve(m, nil, false)
	var f *problem.Failure
	if !errors.As(err, &f) || f.Code != problem.CodeUsage || len(f.Problems) != 2 {
		t.Fatalf("Resolve = %v", err)
	}
	got, err := Resolve(m, []string{"name=blue-fox"}, true)
	if err != nil || got["name"] != "blue-fox" || got["owner"] != "Nobody" {
		t.Errorf("Resolve = %v, %v", got, err)
	}
}

func TestReadReturnsTheMissingQuestionsToAsk(t *testing.T) {
	m, err := manifest.Parse([]byte(manifestText))
	if err != nil {
		t.Fatal(err)
	}
	answers, missing, problems := Read(m, []string{"name=blue-fox", "name=red-fox", "nope"}, false, true)
	if len(missing) != 1 || missing[0].Name != "owner" {
		t.Errorf("missing %v", missing)
	}
	if len(problems) != 2 || problems[0].Rule != "answer-twice" || problems[1].Rule != "answer-malformed" {
		t.Errorf("problems %v", problems)
	}
	if answers["name"] != "blue-fox" {
		t.Errorf("answers %v", answers)
	}
}
