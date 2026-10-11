package prompt

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
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
	got, err := NewLines(strings.NewReader("blue-fox\n"), &out).Ask("Name?", nil, kebab)
	if err != nil || got != "blue-fox" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
	if out.String() != "Name? " {
		t.Errorf("asked %q", out.String())
	}
}

func TestAskAsksAgainUntilTheCheckTakesTheAnswer(t *testing.T) {
	var out strings.Builder
	got, err := NewLines(strings.NewReader("Blue_Fox\r\nblue-fox\r\n"), &out).Ask("Name?", nil, kebab)
	if err != nil || got != "blue-fox" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
	if strings.Count(out.String(), "Name? ") != 2 || !strings.Contains(out.String(), `"Blue_Fox" is not an answer it takes`) {
		t.Errorf("asked %q", out.String())
	}
}

func TestAskShowsAndTakesTheDefaultOnAnEmptyLine(t *testing.T) {
	var out strings.Builder
	module := "example.com/you/project"
	got, err := NewLines(strings.NewReader("\n"), &out).Ask("Module?", &module, nil)
	if err != nil || got != "example.com/you/project" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
	if out.String() != "Module? [example.com/you/project] " {
		t.Errorf("asked %q", out.String())
	}
}

func TestAskTakesALastLineWithNoNewline(t *testing.T) {
	got, err := NewLines(strings.NewReader("blue-fox"), &strings.Builder{}).Ask("Name?", nil, kebab)
	if err != nil || got != "blue-fox" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
}

func TestAskEndsWithErrNoAnswerWhenTheInputEnds(t *testing.T) {
	for _, in := range []string{"", "Blue_Fox\n", "Blue_Fox"} {
		_, err := NewLines(strings.NewReader(in), &strings.Builder{}).Ask("Name?", nil, kebab)
		if !errors.Is(err, ErrNoAnswer) {
			t.Errorf("input %q: Ask = %v, want ErrNoAnswer", in, err)
		}
	}
}

func TestAskShowsTheDefaultOnceEachTimeItAsks(t *testing.T) {
	var out strings.Builder
	module := "example.com/you/project"
	got, err := NewLines(strings.NewReader("Blue_Fox\nblue-fox\n"), &out).Ask("Module?", &module, kebab)
	if err != nil || got != "blue-fox" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
	if want := "Module? [example.com/you/project]   \"Blue_Fox\" is not an answer it takes: lowercase words joined by dashes\nModule? [example.com/you/project] "; out.String() != want {
		t.Errorf("asked %q", out.String())
	}
}

// A line cut short by a read that fails is no answer: the failure is
// returned, the part read never taken, as only the input ending ends a last
// line with no newline.
func TestAskReturnsAReadFailureOverThePartOfALineItRead(t *testing.T) {
	broken := errors.New("the terminal hung up")
	in := io.MultiReader(strings.NewReader("blue-fox"), iotest.ErrReader(broken))
	got, err := NewLines(in, &strings.Builder{}).Ask("Name?", nil, kebab)
	if !errors.Is(err, broken) || got != "" {
		t.Errorf("Ask = %q, %v, want the read's failure", got, err)
	}
}

// A question is asked only when stdin and stdout are both terminals: one of
// them redirected, to a file or a pipe, asks nothing.
func TestTerminalsAreStdinAndStdoutBoth(t *testing.T) {
	const in, out = 0, 1
	for _, c := range []struct {
		terminals []int
		want      bool
	}{{[]int{in, out}, true}, {[]int{in}, false}, {[]int{out}, false}, {nil, false}} {
		isTerminal := func(fd int) bool { return slices.Contains(c.terminals, fd) }
		if got := terminals(isTerminal, in, out); got != c.want {
			t.Errorf("with %v terminals, terminals = %v", c.terminals, got)
		}
	}
}
