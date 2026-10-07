package caseform

import (
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// answerWords are an answer's words, as a command line gives it in kebab
// case: one to five words of lowercase letters and digits.
var answerWords = rapid.Custom(func(t *rapid.T) Words {
	return Words(rapid.SliceOfN(rapid.StringMatching(`[a-z0-9]{1,6}`), 1, 5).Draw(t, "words"))
})

// An answer's forms read back to its words: its kebab form as it is, its
// snake and upper snake forms with their underscores read as dashes. The
// camel and Pascal forms have no word breaks to read back, so they are not
// among them.
func TestAnAnswersFormsRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		words := answerWords.Draw(t, "answer")
		for _, form := range []string{
			words.Kebab(),
			strings.ReplaceAll(words.Snake(), "_", "-"),
			strings.ToLower(strings.ReplaceAll(words.UpperSnake(), "_", "-")),
		} {
			read, err := Parse(form)
			if err != nil || !slices.Equal(read, words) {
				t.Fatalf("%q reads back as %q, %v, not %q", form, read, err, words)
			}
		}
	})
}
