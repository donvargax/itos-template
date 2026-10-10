package manifest

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// Version 5 adds setup, on the top for the root, on each stack and on each
// feature: steps, each a list of words as a check is, read as written.
func TestParseReadsSetupFromVersion5(t *testing.T) {
	m, err := Parse([]byte(`version: 5
setup: [[itos, init, --agent-rules], [echo, "", 'it''s']]
stacks: [{name: go, setup: [[go, mod, download]]}]
features: [{name: cli, stack: go, setup: [[go, build, ./cmd/acme-widget]]}]
`))
	if err != nil {
		t.Fatal(err)
	}
	words := func(steps []Step) [][]string {
		var all [][]string
		for _, s := range steps {
			all = append(all, s.Words)
		}
		return all
	}
	for _, c := range []struct {
		where string
		got   [][]string
		want  [][]string
	}{
		{"the root's", words(m.Setup), [][]string{{"itos", "init", "--agent-rules"}, {"echo", "", "it's"}}},
		{"the stack's", words(m.Stacks[0].Setup), [][]string{{"go", "mod", "download"}}},
		{"the feature's", words(m.Features[0].Setup), [][]string{{"go", "build", "./cmd/acme-widget"}}},
	} {
		if !slices.EqualFunc(c.got, c.want, slices.Equal) {
			t.Errorf("%s setup is %q, not %q", c.where, c.got, c.want)
		}
	}
	if acme(t).Setup != nil {
		t.Error("acme, which lists no setup step, lists one")
	}
}

func TestParseSaysSetupIsOfVersion5(t *testing.T) {
	_, err := Parse([]byte("version: 4\nsetup: [[a]]\nstacks: [{name: go, setup: []}]\nfeatures: [{name: cli, stack: go, setup: [[b]]}]\n"))
	var invalid *Invalid
	want := []string{
		"the root's setup is a key of version 5: write version: 5",
		"the stack go's setup is a key of version 5: write version: 5",
		"the feature go/cli's setup is a key of version 5: write version: 5",
	}
	if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, want) {
		t.Errorf("Parse = %v", err)
	}
}

func TestParseSaysASetupStepNamesAProgram(t *testing.T) {
	_, err := Parse([]byte("version: 5\nsetup: [[go], []]\nstacks: [{name: go, setup: [['', x]]}]\n"))
	var invalid *Invalid
	want := []string{
		"the root's setup step 2 names no program: a step is a list of words, the program first",
		"the stack go's setup step 1 names no program: a step is a list of words, the program first",
	}
	if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, want) {
		t.Errorf("Parse = %v", err)
	}
}

func TestParseSaysASetupStepIsAListOfWords(t *testing.T) {
	_, err := Parse([]byte("version: 5\nstacks: [{name: go}]\nsetup:\n  - go mod download\n"))
	if err == nil || !strings.Contains(err.Error(), "line 4: a setup step is a list of words") {
		t.Errorf("Parse = %v", err)
	}
}

// A step is printed for the person to paste, so a word holding a control
// character, which can make a terminal show another step, is refused, each
// named with its line and the character, whoever's step it is; a word that
// only needs quoting is taken.
func TestParseRefusesAControlCharacterInASetupWordNamingItsLine(t *testing.T) {
	_, err := Parse([]byte(`version: 5
setup:
  - [echo, "a\eb", ok]
  - [echo, "\t", "\r\n"]
stacks:
  - name: go
    setup: [[sh, -c, "\x7f"]]
features:
  - {name: cli, stack: go, setup: [[echo, "\u0085", "\x00"]]}
  - {name: web, stack: go, setup: [[echo, "é ünïcode $HOME 'quoted'"]]}
`))
	var invalid *Invalid
	why := ": a terminal shown it can show another step than the one run, so write the word without it"
	want := []string{
		`line 3: the root's setup step 1 holds a control character, U+001B, in the word "a\x1bb"` + why,
		`line 4: the root's setup step 2 holds a control character, U+0009, in the word "\t"` + why,
		`line 4: the root's setup step 2 holds a control character, U+000D, in the word "\r\n"` + why,
		`line 7: the stack go's setup step 1 holds a control character, U+007F, in the word "\x7f"` + why,
		`line 9: the feature go/cli's setup step 1 holds a control character, U+0085, in the word "\u0085"` + why,
		`line 9: the feature go/cli's setup step 1 holds a control character, U+0000, in the word "\x00"` + why,
	}
	if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, want) {
		t.Errorf("Parse = %v", err)
	}
}

// A character that prints as nothing or moves the text around it, a format
// character (Cf) or a line or paragraph separator (Zl, Zp), is refused as a
// control character is, each named with its line and its code point, the
// word escaped so the message itself shows it. A word holding both kinds is
// named once for each kind, by the first character of that kind, however
// many it holds. A word of other Unicode, a no-break space included, is
// taken.
func TestParseRefusesAnInvisibleCharacterInASetupWordNamingItsLine(t *testing.T) {
	_, err := Parse([]byte(`version: 5
setup:
  - [echo, "ma\u202Ede", "\u200B"]
  - [echo, "\u2028", "\u2029", "\u00AD"]
stacks:
  - name: go
    setup: [[sh, -c, "\uFEFFx"]]
features:
  - {name: cli, stack: go, setup: [[echo, "\u200D\e\u202E\x01"]]}
  - {name: web, stack: go, setup: [[echo, "é — ünïcode ‘quoted’ \u00A0"]]}
`))
	var invalid *Invalid
	why := ": a terminal shown it can show another step than the one run, so write the word without it"
	invisible := func(line int, where string, step int, r, word string) string {
		return fmt.Sprintf("line %d: %s setup step %d holds %s, a character that prints as nothing or moves the text around it, in the word %s", line, where, step, r, word) + why
	}
	want := []string{
		invisible(3, "the root's", 1, "U+202E", `"ma\u202ede"`),
		invisible(3, "the root's", 1, "U+200B", `"\u200b"`),
		invisible(4, "the root's", 2, "U+2028", `"\u2028"`),
		invisible(4, "the root's", 2, "U+2029", `"\u2029"`),
		invisible(4, "the root's", 2, "U+00AD", `"\u00ad"`),
		invisible(7, "the stack go's", 1, "U+FEFF", `"\ufeffx"`),
		`line 9: the feature go/cli's setup step 1 holds a control character, U+001B, in the word "\u200d\x1b\u202e\x01"` + why,
		invisible(9, "the feature go/cli's", 1, "U+200D", `"\u200d\x1b\u202e\x01"`),
	}
	if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, want) {
		t.Errorf("Parse = %v\nwant %q", err, want)
	}
}

