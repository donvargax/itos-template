// The steps of adopt.feature: an existing project, made of a release's
// render but holding no record, the changes a scenario makes to it, and
// what adopt prints, its preview read line by line and its --json object
// parsed, never searched. The snapshot its scenarios compare a project to
// is snapshot_test.go's.
//
// The existing project is what new renders for the release, its files
// copied into a git repository of its own whose first commit is an ordinary
// one: a project made before the template, so its history holds no record
// and no commit of new's.
package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

func (w *world) adoptSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the template "([^"]*)" with the release "([^"]*)"$`, w.templateWithRelease)
	sc.Step(`^an existing project in "([^"]*)" matching that release with stack "([^"]*)", features "([^"]*)", name "([^"]*)" and module "([^"]*)"$`, w.existingProject)
	sc.Step(`^the template's current branches differ from the release "([^"]*)"$`, w.headsDifferFrom)
	sc.Step(`^the template has a release "([^"]*)" with the same rendered files and checks that always fail if run$`, w.releaseWithFailingCheck)

	sc.Step(`^a committed project change to "([^"]*)" containing "([^"]*)"$`, w.committedChange)
	sc.Step(`^a committed binary change to "([^"]*)" containing "([^"]*)" after a NUL$`, w.committedBinaryChange)
	sc.Step(`^a committed (?:project )?deletion of "([^"]*)"$`, w.committedDeletion)
	sc.Step(`^a committed (?:project-only )?file "([^"]*)" containing "([^"]*)"$`, w.committedFile)
	sc.Step(`^"([^"]*)" has a committed executable mode unlike the baseline$`, w.committedExecutable)
	sc.Step(`^"([^"]*)" has a committed ignore rule for "([^"]*)"$`, w.committedIgnoreRule)
	sc.Step(`^"([^"]*)" has (an unstaged tracked change|a staged tracked change|a non-ignored untracked file)$`, w.dirtyProject)
	sc.Step(`^a non-ignored untracked file "([^"]*)" containing "([^"]*)"$`, func(file, text string) error {
		return w.untrackedFile(file, text, false)
	})
	sc.Step(`^an ignored untracked file "([^"]*)" containing "([^"]*)"$`, func(file, text string) error {
		return w.untrackedFile(file, text, true)
	})
	sc.Step(`^"([^"]*)" is (a folder outside any git repository|a bare git repository|a git repository without a commit)$`, w.projectIs)

	sc.Step(`^a saved (?:adoption )?snapshot of "([^"]*)"$`, w.saveSnapshot)
	sc.Step(`^the (?:adoption )?snapshot of "([^"]*)" is unchanged$`, func(dir string) error {
		return w.snapshotUnchanged(dir, false)
	})
	sc.Step(`^the adoption snapshot of "([^"]*)" is unchanged except for the new unstaged record$`, func(dir string) error {
		return w.snapshotUnchanged(dir, true)
	})

	sc.Step(`^itos-template runs in "([^"]*)" with "([^"]*)"$`, func(dir, args string) error {
		return w.runWithIn(w.path(dir), w.env(), args)
	})

	sc.Step(`^the record in "([^"]*)" names the release "([^"]*)" and its exact branch commits$`, w.recordNamesReleaseCommits)
	sc.Step(`^its standard output identifies "([^"]*)" as "([^"]*)"$`, func(file, kind string) error {
		_, err := w.differenceLine(file, kind, "")
		return err
	})
	sc.Step(`^its standard output identifies "([^"]*)" as a binary modification$`, func(file string) error {
		_, err := w.differenceLine(file, "modified", "binary")
		return err
	})
	sc.Step(`^its standard output identifies a mode change for "([^"]*)"$`, func(file string) error {
		_, err := w.differenceLine(file, "", "mode")
		return err
	})
	sc.Step(`^its standard output names the baseline "([^"]*)" and its exact branch commits$`, w.outputNamesBaseline)
	sc.Step(`^its standard output shows a text diff from the baseline to "([^"]*)" containing "([^"]*)"$`, w.outputShowsDiff)
	sc.Step(`^its standard output is one adoption JSON object with schema 1, ok (true|false) and written (true|false)$`, w.adoptionObject)
	sc.Step(`^its adoption JSON baseline names "([^"]*)" and its exact branch commits$`, w.adoptionBaseline)
	sc.Step(`^its adoption JSON differences include "([^"]*)" with kind "([^"]*)", binary (true|false) and mode_changed (true|false)$`, w.adoptionDifference)
	sc.Step(`^its adoption JSON problems include a rule ID and a fix$`, w.adoptionProblemWithFix)
}

