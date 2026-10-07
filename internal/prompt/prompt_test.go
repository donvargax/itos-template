package prompt

import (
	"errors"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Asking on a terminal is held here: the scenarios run with no terminal, so
// none can reach it (decision 18).

func kebab(s string) error {
	if s == "" || strings.ContainsAny(s, " _ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return errors.New("lowercase words joined by dashes")
	}
	return nil
}

func TestAskTakesAnAnswerItsCheckTakes(t *testing.T) {
	var out strings.Builder
	got, err := NewLines(strings.NewReader("blue-fox\n"), &out).Ask(port.Question{Text: "Name?", Check: kebab})
	if err != nil || got != "blue-fox" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
	if out.String() != "Name? " {
		t.Errorf("asked %q", out.String())
	}
}

func TestAskAsksAgainUntilTheCheckTakesTheAnswer(t *testing.T) {
	var out strings.Builder
	got, err := NewLines(strings.NewReader("Blue_Fox\r\nblue-fox\r\n"), &out).Ask(port.Question{Text: "Name?", Check: kebab})
	if err != nil || got != "blue-fox" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
	if strings.Count(out.String(), "Name? ") != 2 || !strings.Contains(out.String(), `"Blue_Fox" is not an answer it takes`) {
		t.Errorf("asked %q", out.String())
	}
}

func TestAskShowsAndTakesTheDefaultOnAnEmptyLine(t *testing.T) {
	var out strings.Builder
	got, err := NewLines(strings.NewReader("\n"), &out).Ask(port.Question{Text: "Module?", Default: "example.com/you/project", HasDefault: true})
	if err != nil || got != "example.com/you/project" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
	if out.String() != "Module? [example.com/you/project] " {
		t.Errorf("asked %q", out.String())
	}
}

func TestAskTakesALastLineWithNoNewline(t *testing.T) {
	got, err := NewLines(strings.NewReader("blue-fox"), &strings.Builder{}).Ask(port.Question{Text: "Name?", Check: kebab})
	if err != nil || got != "blue-fox" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
}

func TestAskEndsWithErrNoAnswerWhenTheInputEnds(t *testing.T) {
	for _, in := range []string{"", "Blue_Fox\n", "Blue_Fox"} {
		_, err := NewLines(strings.NewReader(in), &strings.Builder{}).Ask(port.Question{Text: "Name?", Check: kebab})
		if !errors.Is(err, ErrNoAnswer) {
			t.Errorf("input %q: Ask = %v, want ErrNoAnswer", in, err)
		}
	}
}
