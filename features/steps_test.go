// The steps. itos-template is a black box here: each scenario gets a scratch
// git repository in a temporary folder, runs the binary TestFeatures built
// from this tree in it, and reads its exit code and output. Nothing here
// imports or reads itos-template's code, so the steps judge it by its command
// line alone. Harvested from itos's (github.com/donvargax/itos,
// features/steps_test.go), its domain's steps left behind.
package features

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/cucumber/godog"
)

// One scenario's state.
type world struct {
	root string // the itos-template checkout: where go.mod is
	bin  string // the itos-template binary under test
	dir  string // the scratch repository

	exit           int
	stdout, stderr string

	template    string // the fixture template's path, with /, for {template}
	templateDir string // the fixture template's folder

	named []string // the combinations of check's report the scenario named
}

func initializeScenario(sc *godog.ScenarioContext, root, bin string) {
	w := &world{root: root, bin: bin}
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		return ctx, w.setUp()
	})
	// The scenario's error is godog's already: returned here, it would be
	// reported twice, as the hook's and as the step's.
	sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		_ = os.RemoveAll(w.dir)
		return ctx, nil
	})

	// Split before {template} is expanded, so a path with a space stays one
	// argument. Quotes also preserve empty words for completion requests.
	sc.Step(`^itos-template runs with "([^"]*)"$`, func(args string) error {
		return w.runWith(w.env(), args)
	})

	sc.Step(`^it exits with code (\d+)$`, w.exitsWith)
	sc.Step(`^its standard output lists "([^"]*)"$`, func(text string) error {
		if !slices.Contains(outputLines(w.stdout), text) {
			return fmt.Errorf("standard output does not list %q\n%s", text, w.report())
		}
		return nil
	})
	sc.Step(`^its standard output does not list "([^"]*)"$`, func(text string) error {
		if slices.Contains(outputLines(w.stdout), text) {
			return fmt.Errorf("standard output lists %q\n%s", text, w.report())
		}
		return nil
	})
	sc.Step(`^the last line of its standard output is "([^"]*)"$`, func(text string) error {
		lines := outputLines(w.stdout)
		last := strings.TrimSuffix(lines[len(lines)-1], "\r")
		if last != text {
			return fmt.Errorf("the last line of standard output is %q, not %q\n%s", last, text, w.report())
		}
		return nil
	})
	sc.Step(`^its standard output says "([^"]*)"$`, func(text string) error {
		if !strings.Contains(w.stdout, text) {
			return fmt.Errorf("standard output does not say %q\n%s", text, w.report())
		}
		return nil
	})
	sc.Step(`^its standard output does not say "([^"]*)"$`, func(text string) error {
		if strings.Contains(w.stdout, text) {
			return fmt.Errorf("standard output says %q\n%s", text, w.report())
		}
		return nil
	})
	sc.Step(`^the first line of its standard output is "([^"]*)" and the stamped version$`, func(name string) error {
		return w.firstLineIs(name + " " + stampedVersion)
	})
	sc.Step(`^the second line of its standard output is "([^"]*)" and the stamped commit$`, func(word string) error {
		return w.secondLineIs(word + " " + stampedCommit)
	})
	w.newSteps(sc)
	w.checkSteps(sc)
}

// setUp makes the scenario's scratch repository, an empty git repository on
// main.
func (w *world) setUp() error {
	var err error
	if w.dir, err = os.MkdirTemp("", "itos-template-features-"); err != nil {
		return err
	}
	return w.git("init", "-q", "-b", "main")
}

// The folder holding go.mod, above the working directory go test gives.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the working directory")
		}
		dir = parent
	}
}

// The environment every command runs in: the caller's, less what would make
// git, itos or itos-template read anything but the scratch repository (a
// hook's GIT_DIR, the caller's ITOS_TEMPLATE_ settings, CI's), with no global
// or system git config and a fixed identity, and the caller's PATH without
// its claude, its itos, its itos extensions or a git that is an itos
// (callerPath).
func (w *world) env() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "GIT_") || strings.HasPrefix(name, "ITOS_") ||
			strings.HasPrefix(name, "GITHUB_") || strings.HasPrefix(name, "GH_") || name == "CI" ||
			strings.EqualFold(name, "PATH") {
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=itos-template features",
		"GIT_AUTHOR_EMAIL=features@localhost",
		"GIT_COMMITTER_NAME=itos-template features",
		"GIT_COMMITTER_EMAIL=features@localhost",
		"PATH="+callerPath(),
	)
}

