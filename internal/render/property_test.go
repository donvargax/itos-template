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

// A leftover's generators keep the literal's letters (a to m) apart from
// every other text's (n to z, digits and the separators), so the only
// spelling of the literal's words in a file is the one a property put
// there.
var (
	scanLiteral = rapid.Custom(func(t *rapid.T) caseform.Words {
		return caseform.Words(rapid.SliceOfN(rapid.StringMatching(`[a-m]{2,5}`), 2, 3).Draw(t, "words"))
	})
	scanAnswer = rapid.Custom(func(t *rapid.T) caseform.Words {
		return caseform.Words(rapid.SliceOfN(rapid.StringMatching(`[n-z][n-z0-9]{0,4}`), 1, 3).Draw(t, "words"))
	})
	filler = rapid.StringMatching(`[n-z0-9 ./_-]{0,8}`)
)

// A render of files that hold the literal only in its five forms, among
// other text, keeps no literal: each form is replaced by the answer's, and
// the scan finds nothing of the answers (decision 21).
func TestARenderOfTheFiveFormsKeepsNoLiteral(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		literal, answer := scanLiteral.Draw(t, "literal"), scanAnswer.Draw(t, "answer")
		var pairs []string
		for i, form := range literal.Forms() {
			pairs = append(pairs, form, answer.Forms()[i])
		}
		piece := rapid.OneOf(rapid.SampledFrom(literal.Forms()), filler)
		var tree []port.File
		for i := range rapid.IntRange(1, 4).Draw(t, "files") {
			path := fmt.Sprintf("%s/f%d", piece.Draw(t, "folder")+"x", i)
			var b strings.Builder
			for range rapid.IntRange(0, 5).Draw(t, "lines") {
				for range rapid.IntRange(0, 3).Draw(t, "pieces") {
					b.WriteString(piece.Draw(t, "piece"))
				}
				b.WriteString(rapid.SampledFrom(lineEndings).Draw(t, "line ending"))
			}
			tree = append(tree, port.File{Path: path, Mode: 0o644, Data: []byte(b.String())})
		}
		files, err := Plan(tree, func(string) bool { return true }, NewReplacer(pairs))
		if err != nil {
			t.Skip(err) // the answers named two files one name: not this property's
		}
		if got := NewScan([]caseform.Words{literal}, nil).Leftovers(files); len(got) != 0 {
			t.Fatalf("the render keeps %v", got)
		}
	})
}

// Any spelling of a literal's words, in order, in any case, joined by
// nothing, a space, ".", "-", "_" or "/", is found where it is: on its
// line, or in its path, with the text as it is written.
func TestAnySpellingOfALiteralIsFoundWhereItIs(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		literal := scanLiteral.Draw(t, "literal")
		var spelling strings.Builder
		for i, word := range literal {
			if i > 0 {
				spelling.WriteString(rapid.SampledFrom([]string{"", " ", ".", "-", "_", "/"}).Draw(t, "join"))
			}
			for _, r := range word {
				if rapid.Bool().Draw(t, "upper") {
					r -= 'a' - 'A'
				}
				spelling.WriteRune(r)
			}
		}
		text := spelling.String()
		lines := rapid.SliceOfN(filler, 1, 5).Draw(t, "lines")
		var want Leftover
		path := "x" + filler.Draw(t, "path")
		if rapid.Bool().Draw(t, "in the path") {
			path += text + "x"
			want = Leftover{Path: path, Line: 0, Text: text}
		} else {
			at := rapid.IntRange(0, len(lines)-1).Draw(t, "line")
			lines[at] += text + filler.Draw(t, "after")
			want = Leftover{Path: path, Line: at + 1, Text: text}
		}
		file := port.File{Path: path, Mode: 0o644, Data: []byte(strings.Join(lines, rapid.SampledFrom(lineEndings[:3]).Draw(t, "line ending")))}
		if got := NewScan([]caseform.Words{literal}, nil).Leftovers([]port.File{file}); !slices.Equal(got, []Leftover{want}) {
			t.Fatalf("Leftovers = %v, not %v", got, []Leftover{want})
		}
	})
}

// The first commit's message a manifest gives, made of the literal's five
// forms among other text, a header and lines after it each with any line
// ending, renders with every literal replaced, so the scan finds none of
// it, and with each line ending where the message had it: what a template's
// commit rules read is the message as written, the answers in place.
func TestAFirstCommitsMessageKeepsNoLiteralAndItsLineEndings(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		literal, answer := scanLiteral.Draw(t, "literal"), scanAnswer.Draw(t, "answer")
		var pairs []string
		for i, form := range literal.Forms() {
			pairs = append(pairs, form, answer.Forms()[i])
		}
		piece := rapid.OneOf(rapid.SampledFrom(literal.Forms()), filler)
		var b strings.Builder
		for range rapid.IntRange(1, 6).Draw(t, "lines") {
			for range rapid.IntRange(1, 3).Draw(t, "pieces") {
				b.WriteString(piece.Draw(t, "piece"))
			}
			b.WriteString(rapid.SampledFrom(lineEndings).Draw(t, "line ending"))
		}
		message := b.String()
		got := NewReplacer(pairs).Text(message)
		file := port.File{Path: "message", Mode: 0o644, Data: []byte(got)}
		if left := NewScan([]caseform.Words{literal}, nil).Leftovers([]port.File{file}); len(left) != 0 {
			t.Fatalf("%q renders as %q, keeping %v", message, got, left)
		}
		var want, endings []string
		for _, line := range lines(message) {
			want = append(want, ending(line))
		}
		for _, line := range lines(got) {
			endings = append(endings, ending(line))
		}
		if !slices.Equal(endings, want) {
			t.Fatalf("%q renders as %q, its line endings %q, not %q", message, got, endings, want)
		}
	})
}
