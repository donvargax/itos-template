// Package prompt is the infra that asks a person for what a command line
// left out, on a terminal only (docs/CLI.md, rule 37; decision 11): the
// domain's port.Asker, a plain line prompt, so the pick lists the idea
// new-picker brings (charmbracelet/huh) can take its place without the
// domain changing. It imports no package of ours but the ports it
// implements (decision 17).
package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/donvargax/itos-template/internal/template/port"
)

// ErrNoAnswer is the input ending before an answer was given.
var ErrNoAnswer = errors.New("the input ended before an answer was given")

// Lines asks on Out, a question a line with its default in brackets, and
// reads each answer as a line of In: an empty line takes the default, and
// an answer its check refuses is said why and asked again.
type Lines struct {
	In  *bufio.Reader
	Out io.Writer
}

var _ port.Asker = (*Lines)(nil)

// NewLines asks on out, reading in.
func NewLines(in io.Reader, out io.Writer) *Lines {
	return &Lines{In: bufio.NewReader(in), Out: out}
}

// Ask asks q until it is answered with an answer its check takes, and
// returns that answer.
func (l *Lines) Ask(q port.Question) (string, error) {
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
