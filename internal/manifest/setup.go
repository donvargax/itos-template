package manifest

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// Step is a setup step, from version 5: its words, the program first, as a
// check's are, which new prints for the person to run and never runs. It is
// written as its words, a list, never one string.
type Step struct {
	Words Words

	lines []int // each word's line in the manifest
}

// UnmarshalYAML reads a step, refusing a string with a sentence saying why,
// and keeps each word's line, so a problem in a word names where it is.
func (s *Step) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.SequenceNode {
		return fmt.Errorf("line %d: a setup step is a list of words, the program first, as [go, mod, download]: it is printed word by word, so it is never one string", n.Line)
	}
	for _, word := range n.Content {
		s.lines = append(s.lines, word.Line)
	}
	return n.Decode(&s.Words)
}

// MarshalYAML writes a step as its words.
func (s Step) MarshalYAML() (any, error) { return []string(s.Words), nil }

// checkSetup refuses setup before version 5, a step with no program, and a
// word holding a control character, tab and line endings included, or an
// invisible one. A step is printed for the person to read and paste, from a
// template not yet trusted, and quoting cannot make either safe to print: an
// escape sequence can make a terminal show a step other than the one
// pasted, and a character drawnAsNothing holds (a right-to-left override, a
// zero-width space, a Hangul filler, a variation selector, a line
// separator) prints as nothing or moves the text around it, the Trojan
// Source attack (CVE-2021-42574) and the invisible identifier beside it. A
// word holding both kinds is named once for each, by the first character of
// that kind, and the word is quoted with every character outside ASCII
// escaped: a Hangul filler or a variation selector is a printable letter or
// mark to Go, so plain quoting would print the very character refused.
func (m *Manifest) checkSetup(add func(string, ...any)) {
	each := func(where string, steps []Step) {
		if steps != nil && m.Version < 5 {
			add("%s setup is a key of version 5: write version: 5", where)
		}
		for i, step := range steps {
			if len(step.Words) == 0 || step.Words[0] == "" {
				add("%s setup step %d names no program: a step is a list of words, the program first", where, i+1)
			}
			for j, word := range step.Words {
				if at := strings.IndexFunc(word, unicode.IsControl); at >= 0 {
					r, _ := utf8.DecodeRuneInString(word[at:])
					add("line %d: %s setup step %d holds a control character, %U, in the word %+q: a terminal shown it can show another step than the one run, so write the word without it", step.lines[j], where, i+1, r, word)
				}
				if at := strings.IndexFunc(word, drawnAsNothing); at >= 0 {
					r, _ := utf8.DecodeRuneInString(word[at:])
					add("line %d: %s setup step %d holds %U, a character that prints as nothing or moves the text around it, in the word %+q: a terminal shown it can show another step than the one run, so write the word without it", step.lines[j], where, i+1, r, word)
				}
			}
		}
	}
	each("the root's", m.Setup)
	for _, s := range m.Stacks {
		each("the stack "+s.Name+"'s", s.Setup)
	}
	for _, f := range m.Features {
		each("the feature "+f.Branch()+"'s", f.Setup)
	}
}

// drawnAsNothing says whether r is one of Unicode's Default_Ignorable_Code_Point,
// what a renderer shows nothing for, or a line or paragraph separator, which
// breaks the line where a terminal or an editor honours it though it is no
// control character. The set is built from its sources in Go's tables, so it
// grows with Unicode: every format character (Cf), a right-to-left override
// and a zero-width joiner among them, Other_Default_Ignorable_Code_Point (the
// Hangul fillers, the combining grapheme joiner U+034F) and the variation
// selectors, U+FE0F after an emoji included, with Zl and Zp. Cf is taken
// whole, a few format characters a renderer draws (U+0600, the Arabic number
// sign) with it: none has a use in a command.
func drawnAsNothing(r rune) bool {
	return unicode.In(r, unicode.Cf, unicode.Other_Default_Ignorable_Code_Point, unicode.Variation_Selector, unicode.Zl, unicode.Zp)
}

// SetupOf are the setup steps of the combination c, in the order check runs
// its checks: the root's, then its stack's, then each of its features' in
// the manifest's order.
func (m *Manifest) SetupOf(c Combination) []Step {
	return inCheckOrder(m, c, m.Setup, func(s Stack) []Step { return s.Setup }, func(f Feature) []Step { return f.Setup })
}
