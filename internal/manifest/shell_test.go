package manifest

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestQuoteLeavesAWordNoShellReadsSpeciallyAsItIs(t *testing.T) {
	for _, word := range []string{"go", "core.hooksPath", "tools/hooks/pre-commit", "--agent-rules", "a=b", "user@host:path", "50%", "a+b,c", "_", "AZaz09"} {
		if got := Quote(word); got != word {
			t.Errorf("Quote(%q) = %s", word, got)
		}
	}
}

func TestQuotePutsAnyOtherWordInSingleQuotes(t *testing.T) {
	for word, want := range map[string]string{
		"":                 `''`,
		"a b":              `'a b'`,
		"it's":             `'it'"'"'s'`,
		"'":                `''"'"''`,
		"$HOME":            `'$HOME'`,
		"~":                `'~'`,
		"*.go":             `'*.go'`,
		`back\slash`:       `'back\slash'`,
		`"double"`:         `'"double"'`,
		"a;b|c&d":          `'a;b|c&d'`,
		"é":                `'é'`,
		"line\nbreak":      "'line\nbreak'",
		"#comment":         `'#comment'`,
		"!history":         `'!history'`,
		"`cmd`":            "'`cmd`'",
		"(x)":              `'(x)'`,
		"a\xffb":           "'a\xffb'",
		"go && go vet ./…": `'go && go vet ./…'`,
	} {
		if got := Quote(word); got != want {
			t.Errorf("Quote(%q) = %s, not %s", word, got, want)
		}
	}
}

func TestShellQuotesEachWordAndJoinsThemByOneSpace(t *testing.T) {
	got := Words{"sh", "-c", "go mod download && go vet ./...", ""}.Shell()
	want := `sh -c 'go mod download && go vet ./...' ''`
	if got != want {
		t.Errorf("Shell = %s, not %s", got, want)
	}
}

// shellWords are words a step may hold, and more: any text but a NUL, which
// no shell word can hold, with the characters a shell reads specially drawn
// often.
var shellWords = rapid.OneOf(
	rapid.String(),
	rapid.StringOf(rapid.SampledFrom([]rune("'\"\\$`!*?[]{}()<>|&;#~=%@+:,./_- \t\nazAZ09é"))),
).Filter(func(word string) bool { return !strings.ContainsRune(word, 0) })

// Any words, quoted and read back by the real sh, are the same words, byte
// for byte: what a person pastes runs the step's own words. The words go to
// sh on its standard input, as a pasted line does, so no system's command
// line reads them first. Where no sh is on the PATH there is nothing to
// read them back, and the property is skipped.
func TestShReadsQuotedWordsBackByteForByte(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on the PATH to read the words back")
	}
	rapid.Check(t, func(t *rapid.T) {
		words := Words(rapid.SliceOfN(shellWords, 1, 4).Draw(t, "words"))
		cmd := exec.Command(sh)
		cmd.Stdin = strings.NewReader("printf '[%s]' " + words.Shell() + "\n")
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("sh refused %s: %v\n%s", words.Shell(), err, stderr.String())
		}
		want := "[" + strings.Join(words, "][") + "]"
		if got := stdout.String(); got != want {
			t.Fatalf("sh read %s back as %q, not %q", words.Shell(), got, want)
		}
	})
}
