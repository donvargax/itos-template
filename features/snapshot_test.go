// The snapshot of a folder, saved before a command runs and compared
// after: every file and folder under it, .git included, each with its mode
// and contents, so a command that changed HEAD, the index, a ref or an
// object, or left any file behind, is caught. adopt.feature's steps take
// one; any command that must leave a project as it found it can. And the
// scenario's trash, where a step moves what it takes out of the way.
package features

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// entry is what a snapshot keeps of a path under a folder: its mode, and a
// file's contents or a link's target.
type entry struct {
	mode fs.FileMode
	data string
}

// snapshotOf is every path under the folder dir, .git included, with /, and
// what is there.
func (w *world) snapshotOf(dir string) (map[string]entry, error) {
	top := w.path(dir)
	snap := map[string]entry{}
	err := filepath.WalkDir(top, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(top, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		e := entry{mode: info.Mode()}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			e.data, err = os.Readlink(p)
		case info.Mode().IsRegular():
			var data []byte
			data, err = os.ReadFile(p)
			e.data = string(data)
		}
		snap[filepath.ToSlash(rel)] = e
		return err
	})
	return snap, err
}

// saveSnapshot keeps the snapshot of the folder dir, for the steps that
// compare it after.
func (w *world) saveSnapshot(dir string) error {
	snap, err := w.snapshotOf(dir)
	if err != nil {
		return err
	}
	if w.snapshots == nil {
		w.snapshots = map[string]map[string]entry{}
	}
	w.snapshots[dir] = snap
	return nil
}

// snapshotUnchanged is whether the folder dir is as its saved snapshot
// has it, every path, .git included, so HEAD, the index and every ref and
// object too; or, with record, as it has it but for one new file at its
// top, .itos-template.yaml, which the index, unchanged, does not hold.
func (w *world) snapshotUnchanged(dir string, record bool) error {
	before, ok := w.snapshots[dir]
	if !ok {
		return fmt.Errorf("no snapshot of %s was saved", dir)
	}
	after, err := w.snapshotOf(dir)
	if err != nil {
		return err
	}
	if record {
		e, ok := after[".itos-template.yaml"]
		switch {
		case !ok:
			return fmt.Errorf("%s holds no record\n%s", dir, w.report())
		case !e.mode.IsRegular():
			return fmt.Errorf("%s's record is no file: %v", dir, e.mode)
		}
		if _, was := before[".itos-template.yaml"]; was {
			return fmt.Errorf("%s held a record before", dir)
		}
		delete(after, ".itos-template.yaml")
	}
	var changed []string
	for _, p := range slices.Sorted(maps.Keys(before)) {
		if e, ok := after[p]; !ok {
			changed = append(changed, "removed "+p)
		} else if e != before[p] {
			changed = append(changed, "changed "+p)
		}
	}
	for _, p := range slices.Sorted(maps.Keys(after)) {
		if _, ok := before[p]; !ok {
			changed = append(changed, "added "+p)
		}
	}
	if len(changed) > 0 {
		return fmt.Errorf("%s changed:\n%s\n%s", dir, strings.Join(changed, "\n"), w.report())
	}
	return nil
}

// trashFolder is the scenario's trash, a temporary folder removed after it,
// made the first time it is asked for.
func (w *world) trashFolder() (string, error) {
	if w.trash == "" {
		var err error
		if w.trash, err = os.MkdirTemp("", "itos-template-features-trash-"); err != nil {
			return "", err
		}
	}
	return w.trash, nil
}

// moveAway moves p into the scenario's trash.
func (w *world) moveAway(p string) error {
	trash, err := w.trashFolder()
	if err != nil {
		return err
	}
	to, err := os.MkdirTemp(trash, "moved-")
	if err != nil {
		return err
	}
	return os.Rename(p, filepath.Join(to, filepath.Base(p)))
}
