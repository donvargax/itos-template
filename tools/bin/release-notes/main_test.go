package main

import (
	"strings"
	"testing"
)

// What a commit marked as breaking asks: its footer's text, wrapped lines
// kept, or its header for a ! alone; a footer outside the last paragraph is
// not one.
func TestBreaking(t *testing.T) {
	cases := map[string]string{
		"feat: a thing":      "",
		"feat!: drop a flag": "feat!: drop a flag",
		"feat!: drop a flag\n\nBREAKING-CHANGE: use --x":                                        "use --x",
		"fix: a bug\n\nWhy.\n\nBREAKING CHANGE: a key goes":                                     "a key goes",
		"fix: a bug\n\nWhy.\n\nBREAKING-CHANGE: rename the\n  flag in scripts\nUpgrading: none": "rename the\nflag in scripts",
		"fix: a bug\n\nBREAKING-CHANGE: not a footer here\n\nTask: T-1":                         "",
	}
	for message, want := range cases {
		if got := breaking(message); got != want {
			t.Errorf("breaking(%q) = %q, want %q", message, got, want)
		}
	}
}

// itos commit footers lists a footer's first line; whole gives it the lines
// the commit wrapped it onto.
func TestWhole(t *testing.T) {
	commits := parseLog("abcdef1234\x1ffeat: a thing\n\nWhy.\n\nUpgrading: rename the flag\n  in your scripts\nScenarios: ID-CLI-02\n\x1e\n")
	listed := []footer{{SHA: "abcdef1", Subject: "feat: a thing", Text: "rename the flag"}}
	whole(listed, "Upgrading", commits)
	if want := "rename the flag\nin your scripts"; listed[0].Text != want {
		t.Errorf("whole: %q, want %q", listed[0].Text, want)
	}
}

// The notes: the title, the count and why, what changed, then Upgrading with
// every breaking change and every Upgrading footer quoted, and how to check.
func TestRender(t *testing.T) {
	n := notes{
		Version:    "1.0.0",
		Repository: repository,
		From:       "v0.1.0",
		Changed:    "### Features\n\n- A thing",
		Commits: parseLog("a1b2c3d4\x1ffeat!: drop a flag\n\nBREAKING-CHANGE: use --x\n\x1e" +
			"b2c3d4e5\x1ffix: a bug\n\nUpgrading: none\n\x1e" +
			"c3d4e5f6\x1fdocs: say so\n\x1e"),
		Upgrading: []footer{{SHA: "a1b2c3d", Subject: "feat!: drop a flag", Text: "Use --x."}},
	}
	got, err := n.render()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# itos-template 1.0.0\n",
		"Cut by CI from the 3 commits since v0.1.0: 1 feat, 1 fix and 1 breaking change,",
		"https://github.com/donvargax/itos-template/compare/v0.1.0...v1.0.0",
		"## What changed\n\n### Features\n\n- A thing\n",
		"   - Breaking, a1b2c3d `feat!: drop a flag`:\n     > use --x\n",
		"   - a1b2c3d `feat!: drop a flag`:\n     > Use --x.\n",
		"`itos-template --version` prints `itos-template 1.0.0`",
		"gh attestation verify <archive> -R donvargax/itos-template",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the notes lack %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "## Upgrading") < strings.Index(got, "## What changed") {
		t.Errorf("Upgrading is not after What changed:\n%s", got)
	}

	first, err := notes{Version: "0.1.0", Repository: repository, Commits: parseLog("a1\x1ffeat: a thing\n\x1e")}.render()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"since the first commit: 1 feat, 0 fixes and 0 breaking changes",
		"No commit since the first commit is a breaking change. Every `feat` and `fix` since the first commit says `Upgrading: none`.",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("a first release's notes lack %q:\n%s", want, first)
		}
	}
	if strings.Contains(first, "compare/") {
		t.Errorf("a first release's notes link a comparison:\n%s", first)
	}
}
