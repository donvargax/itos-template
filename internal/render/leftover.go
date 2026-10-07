package render

import (
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos-template/internal/caseform"
	"github.com/donvargax/itos-template/internal/template/port"
)

// Leftover is a literal a render kept: the text found, in the path Path,
// on the line Line of its contents (from 1), or in the path itself when
// Line is 0.
type Leftover struct {
	Path string
	Line int
	Text string
}

// Scan finds the literals a render left in a form no answer replaced
// (decision 21): check's own scan, so an author either writes each spot in
// one of the five forms or names it otherwise.
type Scan struct {
	patterns []*regexp.Regexp
}

// NewScan looks for each case-forms literal's words, in order and in any
// case, joined by nothing, a space, ".", "-", "_" or "/" ("Acme Widget" in
// a heading, "acmewidget" in a host name: the person's call, 2026-10-07),
// and for each literal without case forms as it is written.
func NewScan(words []caseform.Words, written []string) *Scan {
	s := &Scan{}
	for _, w := range words {
		quoted := make([]string, len(w))
		for i, word := range w {
			quoted[i] = regexp.QuoteMeta(word)
		}
		s.patterns = append(s.patterns, regexp.MustCompile(`(?i)`+strings.Join(quoted, `[ ./_-]?`)))
	}
	for _, text := range written {
		s.patterns = append(s.patterns, regexp.MustCompile(regexp.QuoteMeta(text)))
	}
	return s
}

// lineBreak ends a line: \n, \r\n, or \r alone, as check's report counts
// a check's output's lines.
var lineBreak = regexp.MustCompile(`\r\n|\r|\n`)

// Leftovers are the literals left in files, a render's planned files: in
// each file's path, then on each line of a text file's contents (no NUL in
// its first 8000 bytes, as git tells them) or a link's target, in the
// files' order and each line's from its start. A binary file is not read,
// as no answer replaces anything in it.
func (s *Scan) Leftovers(files []port.File) []Leftover {
	var found []Leftover
	for _, f := range files {
		found = append(found, s.find(f.Path, 0, f.Path)...)
		if f.Mode&fs.ModeSymlink == 0 && !isText(f.Data) {
			continue
		}
		for i, line := range lineBreak.Split(string(f.Data), -1) {
			found = append(found, s.find(f.Path, i+1, line)...)
		}
	}
	return found
}

// find is every literal in text, the path's or its line's, in the order
// they start.
func (s *Scan) find(path string, line int, text string) []Leftover {
	type match struct {
		at   int
		text string
	}
	var matches []match
	for _, p := range s.patterns {
		for _, m := range p.FindAllStringIndex(text, -1) {
			matches = append(matches, match{m[0], text[m[0]:m[1]]})
		}
	}
	slices.SortStableFunc(matches, func(a, b match) int { return a.at - b.at })
	found := make([]Leftover, len(matches))
	for i, m := range matches {
		found[i] = Leftover{Path: path, Line: line, Text: m.text}
	}
	return found
}
