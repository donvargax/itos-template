// Package render makes a project's files from a template's merged tree
// (decision 1): each literal replaced by its answer in every text file's
// contents, in each element of every path and in a symbolic link's target.
// It is domain, and does no I/O: the tree comes from the template's
// port.Repository, and what it makes is written through the project's
// port.Disk (package project).
//
// A render is the same on every system and every machine, as an update
// needs it (decision 3): replacement works within lines and never touches a
// line ending, so a file with CRLF keeps it, and a binary file is copied as
// it is. Every name is checked before anything is written.
package render

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Replacer replaces a template's literals by their answers, in a text
// file's contents and in each element of a path. Within a text file it
// replaces within lines and never touches a line ending, so a file with CRLF
// keeps it.
type Replacer struct {
	r *strings.Replacer
}

// NewReplacer replaces each old string of pairs (old, new, old, new…) by its
// new one; where two match at one position, the first pair wins.
func NewReplacer(pairs []string) *Replacer {
	return &Replacer{r: strings.NewReplacer(pairs...)}
}

// Contents is data with every literal replaced when it is a text file's (no
// NUL in its first 8000 bytes, as git tells them), else data as it is.
func (r *Replacer) Contents(data []byte) []byte {
	if !isText(data) {
		return data
	}
	return []byte(r.r.Replace(string(data)))
}

// Text is s with every literal replaced, as in a text file's contents: what
// check makes of a check's words, and new of a link's target.
func (r *Replacer) Text(s string) string { return r.r.Replace(s) }

// Path is the template's path p (with /) with every literal replaced in each
// of its elements, never across them. An element no file can be named (empty,
// . or .., .git, or holding a / or a \) is a *BadName.
func (r *Replacer) Path(p string) (string, error) {
	elements := strings.Split(p, "/")
	for i, e := range elements {
		replaced := r.r.Replace(e)
		if replaced == "" || replaced == "." || replaced == ".." || strings.ContainsAny(replaced, "/\\\x00") ||
			strings.EqualFold(replaced, ".git") {
			return "", &BadName{Path: p, Would: strings.Join(append(elements[:i:i], replaced), "/"), Element: replaced}
		}
		elements[i] = replaced
	}
	return strings.Join(elements, "/"), nil
}

// Plan is the files a render of tree writes: those keep takes, each path's
// literals replaced, and each text file's contents and each link's target.
// Two files of one name, a file where another needs a folder, a name the
// answers make impossible and a submodule are refused, each an Error,
// before anything is written. Every file keeps its mode.
func Plan(tree []port.File, keep func(path string) bool, r *Replacer) ([]port.File, error) {
	var files []port.File
	from := map[string]string{}
	for _, f := range tree {
		if !keep(f.Path) {
			continue
		}
		if f.Mode&fs.ModeIrregular != 0 {
			return nil, &Submodule{Path: f.Path}
		}
		to, err := r.Path(f.Path)
		if err != nil {
			return nil, err
		}
		if other, ok := from[to]; ok {
			return nil, &SameName{First: other, Second: f.Path, To: to}
		}
		from[to] = f.Path
		var data []byte
		if f.Mode&fs.ModeSymlink != 0 {
			data = []byte(r.Text(string(f.Data)))
		} else {
			data = r.Contents(f.Data)
		}
		files = append(files, port.File{Path: to, Mode: f.Mode, Data: data})
	}
	for to, p := range from {
		for dir := path.Dir(to); dir != "."; dir = path.Dir(dir) {
			if other, ok := from[dir]; ok {
				return nil, &FileIsFolder{File: other, Folder: dir, Of: p}
			}
		}
	}
	return files, nil
}

// Executable is whether f is a file to record as executable.
func Executable(f port.File) bool { return f.Mode.IsRegular() && f.Mode&0o111 != 0 }

// isText is whether data is a text file's contents, as git tells them: no
// NUL in its first 8000 bytes.
func isText(data []byte) bool {
	return !bytes.Contains(data[:min(len(data), 8000)], []byte{0})
}

// Error is a template that cannot be rendered with the answers given, a
// sealed set (decision 17): internal/cli gives each kind its exit code.
//
//sumtype:decl
type Error interface {
	error
	renderError()
}

// BadName is the template's Path that the answers would name Would, its
// element Element being a name no file can have.
type BadName struct{ Path, Would, Element string }

// SameName is the template's files First and Second that the answers would
// both name To.
type SameName struct{ First, Second, To string }

// FileIsFolder is the template's file File that the answers would name
// Folder, a folder of the file Of.
type FileIsFolder struct{ File, Folder, Of string }

// Submodule is a submodule the template holds at Path, which a render
// cannot write: a defect of the template.
type Submodule struct{ Path string }

func (*BadName) renderError()      {}
func (*SameName) renderError()     {}
func (*FileIsFolder) renderError() {}
func (*Submodule) renderError()    {}

func (e *BadName) Error() string {
	return fmt.Sprintf("the answers name %s %q", e.Path, e.Would)
}

func (e *SameName) Error() string {
	return fmt.Sprintf("the answers name both %s and %s %s", e.First, e.Second, e.To)
}

func (e *FileIsFolder) Error() string {
	return fmt.Sprintf("the answers name the file %s %s, a folder of %s", e.File, e.Folder, e.Of)
}

func (e *Submodule) Error() string { return "a submodule at " + e.Path }
