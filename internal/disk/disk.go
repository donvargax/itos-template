// Package disk is the infra that writes a project's files into its folder
// (decision 17): the project's port.Disk, on the file system of the system
// it runs on. It imports no package of ours but the ports it implements.
//
// A file's path comes with / and is written with the system's separator; a
// file git records as executable is written so where the system has the bit
// (on windows, where it has none, the commit records it instead); a
// symbolic link is made as one, which windows refuses without the right to.
// Its failures are a sealed set (Error), which internal/cli gives exit
// codes.
package disk

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Disk is the file system: the project's port.Disk.
type Disk struct{}

var _ port.Disk = Disk{}

// Look says what is at folder.
func (Disk) Look(folder string) (port.Contents, error) {
	info, err := os.Stat(folder)
	if errors.Is(err, fs.ErrNotExist) {
		return port.Missing, nil
	}
	if err != nil {
		return 0, &Unreadable{Folder: folder, Err: err}
	}
	if !info.IsDir() {
		return port.NotFolder, nil
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return 0, &Unreadable{Folder: folder, Err: err}
	}
	if len(entries) > 0 {
		return port.Full, nil
	}
	return port.Empty, nil
}

// Write writes files into folder, making it and every folder a file needs.
func (Disk) Write(folder string, files []port.File) error {
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return &Unmakeable{Folder: folder, Err: err}
	}
	for _, f := range files {
		to := filepath.Join(folder, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Join(folder, filepath.FromSlash(path.Dir(f.Path))), 0o755); err != nil {
			return &Unwritable{Folder: folder, Err: err}
		}
		if f.Mode&fs.ModeSymlink != 0 {
			if err := os.Symlink(string(f.Data), to); err != nil {
				return &Unwritable{Folder: folder, Link: f.Path, Err: err}
			}
			continue
		}
		if err := os.WriteFile(to, f.Data, f.Mode.Perm()); err != nil {
			return &Unwritable{Folder: folder, Err: err}
		}
	}
	return nil
}

// Clear removes what a failed Write left in folder: the folder when Write
// made it, else everything in it, as it was empty. What cannot be removed
// is left: the write's failure is the one to report.
func (Disk) Clear(folder string, made bool) {
	if made {
		_ = os.RemoveAll(folder)
		return
	}
	entries, _ := os.ReadDir(folder)
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(folder, e.Name()))
	}
}

// Error is how the file system failed a project's folder, a sealed set
// (decision 17): internal/cli gives each kind its exit code.
//
//sumtype:decl
type Error interface {
	error
	diskError()
}

// Unreadable is a folder whose contents cannot be read.
type Unreadable struct {
	Folder string
	Err    error
}

// Unmakeable is a folder that cannot be made.
type Unmakeable struct {
	Folder string
	Err    error
}

// Unwritable is a project that cannot be written into its folder: a file,
// or the symbolic link Link (its path, with /) the system would not make.
type Unwritable struct {
	Folder string
	Link   string
	Err    error
}

func (*Unreadable) diskError() {}
func (*Unmakeable) diskError() {}
func (*Unwritable) diskError() {}

func (e *Unreadable) Error() string { return fmt.Sprintf("reading %s: %v", e.Folder, e.Err) }
func (e *Unmakeable) Error() string { return fmt.Sprintf("making %s: %v", e.Folder, e.Err) }
func (e *Unwritable) Error() string { return fmt.Sprintf("writing in %s: %v", e.Folder, e.Err) }

func (e *Unreadable) Unwrap() error { return e.Err }
func (e *Unmakeable) Unwrap() error { return e.Err }
func (e *Unwritable) Unwrap() error { return e.Err }