// The programs no scenario reaches on the caller's PATH: its claude, its itos
// and its itos extensions (itos-*). An itos installed on the machine would
// run its own policy where a scenario meant only itos-template (itos's
// T-087), and itos runs and lists every extension it finds, so one installed
// on the machine would change what a scenario sees (itos's T-080).
var hiddenAlways = []string{"claude", "itos", "itos-*"}

var (
	hiddenPathsMu sync.Mutex
	hiddenPaths   = map[string]string{}
	callerPathDir string
)

// callerPath is the caller's PATH with none of hiddenAlways on it, so no
// scenario reaches the Claude Code or the itos of the machine it runs on: each
// folder holding one is replaced by a folder of links to everything else in
// it, or left out where links cannot be made (windows without the right to
// make them).
func callerPath() string { return pathHiding(hiddenAlways...) }

// pathHiding is the caller's PATH with none of the programs named on it, nor
// a git that is an itos, made once a run for each set of names. A git shim of
// the caller's (itos git-shim install) would run the global itos for a
// scenario's git commit and git push; it is told from the real git by the
// rule itos itself goes by (isItos, git_test.go), so every git a scenario's
// commands start is the real one, as the steps' own are (gitBin, itos's
// T-104).
func pathHiding(names ...string) string {
	hiddenPathsMu.Lock()
	defer hiddenPathsMu.Unlock()
	key := strings.Join(names, " ")
	if text, ok := hiddenPaths[key]; ok {
		return text
	}
	var folders []string
	for i, folder := range filepath.SplitList(os.Getenv("PATH")) {
		hidden := func(e os.DirEntry) bool {
			return isOneOf(e, names) || isOneOf(e, []string{"git"}) && isItos(filepath.Join(folder, e.Name()))
		}
		entries, err := os.ReadDir(folder)
		if err != nil || !slices.ContainsFunc(entries, hidden) {
			folders = append(folders, folder)
			continue
		}
		if callerPathDir == "" {
			if callerPathDir, err = os.MkdirTemp("", "itos-template-features-path-"); err != nil {
				continue
			}
		}
		links := filepath.Join(callerPathDir, strconv.Itoa(len(hiddenPaths)), strconv.Itoa(i))
		if linkAllBut(folder, links, entries, hidden) == nil {
			folders = append(folders, links)
		}
	}
	text := strings.Join(folders, string(os.PathListSeparator))
	hiddenPaths[key] = text
	return text
}

// removeCallerPath removes the folders callerPath and pathHiding made.
func removeCallerPath() {
	if callerPathDir != "" {
		_ = os.RemoveAll(callerPathDir)
	}
}

// isOneOf is whether a folder's entry is a program of the names the PATH
// would find: the name, or on windows the name with any extension, in any
// case. A name ending in * is a prefix, so itos-* is every itos extension.
func isOneOf(e os.DirEntry, names []string) bool {
	name := e.Name()
	same := func(a, b string) bool { return a == b }
	starts := strings.HasPrefix
	if runtime.GOOS == "windows" {
		name = strings.TrimSuffix(name, filepath.Ext(name))
		same = strings.EqualFold
		starts = func(s, prefix string) bool {
			return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
		}
	}
	return slices.ContainsFunc(names, func(n string) bool {
		if prefix, ok := strings.CutSuffix(n, "*"); ok {
			return starts(name, prefix)
		}
		return same(name, n)
	})
}

