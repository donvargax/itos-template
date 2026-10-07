package template

import (
	"strings"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/render"
	"github.com/donvargax/itos-template/internal/template/port"
)

// checker is who each render check makes is committed by: a render no one
// keeps, so a CI with no git identity configured can check a template.
var checker = port.Identity{Name: "itos-template check", Email: "itos-template@localhost"}

// Result is what checking one combination found: Err, why it was not
// rendered, or else each of its checks as it ran.
type Result struct {
	Combination manifest.Combination
	Err         error
	Checks      []Ran
}

// Passed is whether the combination rendered and every check passed.
func (r Result) Passed() bool {
	if r.Err != nil {
		return false
	}
	for _, c := range r.Checks {
		if c.Status != Passed {
			return false
		}
	}
	return true
}

// Ran is a check of a combination: its words as it ran, the answers in
// place of the literals, whether it passed, and what it wrote when it failed.
type Ran struct {
	Check  manifest.Check
	Status Status
	Output []byte // its standard output and error, as they came
}

// Lines are what the check wrote, line by line, as Lines reads them.
func (r Ran) Lines() []string { return Lines(string(r.Output)) }

// Lines are text's lines, a line ending being \n, \r\n or \r alone: the
// last line ending ends the last line, and text with none is no line at all.
func Lines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// Status is how a check ended.
type Status int

// A check passed (exited 0), failed, or was skipped after a failure before
// it.
const (
	Passed Status = iota
	Failed
	Skipped
)

// Check renders every combination the manifest allows with answers, each
// as new renders it (Render) into a temporary folder of folders' by w, runs
// that render's checks there with run and removes the folder, and gives
// each its Result, in the manifest's order, as it is found. A
// combination's checks stop at the first that fails, as a CI job's steps
// do: what follows a failed build only fails after it. Every combination is
// checked whatever failed before it; an error each returns stops the check
// and is returned.
//
// A check's words have the literals replaced by the answers as a text
// file's contents do, so a check can name a file the answers renamed.
func (t *Template) Check(answers answer.Set, folders port.Folders, w project.Writer, run port.Runner, each func(Result) error) error {
	replacer := render.NewReplacer(t.Manifest.Replacements(answers))
	for _, c := range t.Manifest.Combinations() {
		if err := each(t.checkCombination(c, answers, replacer, folders, w, run)); err != nil {
			return err
		}
	}
	return nil
}

func (t *Template) checkCombination(c manifest.Combination, answers answer.Set, r *render.Replacer, folders port.Folders, w project.Writer, run port.Runner) Result {
	result := Result{Combination: c}
	dir, err := folders.Make()
	if err != nil {
		result.Err = err
		return result
	}
	defer folders.Remove(dir)
	if _, err := t.Render(c, answers, project.Folder{Path: dir}, &checker, w); err != nil {
		result.Err = err
		return result
	}
	failed := false
	for _, check := range t.Manifest.ChecksOf(c) {
		words := make(manifest.Check, len(check))
		for i, word := range check {
			words[i] = r.Text(word)
		}
		if failed {
			result.Checks = append(result.Checks, Ran{Check: words, Status: Skipped})
			continue
		}
		output, ok := run.Run(dir, words)
		ran := Ran{Check: words, Status: Passed, Output: output}
		if !ok {
			ran.Status, failed = Failed, true
		}
		result.Checks = append(result.Checks, ran)
	}
	return result
}
