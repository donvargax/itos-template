package project

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
	"pgregory.net/rapid"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/manifest"
)

// branch is a stack's, a feature's or a branch's name.
var branch = rapid.StringMatching(`[a-z0-9][a-z0-9._-]{0,6}(/[a-z0-9][a-z0-9._-]{0,6})?`)

// records are what a render records, as the commands let it be: the
// template as it was named (a path, a URL or any text a command line gave)
// and the answers, each through the check the commands judge it by, a
// stack, its features (none given as nil or as empty), each branch's
// commit, and the paths it declines, nil for none. An answer is any text
// a question with no pattern takes
// (manifest.Question.Check); the template, any text template.CheckName
// takes, which is manifest.Refused: package template imports this one, so
// its test cannot call CheckName by its name. Each is drawn as any text,
// then cut of what the check refuses (free) and filtered by the check, so
// it ranges over everything real input can give, letters outside ASCII and
// the joiners where a script spells with them included, and nothing the
// commands refuse, such as a tab before a line feed, which go.yaml.in/yaml/v3
// writes as a block scalar it cannot read back (go-yaml/yaml#660, bug-6).
var records = rapid.Custom(func(t *rapid.T) Record {
	r := Record{
		Template: rapid.OneOf(
			rapid.StringMatching(`\.\./[a-z-]{1,8}`),
			rapid.StringMatching(`https://example\.com/[a-z]{1,6}/[a-z-]{1,8}\.git`),
			rapid.Map(rapid.StringN(1, 12, -1), free).Filter(func(name string) bool {
				_, refused := manifest.Refused(name)
				return name != "" && !refused
			}),
		).Draw(t, "template"),
		Stack:    branch.Draw(t, "stack"),
		Features: rapid.SliceOfNDistinct(branch, 0, 3, rapid.ID[string]).Draw(t, "features"),
		Answers: answer.Set(rapid.MapOfN(rapid.StringMatching(`[a-z][a-z0-9_-]{0,5}`), rapid.Map(rapid.StringN(1, 12, -1), free).Filter(func(a string) bool {
			return (&manifest.Question{}).Check(a) == nil
		}), 0, 4).Draw(t, "answers")),
		Commits: rapid.MapOfN(branch, rapid.StringMatching(`[0-9a-f]{40}`), 1, 4).Draw(t, "commits"),
	}
	if len(r.Features) == 0 && rapid.Bool().Draw(t, "features nil") {
		r.Features = nil
	}
	if declined := rapid.SliceOfNDistinct(branch, 0, 3, rapid.ID[string]).Draw(t, "declined"); len(declined) > 0 {
		r.Declined = declined
	}
	return r
})

// free is text cut, one character at a time, of each character free text
// may not hold (manifest.Refused), until it holds none; "" when a cut
// would change nothing, which no valid text gives.
func free(text string) string {
	for {
		r, refused := manifest.Refused(text)
		if !refused {
			return text
		}
		cut := strings.ReplaceAll(text, string(r), "")
		if cut == text {
			return ""
		}
		text = cut
	}
}

// A Record written and read back is the same Record: what Write puts in the
// project's folder reads, as update will read it, as the Record new made,
// in the record's version. No features written as nil or as empty read
// back as none; the declined paths read back in their order.
func TestARecordWrittenAndReadBackIsTheSameRecord(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := records.Draw(t, "record")
		w, d, _ := fakes()
		if _, err := w.Write(Folder{Path: "made", New: true}, files, r, "", nil); err != nil {
			t.Fatal(err)
		}
		var read Record
		if err := yaml.Unmarshal(d.Folders["made"][RecordFile].Data, &read); err != nil {
			t.Fatalf("the record written does not read: %v", err)
		}
		if read.Version != RecordVersion || read.Template != r.Template || read.Stack != r.Stack ||
			!slices.Equal(read.Features, r.Features) || !maps.Equal(read.Answers, r.Answers) || !maps.Equal(read.Commits, r.Commits) ||
			!slices.Equal(read.Declined, r.Declined) {
			t.Fatalf("read %#v, wrote %#v", read, r)
		}
	})
}
