package main

import (
	"strings"
	"testing"
)

// The bump the commits since the last release ask for, the cases itos's
// tools/selftest/release-cut.ts proves over scratch histories.
func TestBumpOf(t *testing.T) {
	cases := []struct {
		name     string
		messages []string
		kind     string
	}{
		{"a feat", []string{"feat: a thing"}, "minor"},
		{"a fix", []string{"fix: a bug"}, "patch"},
		{"a scoped fix", []string{"fix(cli): a bug"}, "patch"},
		{"a feat and a fix", []string{"fix: a bug", "feat: a thing", "docs: say so"}, "minor"},
		{"a BREAKING-CHANGE: footer", []string{"feat: a thing", "fix: a bug\n\nWhy.\n\nBREAKING-CHANGE: a key goes"}, "major"},
		{"a BREAKING CHANGE: footer", []string{"fix: a bug\n\nBREAKING CHANGE: a key goes\nTask: T-1"}, "major"},
		{"a feat! header", []string{"feat!: a thing"}, "major"},
		{"a refactor! header", []string{"refactor(config)!: a key goes"}, "major"},
		{"docs, ci, test and build commits", []string{"docs: say so", "ci: run it", "test: prove it", "build: pin it"}, "none"},
		{"a BREAKING-CHANGE line outside the last paragraph", []string{"fix: a bug\n\nBREAKING-CHANGE: not a footer here\n\nTask: T-1"}, "patch"},
		{"no commit", nil, "none"},
	}
	for _, c := range cases {
		b := bumpOf(c.messages)
		if b.kind != c.kind || b.commits != len(c.messages) {
			t.Errorf("%s: bumpOf = %s over %d commit(s), want %s over %d", c.name, b.kind, b.commits, c.kind, len(c.messages))
		}
	}
}

// The next version from the last release's tag, 0.0.0 with none.
func TestBumped(t *testing.T) {
	cases := []struct{ tag, kind, want string }{
		{"", "minor", "0.1.0"},
		{"", "patch", "0.0.1"},
		{"", "major", "1.0.0"},
		{"v1.2.3", "minor", "1.3.0"},
		{"v1.2.3", "patch", "1.2.4"},
		{"v1.2.3", "major", "2.0.0"},
		{"v1.9.9", "minor", "1.10.0"},
	}
	for _, c := range cases {
		if got := bumped(c.tag, c.kind); got != c.want {
			t.Errorf("bumped(%q, %s) = %s, want %s", c.tag, c.kind, got, c.want)
		}
	}
}

// A version whose major go.mod's module path does not match is refused: v0
// and v1 from a path with no /vN suffix, vN from v2 from one ending in /vN.
func TestMismatch(t *testing.T) {
	const module = "github.com/donvargax/itos-template"
	cases := []struct {
		next, module string
		refused      bool
	}{
		{"0.1.0", module, false},
		{"1.4.0", module, false},
		{"2.0.0", module, true},
		{"2.0.0", module + "/v2", false},
		{"3.0.0", module + "/v2", true},
		{"1.0.0", module + "/v2", true},
		{"5.0.0", "", false},
	}
	for _, c := range cases {
		problem := mismatch(c.next, c.module)
		if (problem != "") != c.refused {
			t.Errorf("mismatch(%s, %q) = %q, want refused %v", c.next, c.module, problem, c.refused)
		}
	}
	if problem := mismatch("2.0.0", module); !strings.Contains(problem, module+"/v2") {
		t.Errorf("the refusal does not name the path a v2 needs: %s", problem)
	}
}

// git log's records split into each commit's message, trimmed.
func TestMessages(t *testing.T) {
	log := "aaa\x1ffeat: a thing\n\nWhy.\n\x1e\nbbb\x1ffix: a bug\n\x1e\n"
	got := messages(log)
	if len(got) != 2 || got[0] != "feat: a thing\n\nWhy." || got[1] != "fix: a bug" {
		t.Errorf("messages = %q", got)
	}
}
