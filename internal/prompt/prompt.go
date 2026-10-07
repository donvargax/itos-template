// Package prompt asks a person for what a command line left out, on a
// terminal only (docs/CLI.md, rule 37; decision 11): a plain line prompt
// behind a small interface, so the pick lists the idea new-picker brings
// (charmbracelet/huh) can take its place without new changing.
package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Question is one thing to ask: its text, its default when it has one, and
// the check an answer must pass, asked again until it does.
type Question struct {
	Text       string
	Default    string
	HasDefault bool
	Check      func(answer string) error
}

// Asker asks questions.
type Asker interface {
	Ask(q Question) (string, error)
}

// ErrNoAnswer is the input ending before an answer was given.
var ErrNoAnswer = errors.New("the input ended before an answer was given")

// Lines asks on Out, a question a line with its default in brackets, and
// reads each answer as a line of In: an empty line takes the default, and
// an answer its check refuses is said why and asked again.
type Lines struct {
	In  *bufio.Reader
	Out io.Writer
}

// NewLines asks on out, reading in.
func NewLines(in io.Reader, out io.Writer) *Lines {
	return &Lines{In: bufio.NewReader(in), Out: out}
}

func (l *Lines) Ask(q Question) (string, error) {
	for {
		text := q.Text
		if q.HasDefault {
			text += " [" + q.Default + "]"
		}
		if _, err := fmt.Fprint(l.Out, text+" "); err != nil {
			return "", err
		}
		line, err := l.In.ReadString('\n')
		if err != nil && (!errors.Is(err, io.EOF) || line == "") {
			if errors.Is(err, io.EOF) {
				_, _ = fmt.Fprintln(l.Out)
				return "", ErrNoAnswer
			}
			return "", err
		}
		answer := strings.TrimRight(line, "\r\n")
		if answer == "" && q.HasDefault {
			answer = q.Default
		}
		if q.Check == nil {
			return answer, nil
		}
		checkErr := q.Check(answer)
		if checkErr == nil {
			return answer, nil
		}
		if _, err := fmt.Fprintf(l.Out, "  %q is not an answer it takes: %v\n", answer, checkErr); err != nil {
			return "", err
		}
		if errors.Is(err, io.EOF) {
			return "", ErrNoAnswer
		}
	}
}

// Terminal is whether stdin and stdout are both terminals, the only case a
// question is asked in.
func Terminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
