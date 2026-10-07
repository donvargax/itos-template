// The real git, which the steps run and leave on the scenarios' PATH. itos
// can be linked as git before the real one on the PATH (itos git-shim
// install), so the git the steps run is never "git" looked up on the PATH:
// it is the one ITOS_GIT names, as itos sets it for every program it starts,
// else the first git on the PATH that is not an itos. The rule is itos's own
// (internal/git's Bin and IsItos, its bug 45 and T-104), copied since a
// module cannot import another's internal packages.
package features

import (
	"debug/buildinfo"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

// gitBin is the git the steps run: the one ITOS_GIT names, unless it names an
// itos, else the first git on the PATH that is not an itos, else "git",
// which then fails as a missing git does. Found once a run.
var gitBin = sync.OnceValue(func() string {
	if p := os.Getenv("ITOS_GIT"); p != "" && !isItos(p) {
		return p
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		for _, name := range gitNames() {
			p := filepath.Join(dir, name)
			info, err := os.Stat(p)
			if err != nil || !executable(info) || isItos(p) {
				continue
			}
			return p
		}
	}
	return "git"
})

// isItos is whether the program at p is an itos, so never the real git: a
// symbolic link whose target, at any link of the chain, is a file named itos
// (itos.exe), as git-shim install makes one; or a Go binary built from itos's
// cmd/itos, any major version, as its build information records it, which a
// hard link or a copy of any itos keeps and costs a read of the file, never a
// run of it.
func isItos(p string) bool {
	return linksNamedItos(p) || builtAsItos(p)
}

// linksNamedItos is whether p is a symbolic link whose target, or the target
// of any link after it, is named itos.
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

// itosMain is the main package of an itos build, at any major version of its
// module.
var itosMain = regexp.MustCompile(`^github\.com/donvargax/itos(/v[0-9]+)?/cmd/itos$`)

// builtAsItos is whether the program at p is a Go binary built from itos's
// cmd/itos.
func builtAsItos(p string) bool {
	info, err := buildinfo.ReadFile(p)
	return err == nil && itosMain.MatchString(info.Path)
}

// gitNames are the file names git may have in a PATH folder: git, or on
// windows git with each of PATHEXT's extensions.
func gitNames() []string {
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

// executable is whether a file can be run: a regular file, with an execute
// bit outside windows, which has none.
func executable(info os.FileInfo) bool {
	return info.Mode().IsRegular() && (runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0)
}