// The template and the existing project.

// templateWithRelease is the fixture template name released as version
// alone (releasedTemplate): every branch tagged <branch>/<version>, then
// moved one commit past it, release.txt holding heads there.
func (w *world) templateWithRelease(name, version string) error {
	w.released = version
	return w.releasedTemplate(name, []string{version}, "", nil)
}

// existingProject makes dir an existing project matching the scenario's
// release: what new renders for it with the stack, the feature and the
// answers given, every file but the record copied into dir, which is made a
// git repository of its own whose one commit holds them all, each file git
// records as executable in the render recorded so here too. A project made
// before the template has no record and no commit of new's.
func (w *world) existingProject(dir, stack, feature, name, module string) error {
	if w.released == "" {
		return errors.New("no release in this scenario")
	}
	render, err := os.MkdirTemp("", "itos-template-features-render-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(render) }()
	into := filepath.Join(render, "made")
	if err := w.runEnv(w.dir, w.env(), w.bin, "new", w.template, into, "--ref", w.released,
		"--stack", stack, "--feature", feature, "--answer", "name="+name, "--answer", "module="+module); err != nil {
		return err
	}
	if w.exit != 0 {
		return fmt.Errorf("new could not render the release\n%s", w.report())
	}
	files, err := w.gitOut(into, "ls-files", "-s", "-z")
	if err != nil {
		return err
	}
	project := w.path(dir)
	var executables []string
	for _, entry := range strings.Split(files, "\x00") {
		meta, file, ok := strings.Cut(entry, "\t")
		if !ok || file == ".itos-template.yaml" {
			continue
		}
		if strings.HasPrefix(meta, "100755 ") {
			executables = append(executables, file)
		}
		if err := copyFile(filepath.Join(into, filepath.FromSlash(file)), filepath.Join(project, filepath.FromSlash(file))); err != nil {
			return err
		}
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "--all"}} {
		if err := w.gitIn(project, args...); err != nil {
			return err
		}
	}
	for _, file := range executables {
		if err := w.gitIn(project, "update-index", "--chmod=+x", "--", file); err != nil {
			return err
		}
	}
	if err := w.gitIn(project, "commit", "-q", "-m", "Start the project"); err != nil {
		return err
	}
	w.selection = []string{"main", "stack/" + stack, stack + "/" + feature}
	return w.noChanges(dir)
}

// copyFile copies the file or symbolic link from to the path to, making the
// folders it needs, a file keeping its execute bit where the system has one.
func copyFile(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(from)
		if err != nil {
			return err
		}
		return os.Symlink(target, to)
	}
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, data, info.Mode().Perm())
}

// headsDifferFrom is whether every branch the scenario's project was
// rendered from has moved past its tag of version: releasedTemplate moves
// each one commit past the last release, release.txt holding heads, so a
// render of the heads differs from the release's.
func (w *world) headsDifferFrom(version string) error {
	for _, b := range w.selection {
		head, err := w.gitOut(w.templateDir, "rev-parse", "refs/heads/"+b)
		if err != nil {
			return err
		}
		tag, err := w.gitOut(w.templateDir, "rev-parse", b+"/"+version+"^{commit}")
		if err != nil {
			return err
		}
		if head == tag {
			return fmt.Errorf("the template's branch %s is at its release %s", b, version)
		}
	}
	return nil
}

