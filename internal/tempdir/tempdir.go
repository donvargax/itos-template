// Package tempdir removes the temporary folders itos-template makes: a
// template's bare clone, and each render check makes and checks.
//
// os.RemoveAll can fail on windows where it succeeds elsewhere: git writes
// its object and pack files read-only, a file is not removed while a
// program still has it open, and a check's own programs (a build's server,
// an antivirus scanning what a build wrote) can hold one for a moment after
// they exit. Remove makes every file writable and tries again, a few times,
// before it gives up.
package tempdir

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Make makes a new temporary folder, its name starting with prefix.
func Make(prefix string) (string, error) {
	return os.MkdirTemp("", prefix)
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

// writable makes every file and folder under dir writable by its owner,
// which on windows clears the read-only attribute git gives its objects.
func writable(dir string) {
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		mode := os.FileMode(0o600)
		if d.IsDir() {
			mode = 0o700
		} else if info, err := d.Info(); err == nil {
			mode = info.Mode().Perm() | 0o600
		}
		_ = os.Chmod(p, mode)
		return nil
	})
}
