package git

import (
	"strings"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Local is a template's clone, the port.Clone branch sync and branch list
// run in: read at its top as a Worktree reads one, its local branches beside
// origin's default branch, and merged into with git merge itself, through
// its command line, as decision 38 has update merge with git's own merge
// (decision 42). The merge is git merge --no-commit --no-ff, so git leaves
// it in progress, MERGE_HEAD naming the branch merged, even where a
// fast-forward was possible, staged or conflicted as it leaves any merge, by
// its own rules and the person's config (core.autocrlf, merge.conflictStyle,
// rerere) on every system. No ref moves: HEAD, the branches and every commit
// stay as they are.
type Local struct {
	*Worktree
}

var _ port.Clone = (*Local)(nil)

// OpenLocal is the repository the folder dir is in, not looked at yet.
func OpenLocal(dir string) *Local { return &Local{Worktree: OpenWorktree(dir)} }

// HasOrigin is whether git's config names a URL for the remote origin.
func (l *Local) HasOrigin() (bool, error) {
	return (&Tagger{top: l.top}).HasOrigin()
}

// DefaultBranch is the branch origin's HEAD names, whatever is checked out
// (originsHead), as release reads its root.
func (l *Local) DefaultBranch() (string, bool) { return originsHead(l.top) }

// Branch is the branch HEAD names, as git symbolic-ref reads it: false for
// a detached HEAD.
func (l *Local) Branch() (string, bool, error) {
	out, err := command{dir: l.top, env: readOnly}.output("symbolic-ref", "--quiet", "HEAD")
	if exitCode(err) == 1 {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	b, ok := strings.CutPrefix(strings.TrimSpace(string(out)), "refs/heads/")
	return b, ok, nil
}

// Head is the commit the local branch is at, refs/heads/<branch>, false
// when there is none.
func (l *Local) Head(branch string) (string, bool, error) {
	return l.repo().Commit(branch)
}

// File is a file's contents at a commit, as a clone's Repo reads one.
func (l *Local) File(commit, path string) ([]byte, bool, error) {
	return l.repo().File(commit, path)
}

// IsAncestor is whether commit is of or one of its ancestors, as git
// merge-base --is-ancestor says.
func (l *Local) IsAncestor(commit, of string) (bool, error) {
	return l.repo().IsAncestor(commit, of)
}

// repo is the clone read as a template's Repo reads its own objects.
func (l *Local) repo() *Repo { return &Repo{dir: l.top} }

// Merging is whether git has a merge in progress: MERGE_HEAD names a
// commit.
func (l *Local) Merging() (bool, error) {
	_, err := command{dir: l.top, env: readOnly}.output("rev-parse", "--quiet", "--verify", "MERGE_HEAD")
	if exitCode(err) == 1 {
		return false, nil
	}
	return err == nil, err
}

// Merge merges the local branch branch into into, the branch checked out,
// with git merge --no-commit --no-ff, the branch named by its whole ref so
// no tag of its name is merged instead, and the message git would give a
// merge of it by its name, which git commit takes. git exits 1 for a merge
// it left in progress with conflicts, some perhaps resolved by the
// person's rerere already; any other failure is git's. The paths it staged
// and left conflicted are git status's, from HEAD.
func (l *Local) Merge(branch, into string) (port.Staged, error) {
	message := "Merge branch '" + branch + "' into " + into
	_, err := run(l.top, "merge", "--no-commit", "--no-ff", "--quiet", "-m", message, "refs/heads/"+branch)
	if err != nil && exitCode(err) != 1 {
		return port.Staged{}, err
	}
	return l.staged()
}

// staged are the paths the index changes from HEAD, and those it holds
// unmerged, as git status lists them: an unmerged entry's two letters are
// one of DD, AU, UD, UA, DU, AA and UU, the rest staged where the index's
// letter is not a space. Untracked files are left out.
func (l *Local) staged() (port.Staged, error) {
	out, err := command{dir: l.top, env: readOnly}.output("status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=no")
	if err != nil {
		return port.Staged{}, err
	}
	var s port.Staged
	for _, entry := range strings.Split(string(out), "\x00") {
		if len(entry) < 4 {
			continue
		}
		xy, p := entry[:2], entry[3:]
		switch {
		case xy == "DD" || xy == "AA" || strings.Contains(xy, "U"):
			s.Conflicts = append(s.Conflicts, p)
		case xy[0] != ' ':
			s.Paths = append(s.Paths, p)
		}
	}
	return s, nil
}
