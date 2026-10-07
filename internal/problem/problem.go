// Package problem is what a command says when it refuses or fails: every
// problem, each with its rule ID for --json and a sentence for people, and
// the exit code (docs/CLI.md, "Exit codes"). The code is chosen where the
// failure is made, from its kind (docs/CLI.md, rule 31), so a *Failure
// carries it up to the command's edge, which only reports it: a line on
// stderr for each problem (rule 32), and with --json the failure's object
// on stdout (rule 29).
package problem

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// The exit codes a failure carries (docs/CLI.md, "Exit codes").
const (
	CodeRefused     = 1
	CodeUsage       = 2
	CodeEnvironment = 3
	CodeInternal    = 70
)

// Problem is one thing wrong, its rule ID and a sentence for people.
type Problem struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// Failure is a command refusing or failing: its exit code and every problem.
type Failure struct {
	Code     int
	Problems []Problem
}

func (f *Failure) Error() string {
	var lines []string
	for _, p := range f.Problems {
		lines = append(lines, p.Message)
	}
	return strings.Join(lines, "\n")
}

// New is a failure of one problem, its message format with args.
func New(code int, rule, format string, args ...any) *Failure {
	return &Failure{Code: code, Problems: []Problem{{Rule: rule, Message: fmt.Sprintf(format, args...)}}}
}

// Internal is err as an internal error, a defect of ours: exit 70.
func Internal(err error) *Failure {
	return New(CodeInternal, "internal", "%v", err)
}

// As is err as a *Failure, an error no code classified being an internal
// one (docs/CLI.md, rule 31).
func As(err error) *Failure {
	var f *Failure
	if errors.As(err, &f) {
		return f
	}
	return Internal(err)
}

// Report writes f at a command's edge, with withJSON its object on stdout
// first, then each problem a line on stderr, and returns its exit code.
func (f *Failure) Report(stdout, stderr io.Writer, withJSON bool) int {
	if withJSON {
		WriteJSON(stdout, stderr, struct {
			Schema   int       `json:"schema"`
			OK       bool      `json:"ok"`
			Problems []Problem `json:"problems"`
		}{1, false, f.Problems})
	}
	for _, p := range f.Problems {
		Line(stderr, p.Message)
	}
	return f.Code
}

// WriteJSON writes v on stdout as one line of JSON, as --json prints it;
// a failure to is a line on stderr.
func WriteJSON(stdout, stderr io.Writer, v any) {
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		Line(stderr, err.Error())
	}
}

// Line writes message on stderr as a line for people, itos-template: first
// (docs/CLI.md, rule 32). A stderr that cannot be written leaves nowhere
// else to say so.
func Line(stderr io.Writer, message string) {
	_, _ = fmt.Fprintf(stderr, "itos-template: %s\n", message)
}
