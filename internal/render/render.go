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
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
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
// Every file keeps its mode. Nothing is written before Plan has looked at
// every name, and it refuses with every problem it finds, each an Error
// joined (errors.Join), so a template's author fixes them all at once, as
// new names every missing answer at once: every submodule kept, else every
// name the answers make that no file can have, two files of one name, or a
// file where another needs a folder.
//
// The problems come in a fixed order, whatever the order of tree, as a
// render, a refusal included, is the same every time (decision 3):
// submodules in the order of their paths, and the clashes in the order of
// the names the answers make. Two files of one name are named in the order
// of their paths too.
func Plan(tree []port.File, keep func(path string) bool, r *Replacer) ([]port.File, error) {
	var submodules []string
	for _, f := range tree {
		if keep(f.Path) && f.Mode&fs.ModeIrregular != 0 {
			submodules = append(submodules, f.Path)
		}
	}
	if len(submodules) > 0 {
		var errs []error
		for _, p := range slices.Sorted(slices.Values(submodules)) {
			errs = append(errs, &Submodule{Path: p})
		}
		return nil, errors.Join(errs...)
	}
	var files []port.File
	var clashes []clash
	from := map[string][]string{} // each name the answers make, the template's paths they make it of
	for _, f := range tree {
		if !keep(f.Path) {
			continue
		}
		to, err := r.Path(f.Path)
		var bad *BadName
		switch {
		case errors.As(err, &bad):
			clashes = append(clashes, clash{name: bad.Would, then: bad.Path, err: bad})
			continue
		case err != nil:
			return nil, err
		}
		from[to] = append(from[to], f.Path)
		var data []byte
		if f.Mode&fs.ModeSymlink != 0 {
			data = []byte(r.Text(string(f.Data)))
		} else {
			data = r.Contents(f.Data)
		}
		files = append(files, port.File{Path: to, Mode: f.Mode, Data: data})
	}
	for to, paths := range from {
		paths = slices.Sorted(slices.Values(paths))
		for _, p := range paths[1:] {
			clashes = append(clashes, clash{name: to, then: p, err: &SameName{First: paths[0], Second: p, To: to}})
		}
		for dir := path.Dir(to); dir != "."; dir = path.Dir(dir) {
			if others, ok := from[dir]; ok {
				clashes = append(clashes, clash{name: to, then: dir, err: &FileIsFolder{File: slices.Min(others), Folder: dir, Of: paths[0]}})
			}
		}
	}
	if len(clashes) > 0 {
		slices.SortFunc(clashes, func(a, b clash) int {
			return cmp.Or(strings.Compare(a.name, b.name), strings.Compare(a.then, b.then), strings.Compare(a.err.Error(), b.err.Error()))
		})
		errs := make([]error, len(clashes))
		for i, c := range clashes {
			errs[i] = c.err
		}
		return nil, errors.Join(errs...)
	}
	return files, nil
}

// clash is a name problem Plan found, with what orders it among the others:
// the name the answers make, then the path or folder it clashes over.
type clash struct {
	name, then string
	err        Error
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
