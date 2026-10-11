// Package adoption is an existing project linked to a template's baseline
// (decision 24, features/adopt.feature): a project made before the
// template, with files and history of its own, gets the record new would
// have written for the release it matches, so update can take it from
// there (decisions 3 and 10).
//
// The baseline is a render of the release the person names, with the
// answers they give, made by package template as new makes one and never
// written (template.Baseline). Here it is compared with the project's
// working tree as git sees it (port.Checkout): each path the two hold
// otherwise is a Difference, project-only, modified or missing from the
// project, a binary file and a mode git records otherwise told apart.
//
// A difference is accepted in one of two ways. Force accepts it as it is:
// adopting writes the record alone, unstaged and uncommitted, beside
// files, HEAD and index left as they are, so a missing file is a deletion
// of the project's that a later update keeps only while the template
// leaves the file unchanged. Merge takes the baseline in (GitHub issue #3):
// git's merge, as update merges (decision 38), merges the baseline into
// the project over an empty base, the record with it, so a file the project
// lacks is added, one both hold alike or the project alone holds is left
// as it is, and one both hold otherwise is a conflict both added; the
// merge and the record are staged, as update leaves its own (decision 37),
// HEAD unmoved and nothing committed.
//
// A path the project declines (How.Decline), a file or a folder of the
// baseline, is left out of the baseline before anything is compared or
// merged, and the record lists it, so every later update leaves it out too.
//
// It is domain (decision 17): it reaches the project's repository only
// through a port.Checkout, git's merge only through a port.Merger and git's
// diff only through a port.Differ, so its tests run it against fakes of
// them. Its refusals are a sealed set (Error), plain domain types carrying
// no exit code or wording.
package adoption

import (
	"bytes"
	"cmp"
	"errors"
	"io/fs"
	"slices"
	"strings"

	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/template/port"
)

// Kind is how a path of the project differs from the baseline.
type Kind string

// The kinds of a Difference.
const (
	ProjectOnly Kind = "project-only" // the project holds it, the baseline does not
	Modified    Kind = "modified"     // both hold it, its contents or its mode otherwise
	Missing     Kind = "missing"      // the baseline holds it, the project does not
)

// Difference is a path the project holds otherwise than the baseline: its
// path from the project's top, with /; its kind; whether either side's
// contents are binary, as git tells them (a NUL in the first 8000 bytes);
// whether the two record its mode otherwise, From the baseline's and To the
// project's; and, when asked for, git's diff of its text from the baseline
// to the project.
type Difference struct {
	Path        string      `json:"path"`
	Kind        Kind        `json:"kind"`
	Binary      bool        `json:"binary"`
	ModeChanged bool        `json:"mode_changed"`
	From        fs.FileMode `json:"-"`
	To          fs.FileMode `json:"-"`
	Diff        string      `json:"diff,omitempty"`
}

// Adoption is what adopting found and did: the baseline's record, the
// differences, sorted by path, whether the project's index or working tree
// was dirty, whether the record was written, and the paths a merge left
// conflicted, sorted, as update names its own.
type Adoption struct {
	Written     bool            `json:"written"`
	Dirty       bool            `json:"dirty"`
	Baseline    *project.Record `json:"baseline"`
	Differences []Difference    `json:"differences"`
	Conflicts   []port.Unmerged `json:"conflicts"`
}

// How is how adopting goes: DryRun previews and writes nothing; Force
// accepts the differences, writing the record over them; Merge accepts them
// by merging the baseline in, the record staged with it; Diff asks for each
// text difference's diff. Decline are the baseline's paths the project
// turns down, as a command line gives them. The UI never gives Force and
// Merge together.
type How struct {
	DryRun, Force, Merge, Diff bool
	Decline                    []string
}

// Unrelated is how a merge's conflict markers name its base, which a
// conflict style showing the base marks: an empty tree, as git merge names
// the base of histories with no commit in common.
const Unrelated = "empty tree"