// linkAllBut makes links a folder of links to every entry of folder but the
// hidden ones.
func linkAllBut(folder, links string, entries []os.DirEntry, hidden func(os.DirEntry) bool) error {
	if err := os.MkdirAll(links, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if hidden(e) {
			continue
		}
		if err := os.Symlink(filepath.Join(folder, e.Name()), filepath.Join(links, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) git(args ...string) error { return w.gitIn(w.dir, args...) }

// git run in the folder dir: the real git (gitBin), never an itos linked as
// git.
func (w *world) gitIn(dir string, args ...string) error {
	cmd := exec.Command(gitBin(), args...)
	cmd.Dir = dir
	cmd.Env = w.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

// When steps.

// A program run in the folder dir, its exit code and output what the Then
// steps read.
func (w *world) run(dir, program string, args ...string) error {
	return w.runEnv(dir, w.env(), program, args...)
}

// runWith runs itos-template in the scratch repository with args, split
// before {template} is expanded, in the environment env.
func (w *world) runWith(env []string, args string) error {
	fields, err := commandWords(args)
	if err != nil {
		return err
	}
	for i, f := range fields {
		fields[i] = w.expand(f)
	}
	return w.runEnv(w.dir, env, w.bin, fields...)
}

// commandWords splits a scenario's command line without involving a shell.
// Quotes group words and preserve an explicitly empty argument, as the
// completion protocol needs for a new token. Backslashes remain literal, so
// Windows paths survive the acceptance harness.
func commandWords(line string) ([]string, error) {
	var words []string
	var word strings.Builder
	var quote rune
	started := false
	for _, r := range line {
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			word.WriteRune(r)
			started = true
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
			started = true
		case ' ', '\t', '\n', '\r':
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
		default:
			word.WriteRune(r)
			started = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote in scenario command %q", line)
	}
	if started {
		words = append(words, word.String())
	}
	return words, nil
}

// noGit is the scenarios' environment with no git on the PATH: every folder
// holding one replaced by a folder of links to the rest, or left out, as
// callerPath does for what it hides.
func (w *world) noGit() []string {
	return setEnv(w.env(), "PATH", pathHiding(append(slices.Clone(hiddenAlways), "git")...))
}

// knowingNoOne is the scenarios' environment with no identity for git: no
// GIT_AUTHOR_* or GIT_COMMITTER_* variable, and user.useConfigOnly set
// (through GIT_CONFIG_COUNT, which outranks every config file), so git
// refuses to guess one from the machine, which it would do differently on
// each system.
func (w *world) knowingNoOne() []string {
	var env []string
	for _, kv := range w.env() {
		name, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(name, "GIT_AUTHOR_") && !strings.HasPrefix(name, "GIT_COMMITTER_") {
			env = append(env, kv)
		}
	}
	return append(env, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=user.useConfigOnly", "GIT_CONFIG_VALUE_0=true")
}

// withGitConfig is the scenarios' environment with git's config key set to
// value for this run alone, as a person's own config would set it: through
// GIT_CONFIG_COUNT, which every git the run starts reads and which outranks
// every config file, so no config of the person's (global or system) is
// written or read.
func (w *world) withGitConfig(key, value string) []string {
	return append(w.env(), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0="+key, "GIT_CONFIG_VALUE_0="+value)
}

// setEnv is env with the variable name, in any case, set to value.
func setEnv(env []string, name, value string) []string {
	var out []string
	for _, kv := range env {
		if n, _, _ := strings.Cut(kv, "="); !strings.EqualFold(n, name) {
			out = append(out, kv)
		}
	}
	return append(out, name+"="+value)
}

// runEnv runs program in the folder dir in the environment env, its exit
// code and output what the Then steps read.
func (w *world) runEnv(dir string, env []string, program string, args ...string) error {
	cmd := exec.Command(program, args...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	w.stdout, w.stderr = stdout.String(), stderr.String()
	var exit *exec.ExitError
	switch {
	case err == nil:
		w.exit = 0
	case errors.As(err, &exit):
		w.exit = exit.ExitCode()
	default:
		return fmt.Errorf("running %s: %w", program, err)
	}
	return nil
}

// Then steps.

func (w *world) report() string {
	return fmt.Sprintf("exit %d\n--- stdout\n%s--- stderr\n%s", w.exit, w.stdout, w.stderr)
}

func outputLines(output string) []string {
	return strings.Split(strings.TrimSuffix(output, "\n"), "\n")
}

func (w *world) exitsWith(code int) error {
	if w.exit != code {
		return fmt.Errorf("itos-template exited %d, not %d\n%s", w.exit, code, w.report())
	}
	return nil
}

// secondLineIs is whether standard output's second line is text, a missing
// line read as an empty one and a line ending in \r\n as one in \n.
func (w *world) secondLineIs(text string) error {
	_, rest, _ := strings.Cut(w.stdout, "\n")
	second, _, _ := strings.Cut(rest, "\n")
	if second = strings.TrimSuffix(second, "\r"); second != text {
		return fmt.Errorf("the second line of standard output is %q, not %q\n%s", second, text, w.report())
	}
	return nil
}

// firstLineIs is whether standard output's first line is text, a line ending
// in \r\n read as one in \n.
func (w *world) firstLineIs(text string) error {
	first, _, _ := strings.Cut(w.stdout, "\n")
	if first = strings.TrimSuffix(first, "\r"); first != text {
		return fmt.Errorf("the first line of standard output is %q, not %q\n%s", first, text, w.report())
	}
	return nil
}
