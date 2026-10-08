package template

import (
	"errors"
	"slices"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/template/port"
)

// Choose is the combination choice names, checked against m, and the
// answers to m's questions: those given, with defaults a missing one's
// default, and what is still missing asked of ask, when there is one (a
// terminal; nil when there is none). What it asks is data, the stacks to
// choose among or the manifest's question, and a stack not among them is an
// *UnknownStack: the wording is the UI's (decision 17).
//
// A stack not named is asked first, as the features are named within it.
// Then every problem is found and returned, joined: first the names the
// manifest does not know and the answers it does not take, every one, a
// usage error; only when there are none, the combinations the template
// refuses (a feature of another stack, one without a feature it needs, a
// combination listed as unsupported). Only then are the answers asked, so
// nobody answers questions for a refusal.
func Choose(m *manifest.Manifest, choice manifest.Choice, given answer.Given, defaults bool, ask port.Asker) (manifest.Combination, answer.Set, error) {
	var unknown, refused []error
	stackName := choice.Stack
	if stackName == "" && ask != nil && len(m.Stacks) > 0 {
		var err error
		stackName, err = ask.Ask(port.StackChoice{
			Stacks: m.StackNames(),
			Check: func(a string) error {
				if _, ok := m.Stack(a); !ok {
					return &UnknownStack{Name: a, Stacks: m.StackNames()}
				}
				return nil
			},
		})
		if err != nil {
			return manifest.Combination{}, nil, &NoStack{Stacks: m.StackNames()}
		}
	}
	stack, stackOK := m.Stack(stackName)
	switch {
	case stackName == "":
		unknown = append(unknown, &NoStack{Stacks: m.StackNames()})
	case !stackOK:
		unknown = append(unknown, &UnknownStack{Name: stackName, Stacks: m.StackNames()})
	}

	var chosen []manifest.Feature
	for _, name := range choice.Features {
		found := m.FeaturesNamed(name, stackName)
		switch {
		case len(found) == 0:
			unknown = append(unknown, &UnknownFeature{Name: name, Stack: stackName, Known: m.FeatureNames(stackName)})
		case !stackOK:
		case found[0].Stack != stackName:
			refused = append(refused, &OtherStack{Feature: found[0], Stack: stackName})
		default:
			chosen = append(chosen, found[0])
		}
	}
	c := manifest.Combination{Stack: stack, Features: m.Ordered(chosen)}
	for _, f := range c.Features {
		for _, need := range f.Needs {
			if !slices.ContainsFunc(c.Features, func(other manifest.Feature) bool { return other.Name == need }) {
				refused = append(refused, &Needs{Feature: f.Name, Need: need})
			}
		}
	}
	if stackOK && len(refused) == 0 {
		if _, ok := m.IsUnsupported(c); ok {
			refused = append(refused, &Unsupported{Combination: c})
		}
	}

	answers, missing, problems := answer.Read(m, given, defaults, ask != nil)
	unknown = append(unknown, problems...)
	if len(unknown) > 0 {
		return manifest.Combination{}, nil, errors.Join(unknown...)
	}
	if len(refused) > 0 {
		return manifest.Combination{}, nil, errors.Join(refused...)
	}
	for _, q := range missing {
		given, err := ask.Ask(port.Answer{Text: q.Question, Default: q.Default, Check: q.Check})
		if err != nil {
			return manifest.Combination{}, nil, &answer.NotAnswered{Question: q, Err: err}
		}
		answers[q.Name] = given
	}
	return c, answers, nil
}