// Open looks at the project co was opened in, before anything is rendered,
// and gives what it is. It refuses a folder in no git repository
// (*NotRepository) and a repository with no working tree (*NoWorkTree);
// unless how.DryRun, it refuses too a repository with no commit
// (*NoCommit), a dirty index or working tree, untracked files git does not
// ignore included (*Dirty), one holding a record already (*HasRecord), and,
// with how.Merge, a merger that cannot merge, as it says, every one of
// these found, joined: --force and --merge accept differences from the
// baseline, never these.
func Open(co port.Checkout, how How, merger port.Merger) (port.CheckoutState, error) {
	state, err := co.Look()
	switch {
	case err != nil:
		return state, err
	case !state.Repository:
		return state, &NotRepository{Why: state.Why}
	case !state.WorkTree:
		return state, &NoWorkTree{}
	case how.DryRun:
		return state, nil
	}
	var refused []error
	if !state.Committed {
		refused = append(refused, &NoCommit{Top: state.Top})
	}
	if len(state.Changed) > 0 {
		refused = append(refused, &Dirty{Paths: state.Changed})
	}
	held, err := co.Holds(project.RecordFile)
	if err != nil {
		return state, err
	}
	if held {
		refused = append(refused, &HasRecord{File: project.RecordFile})
	}
	if how.Merge {
		refused = append(refused, merger.CanMerge())
	}
	return state, errors.Join(refused...)
}

// Adopt compares the project co holds, as Open found it in state, with
// baseline, the render record names, and, unless how.DryRun, writes record
// as the project's .itos-template.yaml: refused, a *Differs, when the
// project differs from the baseline and neither how.Force nor how.Merge
// accepts it, and a *HasRecord when a record appeared since Open looked.
// With how.Merge, merger merges baseline and the record into the project
// instead, over an empty base, a *Conflicted when the merge leaves
// conflicts. With how.Diff, each text difference has differ's diff. The
// Adoption comes back with a refusal too, so its differences and conflicts
// are reported with it.
//
// The paths how.Decline names are judged first (project.Decline), then
// each must be a file or a folder of the baseline, else a *Undeclinable
// naming every one that is neither, so a misspelt path is caught; the
// record lists them, and the baseline is compared, merged and written
// without them, as every later update renders it.
func Adopt(co port.Checkout, state port.CheckoutState, baseline []port.File, record project.Record, how How, merger port.Merger, differ port.Differ) (*Adoption, error) {
	declined, err := project.Decline(how.Decline)
	if err != nil {
		return nil, err
	}
	if unheld := unheld(declined, baseline); len(unheld) > 0 {
		return nil, &Undeclinable{Paths: unheld}
	}
	record.Declined = declined
	baseline = record.Keep(baseline)
	paths := make([]string, len(baseline))
	for i, f := range baseline {
		paths[i] = f.Path
	}
	files, err := co.Files(paths)
	if err != nil {
		return nil, err
	}
	a := &Adoption{Baseline: &record, Dirty: len(state.Changed) > 0, Differences: Compare(baseline, files), Conflicts: []port.Unmerged{}}
	if how.Diff {
		if err := diffs(a.Differences, baseline, files, differ); err != nil {
			return nil, err
		}
	}
	if how.DryRun {
		return a, nil
	}
	if len(a.Differences) > 0 && !how.Force && !how.Merge {
		return a, &Differs{Count: len(a.Differences)}
	}
	file, err := record.File()
	if err != nil {
		return nil, err
	}
	if how.Merge {
		return merge(a, merger, append(baseline[:len(baseline):len(baseline)], file), record.Release)
	}
	if err := co.Create(file.Path, file.Data); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return a, &HasRecord{File: file.Path}
		}
		return nil, err
	}
	a.Written = true
	return a, nil
}

// unheld is each path of declined that covers no file of baseline: one
// the baseline holds neither as a file nor as a folder.
func unheld(declined []string, baseline []port.File) []string {
	var paths []string
	for _, d := range declined {
		if !slices.ContainsFunc(baseline, func(f port.File) bool { return manifest.Covers(d, f.Path) }) {
			paths = append(paths, d)
		}
	}
	return paths
}

// merge has merger merge target, the baseline and its record, into the
// project a found, over an empty base, labelled by release, the baseline's:
// the record written and staged with the rest, a *Conflicted naming each
// path the merge leaves conflicted.
func merge(a *Adoption, merger port.Merger, target []port.File, release string) (*Adoption, error) {
	merged, err := merger.Merge(nil, target, port.Labels{Base: Unrelated, Target: "template " + release})
	if err != nil {
		return nil, err
	}
	a.Written = true
	if len(merged.Conflicts) == 0 {
		return a, nil
	}
	a.Conflicts = merged.Conflicts
	paths := make([]string, len(merged.Conflicts))
	for i, c := range merged.Conflicts {
		paths[i] = c.Path
	}
	return a, &Conflicted{Release: release, Paths: paths}
}

