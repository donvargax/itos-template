package render

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
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

// NameError is a render whose file names cannot be written: an answer that
// makes a name no file can have, or two files of one name.
type NameError struct{ Problem string }

func (e *NameError) Error() string { return e.Problem }

// Path is the template's path p (with /) with every literal replaced in each
// of its elements, never across them.
func (r *Replacer) Path(p string) (string, error) {
	elements := strings.Split(p, "/")
	for i, e := range elements {
		replaced := r.r.Replace(e)
		if replaced == "" || replaced == "." || replaced == ".." || strings.ContainsAny(replaced, "/\\\x00") ||
			strings.EqualFold(replaced, ".git") {
			return "", &NameError{fmt.Sprintf("the template's %s would be named %q, which no file can be: the answers make it %q", p, strings.Join(append(elements[:i:i], replaced), "/"), replaced)}
		}
		elements[i] = replaced
	}
	return strings.Join(elements, "/"), nil
}

// File is a file of the render: the template's entry and the path it is
// written to, with /.
type File struct {
	Entry
	To string
}

// DefectError is a template that cannot be rendered as it is.
type DefectError struct{ Problem string }

func (e *DefectError) Error() string { return e.Problem }

// Plan is a render's files, every name checked, nothing written yet.
type Plan struct {
	t     *Template
	r     *Replacer
	Files []File
}

// Plan lists the files of tree a render writes: those keep takes, each
// path's literals replaced. Two files of one name, a file where another
// needs a folder, a name the answers make impossible and a submodule are
// refused before anything is written.
func (t *Template) Plan(tree string, keep func(path string) bool, r *Replacer) (*Plan, error) {
	entries, err := t.Entries(tree)
	if err != nil {
		return nil, err
	}
	plan := &Plan{t: t, r: r}
	from := map[string]string{}
	for _, e := range entries {
		if !keep(e.Path) {
			continue
		}
		if e.Submodule() {
			return nil, &DefectError{fmt.Sprintf("the template holds the submodule %s, which new cannot render", e.Path)}
		}
		to, err := r.Path(e.Path)
		if err != nil {
			return nil, err
		}
		if other, ok := from[to]; ok {
			return nil, &NameError{fmt.Sprintf("the template's %s and %s would both be %s: give answers that tell them apart", other, e.Path, to)}
		}
		from[to] = e.Path
		plan.Files = append(plan.Files, File{Entry: e, To: to})
	}
	for to, p := range from {
		for dir := path.Dir(to); dir != "."; dir = path.Dir(dir) {
			if other, ok := from[dir]; ok {
				return nil, &NameError{fmt.Sprintf("the template's file %s would be %s, a folder of %s: give answers that tell them apart", other, dir, p)}
			}
		}
	}
	return plan, nil
}

// Write writes the plan's files into the folder dir, which exists: each
// text file's contents and each link's target with its literals replaced, a
// file git records as executable written so where the system has the bit.
// It returns the paths (with /) of the executable files, for the commit to
// record them so where the file system cannot (windows).
func (p *Plan) Write(dir string) ([]string, error) {
	oids := make([]string, len(p.Files))
	for i, f := range p.Files {
		oids[i] = f.OID
	}
	var executables []string
	err := p.t.Blobs(oids, func(i int, data []byte) error {
		f := p.Files[i]
		to := osPath(dir, f.To)
		if err := os.MkdirAll(osPath(dir, path.Dir(f.To)), 0o755); err != nil {
			return err
		}
		if f.Link() {
			if err := os.Symlink(p.r.r.Replace(string(data)), to); err != nil {
				return &LinkError{Path: f.To, Err: err}
			}
			return nil
		}
		mode := os.FileMode(0o644)
		if f.Executable() {
			mode = 0o755
			executables = append(executables, f.To)
		}
		return os.WriteFile(to, p.r.Contents(data), mode)
	})
	return executables, err
}

// LinkError is a symbolic link the system would not make (windows without
// the right to).
type LinkError struct {
	Path string
	Err  error
}

func (e *LinkError) Error() string {
	return fmt.Sprintf("cannot make the symbolic link %s: %v", e.Path, e.Err)
}

func (e *LinkError) Unwrap() error { return e.Err }

// IsDefect is whether err says the template cannot be rendered as it is.
func IsDefect(err error) bool {
	var d *DefectError
	var c *ConflictError
	return errors.As(err, &d) || errors.As(err, &c)
}