// A character Unicode lists as Default_Ignorable_Code_Point though it is no
// format character, a letter or a mark drawn as nothing, is refused as one:
// the Hangul fillers and the combining grapheme joiner
// (Other_Default_Ignorable_Code_Point), and the variation selectors, U+FE0F
// after a heart and a supplementary one included. No emoji is excepted. The
// word is escaped to ASCII, since Go counts these characters printable.
func TestParseRefusesADefaultIgnorableCharacterInASetupWordAnEmojiSelectorIncluded(t *testing.T) {
	_, err := Parse([]byte(`version: 5
setup:
  - [echo, "ma\u3164de", "\u115f", "\u1160", "\uffa0"]
  - [echo, "\u034f", "é\u2764\ufe0f", "\U000E0100", "\u180b", "\U000E0001"]
stacks: [{name: go}]
`))
	var invalid *Invalid
	why := ": a terminal shown it can show another step than the one run, so write the word without it"
	invisible := func(line, step int, r, word string) string {
		return fmt.Sprintf("line %d: the root's setup step %d holds %s, a character that prints as nothing or moves the text around it, in the word %s", line, step, r, word) + why
	}
	want := []string{
		invisible(3, 1, "U+3164", `"ma\u3164de"`),
		invisible(3, 1, "U+115F", `"\u115f"`),
		invisible(3, 1, "U+1160", `"\u1160"`),
		invisible(3, 1, "U+FFA0", `"\uffa0"`),
		invisible(4, 2, "U+034F", `"\u034f"`),
		invisible(4, 2, "U+FE0F", `"\u00e9\u2764\ufe0f"`),
		invisible(4, 2, "U+E0100", `"\U000e0100"`),
		invisible(4, 2, "U+180B", `"\u180b"`),
		invisible(4, 2, "U+E0001", `"\U000e0001"`),
	}
	if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, want) {
		t.Errorf("Parse = %v\nwant %q", err, want)
	}
}

// drawnAsNothing holds Unicode's Default_Ignorable_Code_Point, Cf whole,
// with the line and paragraph separators, and nothing a step may hold: a
// letter, an accent, a no-break space, an emoji without its selector.
func TestDrawnAsNothingIsUnicodesDefaultIgnorablesWithTheSeparators(t *testing.T) {
	for _, r := range []rune{0x202E, 0x200B, 0x200C, 0x200D, 0xFEFF, 0x00AD, 0x0600, 0x2028, 0x2029, 0x3164, 0x115F, 0x034F, 0x17B4, 0xE0000, 0xFE00, 0xFE0F, 0x180F, 0xE01EF} {
		if !drawnAsNothing(r) {
			t.Errorf("drawnAsNothing(%U) = false, want true", r)
		}
	}
	for _, r := range []rune{'a', ' ', 0x00E9, 0x00A0, 0x2764, 0x1F600, 0x0301, 0x3000, 0x001B} {
		if drawnAsNothing(r) {
			t.Errorf("drawnAsNothing(%U) = true, want false", r)
		}
	}
}

func TestSetupOfIsTheRootsThenTheStacksThenTheFeaturesInTheManifestsOrder(t *testing.T) {
	m, err := Parse([]byte(`version: 5
setup: [[root]]
stacks: [{name: go, setup: [[stack, one], [stack, two]]}, {name: py, setup: [[py]]}]
features:
  - {name: a, stack: go, setup: [[a]]}
  - {name: b, stack: go, setup: [[b]]}
  - {name: c, stack: go, setup: [[c]]}
`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := m.combination("go", []string{"b", "a"})
	var got []string
	for _, s := range m.SetupOf(b) {
		got = append(got, strings.Join(s.Words, " "))
	}
	want := []string{"root", "stack one", "stack two", "a", "b"}
	if !slices.Equal(got, want) {
		t.Errorf("SetupOf = %q, want %q", got, want)
	}
}

// A manifest with setup steps written back out (as a tool writing the
// format does) reads as the same manifest, each step its words.
func TestASetupStepIsWrittenAsItsWords(t *testing.T) {
	text := "version: 5\nsetup: [[go, mod, download]]\nstacks: [{name: go, setup: [[sh, -c, 'a b']]}]\n"
	m, err := Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(data)
	if err != nil {
		t.Fatalf("written out, the manifest is refused: %v\n%s", err, data)
	}
	if len(again.Setup) != 1 || !slices.Equal(again.Setup[0].Words, Words{"go", "mod", "download"}) ||
		len(again.Stacks[0].Setup) != 1 || !slices.Equal(again.Stacks[0].Setup[0].Words, Words{"sh", "-c", "a b"}) {
		t.Errorf("written out, the manifest reads\n%s", data)
	}
}