// releaseWithFailingCheck is the fixture template, released as the
// scenario's release, then released again as version: release.txt holding
// the scenario's release again on the root branch, and the root given one
// more check, which fails wherever it runs, merged down into every branch
// and tagged <branch>/<version>. So version renders the files the
// scenario's release renders, the manifest being the template's alone, and
// a render of it whose checks ran would fail.
func (w *world) releaseWithFailingCheck(version string) error {
	if w.released == "" {
		return errors.New("no release in this scenario")
	}
	name := "acme"
	key := fmt.Sprintf("%s released as %q, then %s with the same rendered files and a root check that always fails", name, []string{w.released}, version)
	return w.useTemplate(key, func(dir string) error {
		src := filepath.Join(w.root, "features", "testdata", name)
		if err := w.buildTemplate(src, dir); err != nil {
			return err
		}
		if err := w.release(src, dir, []string{w.released}); err != nil {
			return err
		}
		branches, err := branchList(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "release.txt"), []byte(w.released+"\n"), 0o644); err != nil {
			return err
		}
		err = w.changeManifest(dir, key, func(top *yaml.Node) error {
			checks := mappingValue(top, "checks")
			if checks == nil || checks.Kind != yaml.SequenceNode {
				return errors.New("the manifest has no checks of the root")
			}
			checks.Content = append(checks.Content, &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle, Content: []*yaml.Node{
				scalar("git"), scalar("ls-files"), scalar("--error-unmatch"), scalar("no-file-of-any-render"),
			}})
			return nil
		})
		if err != nil {
			return err
		}
		if err := w.mergeInto(dir, branches); err != nil {
			return err
		}
		for _, b := range branches {
			if err := w.gitIn(dir, "tag", b.name+"/"+version, "refs/heads/"+b.name); err != nil {
				return err
			}
		}
		return w.gitIn(dir, "checkout", "-q", branches[0].name)
	})
}

// The changes a scenario makes to the project.

// inProject is a step's path of a file of a project, made/README.md: the
// project's folder, its first element, and the file's path in it, with /.
func inProject(file string) (dir, rel string, err error) {
	dir, rel, ok := strings.Cut(file, "/")
	if !ok || rel == "" {
		return "", "", fmt.Errorf("%s names no file in a project's folder", file)
	}
	return dir, rel, nil
}

// commit commits, in the project dir, what is staged, the message naming
// what.
func (w *world) commit(dir, what string) error {
	return w.gitIn(w.path(dir), "commit", "-q", "-m", what)
}

// committedChange adds a line holding text to the end of file, which the
// project tracks, and commits it.
func (w *world) committedChange(file, text string) error {
	dir, rel, err := inProject(file)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(w.path(file))
	if err != nil {
		return err
	}
	if err := os.WriteFile(w.path(file), append(data, []byte(text+"\n")...), 0o644); err != nil {
		return err
	}
	if err := w.gitIn(w.path(dir), "add", "--", rel); err != nil {
		return err
	}
	return w.commit(dir, "Change "+rel)
}

// committedBinaryChange makes file a NUL followed by text, so git and
// itos-template read it as binary, and commits it.
func (w *world) committedBinaryChange(file, text string) error {
	dir, rel, err := inProject(file)
	if err != nil {
		return err
	}
	if _, err := os.Stat(w.path(file)); err != nil {
		return err
	}
	if err := os.WriteFile(w.path(file), []byte("\x00"+text+"\n"), 0o644); err != nil {
		return err
	}
	if err := w.gitIn(w.path(dir), "add", "--", rel); err != nil {
		return err
	}
	return w.commit(dir, "Change "+rel)
}

// committedDeletion removes file from the project and commits it.
func (w *world) committedDeletion(file string) error {
	dir, rel, err := inProject(file)
	if err != nil {
		return err
	}
	if err := w.gitIn(w.path(dir), "rm", "-q", "--", rel); err != nil {
		return err
	}
	return w.commit(dir, "Remove "+rel)
}

// committedFile adds file, which the project does not hold, holding a line
// of text, and commits it.
func (w *world) committedFile(file, text string) error {
	dir, rel, err := inProject(file)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(w.path(file)); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("the project already holds %s", rel)
	}
	if err := writeFile(w.path(file), text+"\n"); err != nil {
		return err
	}
	if err := w.gitIn(w.path(dir), "add", "--", rel); err != nil {
		return err
	}
	return w.commit(dir, "Add "+rel)
}

