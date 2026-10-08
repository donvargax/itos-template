// Package tempdir is the infra that makes and removes the temporary folders
// itos-template works in (decision 17): a template's bare clone, and each
// render check makes and checks (Renders, check's port.Folders). It
// imports no package of ours but the ports it implements.
//
// os.RemoveAll can fail on windows where it succeeds elsewhere: git writes
// its object and pack files read-only, a file is not removed while a
// program still has it open, and a check's own programs (a build's server,
// an antivirus scanning what a build wrote) can hold one for a moment after
// they exit. Remove makes every file writable and tries again, a few times,
// before it gives up. A folder left behind is only a temporary one, so
// Discard and Renders log a failure to remove one, never return it.
package tempdir

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"time"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Unmakeable is a temporary folder that cannot be made: the one failure of
// this package internal/cli gives an exit code.
type Unmakeable struct{ Err error }

func (e *Unmakeable) Error() string { return fmt.Sprintf("making a temporary folder: %v", e.Err) }

func (e *Unmakeable) Unwrap() error { return e.Err }

// Make makes a new temporary folder, its name starting with prefix. A
// failure is an *Unmakeable.
func Make(prefix string) (string, error) {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", &Unmakeable{Err: err}
	}
	return dir, nil
}

// Remove removes dir and everything in it: a dir that does not exist is
// removed already. On a failure it makes what is left writable and tries
// again, waiting a little longer each time, and returns the last error.
func Remove(dir string) error {
	err := os.RemoveAll(dir)
	for wait := 50 * time.Millisecond; err != nil && wait <= 800*time.Millisecond; wait *= 2 {
		writable(dir)
		time.Sleep(wait)
		err = os.RemoveAll(dir)
	}
	return err
}

// Discard removes dir as Remove does, logging a failure.
func Discard(dir string) {
	if err := Remove(dir); err != nil {
		slog.Warn("cannot remove a temporary folder", "folder", dir, "error", err)
	}
}

// Renders are the temporary folders check renders each combination in:
// check's port.Folders.
type Renders struct{}

var _ port.Folders = Renders{}

// Make makes a render's folder.
func (Renders) Make() (string, error) { return Make("itos-template-check-") }

// Remove removes a render's folder as Remove does, logging a failure.
func (Renders) Remove(dir string) {
	if err := Remove(dir); err != nil {
		slog.Warn("cannot remove a render's temporary folder", "folder", dir, "error", err)
	}
}

// writable makes every file and folder under dir writable by its owner,
// which on windows clears the read-only attribute git gives its objects.
// It works through dir opened as an os.Root, each chmod by its path
// relative to dir: a symlink swapped in for a folder mid-walk cannot
// redirect it outside dir, as a chmod by a path from the walk could.
func writable(dir string) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return
	}
	defer func() { _ = root.Close() }()
	_ = fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		mode := fs.FileMode(0o600)
		if d.IsDir() {
			mode = 0o700
		} else if info, err := d.Info(); err == nil {
			mode = info.Mode().Perm() | 0o600
		}
		_ = root.Chmod(p, mode)
		return nil
	})
}