// Compare is every path project holds otherwise than baseline, sorted by
// path: a path only one holds, or one both hold with other contents or
// another mode. A path with / on both sides, as port.File has it.
func Compare(baseline, project []port.File) []Difference {
	theirs := byPath(baseline)
	ours := byPath(project)
	differences := []Difference{}
	for p, b := range theirs {
		o, ok := ours[p]
		switch {
		case !ok:
			differences = append(differences, Difference{Path: p, Kind: Missing, Binary: binary(b), From: b.Mode})
		case o.Mode != b.Mode || !bytes.Equal(o.Data, b.Data):
			differences = append(differences, Difference{
				Path: p, Kind: Modified, Binary: binary(b) || binary(o),
				ModeChanged: o.Mode != b.Mode, From: b.Mode, To: o.Mode,
			})
		}
	}
	for p, o := range ours {
		if _, ok := theirs[p]; !ok {
			differences = append(differences, Difference{Path: p, Kind: ProjectOnly, Binary: binary(o), To: o.Mode})
		}
	}
	slices.SortFunc(differences, func(a, b Difference) int { return cmp.Compare(a.Path, b.Path) })
	return differences
}

// diffs gives each difference whose text changed, on neither side binary
// nor a submodule, differ's diff of it from baseline to project.
func diffs(differences []Difference, baseline, project []port.File, differ port.Differ) error {
	theirs := byPath(baseline)
	ours := byPath(project)
	for i, d := range differences {
		from, inBaseline := theirs[d.Path]
		to, inProject := ours[d.Path]
		if d.Binary || inBaseline && submodule(from) || inProject && submodule(to) ||
			inBaseline && inProject && bytes.Equal(from.Data, to.Data) {
			continue
		}
		var err error
		differences[i].Diff, err = differ.Diff(side(from, inBaseline), side(to, inProject))
		if err != nil {
			return err
		}
	}
	return nil
}

// side is f when the side holds it, else nil.
func side(f port.File, held bool) *port.File {
	if !held {
		return nil
	}
	return &f
}

func byPath(files []port.File) map[string]port.File {
	m := make(map[string]port.File, len(files))
	for _, f := range files {
		m[f.Path] = f
	}
	return m
}

// binary is whether f's contents are a binary file's, as git tells them: no
// link's target nor a submodule's, which have none, but a file with a NUL
// in its first 8000 bytes.
func binary(f port.File) bool {
	return f.Mode.IsRegular() && bytes.Contains(f.Data[:min(len(f.Data), 8000)], []byte{0})
}

func submodule(f port.File) bool { return f.Mode&fs.ModeIrregular != 0 }

// Error is why a project cannot be adopted, a sealed set (decision 17):
// internal/cli gives each kind its exit code and its line.
//
//sumtype:decl
type Error interface {
	error
	adoptionError()
}

// NotRepository is a folder in no git repository, and why, as git says.
type NotRepository struct{ Why string }

// NoWorkTree is a git repository with no working tree: a bare one, or its
// .git folder.
type NoWorkTree struct{}

// NoCommit is a repository at Top whose HEAD names no commit.
type NoCommit struct{ Top string }

// Dirty is a project whose index or working tree changes Paths from HEAD,
// an untracked file git does not ignore among them.
type Dirty struct{ Paths []string }

// HasRecord is a project holding File, a record, already.
type HasRecord struct{ File string }

// Differs is a project differing from the baseline in Count paths, which
// adopting without Force or Merge refuses.
type Differs struct{ Count int }

// Conflicted is a merge of the baseline, the release Release, that left
// conflicts in Paths.
type Conflicted struct {
	Release string
	Paths   []string
}

// Undeclinable is Paths, paths to decline that the baseline holds neither
// as a file nor as a folder.
type Undeclinable struct{ Paths []string }

func (*NotRepository) adoptionError() {}
func (*NoWorkTree) adoptionError()    {}
func (*NoCommit) adoptionError()      {}
func (*Dirty) adoptionError()         {}
func (*HasRecord) adoptionError()     {}
func (*Differs) adoptionError()       {}
func (*Conflicted) adoptionError()    {}
func (*Undeclinable) adoptionError()  {}

func (e *NotRepository) Error() string { return "in no git repository: " + e.Why }
func (*NoWorkTree) Error() string      { return "a git repository with no working tree" }
func (e *NoCommit) Error() string      { return "no commit in " + e.Top }
func (e *Dirty) Error() string         { return "changes from HEAD" }
func (e *HasRecord) Error() string     { return "a record " + e.File + " already" }
func (e *Differs) Error() string       { return "differences from the baseline" }
func (e *Conflicted) Error() string    { return "conflicts in " + strings.Join(e.Paths, ", ") }
func (e *Undeclinable) Error() string {
	return "declined paths not in the baseline: " + strings.Join(e.Paths, ", ")
}