// committedExecutable records file as executable, as its render does not,
// in the project's index and, where the system has the bit, its working
// tree, so the tree stays clean, and commits it.
func (w *world) committedExecutable(file string) error {
	dir, rel, err := inProject(file)
	if err != nil {
		return err
	}
	if err := os.Chmod(w.path(file), 0o755); err != nil {
		return err
	}
	if err := w.gitIn(w.path(dir), "update-index", "--chmod=+x", "--", rel); err != nil {
		return err
	}
	if err := w.commit(dir, "Make "+rel+" executable"); err != nil {
		return err
	}
	mode, err := w.gitOut(w.path(dir), "ls-tree", "HEAD", "--", rel)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(mode, "100755 ") {
		return fmt.Errorf("the project's HEAD records %s as %q", rel, mode)
	}
	return w.noChanges(dir)
}

// committedIgnoreRule adds pattern to the project's .gitignore and commits
// it.
func (w *world) committedIgnoreRule(dir, pattern string) error {
	f, err := os.OpenFile(filepath.Join(w.path(dir), ".gitignore"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(pattern + "\n")
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := w.gitIn(w.path(dir), "add", "--", ".gitignore"); err != nil {
		return err
	}
	return w.commit(dir, "Ignore "+pattern)
}

// dirtyProject leaves the project dir with one change of the kind state
// names, uncommitted: a line added to its README.md, staged or not, or a
// file it does not track and does not ignore.
func (w *world) dirtyProject(dir, state string) error {
	if state == "a non-ignored untracked file" {
		return w.untrackedFile(dir+"/untracked.txt", "untracked", false)
	}
	readme := filepath.Join(w.path(dir), "README.md")
	data, err := os.ReadFile(readme)
	if err != nil {
		return err
	}
	if err := os.WriteFile(readme, append(data, []byte("dirty\n")...), 0o644); err != nil {
		return err
	}
	if state == "a staged tracked change" {
		return w.gitIn(w.path(dir), "add", "--", "README.md")
	}
	return nil
}

// untrackedFile writes file, which the project does not track, holding a
// line of text: ignored by the project's rules, as git check-ignore says,
// or not, as ignored says.
func (w *world) untrackedFile(file, text string, ignored bool) error {
	dir, rel, err := inProject(file)
	if err != nil {
		return err
	}
	if err := writeFile(w.path(file), text+"\n"); err != nil {
		return err
	}
	err = w.gitIn(w.path(dir), "check-ignore", "-q", "--", rel)
	if ignored && err != nil {
		return fmt.Errorf("the project does not ignore %s: %w", rel, err)
	}
	if !ignored && err == nil {
		return fmt.Errorf("the project ignores %s", rel)
	}
	return nil
}

// writeFile writes text as the file p, making the folders it needs.
func writeFile(p, text string) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(text), 0o644)
}

// projectIs makes the project dir what state says. Its .git, and the
// scratch folder's, are moved out of the way rather than removed: a folder
// of git's objects, which git makes read-only, is not removed alike on
// every system, and the scenario's trash is removed after it. A folder the
// scenario has not made yet is made empty first, a folder of no project.
//
//   - a folder outside any git repository: its files with no .git, in no
//     repository, the scratch folder's .git moved out too;
//   - a bare git repository: a bare clone of the project in its place, or a
//     new bare repository where there was no folder;
//   - a git repository without a commit: its files in a new repository,
//     none of them committed.
func (w *world) projectIs(dir, state string) error {
	project := w.path(dir)
	if _, err := os.Lstat(project); errors.Is(err, fs.ErrNotExist) {
		if state == "a bare git repository" {
			return w.gitIn(w.dir, "init", "-q", "--bare", "--", project)
		}
		if err := os.MkdirAll(project, 0o755); err != nil {
			return err
		}
		if err := w.gitIn(project, "init", "-q", "-b", "main"); err != nil {
			return err
		}
	}
	switch state {
	case "a folder outside any git repository":
		if err := w.moveAway(filepath.Join(project, ".git")); err != nil {
			return err
		}
		if err := w.moveAway(filepath.Join(w.dir, ".git")); err != nil {
			return err
		}
		if out, err := w.gitOut(project, "rev-parse", "--absolute-git-dir"); err == nil {
			return fmt.Errorf("%s is still in the git repository %s", dir, out)
		}
		return nil
	case "a bare git repository":
		trash, err := w.trashFolder()
		if err != nil {
			return err
		}
		bare := filepath.Join(trash, "bare.git")
		if err := w.gitIn(w.dir, "clone", "-q", "--bare", "--", project, bare); err != nil {
			return err
		}
		if err := w.moveAway(project); err != nil {
			return err
		}
		return os.Rename(bare, project)
	default:
		if err := w.moveAway(filepath.Join(project, ".git")); err != nil {
			return err
		}
		return w.gitIn(project, "init", "-q", "-b", "main")
	}
}

