// Package cli is the UI the slices share (decision 17): where a command
// reads and writes, the asking on a terminal, the lines on stderr and the
// --json failure object, and the one place an error becomes an exit code.
//
// The domain's errors and infra's are sealed sets of plain types, carrying
// no exit code, rule ID or wording. Here each set has one switch giving each
// kind its code (docs/CLI.md, "Exit codes"), its rule ID for --json and its
// line for people (rule 32), every kind classified: gochecksumtype refuses a
// switch that leaves one out, and none has a default. Only an error no
// switch classified, a bug, exits 70 (rule 31). Problems that come joined
// (errors.Join), a choice's or a manifest's, are each a line and a problem
// of their own, under one code.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/disk"
	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/prompt"
	"github.com/donvargax/itos-template/internal/render"
	"github.com/donvargax/itos-template/internal/tempdir"
	"github.com/donvargax/itos-template/internal/template"
	"github.com/donvargax/itos-template/internal/template/port"
)

// The exit codes (docs/CLI.md, "Exit codes").
const (
	CodeRefused     = 1
	CodeUsage       = 2
	CodeEnvironment = 3
	CodeInternal    = 70
)

// UI is a command's ends: what it reads, where it writes, and whether
// stdin and stdout are a terminal, the only case it asks.
type UI struct {
	In       io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	Terminal bool
}

// Asker asks on the terminal, questions on stderr and answers read from In;
// nil when there is no terminal, so nothing is asked.
func (u *UI) Asker() port.Asker {
	if !u.Terminal {
		return nil
	}
	return prompt.NewLines(u.In, u.Stderr)
}

// Problem is one thing wrong, as --json gives it: its rule ID and its
// sentence for people.
type Problem struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// Fail reports err, a command refusing or failing, and returns its exit
// code: with withJSON the failure's object on stdout first (rule 29), then
// each problem a line on stderr.
func (u *UI) Fail(err error, withJSON bool) int {
	problems, code := report(err)
	return u.fail(problems, code, withJSON)
}

// Usage reports err, a command line kong could not parse, as Fail does: a
// usage error, exit 2. kong's own code for one is 80.
func (u *UI) Usage(err error, withJSON bool) int {
	return u.fail([]Problem{{Rule: "usage", Message: err.Error()}}, CodeUsage, withJSON)
}

func (u *UI) fail(problems []Problem, code int, withJSON bool) int {
	if withJSON {
		u.JSON(struct {
			Schema   int       `json:"schema"`
			OK       bool      `json:"ok"`
			Problems []Problem `json:"problems"`
		}{1, false, problems})
	}
	for _, p := range problems {
		u.Line(p.Message)
	}
	return code
}

// JSON writes v on stdout as one line of JSON, as --json prints it; a
// failure to is a line on stderr.
func (u *UI) JSON(v any) {
	enc := json.NewEncoder(u.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		u.Line(err.Error())
	}
}

// Line writes message on stderr as a line for people, itos-template: first
// (rule 32). A stderr that cannot be written leaves nowhere else to say so.
func (u *UI) Line(message string) {
	_, _ = fmt.Fprintf(u.Stderr, "itos-template: %s\n", message)
}

// Code is err's exit code.
func Code(err error) int {
	_, code := report(err)
	return code
}

// Message is err as people read it: each problem's sentence, a line each.
func Message(err error) string {
	problems, _ := report(err)
	lines := make([]string, len(problems))
	for i, p := range problems {
		lines[i] = p.Message
	}
	return strings.Join(lines, "\n")
}

// report is every problem of err, joined ones each on its own, and the exit
// code of them all: the highest.
func report(err error) ([]Problem, int) {
	var problems []Problem
	code := 0
	for _, e := range leaves(err) {
		c, p := classify(e)
		problems = append(problems, p...)
		code = max(code, c)
	}
	return problems, code
}

// leaves are the errors err joins (errors.Join), each in turn, or err.
func leaves(err error) []error {
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return []error{err}
	}
	var all []error
	for _, e := range joined.Unwrap() {
		all = append(all, leaves(e)...)
	}
	return all
}

// one is a problem of its rule and its sentence, format with args.
func one(rule, format string, args ...any) []Problem {
	return []Problem{{Rule: rule, Message: fmt.Sprintf(format, args...)}}
}

// internal is err as an internal error, a defect of ours.
func internal(err error) []Problem {
	return []Problem{{Rule: "internal", Message: err.Error()}}
}

// classify is err's exit code and problems, from the sealed set it is of,
// the domain's first, as a domain error may wrap infra's: only an error of
// none is an internal one, exit 70.
func classify(err error) (int, []Problem) {
	var te template.Error
	var re render.Error
	var ae answer.Error
	var pe project.Error
	var ge git.Error
	var de disk.Error
	var tu *tempdir.Unmakeable
	switch {
	case errors.As(err, &te):
		return templateProblem(te)
	case errors.As(err, &re):
		return renderProblem(re)
	case errors.As(err, &ae):
		return answerProblem(ae)
	case errors.As(err, &pe):
		return projectProblem(pe)
	case errors.As(err, &ge):
		return gitProblem(ge)
	case errors.As(err, &de):
		return diskProblem(de)
	case errors.As(err, &tu):
		return CodeEnvironment, one("temporary-folder", "cannot make a temporary folder: %v", tu.Err)
	}
	return CodeInternal, internal(err)
}
