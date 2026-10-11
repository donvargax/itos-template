// Package project is a project made from a template: its folder, the render
// written into it as a git repository whose first commit is the render, and
// its Record of what it was rendered from, .itos-template.yaml (decisions 10
// and 12). The Record is read and written as the file is, and printed as
// new's --json is, with no copy of it in between: update and adopt read the
// same type new writes.
//
// It is domain (decision 17): it reaches the folder only through the
// port.Disk and the git that commits it only through the port.Committer a
// Writer holds, so its tests run it against fakes of both. A write that
// fails removes what it wrote, leaving the folder as it was: missing or
// empty.
package project

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/render"
	"github.com/donvargax/itos-template/internal/template/port"
)

// RecordFile is the made project's record of its render.
const RecordFile = ".itos-template.yaml"

// Record is what a project was rendered from (decision 10): the template, by
// the name template.Recorded gives new (record-name), the release rendered,
// when it was one (decision 36), the stack, the features, the answers, and
// the commit each of the template's branches was at, which reproduces the
// render, a release's or the heads', and the paths the project declines
// (GitHub issue #3), which every update leaves out of both its renders.
// Release is absent for the branch heads, Declined when the project
// declines nothing. Its version is the file's format, which new's --json
// leaves out.
type Record struct {
	Version  int               `yaml:"version" json:"-"`
	Template string            `yaml:"template" json:"template"`
	Release  string            `yaml:"release,omitempty" json:"release,omitempty"`
	Stack    string            `yaml:"stack" json:"stack"`
	Features []string          `yaml:"features" json:"features"`
	Answers  answer.Set        `yaml:"answers" json:"answers"`
	Commits  map[string]string `yaml:"commits" json:"commits"`
	Declined []string          `yaml:"declined,omitempty" json:"declined,omitempty"`
}

// RecordVersion is the record's format, the version a Record is written in.
// The release key joined version 1 (docs/manifest.md, "The record a made
// project keeps"): nothing read a record before it. The declined key joined
// it unbumped too: the reader refuses a key it does not know, so an older
// itos-template refuses a record that declines paths, naming the key,
// rather than misreading it as one declining none, and a record declining
// none, which has no such key, reads the same in every version.
const RecordVersion = 1

const recordHeader = `# The template render this project is kept up to date with, as
# itos-template new made it or adopt linked it: the template, by a name
# update can reach (an absolute path, a URL with no credential), the
# release, when it was one, the stack, the features, the answers, and the
# commit each of the template's branches was at. itos-template update reads
# it; edit it only to change what an update renders: a path listed under
# declined, a file or a folder and what is under it, no update brings.
`