// The record.

// releaseCommits is each branch the scenario's project was rendered from
// and the commit the template's tag of version names on it.
func (w *world) releaseCommits(version string) (map[string]string, error) {
	if len(w.selection) == 0 {
		return nil, errors.New("no project in this scenario names its branches")
	}
	commits := map[string]string{}
	for _, b := range w.selection {
		sha, err := w.gitOut(w.templateDir, "rev-parse", b+"/"+version+"^{commit}")
		if err != nil {
			return nil, err
		}
		commits[b] = sha
	}
	return commits, nil
}

// sameCommits is nil when got names exactly the commits want names.
func sameCommits(got, want map[string]string) error {
	if !maps.Equal(got, want) {
		return fmt.Errorf("the commits named are %v, not the release's %v", got, want)
	}
	return nil
}

// recordNamesReleaseCommits is whether the record in dir names the release
// version and, for each branch the project was rendered from and no other,
// the commit its tag of version names.
func (w *world) recordNamesReleaseCommits(dir, version string) error {
	if err := w.recordNamesRelease(dir, version); err != nil {
		return err
	}
	r, err := w.record(dir)
	if err != nil {
		return err
	}
	want, err := w.releaseCommits(version)
	if err != nil {
		return err
	}
	return sameCommits(r.Commits, want)
}

// The preview.

// differenceLine is the line of the preview naming file: the kind of the
// difference, a space, the path, then what more it says of it in brackets,
// "modified logo.bin (binary)". kind, when given, is the line's first word;
// note, when given, is in its brackets.
func (w *world) differenceLine(file, kind, note string) (string, error) {
	for _, line := range outputLines(strings.ReplaceAll(w.stdout, "\r\n", "\n")) {
		line = strings.TrimSpace(line)
		first, rest, ok := strings.Cut(line, " ")
		if !ok || kind != "" && first != kind {
			continue
		}
		if rest != file && !strings.HasPrefix(rest, file+" (") {
			continue
		}
		if note != "" && !strings.Contains(strings.TrimPrefix(rest, file), note) {
			continue
		}
		return line, nil
	}
	return "", fmt.Errorf("standard output has no line naming %s as %q (%s)\n%s", file, kind, note, w.report())
}

// outputNamesBaseline is whether standard output names the release
// version, and, on a line of its own for each, every branch the project was
// rendered from and the commit its tag of version names.
func (w *world) outputNamesBaseline(version string) error {
	if !strings.Contains(w.stdout, version) {
		return fmt.Errorf("standard output does not name %s\n%s", version, w.report())
	}
	want, err := w.releaseCommits(version)
	if err != nil {
		return err
	}
	lines := outputLines(strings.ReplaceAll(w.stdout, "\r\n", "\n"))
	for b, sha := range want {
		if !slices.ContainsFunc(lines, func(line string) bool {
			words := strings.Fields(line)
			return slices.Contains(words, b) && slices.Contains(words, sha)
		}) {
			return fmt.Errorf("standard output has no line naming the branch %s at %s\n%s", b, sha, w.report())
		}
	}
	return nil
}

