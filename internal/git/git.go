// Package git runs the real git: the one ITOS_GIT names, as itos sets it for
// every program it starts, else the first git on the PATH that is not an
// itos. itos can be linked as git before the real one on the PATH (itos
// git-shim install), and itos-template's own git commands (a clone, a merge,
// a project's first commit) must never run an itos's policy instead.
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
	"fmt"
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

// Error is a git command that failed: Missing when no git could be run at
// all, else its exit code and what it said on stderr, from which a caller
// reads the kind of the failure (docs/CLI.md, rule 31).
type Error struct {
	Args    []string
	Code    int
	Stderr  string
	Missing bool
	Err     error
}

func (e *Error) Error() string {
	if e.Missing {
		return fmt.Sprintf("cannot run git: %v", e.Err)
	}
	if line := e.LastLine(); line != "" {
		return line
	}
	return fmt.Sprintf("git %s exited %d", strings.Join(e.Args, " "), e.Code)
}

func (e *Error) Unwrap() error { return e.Err }

// LastLine is the last line git wrote on stderr that is not empty: what it
// said went wrong.
func (e *Error) LastLine() string {
	lines := strings.Split(strings.TrimSpace(e.Stderr), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// Code reads err's git exit code: -1 when err is no *Error.
func Code(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return -1
}

// Command is one git run: in Dir, reading Stdin, with Env added to an
// environment that holds nothing of a git repository around the caller's
// (a hook's GIT_DIR, say), so the command reads only Dir.
type Command struct {
	Dir   string
	Stdin io.Reader
	Env   []string
}

// ownRepository are the variables that point git at a repository other than
// the one its folder holds.
var ownRepository = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR", "GIT_PREFIX", "GIT_NAMESPACE",
}

func (c Command) environ() []string { return Environ(c.Env...) }

// Environ is this process's environment less what points git at another
// repository than the one its folder holds (a hook's GIT_DIR, say), extra
// added: what itos-template's own git commands run in, and the programs it
// runs in a render, which may run git themselves.
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

// Output runs git with args and returns its standard output, or an *Error.
func (c Command) Output(args ...string) ([]byte, error) {
	var stdout bytes.Buffer
	err := c.Stream(&stdout, args...)
	return stdout.Bytes(), err
}

// Stream runs git with args, its standard output written to stdout, and
// returns nil or an *Error.
func (c Command) Stream(stdout io.Writer, args ...string) error {
	cmd := exec.Command(Bin(), args...)
	cmd.Dir = c.Dir
	cmd.Env = c.environ()
	cmd.Stdin = c.Stdin
	cmd.Stdout = stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return &Error{Args: args, Code: exit.ExitCode(), Stderr: stderr.String(), Err: err}
	}
	return &Error{Args: args, Code: -1, Missing: true, Err: err}
}

// Run is Output in dir.
func Run(dir string, args ...string) ([]byte, error) {
	return Command{Dir: dir}.Output(args...)
}
