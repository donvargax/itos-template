// Package git is the infra that runs the real git (decision 17): it clones
// a template, or copies the checkout it runs in, its branches its own or
// else origin's (CloneHere), and merges its branches (Repo,
// port.Repository), and makes a
// project's folder a repository and commits it (Committer, port.Committer).
// It imports no package of ours but the ports it implements. Reading git's
// output and exit codes, and turning them into the ports' terms (a merge's
// conflicts, a commit refused) or its own sealed errors (Error), is its
// anti-corruption layer, kept here; internal/cli gives each of its errors
// an exit code.
//
// The git it runs is the one ITOS_GIT names, as itos sets it for every
// program it starts, else the first git on the PATH that is not an itos.
// itos can be linked as git before the real one on the PATH (itos git-shim
// install), and itos-template's own git commands (a clone, a merge, a
// project's first commit) must never run an itos's policy instead.
//
// The rule is itos's own (its internal/git's Bin and IsItos, its bug 45 and
// T-104), copied since a module cannot import another's internal packages.
// features/git_test.go keeps a copy of it for the steps until the idea
// shared-real-git has them use this one.
package git

import (
	"bytes"
	"debug/buildinfo"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

// Bin is the git itos-template runs: the one ITOS_GIT names, unless it names
// an itos, else the first git on the PATH that is not an itos, else "git",
// which then fails as a missing git does. Found once a run.
var Bin = sync.OnceValue(func() string {
	if p := os.Getenv("ITOS_GIT"); p != "" && !IsItos(p) {
		return p
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		for _, name := range names() {
			p := filepath.Join(dir, name)
			info, err := os.Stat(p)
			if err != nil || !executable(info) || IsItos(p) {
				continue
			}
			return p
		}
	}
	return "git"
})

// IsItos is whether the program at p is an itos, so never the real git: a
// symbolic link whose target, at any link of the chain, is a file named itos
// (itos.exe), as git-shim install makes one; or a Go binary built from itos's
// cmd/itos, any major version, as its build information records it, which a
// hard link or a copy of any itos keeps and costs a read of the file, never a
// run of it.
func IsItos(p string) bool {
	return linksNamedItos(p) || builtAsItos(p)
}

func linksNamedItos(p string) bool {
	for range 40 {
		target, err := os.Readlink(p)
		if err != nil {
			return false
		}
		if strings.TrimSuffix(strings.ToLower(filepath.Base(target)), ".exe") == "itos" {
			return true
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(p), target)
		}
		p = target
	}
	return false
}

var itosMain = regexp.MustCompile(`^github\.com/donvargax/itos(/v[0-9]+)?/cmd/itos$`)

func builtAsItos(p string) bool {
	info, err := buildinfo.ReadFile(p)
	return err == nil && itosMain.MatchString(info.Path)
}

// names are the file names git may have in a PATH folder: git, or on windows
// git with each of PATHEXT's extensions.
func names() []string {
	if runtime.GOOS != "windows" {
		return []string{"git"}
	}
	exts := os.Getenv("PATHEXT")
	if exts == "" {
		exts = ".com;.exe;.bat;.cmd"
	}
	var found []string
	for _, ext := range strings.Split(strings.ToLower(exts), ";") {
		if ext != "" {
			found = append(found, "git"+ext)
		}
	}
	return found
}

func executable(info os.FileInfo) bool {
	return info.Mode().IsRegular() && (runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0)
}

// ownRepository are the variables that point git at a repository other than
// the one its folder holds.
var ownRepository = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR", "GIT_PREFIX", "GIT_NAMESPACE",
}

// Environ is this process's environment less what points git at another
// repository than the one its folder holds (a hook's GIT_DIR, say), extra
// added: what itos-template's own git commands run in, and the programs a
// check runs in a render, which may run git themselves.
func Environ(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !containsFold(ownRepository, name) {
			env = append(env, kv)
		}
	}
	return append(env, extra...)
}

func containsFold(list []string, s string) bool {
	for _, item := range list {
		if strings.EqualFold(item, s) {
			return true
		}
	}
	return false
}

// command is one git run: in dir, reading stdin, with env added to Environ,
// so the command reads only dir's repository.
type command struct {
	dir   string
	stdin io.Reader
	env   []string
}

// output runs git with args and returns its standard output, or a *Failed
// or a *Missing.
func (c command) output(args ...string) ([]byte, error) {
	var stdout bytes.Buffer
	err := c.stream(&stdout, args...)
	return stdout.Bytes(), err
}

// stream runs git with args, its standard output written to stdout, and
// returns nil, a *Failed or a *Missing.
func (c command) stream(stdout io.Writer, args ...string) error {
	cmd := exec.Command(Bin(), args...)
	cmd.Dir = c.dir
	cmd.Env = Environ(c.env...)
	cmd.Stdin = c.stdin
	cmd.Stdout = stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return &Failed{Args: args, Code: exit.ExitCode(), Stderr: stderr.String()}
	}
	return &Missing{Err: err}
}

// run is output in dir.
func run(dir string, args ...string) ([]byte, error) {
	return command{dir: dir}.output(args...)
}

// exitCode reads err's git exit code: -1 when err is no *Failed.
func exitCode(err error) int {
	var f *Failed
	if errors.As(err, &f) {
		return f.Code
	}
	return -1
}
