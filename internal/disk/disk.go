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
	"strings"

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
// It never writes through a symbolic link in a parent folder (a *Through) nor
// over or through a file or link already at a file's own path (a *Occupied):
// each file is created with O_EXCL, which fails rather than follow a link
// there. new writes into an empty folder, so an entry it meets is one the
// render itself made; render.Plan refuses the names that would make two, so
// these are a second layer behind it (decision 41, bug-8 and bug-9), and on a
// disk that folds case or the Unicode marks HFS+ ignores the create and the
// parent check fail as the disk itself tells the names apart.
func (Disk) Write(folder string, files []port.File) error {
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return &Unmakeable{Folder: folder, Err: err}
	}
	for _, f := range files {
		to := filepath.Join(folder, filepath.FromSlash(f.Path))
		if link, ok := throughLink(folder, f.Path); ok {
			return &Through{Folder: folder, Link: link, Path: f.Path}
		}
		if err := os.MkdirAll(filepath.Join(folder, filepath.FromSlash(path.Dir(f.Path))), 0o755); err != nil {
			return &Unwritable{Folder: folder, Err: err}
		}
		if f.Mode&fs.ModeSymlink != 0 {
			if err := os.Symlink(string(f.Data), to); err != nil {
				return &Unwritable{Folder: folder, Link: f.Path, Err: err}
			}
			continue
		}
		file, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, f.Mode.Perm())
		if errors.Is(err, fs.ErrExist) {
			return &Occupied{Folder: folder, Path: f.Path}
		}
		if err == nil {
			_, err = file.Write(f.Data)
			err = errors.Join(err, file.Close())
		}
		if err != nil {
			return &Unwritable{Folder: folder, Err: err}
		}
	}
	return nil
}

// throughLink is the first parent folder of rel, under folder, that is a
// symbolic link and so would send a write out of folder, with whether it
// found one. It walks only the folders rel needs, below folder, never folder
// itself or above, which are the person's, not the render's; a component not
// yet there stops the walk, as Write's MkdirAll will make it, not follow a
// link. The link's path is relative to folder, with /.
func throughLink(folder, rel string) (string, bool) {
	at := folder
	var up string
	for _, e := range strings.Split(path.Dir(rel), "/") {
		if e == "." {
			continue
		}
		at = filepath.Join(at, e)
		if up == "" {
			up = e
		} else {
			up += "/" + e
		}
		info, err := os.Lstat(at)
		if err != nil {
			return "", false
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return up, true
		}
	}
	return "", false
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

// Through is a file Path whose parent folder Link, under Folder, is a
// symbolic link: writing it would go through the link, out of Folder.
type Through struct {
	Folder string
	Link   string
	Path   string
}

// Occupied is a file Path that something is already at in Folder, which Write
// would write over or through: new writes into an empty folder, so an entry
// at a render's path is a file or link O_EXCL refused to follow.
type Occupied struct {
	Folder string
	Path   string
}

func (*Unreadable) diskError() {}
func (*Unmakeable) diskError() {}
func (*Unwritable) diskError() {}
func (*Through) diskError()    {}
func (*Occupied) diskError()   {}

func (e *Unreadable) Error() string { return fmt.Sprintf("reading %s: %v", e.Folder, e.Err) }
func (e *Unmakeable) Error() string { return fmt.Sprintf("making %s: %v", e.Folder, e.Err) }
func (e *Unwritable) Error() string { return fmt.Sprintf("writing in %s: %v", e.Folder, e.Err) }
func (e *Through) Error() string {
	return fmt.Sprintf("writing %s in %s: its parent %s is a symbolic link", e.Path, e.Folder, e.Link)
}
func (e *Occupied) Error() string {
	return fmt.Sprintf("writing %s in %s: something is already there", e.Path, e.Folder)
}

func (e *Unreadable) Unwrap() error { return e.Err }
func (e *Unmakeable) Unwrap() error { return e.Err }
func (e *Unwritable) Unwrap() error { return e.Err }
