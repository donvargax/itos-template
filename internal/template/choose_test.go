package template

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

func choose(t *testing.T, choice manifest.Choice, given answer.Given, defaults bool, terminal *porttest.Terminal) (manifest.Combination, answer.Set, error) {
	t.Helper()
	m := open(t, acme()).Manifest
	if terminal == nil {
		return Choose(m, choice, given, defaults, nil)
	}
	return Choose(m, choice, given, defaults, terminal)
}

// problems are the errors err joins, or err.
func problems(err error) []error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}
	return []error{err}
}

func TestChooseAsksOnATerminalForWhatTheCommandLineLeftOut(t *testing.T) {
	terminal := &porttest.Terminal{Lines: []string{"sh", "blue-fox", ""}}
	c, answers, err := choose(t, manifest.Choice{}, nil, false, terminal)
	if err != nil {
		t.Fatal(err)
	}
	if len(terminal.Asked) != 3 {
		t.Fatalf("asked %+v", terminal.Asked)
	}
	stack, isStack := terminal.Asked[0].(port.StackChoice)
	name, isName := terminal.Asked[1].(port.Answer)
	owner, isOwner := terminal.Asked[2].(port.Answer)
	if !isStack || !slices.Equal(stack.Stacks, []string{"sh", "py"}) {
		t.Errorf("asked first %+v, not the choice of a stack among sh, py", terminal.Asked[0])
	}
	if !isName || name.Text != "Name?" || name.Default != nil || !isOwner || owner.Text != "Owner?" || owner.Default == nil || *owner.Default != "Nobody" {
		t.Errorf("then asked %+v and %+v, not the manifest's questions", terminal.Asked[1], terminal.Asked[2])
	}
	if c.Name() != "sh" || answers["name"] != "blue-fox" || answers["owner"] != "Nobody" {
		t.Errorf("chose %s, %v", c.Name(), answers)
	}
}

func TestChooseAsksAgainForAStackTheTemplateLacks(t *testing.T) {
	terminal := &porttest.Terminal{Lines: []string{"rust", "py"}}
	c, _, err := choose(t, manifest.Choice{}, answer.Given{"name=blue-fox"}, true, terminal)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name() != "py" || len(terminal.Asked) != 2 {
		t.Errorf("chose %s, asked %+v", c.Name(), terminal.Asked)
	}
	var unknown *UnknownStack
	if len(terminal.Refused) != 1 || !errors.As(terminal.Refused[0], &unknown) || unknown.Name != "rust" || !slices.Equal(unknown.Stacks, []string{"sh", "py"}) {
		t.Errorf("refused %v, not rust as a stack among sh, py", terminal.Refused)
	}
}

func TestChooseRefusesWhenTheInputEndsBeforeAnAnswer(t *testing.T) {
	_, _, err := choose(t, manifest.Choice{Stack: "sh"}, nil, false, &porttest.Terminal{})
	var unanswered *answer.NotAnswered
	if !errors.As(err, &unanswered) || unanswered.Question.Name != "name" || !errors.Is(err, porttest.ErrInputEnded) {
		t.Fatalf("Choose = %v", err)
	}
	_, _, err = choose(t, manifest.Choice{}, nil, false, &porttest.Terminal{})
	var none *NoStack
	if !errors.As(err, &none) || !slices.Equal(none.Stacks, []string{"sh", "py"}) {
		t.Fatalf("Choose with no stack answered = %v", err)
	}
}

// Without a terminal nothing is asked: every name the manifest does not
// know and every answer missing or not taken is named, together.
func TestChooseNamesEveryUnknownNameAndAnswerTogether(t *testing.T) {
	_, _, err := choose(t, manifest.Choice{Stack: "rust", Features: []string{"nosuch"}}, answer.Given{"name=Blue"}, false, nil)
	got := problems(err)
	var stack *UnknownStack
	var feature *UnknownFeature
	var notTaken *answer.NotTaken
	var missing *answer.Missing
	if len(got) != 4 || !errors.As(got[0], &stack) || !errors.As(got[1], &feature) || !errors.As(got[2], &notTaken) || !errors.As(got[3], &missing) {
		t.Fatalf("Choose = %v", err)
	}
	if stack.Name != "rust" || feature.Name != "nosuch" || len(feature.Known) != 0 || missing.Question.Name != "owner" {
		t.Errorf("the problems are %+v, %+v, %+v", stack, feature, missing)
	}
	_, _, err = choose(t, manifest.Choice{Features: []string{"extra"}}, answer.Given{"name=blue-fox"}, true, nil)
	var noStack *NoStack
	if got := problems(err); len(got) != 1 || !errors.As(got[0], &noStack) {
		t.Errorf("no stack: %v", err)
	}
	_, _, err = choose(t, manifest.Choice{Stack: "sh", Features: []string{"nosuch"}}, answer.Given{"name=blue-fox"}, true, nil)
	if !errors.As(err, &feature) || feature.Stack != "sh" || !slices.Equal(feature.Known, []string{"extra", "more"}) {
		t.Errorf("a feature the stack lacks: %v", err)
	}
}

// A combination the template refuses is reported only when every name is
// known and every answer taken, and nothing is asked for it.
func TestChooseRefusesACombinationTheTemplateRefusesAskingNothing(t *testing.T) {
	terminal := &porttest.Terminal{Lines: []string{"blue-fox", ""}}
	_, _, err := choose(t, manifest.Choice{Stack: "sh", Features: []string{"tool", "more"}}, nil, false, terminal)
	got := problems(err)
	var other *OtherStack
	var needs *Needs
	if len(got) != 2 || !errors.As(got[0], &other) || !errors.As(got[1], &needs) {
		t.Fatalf("Choose = %v", err)
	}
	if other.Feature.Branch() != "py/tool" || other.Stack != "sh" || needs.Feature != "more" || needs.Need != "extra" {
		t.Errorf("the problems are %+v, %+v", other, needs)
	}
	if len(terminal.Asked) != 0 {
		t.Errorf("asked %+v", terminal.Asked)
	}
	_, _, err = choose(t, manifest.Choice{Stack: "sh", Features: []string{"tool"}}, answer.Given{"nope", "name=blue-fox"}, true, nil)
	var malformed *answer.Malformed
	if got := problems(err); len(got) != 1 || !errors.As(got[0], &malformed) {
		t.Errorf("a refusal and a malformed answer: %v", err)
	}
}

func TestChooseRefusesACombinationTheManifestListsAsUnsupported(t *testing.T) {
	m, err := manifest.Parse([]byte(strings.Replace(manifestText, "template_only:", "unsupported:\n  - stack: sh\n    features: [extra]\ntemplate_only:", 1)))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = Choose(m, manifest.Choice{Stack: "sh", Features: []string{"extra"}}, answer.Given{"name=blue-fox"}, true, nil)
	var unsupported *Unsupported
	if !errors.As(err, &unsupported) || unsupported.Combination.Name() != "sh + extra" {
		t.Fatalf("Choose = %v", err)
	}
	if _, _, err := Choose(m, manifest.Choice{Stack: "sh"}, answer.Given{"name=blue-fox"}, true, nil); err != nil {
		t.Errorf("the stack alone, which is supported: %v", err)
	}
}

func TestChooseTakesAFeatureByItsBranchAndOrdersTheFeaturesAsTheManifest(t *testing.T) {
	c, _, err := choose(t, manifest.Choice{Stack: "sh", Features: []string{"more", "sh/extra"}}, answer.Given{"name=blue-fox"}, true, nil)
	if err != nil || c.Name() != "sh + extra + more" {
		t.Errorf("Choose = %s, %v", c.Name(), err)
	}
}
