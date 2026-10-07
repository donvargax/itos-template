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
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/render"
	"github.com/donvargax/itos-template/internal/template/port"
)

// RecordFile is the made project's record of its render.
const RecordFile = ".itos-template.yaml"

// Record is what a project was rendered from (decision 10): the template as
// it was named, the stack, the features, the answers, and the commit each of
// the template's branches was at. Its version is the file's format, which
// new's --json leaves out.
type Record struct {
	Version  int               `yaml:"version" json:"-"`
	Template string            `yaml:"template" json:"template"`
	Stack    string            `yaml:"stack" json:"stack"`
	Features []string          `yaml:"features" json:"features"`
	Answers  answer.Set        `yaml:"answers" json:"answers"`
	Commits  map[string]string `yaml:"commits" json:"commits"`
}

// RecordVersion is the record's format, the version a Record is written in.
const RecordVersion = 1

const recordHeader = `# What itos-template new rendered this project from: the template as it was
# named, the stack, the features, the answers, and the commit each of the
# template's branches was at. itos-template update reads it; edit it only to
# change what an update renders.
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
// anything is written. A failure removes what was written; a commit git
// refuses is a *CommitRefused.
func (w Writer) Write(into Folder, files []port.File, record Record, by *port.Identity) (*Project, error) {
	if by == nil {
		if err := w.Git.Identity(); err != nil {
			return nil, err
		}
	}
	p := &Project{Folder: into.Path, Record: record}
	p.Version = RecordVersion
	data, err := p.Marshal()
	if err != nil {
		return nil, err
	}
	var executables []string
	for _, f := range files {
		if render.Executable(f) {
			executables = append(executables, f.Path)
		}
	}
	files = append(files[:len(files):len(files)], port.File{Path: RecordFile, Mode: 0o644, Data: data})
	if err := w.Disk.Write(into.Path, files); err != nil {
		w.Disk.Clear(into.Path, into.New)
		return nil, err
	}
	p.Commit, err = w.Git.Commit(into.Path, p.commitMessage(), executables, by)
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
	return fmt.Sprintf("chore: make the project from its template\n\nMade by itos-template new from %s: the stack %s, %s. %s records the render.\n",
		p.Template, p.Stack, features, RecordFile)
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

func (*NotEmpty) projectError()      {}
func (*NotFolder) projectError()     {}
func (*CommitRefused) projectError() {}

func (e *NotEmpty) Error() string      { return e.Folder + " has files in it" }
func (e *NotFolder) Error() string     { return e.Folder + " is a file" }
func (e *CommitRefused) Error() string { return fmt.Sprintf("the first commit refused: %v", e.Err) }
func (e *CommitRefused) Unwrap() error { return e.Err }
