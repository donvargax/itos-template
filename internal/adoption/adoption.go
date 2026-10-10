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
// project, a binary file and a mode git records otherwise told apart. A
// missing file is a deletion the project made, kept for a later update,
// never a file adoption installs, and a difference is accepted with Force,
// never applied: adopting writes the record alone, unstaged and
// uncommitted, beside files, HEAD and index left as they are.
//
// It is domain (decision 17): it reaches the project's repository only
// through a port.Checkout and git's diff only through a port.Differ, so its
// tests run it against fakes of both. Its refusals are a sealed set
// (Error), plain domain types carrying no exit code or wording.
package adoption

import (
	"bytes"
	"cmp"
	"errors"
	"io/fs"
	"slices"

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
// was dirty, and whether the record was written.
type Adoption struct {
	Written     bool            `json:"written"`
	Dirty       bool            `json:"dirty"`
	Baseline    *project.Record `json:"baseline"`
	Differences []Difference    `json:"differences"`
}

// How is how adopting goes: DryRun previews and writes nothing; Force
// accepts the differences, writing the record over them; Diff asks for each
// text difference's diff.
type How struct {
	DryRun, Force, Diff bool
}

// Open looks at the project co was opened in, before anything is rendered,
// and gives what it is. It refuses a folder in no git repository
// (*NotRepository) and a repository with no working tree (*NoWorkTree);
// unless dryRun, it refuses too a repository with no commit (*NoCommit), a
// dirty index or working tree, untracked files git does not ignore
// included (*Dirty), and one holding a record already (*HasRecord), every
// one of these found, joined: --force accepts differences from the
// baseline, never these.
func Open(co port.Checkout, dryRun bool) (port.CheckoutState, error) {
	state, err := co.Look()
	switch {
	case err != nil:
		return state, err
	case !state.Repository:
		return state, &NotRepository{Why: state.Why}
	case !state.WorkTree:
		return state, &NoWorkTree{}
	case dryRun:
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
	return state, errors.Join(refused...)
}

// Adopt compares the project co holds, as Open found it in state, with
// baseline, the render record names, and, unless how.DryRun, writes record
// as the project's .itos-template.yaml: refused, a *Differs, when the
// project differs from the baseline and how.Force does not accept it, and
// a *HasRecord when a record appeared since Open looked. With how.Diff,
// each text difference has differ's diff. The Adoption comes back with a
// refusal too, so its differences are reported with it.
func Adopt(co port.Checkout, state port.CheckoutState, baseline []port.File, record project.Record, how How, differ port.Differ) (*Adoption, error) {
	paths := make([]string, len(baseline))
	for i, f := range baseline {
		paths[i] = f.Path
	}
	files, err := co.Files(paths)
	if err != nil {
		return nil, err
	}
	a := &Adoption{Baseline: &record, Dirty: len(state.Changed) > 0, Differences: Compare(baseline, files)}
	if how.Diff {
		if err := diffs(a.Differences, baseline, files, differ); err != nil {
			return nil, err
		}
	}
	if how.DryRun {
		return a, nil
	}
	if len(a.Differences) > 0 && !how.Force {
		return a, &Differs{Count: len(a.Differences)}
	}
	file, err := record.File()
	if err != nil {
		return nil, err
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
// adopting without Force refuses.
type Differs struct{ Count int }

func (*NotRepository) adoptionError() {}
func (*NoWorkTree) adoptionError()    {}
func (*NoCommit) adoptionError()      {}
func (*Dirty) adoptionError()         {}
func (*HasRecord) adoptionError()     {}
func (*Differs) adoptionError()       {}

func (e *NotRepository) Error() string { return "in no git repository: " + e.Why }
func (*NoWorkTree) Error() string      { return "a git repository with no working tree" }
func (e *NoCommit) Error() string      { return "no commit in " + e.Top }
func (e *Dirty) Error() string         { return "changes from HEAD" }
func (e *HasRecord) Error() string     { return "a record " + e.File + " already" }
func (e *Differs) Error() string       { return "differences from the baseline" }
