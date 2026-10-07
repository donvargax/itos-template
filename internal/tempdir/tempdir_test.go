package tempdir

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Removing a folder as windows refuses to is held here: the scenarios run
// on windows, but none can make a folder windows will not remove at once,
// nor see a temporary folder left behind (decision 18).

// A folder holding read-only files and folders, as git leaves its objects,
// is removed whole.
func TestRemoveRemovesReadOnlyFiles(t *testing.T) {
	dir, err := Make("itos-template-tempdir-test-")
	if err != nil {
		t.Fatal(err)
	}
	objects := filepath.Join(dir, "repo.git", "objects", "ab")
	if err := os.MkdirAll(objects, 0o755); err != nil {
		t.Fatal(err)
	}
	object := filepath.Join(objects, "cdef")
	if err := os.WriteFile(object, []byte("blob"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(objects, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s is still there: %v", dir, err)
	}
}

func TestRemoveTakesAFolderThatIsGone(t *testing.T) {
	if err := Remove(filepath.Join(t.TempDir(), "gone")); err != nil {
		t.Errorf("Remove: %v", err)
	}
}
