package render

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/donvargax/itos-template/internal/caseform"
	"github.com/donvargax/itos-template/internal/template/port"
)

// literal is a template's case-forms literal as the manifest takes it: two
// or three words of lowercase letters, so its five forms differ.
var literal = rapid.Custom(func(t *rapid.T) caseform.Words {
	return caseform.Words(rapid.SliceOfN(rapid.StringMatching(`[a-z]{2,5}`), 2, 3).Draw(t, "words"))
})

// replacement is what a render replaces: one or two literals in their five
// forms, each form by an answer typed on one line, never empty as no
// question takes an empty answer, the longest literal first
// as the manifest orders them; and the forms, to put in a file.
type replacement struct {
	*Replacer
	olds []string
}

var replacements = rapid.Custom(func(t *rapid.T) replacement {
	var olds, pairs []string
	for _, words := range rapid.SliceOfN(literal, 1, 2).Draw(t, "literals") {
		olds = append(olds, words.Forms()...)
	}
	olds = slices.Compact(olds)
	sort.SliceStable(olds, func(i, j int) bool { return len(olds[i]) > len(olds[j]) })
	for _, old := range olds {
		pairs = append(pairs, old, rapid.StringMatching(`[^\r\n]{1,8}`).Draw(t, "answer to "+old))
	}
	return replacement{NewReplacer(pairs), olds}
})

// lineEndings are what end a line: \n, \r\n, \r, and nothing at the end of
// a file whose last line has no ending.
var lineEndings = []string{"\n", "\r\n", "\r", ""}

// textFile is a text file's contents: lines of the literals' forms and other
// text, never a NUL, each with any line ending.
func textFile(t *rapid.T, olds []string) []byte {
	var b strings.Builder
	for range rapid.IntRange(0, 6).Draw(t, "lines") {
		for range rapid.IntRange(0, 4).Draw(t, "pieces") {
			b.WriteString(rapid.OneOf(rapid.SampledFrom(olds), rapid.StringMatching(`[^\r\n\x00]{0,6}`)).Draw(t, "piece"))
		}
		b.WriteString(rapid.SampledFrom(lineEndings).Draw(t, "line ending"))
	}
	return []byte(b.String())
}

// binaryFile is a binary file's contents: the literals' forms among any
// bytes, a NUL among the first 8000, as git tells a binary file.
func binaryFile(t *rapid.T, olds []string) []byte {
	var data []byte
	for range rapid.IntRange(0, 6).Draw(t, "pieces") {
		data = append(data, rapid.OneOf(
			rapid.Map(rapid.SampledFrom(olds), func(s string) []byte { return []byte(s) }),
			rapid.SliceOfN(rapid.Byte(), 0, 6),
		).Draw(t, "piece")...)
	}
	return slices.Insert(data, rapid.IntRange(0, len(data)).Draw(t, "NUL at"), 0)
}

// lines are text cut after each line ending, as Contents must keep them.
func lines(text string) []string {
	var cut []string
	for text != "" {
		i := strings.IndexAny(text, "\r\n")
		switch {
		case i < 0:
			i = len(text)
		case strings.HasPrefix(text[i:], "\r\n"):
			i += 2
		default:
			i++
		}
		cut, text = append(cut, text[:i]), text[i:]
	}
	return cut
}

// ending is the line ending a line of lines ends in.
func ending(line string) string {
	return line[len(strings.TrimRight(line, "\r\n")):]
}

// A render works within lines and never touches a line ending: each text
// file it plans is the template's, line by line, the literals replaced in
// each line and its line ending kept, so a file with CRLF, LF or CR keeps
// it.
func TestLineEndingsAreNeverChanged(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := replacements.Draw(t, "replacements")
		var tree []port.File
		for i := range rapid.IntRange(1, 4).Draw(t, "files") {
			tree = append(tree, port.File{Path: fmt.Sprintf("file%d.txt", i), Mode: 0o644, Data: textFile(t, r.olds)})
		}
		planned, err := Plan(tree, all, r.Replacer)
		if err != nil {
			t.Fatal(err)
		}
		for i, f := range planned {
			var want, endings, gotEndings []string
			for _, line := range lines(string(tree[i].Data)) {
				end := ending(line)
				want = append(want, r.Text(strings.TrimSuffix(line, end))+end)
				endings = append(endings, end)
			}
			for _, line := range lines(string(f.Data)) {
				gotEndings = append(gotEndings, ending(line))
			}
			if string(f.Data) != strings.Join(want, "") || !slices.Equal(gotEndings, endings) {
				t.Fatalf("%s is %q from %q, not %q, its line endings %q", f.Path, f.Data, tree[i].Data, strings.Join(want, ""), endings)
			}
		}
	})
}

// A binary file is copied byte for byte, whatever literals it holds, beside
// text files that have them replaced.
func TestABinaryFileIsCopiedByteForByte(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := replacements.Draw(t, "replacements")
		var tree []port.File
		for i := range rapid.IntRange(1, 4).Draw(t, "files") {
			if rapid.Bool().Draw(t, "binary") {
				tree = append(tree, port.File{Path: fmt.Sprintf("file%d.bin", i), Mode: 0o644, Data: binaryFile(t, r.olds)})
			} else {
				tree = append(tree, port.File{Path: fmt.Sprintf("file%d.txt", i), Mode: 0o644, Data: textFile(t, r.olds)})
			}
		}
		planned, err := Plan(tree, all, r.Replacer)
		if err != nil {
			t.Fatal(err)
		}
		for i, f := range planned {
			if strings.HasSuffix(f.Path, ".bin") && !bytes.Equal(f.Data, tree[i].Data) {
				t.Fatalf("%s is %q, not %q as the template holds it", f.Path, f.Data, tree[i].Data)
			}
		}
	})
}
