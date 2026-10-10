package git

import "strings"

// Here is the name of the template check reads when none is named: the
// repository it runs in.
const Here = "."

// CloneHere copies the repository whose top is top, as Top gives the
// working folder's, whatever folder of it that is, bare into dir, which must
// not exist: the template check reads when none is named, a CI checkout as
// git clone or actions/checkout leaves one. Each branch is the local branch of its name,
// else origin's remote-tracking branch of that name, so the branches a
// checkout holds only as origin's are read with no fetch of them first.
// Its tags are copied too, so a release (check --ref) is read from the tags
// the checkout holds.
//
// The root branch, the copy's HEAD, is the branch the checkout's HEAD names;
// a detached HEAD (actions/checkout on a pull request) names none, so it is
// the branch origin's HEAD names, as git clone records it, else as origin
// says it, asked with git ls-remote, since actions/checkout records none.
// When none of them names one, the copy has no root (DefaultBranch fails).
//
// A failure is an *Unreachable naming Here, or a *Missing when git cannot be
// run.
func CloneHere(top, dir string) (*Repo, error) {
	if _, err := run("", "init", "--bare", "--quiet", dir); err != nil {
		return nil, err
	}
	r := &Repo{dir: dir}
	// Origin's first, the local branches over them, so a local branch wins.
	for _, refspecs := range [][]string{
		{"+refs/remotes/origin/*:refs/heads/*", "^refs/remotes/origin/HEAD"},
		{"+refs/heads/*:refs/heads/*", "+refs/tags/*:refs/tags/*"},
	} {
		args := append([]string{"fetch", "--quiet", "--no-tags", top}, refspecs...)
		if _, err := r.git(args...); err != nil {
			return nil, unreachable(Here, err)
		}
	}
	root, ok := rootOf(top)
	if !ok {
		r.noRoot = true
		return r, nil
	}
	if _, err := r.git("symbolic-ref", "HEAD", "refs/heads/"+root); err != nil {
		return nil, err
	}
	return r, nil
}

// Top is the top folder of the repository the working folder is in, an
// absolute path as git gives it (with / on windows too), or a bare
// repository's own folder; an *Unreachable naming Here when the working
// folder is in no repository, or a *Missing when git cannot be run.
func Top() (string, error) {
	if out, err := run("", "rev-parse", "--show-toplevel"); err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	out, err := run("", "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", unreachable(Here, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// rootOf is the root branch of the repository at top: the branch its HEAD
// names, else the one origin's HEAD names, recorded or asked of origin; and
// false when none names one.
func rootOf(top string) (string, bool) {
	if ref, err := run(top, "symbolic-ref", "--quiet", "HEAD"); err == nil {
		if b, ok := strings.CutPrefix(strings.TrimSpace(string(ref)), "refs/heads/"); ok {
			return b, true
		}
	}
	if ref, err := run(top, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil {
		if b, ok := strings.CutPrefix(strings.TrimSpace(string(ref)), "refs/remotes/origin/"); ok {
			return b, true
		}
	}
	// Never a prompt for credentials: a CI step has no one to answer it.
	out, err := command{dir: top, env: []string{"GIT_TERMINAL_PROMPT=0"}}.output("ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(out), "\n") {
		ref, name, _ := strings.Cut(strings.TrimSpace(line), "\t")
		if name != "HEAD" {
			continue
		}
		if b, ok := strings.CutPrefix(ref, "ref: refs/heads/"); ok {
			return b, true
		}
	}
	return "", false
}
