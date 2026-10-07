// Package project is a project made from a template: its folder, the render
// written into it as a git repository whose first commit is the render, and
// its Record of what it was rendered from, .itos-template.yaml (decisions 10
// and 12). The Record is read and written as the file is, and printed as
// new's --json is, with no copy of it in between: update and adopt read the
// same type new writes.
//
// A write that fails removes what it wrote, leaving the folder as it was:
// missing or empty.
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos-template/internal/git"
	"github.com/donvargax/itos-template/internal/problem"
	"github.com/donvargax/itos-template/internal/render"
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
	Answers  map[string]string `yaml:"answers" json:"answers"`
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

// Write writes plan and its record into folder, missing or empty (created
// says whether Write makes it), makes it a git repository and commits it all
// as its first commit. env, when given, is added to the environment of the
// commit's git. A failure removes what was written.
func Write(folder string, created bool, plan *render.Plan, record Record, env []string) (*Project, error) {
	p := &Project{Folder: folder, Record: record}
	p.Version = RecordVersion
	var err error
	if p.Commit, err = p.write(plan, env); err != nil {
		undo(folder, created)
		return nil, err
	}
	return p, nil
}

func (p *Project) write(plan *render.Plan, env []string) (string, error) {
	folder := p.Folder
	g := git.Command{Dir: folder, Env: env}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", problem.New(problem.CodeEnvironment, "folder-unwritable", "cannot make the folder %s: %v", folder, err)
	}
	executables, err := plan.Write(folder)
	if err != nil {
		var link *render.LinkError
		if errors.As(err, &link) {
			return "", problem.New(problem.CodeEnvironment, "folder-unwritable", "cannot write the project in %s: %v", folder, err)
		}
		var g *git.Error
		if errors.As(err, &g) {
			return "", problem.Internal(err)
		}
		return "", problem.New(problem.CodeEnvironment, "folder-unwritable", "cannot write the project in %s: %v", folder, err)
	}
	record, err := p.Marshal()
	if err != nil {
		return "", problem.Internal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, RecordFile), record, 0o644); err != nil {
		return "", problem.New(problem.CodeEnvironment, "folder-unwritable", "cannot write the project in %s: %v", folder, err)
	}
	steps := [][]string{
		{"init", "-q"},
		// The render is committed as written, whatever core.autocrlf says, and
		// every file of it, whatever its .gitignore leaves out: the template
		// holds them.
		{"-c", "core.autocrlf=false", "-c", "core.safecrlf=false", "add", "--all", "--force", "--", "."},
	}
	if len(executables) > 0 {
		// Where the file system has no execute bit (windows), git takes it from
		// here, as the template records it.
		steps = append(steps, append([]string{"update-index", "--chmod=+x", "--"}, executables...))
	}
	for _, args := range steps {
		if _, err := g.Output(args...); err != nil {
			return "", problem.Internal(err)
		}
	}
	if _, err := g.Output("commit", "-q", "-m", p.commitMessage()); err != nil {
		return "", problem.New(problem.CodeRefused, "commit-refused", "git refused the project's first commit: %v", err)
	}
	sha, err := g.Output("rev-parse", "HEAD")
	if err != nil {
		return "", problem.Internal(err)
	}
	return strings.TrimSpace(string(sha)), nil
}

func (p *Project) commitMessage() string {
	features := "no features"
	if len(p.Features) > 0 {
		features = "the features " + strings.Join(p.Features, ", ")
	}
	return fmt.Sprintf("chore: make the project from its template\n\nMade by itos-template new from %s: the stack %s, %s. %s records the render.\n",
		p.Template, p.Stack, features, RecordFile)
}

// undo removes what a failed write left in folder: the folder when Write
// made it, else everything in it, as it was empty.
func undo(folder string, created bool) {
	if created {
		_ = os.RemoveAll(folder)
		return
	}
	entries, _ := os.ReadDir(folder)
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(folder, e.Name()))
	}
}