// outputShowsDiff is whether standard output holds git's diff of file from
// the baseline (a/) to the project (b/), with an added line holding text.
func (w *world) outputShowsDiff(file, text string) error {
	lines := outputLines(strings.ReplaceAll(w.stdout, "\r\n", "\n"))
	at := slices.Index(lines, "--- a/"+file)
	if at < 0 || at+1 >= len(lines) || lines[at+1] != "+++ b/"+file {
		return fmt.Errorf("standard output shows no diff from a/%s to b/%s\n%s", file, file, w.report())
	}
	for _, line := range lines[at+2:] {
		if strings.HasPrefix(line, "diff ") {
			break
		}
		if strings.HasPrefix(line, "+") && strings.Contains(line, text) {
			return nil
		}
	}
	return fmt.Errorf("the diff of %s adds no line holding %q\n%s", file, text, w.report())
}

// The --json object.

// adoption is adopt's --json object, as far as the steps read it.
type adoption struct {
	Schema   int   `json:"schema"`
	OK       *bool `json:"ok"`
	Written  *bool `json:"written"`
	Baseline *struct {
		Release string            `json:"release"`
		Commits map[string]string `json:"commits"`
	} `json:"baseline"`
	Differences []struct {
		Path        string `json:"path"`
		Kind        string `json:"kind"`
		Binary      *bool  `json:"binary"`
		ModeChanged *bool  `json:"mode_changed"`
	} `json:"differences"`
	Problems []adoptionProblem `json:"problems"`
}

// adoptionProblem is a problem of adopt's --json object: its rule ID and
// its fix, which the person reads.
type adoptionProblem struct {
	Rule string `json:"rule"`
	Fix  string `json:"fix"`
}

// adoptionJSON is standard output read as one JSON object, and nothing
// after it.
func (w *world) adoptionJSON() (*adoption, error) {
	var a adoption
	dec := json.NewDecoder(strings.NewReader(w.stdout))
	if err := dec.Decode(&a); err != nil {
		return nil, fmt.Errorf("standard output is no JSON object: %v\n%s", err, w.report())
	}
	if dec.More() {
		return nil, fmt.Errorf("standard output holds more than one JSON value\n%s", w.report())
	}
	return &a, nil
}

func (w *world) adoptionObject(ok, written string) error {
	a, err := w.adoptionJSON()
	if err != nil {
		return err
	}
	switch {
	case a.Schema != 1:
		return fmt.Errorf("the object's schema is %d, not 1\n%s", a.Schema, w.report())
	case a.OK == nil || fmt.Sprint(*a.OK) != ok:
		return fmt.Errorf("the object's ok is not %s\n%s", ok, w.report())
	case a.Written == nil || fmt.Sprint(*a.Written) != written:
		return fmt.Errorf("the object's written is not %s\n%s", written, w.report())
	}
	return nil
}

func (w *world) adoptionBaseline(version string) error {
	a, err := w.adoptionJSON()
	if err != nil {
		return err
	}
	if a.Baseline == nil || a.Baseline.Release != version {
		return fmt.Errorf("the object's baseline does not name the release %s\n%s", version, w.report())
	}
	want, err := w.releaseCommits(version)
	if err != nil {
		return err
	}
	return sameCommits(a.Baseline.Commits, want)
}

func (w *world) adoptionDifference(file, kind, binary, modeChanged string) error {
	a, err := w.adoptionJSON()
	if err != nil {
		return err
	}
	for _, d := range a.Differences {
		if d.Path != file {
			continue
		}
		if d.Kind != kind || d.Binary == nil || fmt.Sprint(*d.Binary) != binary ||
			d.ModeChanged == nil || fmt.Sprint(*d.ModeChanged) != modeChanged {
			return fmt.Errorf("the object's difference of %s is not of kind %s, binary %s and mode_changed %s\n%s", file, kind, binary, modeChanged, w.report())
		}
		return nil
	}
	return fmt.Errorf("the object's differences do not include %s\n%s", file, w.report())
}

func (w *world) adoptionProblemWithFix() error {
	a, err := w.adoptionJSON()
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(a.Problems, func(p adoptionProblem) bool {
		return p.Rule != "" && p.Fix != ""
	}) {
		return fmt.Errorf("no problem of the object has both a rule ID and a fix\n%s", w.report())
	}
	return nil
}
