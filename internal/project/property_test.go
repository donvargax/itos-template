package project

import (
	"maps"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"
	"pgregory.net/rapid"

	"github.com/donvargax/itos-template/internal/answer"
)

// branch is a stack's, a feature's or a branch's name.
var branch = rapid.StringMatching(`[a-z0-9][a-z0-9._-]{0,6}(/[a-z0-9][a-z0-9._-]{0,6})?`)

// records are what a render records: the template as it was named (a path,
// a URL or anything a command line gave), a stack, its features (none
// given as nil or as empty), the answers, each any text but an empty one,
// as a question with no pattern takes, and each branch's commit.
var records = rapid.Custom(func(t *rapid.T) Record {
	r := Record{
		Template: rapid.OneOf(
			rapid.StringMatching(`\.\./[a-z-]{1,8}`),
			rapid.StringMatching(`https://example\.com/[a-z]{1,6}/[a-z-]{1,8}\.git`),
			rapid.StringN(1, 12, -1),
		).Draw(t, "template"),
		Stack:    branch.Draw(t, "stack"),
		Features: rapid.SliceOfNDistinct(branch, 0, 3, rapid.ID[string]).Draw(t, "features"),
		Answers:  answer.Set(rapid.MapOfN(rapid.StringMatching(`[a-z][a-z0-9_-]{0,5}`), rapid.StringN(1, 12, -1), 0, 4).Draw(t, "answers")),
		Commits:  rapid.MapOfN(branch, rapid.StringMatching(`[0-9a-f]{40}`), 1, 4).Draw(t, "commits"),
	}
	if len(r.Features) == 0 && rapid.Bool().Draw(t, "features nil") {
		r.Features = nil
	}
	return r
})

// A Record written and read back is the same Record: what Write puts in the
// project's folder reads, as update will read it, as the Record new made,
// in the record's version. No features written as nil or as empty read
// back as none.
func TestARecordWrittenAndReadBackIsTheSameRecord(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := records.Draw(t, "record")
		w, d, _ := fakes()
		if _, err := w.Write(Folder{Path: "made", New: true}, files, r, nil); err != nil {
			t.Fatal(err)
		}
		var read Record
		if err := yaml.Unmarshal(d.Folders["made"][RecordFile].Data, &read); err != nil {
			t.Fatalf("the record written does not read: %v", err)
		}
		if read.Version != RecordVersion || read.Template != r.Template || read.Stack != r.Stack ||
			!slices.Equal(read.Features, r.Features) || !maps.Equal(read.Answers, r.Answers) || !maps.Equal(read.Commits, r.Commits) {
			t.Fatalf("read %#v, wrote %#v", read, r)
		}
	})
}
