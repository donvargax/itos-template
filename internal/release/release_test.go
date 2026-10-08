package release

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos-template/internal/release/releasetest"
)

// What the release cut reads of a commit: its type, and whether it is marked
// as breaking (a ! header, or the footer in the last paragraph only).
func TestTypeAndBreaking(t *testing.T) {
	cases := []struct {
		message, typ string
		breaking     bool
	}{
		{"feat: add the archive", "feat", false},
		{"fix(cli): mend the archive", "fix", false},
		{"docs: describe the archive", "docs", false},
		{"refactor!: rename the archive", "refactor", true},
		{"feat(config)!: drop a key", "feat", true},
		{"chore: tidy\n\nWhy.\n\nBREAKING-CHANGE: gone", "chore", true},
		{"chore: tidy\n\nBREAKING CHANGE: gone\n\nWhy.", "chore", false},
		{"chore: tidy\n\nWhy.\n\nBREAKING CHANGE: gone", "chore", true},
		{"Merge branch 'main'", "", false},
		{"feature: not a type the release cut counts", "feature", false},
		{"feat:no space after the colon is not a header", "", false},
	}
	for _, c := range cases {
		if got := Type(c.message); got != c.typ {
			t.Errorf("Type(%q) = %q, want %q", c.message, got, c.typ)
		}
		if got := Breaking(c.message); got != c.breaking {
			t.Errorf("Breaking(%q) = %v, want %v", c.message, got, c.breaking)
		}
	}
}

func TestNewest(t *testing.T) {
	cases := []struct {
		tags []string
		want string
	}{
		{nil, ""},
		{[]string{"latest", "1.2.0", "v1.2", "v1.2.0.1", "V1.2.0"}, ""},
		{[]string{"v1.2.0", "v1.10.0", "v1.9.9"}, "v1.10.0"},
		{[]string{"v2.0.0-rc.1", "v1.9.0"}, "v1.9.0"},
		{[]string{"v2.0.0-rc.1", "v2.0.0"}, "v2.0.0"},
		{[]string{"v2.0.0+build.1", "v1.9.0"}, "v1.9.0"},
		{[]string{"v2.0.0-rc.1", "v2.0.0+build.1"}, ""},
		{[]string{"v1.0.0", "v0.1.0", "v0.10.0"}, "v1.0.0"},
		{[]string{"v1.09.0", "v1.10.0"}, "v1.10.0"},
		{[]string{"v1.09.0", "v1.8.0"}, "v1.09.0"},
		{[]string{"v01.0.0", "v0.1.0"}, "v01.0.0"},
		{[]string{"v01.0.0", "v1.0.0", "v001.0.0"}, "v1.0.0"},
		{[]string{"v1.0.0", "v01.0.0"}, "v1.0.0"},
		{[]string{"v99999999999999999999.0.0", "v9.0.0"}, "v99999999999999999999.0.0"},
	}
	for _, c := range cases {
		if got := Newest(c.tags); got != c.want {
			t.Errorf("Newest(%q) = %q, want %q", c.tags, got, c.want)
		}
	}
}

// What moved beneath the binary (decision 23): a linked module's version, a
// module newly linked or no longer linked, and the toolchain, each named;
// the same modules at the same versions with the same toolchain is nothing.
func TestMoved(t *testing.T) {
	old := Build{
		Modules:   map[string]string{"example.com/a": "v1.0.0", "example.com/b": "v1.0.0", "example.com/r": "v1.0.0 => ./r"},
		Toolchain: "go1.27.1",
	}
	cases := []struct {
		name string
		new  Build
		want []string
	}{
		{"the same", Build{Modules: map[string]string{"example.com/a": "v1.0.0", "example.com/b": "v1.0.0", "example.com/r": "v1.0.0 => ./r"}, Toolchain: "go1.27.1"}, nil},
		{"a version moved", Build{Modules: map[string]string{"example.com/a": "v1.1.0", "example.com/b": "v1.0.0", "example.com/r": "v1.0.0 => ./r"}, Toolchain: "go1.27.1"},
			[]string{"example.com/a v1.0.0 to v1.1.0"}},
		{"a replacement moved", Build{Modules: map[string]string{"example.com/a": "v1.0.0", "example.com/b": "v1.0.0", "example.com/r": "v1.0.0 => example.com/s v1.0.1"}, Toolchain: "go1.27.1"},
			[]string{"example.com/r v1.0.0 => ./r to v1.0.0 => example.com/s v1.0.1"}},
		{"one came and one went", Build{Modules: map[string]string{"example.com/a": "v1.0.0", "example.com/c": "v0.1.0", "example.com/r": "v1.0.0 => ./r"}, Toolchain: "go1.27.1"},
			[]string{"example.com/b v1.0.0, no longer linked", "example.com/c v0.1.0, newly linked"}},
		{"the toolchain moved", Build{Modules: map[string]string{"example.com/a": "v1.0.0", "example.com/b": "v1.0.0", "example.com/r": "v1.0.0 => ./r"}, Toolchain: "go1.27.2"},
			[]string{"the toolchain go1.27.1 to go1.27.2"}},
	}
	for _, c := range cases {
		got := Moved(old, c.new)
		if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("%s: Moved = %q, want %q", c.name, got, c.want)
		}
	}
}

// Each side read from its commit of an example repository (decision 19):
// the module the binary links and the toolchain, as they moved in a commit;
// a module only its tests import, moved, is nothing; and a revision git does
// not know is an error, never none.
func TestBinaryMoved(t *testing.T) {
	repo := releasetest.Example(t, true)
	releasetest.Change(t, filepath.Join(repo, "go.mod"), "example.com/lib v1.0.0", "example.com/lib v1.1.0")
	releasetest.Change(t, filepath.Join(repo, "go.mod"), "toolchain go1.24.0", "toolchain go1.24.1")
	releasetest.Commit(t, repo, "build: move lib and Go")
	releasetest.Change(t, filepath.Join(repo, "go.mod"), "example.com/testonly v1.0.0", "example.com/testonly v1.1.0")
	releasetest.Commit(t, repo, "build: move testonly")
	t.Chdir(repo)
	cases := []struct {
		from, to string
		want     []string
	}{
		{"v0.1.0", "HEAD~1", []string{"example.com/lib v1.0.0 => ./lib to v1.1.0 => ./lib", "the toolchain go1.24.0 to go1.24.1"}},
		{"HEAD~1", "HEAD", nil},
		{"v0.1.0", "v0.1.0", nil},
	}
	for _, c := range cases {
		got, err := BinaryMoved(c.from, c.to)
		if err != nil {
			t.Fatalf("BinaryMoved(%s, %s): %v", c.from, c.to, err)
		}
		if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("BinaryMoved(%s, %s) = %q, want %q", c.from, c.to, got, c.want)
		}
	}
	if _, err := BinaryMoved("v9.9.9", "HEAD"); err == nil || !strings.Contains(err.Error(), "v9.9.9") {
		t.Errorf("BinaryMoved from a tag that is not there: %v, want an error naming it", err)
	}
}