// Marshal is r as .itos-template.yaml holds it, under a comment saying what
// it is.
func (r *Record) Marshal() ([]byte, error) {
	var b strings.Builder
	b.WriteString(recordHeader)
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(r); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// File is r as the file .itos-template.yaml, in the format RecordVersion
// names, as new writes it into a project and adopt beside one.
func (r Record) File() (port.File, error) {
	r.Version = RecordVersion
	data, err := r.Marshal()
	if err != nil {
		return port.File{}, err
	}
	return port.File{Path: RecordFile, Mode: 0o644, Data: data}, nil
}

// ReadRecord is the record data holds, the file .itos-template.yaml as new
// and adopt write it, read strictly, as the manifest is (manifest.Decode,
// decision 22): a *BadRecord, naming every problem, when it is not one
// this itos-template reads, of the format RecordVersion names, naming a
// template, a stack and the commit of each branch rendered.
func ReadRecord(data []byte) (Record, error) {
	var r Record
	problems := manifest.Decode(data, &r)
	if len(problems) == 0 {
		if r.Version != RecordVersion {
			problems = append(problems, fmt.Sprintf("version is %d: this itos-template reads version %d", r.Version, RecordVersion))
		}
		for key, empty := range map[string]bool{"template": r.Template == "", "stack": r.Stack == "", "commits": len(r.Commits) == 0} {
			if empty {
				problems = append(problems, "it names no "+key)
			}
		}
		problems = append(problems, declinedProblems(r.Declined)...)
	}
	if len(problems) > 0 {
		slices.Sort(problems)
		return Record{}, &BadRecord{File: RecordFile, Problems: problems}
	}
	return r, nil
}

// declinedProblems is a sentence for each path of declined that is not a
// path in the project, written as a template_only path is
// (manifest.IsTreePath), and for each given twice.
func declinedProblems(declined []string) []string {
	var problems []string
	seen := map[string]bool{}
	for _, p := range declined {
		switch {
		case !manifest.IsTreePath(p):
			problems = append(problems, fmt.Sprintf("the declined path %q is not a path in the project: write it relative to its top, with /", p))
		case seen[p]:
			problems = append(problems, fmt.Sprintf("the declined path %q is listed twice: list it once", p))
		}
		seen[p] = true
	}
	return problems
}

// Decline is paths, the paths a project declines as a command line gives
// them, as a record lists them: in the order given, each once, nil for
// none. A path that is not one in the project, written as a template_only
// path is (manifest.IsTreePath), is a *BadDeclined naming every such path.
func Decline(paths []string) ([]string, error) {
	var declined, bad []string
	for _, p := range paths {
		switch {
		case !manifest.IsTreePath(p):
			bad = append(bad, p)
		case !slices.Contains(declined, p):
			declined = append(declined, p)
		}
	}
	if len(bad) > 0 {
		return nil, &BadDeclined{Paths: bad}
	}
	return declined, nil
}

// Declines is whether the record declines the path p of a render, with /:
// p is a declined path or under one (manifest.Covers).
func (r *Record) Declines(p string) bool {
	return slices.ContainsFunc(r.Declined, func(declined string) bool { return manifest.Covers(declined, p) })
}

// Keep is files less those the record declines, in their order: a render
// as an update merges it.
func (r *Record) Keep(files []port.File) []port.File {
	kept := make([]port.File, 0, len(files))
	for _, f := range files {
		if !r.Declines(f.Path) {
			kept = append(kept, f)
		}
	}
	return kept
}

// Project is a made project: its folder, its record, and its first commit.
type Project struct {
	Folder string `json:"folder"`
	Record
	Commit string `json:"commit"`
}

// Folder is where a project is written: its path, and whether it is New,
// missing until the project is written, so a failed write removes it.
type Folder struct {
	Path string
	New  bool
}

// Writer writes a project: its files on Disk, committed by Git.
type Writer struct {
	Disk port.Disk
	Git  port.Committer
}

// Look is the folder at path, refused unless it is missing or empty: a
// *NotFolder or a *NotEmpty.
func (w Writer) Look(path string) (Folder, error) {
	contents, err := w.Disk.Look(path)
	if err != nil {
		return Folder{}, err
	}
	switch contents {
	case port.Missing:
		return Folder{Path: path, New: true}, nil
	case port.Empty:
		return Folder{Path: path}, nil
	case port.NotFolder:
		return Folder{}, &NotFolder{Folder: path}
	default:
		return Folder{}, &NotEmpty{Folder: path}
	}
}

// Write writes files and record into the folder, missing or empty, and
// commits it all as a new git repository's first commit, by by when it is
// given, else by whoever git's config names, which git must know before
// anything is written. The commit's message is message, the template's
// (its manifest's first_commit, rendered), or new's own when it is "",
// saying what the project was made from. A failure removes what was
// written; a commit git refuses is a *CommitRefused.
func (w Writer) Write(into Folder, files []port.File, record Record, message string, by *port.Identity) (*Project, error) {
	if by == nil {
		if err := w.Git.Identity(); err != nil {
			return nil, err
		}
	}
	p := &Project{Folder: into.Path, Record: record}
	p.Version = RecordVersion
	file, err := record.File()
	if err != nil {
		return nil, err
	}
	var executables []string
	for _, f := range files {
		if render.Executable(f) {
			executables = append(executables, f.Path)
		}
	}
	files = append(files[:len(files):len(files)], file)
	if err := w.Disk.Write(into.Path, files); err != nil {
		w.Disk.Clear(into.Path, into.New)
		return nil, err
	}
	if message == "" {
		message = p.commitMessage()
	}
	p.Commit, err = w.Git.Commit(into.Path, message, executables, by)
	if err != nil {
		w.Disk.Clear(into.Path, into.New)
		var refused *port.Refused
		if errors.As(err, &refused) {
			return nil, &CommitRefused{Err: refused.Err}
		}
		return nil, err
	}
	return p, nil
}

func (p *Project) commitMessage() string {
	features := "no features"
	if len(p.Features) > 0 {
		features = "the features " + strings.Join(p.Features, ", ")
	}
	return fmt.Sprintf("chore: make the project from its template\n\nMade by itos-template new from %s%s: the stack %s, %s. %s records the render.\n",
		p.Template, p.AtRelease(), p.Stack, features, RecordFile)
}

// AtRelease is " at its release <version>" for a record of a release, how
// a sentence naming the template names it, and "" for one of the heads.
func (r *Record) AtRelease() string {
	if r.Release == "" {
		return ""
	}
	return " at its release " + r.Release
}

// Error is a project that cannot be made, a sealed set (decision 17):
// internal/cli gives each kind its exit code.
//
//sumtype:decl
type Error interface {
	error
	projectError()
}

// NotEmpty is a project's folder that has files in it.
type NotEmpty struct{ Folder string }

// NotFolder is a project's folder that is a file.
type NotFolder struct{ Folder string }

// CommitRefused is the project's first commit refused by git (a hook), and
// why.
type CommitRefused struct{ Err error }

// BadRecord is a project's record, File, that cannot be read: every problem
// found in it.
type BadRecord struct {
	File     string
	Problems []string
}

// BadDeclined is Paths, paths a project is to decline that are not paths
// in it.
type BadDeclined struct{ Paths []string }

func (*NotEmpty) projectError()      {}
func (*NotFolder) projectError()     {}
func (*CommitRefused) projectError() {}
func (*BadRecord) projectError()     {}
func (*BadDeclined) projectError()   {}

func (e *NotEmpty) Error() string      { return e.Folder + " has files in it" }
func (e *NotFolder) Error() string     { return e.Folder + " is a file" }
func (e *CommitRefused) Error() string { return fmt.Sprintf("the first commit refused: %v", e.Err) }
func (e *CommitRefused) Unwrap() error { return e.Err }
func (e *BadRecord) Error() string {
	return e.File + ": " + strings.Join(e.Problems, "; ")
}
func (e *BadDeclined) Error() string {
	return "declined paths not in the project: " + strings.Join(e.Paths, ", ")
}
