// The steps of update.feature: a project made by new from a release, the
// variants of the releases fixture its scenarios name, each changing one
// release only, the template's tags moved, and what update leaves: the
// project's HEAD and refs, its index (what is staged, a conflict's stages),
// its files, and its --json object, parsed, never searched. The snapshots
// of a whole folder are snapshot_test.go's, the steps adopt.feature takes
// them with widened to update's words.
package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

func (w *world) updateSteps(sc *godog.ScenarioContext) {
	sc.Step(`^a project in "([^"]*)" made by new from the release "([^"]*)" with stack "([^"]*)", features "([^"]*)" and name "([^"]*)"$`, w.madeByNew)

	const released = `^the template "([^"]*)" released as ` + versions + `, whose (v[0-9][^ ]*) `
	sc.Step(released+`adds "([^"]*)" containing "([^"]*)"$`, w.releaseAddsFile)
	sc.Step(released+`removes "([^"]*)"$`, w.releaseRemovesFile)
	sc.Step(released+`renames "([^"]*)" to "([^"]*)"$`, w.releaseRenamesFile)
	sc.Step(released+`changes "([^"]*)"$`, w.releaseChangesFile)
	sc.Step(released+`adds the question "([^"]*)" for the literal "([^"]*)" in "([^"]*)", its default "([^"]*)"$`, w.releaseAddsQuestion)
	sc.Step(`^the template's tags of "([^"]*)" moved to commits whose release\.txt holds "([^"]*)"$`, w.tagsMoved)

	sc.Step(`^a saved snapshot of the HEAD of "([^"]*)"$`, w.saveHead)
	sc.Step(`^the HEAD of "([^"]*)" is unchanged$`, w.headUnchanged)
	sc.Step(`^"([^"]*)" has staged changes to (".*") only, and nothing unstaged$`, w.stagedOnly)
	sc.Step(`^"([^"]*)" has the record staged$`, func(dir string) error {
		return w.stagedAs(dir, ".itos-template.yaml", "modified")
	})
	sc.Step(`^"([^"]*)" has "([^"]*)" staged as (added|removed)$`, w.stagedAs)
	sc.Step(`^the file "([^"]*)" holds git's conflict markers around "([^"]*)" and "([^"]*)"$`, w.conflictMarkers)
	sc.Step(`^the file "([^"]*)" has the line "([^"]*)"$`, w.hasLine)
	sc.Step(`^the file "([^"]*)" names no git object id$`, w.namesNoObjectID)
	sc.Step(`^the index of "([^"]*)" holds "([^"]*)" unmerged, its base "([^"]*)", the project's "([^"]*)" and the release's "([^"]*)"$`, func(dir, file, base, ours, theirs string) error {
		return w.unmerged(dir, file, map[string]string{"1": base, "2": ours, "3": theirs})
	})
	sc.Step(`^the index of "([^"]*)" holds "([^"]*)" unmerged, its base "([^"]*)", no project side and the release's "([^"]*)"$`, func(dir, file, base, theirs string) error {
		return w.unmerged(dir, file, map[string]string{"1": base, "3": theirs})
	})
	sc.Step(`^its update JSON conflicts are (".*")$`, w.updateConflicts)
}

// The project.

// madeByNew makes the project dir as a person runs new: from the scenario's
// template's release version, with the stack, the features (a list of
// names, joined by commas), the answer name and --defaults, so its module is
// the question's default; new commits it, as its first commit.
func (w *world) madeByNew(dir, version, stack, features, name string) error {
	args := []string{"new", w.template, dir, "--ref", version, "--stack", stack}
	w.selection = []string{"main", "stack/" + stack}
	for _, f := range strings.Split(features, ",") {
		if f = strings.TrimSpace(f); f != "" {
			args = append(args, "--feature", f)
			w.selection = append(w.selection, stack+"/"+f)
		}
	}
	args = append(args, "--answer", "name="+name, "--defaults")
	if err := w.runEnv(w.dir, w.env(), w.bin, args...); err != nil {
		return err
	}
	if w.exit != 0 {
		return fmt.Errorf("new could not make the project\n%s", w.report())
	}
	w.made = append(w.made, dir)
	return w.noChanges(dir)
}

// The releases fixture's variants.

// releasedWith is the fixture template name released as each of list's
// versions (releaseChanging), its release version changed by change, made
// on the root branch and staged there, which what names in the template's
// key.
func (w *world) releasedWith(name, list, version, what string, change func(dir string) error) error {
	versions := quotedList(list)
	if !slices.Contains(versions, version) {
		return fmt.Errorf("%s is not one of the releases %q", version, versions)
	}
	key := fmt.Sprintf("%s released as %q, whose %s %s", name, versions, version, what)
	return w.useTemplate(key, func(dir string) error {
		src := filepath.Join(w.root, "features", "testdata", name)
		if err := w.buildTemplate(src, dir); err != nil {
			return err
		}
		return w.releaseChanging(src, dir, versions, map[string]func(string) error{version: change})
	})
}

// releaseAddsFile is the release version adding file (with /) holding a
// line of text.
func (w *world) releaseAddsFile(name, list, version, file, text string) error {
	return w.releasedWith(name, list, version, fmt.Sprintf("adds %q containing %q", file, text), func(dir string) error {
		if err := writeFile(filepath.Join(dir, filepath.FromSlash(file)), text+"\n"); err != nil {
			return err
		}
		return w.gitIn(dir, "add", "--", file)
	})
}

// releaseRemovesFile is the release version removing file.
func (w *world) releaseRemovesFile(name, list, version, file string) error {
	return w.releasedWith(name, list, version, fmt.Sprintf("removes %q", file), func(dir string) error {
		return w.gitIn(dir, "rm", "-q", "--", file)
	})
}

// releaseRenamesFile is the release version renaming from to to, its
// contents unchanged.
func (w *world) releaseRenamesFile(name, list, version, from, to string) error {
	return w.releasedWith(name, list, version, fmt.Sprintf("renames %q to %q", from, to), func(dir string) error {
		return w.gitIn(dir, "mv", "--", from, to)
	})
}

// releaseChangesFile is the release version adding a line to file, a
// comment in a YAML or shell file, naming the release.
func (w *world) releaseChangesFile(name, list, version, file string) error {
	return w.releasedWith(name, list, version, fmt.Sprintf("changes %q", file), func(dir string) error {
		p := filepath.Join(dir, filepath.FromSlash(file))
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.WriteFile(p, append(data, []byte("# changed by "+version+"\n")...), 0o644); err != nil {
			return err
		}
		return w.gitIn(dir, "add", "--", file)
	})
}

// releaseAddsQuestion is the release version adding to the manifest the
// question named question, its literal literal and its default def, and
// the file file holding a line of the literal.
func (w *world) releaseAddsQuestion(name, list, version, question, literal, file, def string) error {
	what := fmt.Sprintf("adds the question %q for the literal %q in %q, its default %q", question, literal, file, def)
	return w.releasedWith(name, list, version, what, func(dir string) error {
		err := rewriteManifest(dir, func(top *yaml.Node) error {
			questions := mappingValue(top, "questions")
			if questions == nil || questions.Kind != yaml.SequenceNode {
				return errors.New("the manifest lists no questions")
			}
			questions.Content = append(questions.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
				scalar("name"), scalar(question),
				scalar("literal"), scalar(literal),
				scalar("question"), scalar("Who is " + question + "?"),
				scalar("default"), scalar(def),
			}})
			return nil
		})
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(dir, filepath.FromSlash(file)), literal+"\n"); err != nil {
			return err
		}
		return w.gitIn(dir, "add", "--", "itos-template.yaml", file)
	})
}

// tagsMoved moves every tag of the release version of the scenario's
// template to a commit of its own whose release.txt holds text: a child of
// the commit the tag named, which its branch then merges with -s ours, so
// the tag is on its branch again, the release complete, and the branch's
// files are as they were.
//
// The fixture is shared by the run, so the tags move in a copy of it, the
// same repository with the same commits, and each project the scenario made
// by new names the copy in its record from then on, a committed change:
// its record's commits are the copy's too, as the template's tags moved
// would leave them.
func (w *world) tagsMoved(version, text string) error {
	if w.templateDir == "" {
		return errors.New("no template in this scenario")
	}
	from, named := w.templateDir, w.template
	key := fmt.Sprintf("%s, its tags of %s moved to commits whose release.txt holds %q", from, version, text)
	err := w.useTemplate(key, func(dir string) error {
		if err := os.CopyFS(dir, os.DirFS(from)); err != nil {
			return err
		}
		root, err := w.gitOut(dir, "symbolic-ref", "--short", "HEAD")
		if err != nil {
			return err
		}
		tags, err := w.gitOut(dir, "for-each-ref", "--format=%(refname:strip=2)", "refs/tags/")
		if err != nil {
			return err
		}
		for _, tag := range strings.Fields(tags) {
			branch, ok := strings.CutSuffix(tag, "/"+version)
			if !ok {
				continue
			}
			if err := w.gitIn(dir, "checkout", "-q", "--detach", tag); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dir, "release.txt"), []byte(text+"\n"), 0o644); err != nil {
				return err
			}
			for _, args := range [][]string{
				{"add", "--", "release.txt"},
				{"commit", "-q", "-m", "Move " + tag},
				{"tag", "-f", tag},
				{"checkout", "-q", branch},
				{"merge", "-q", "-s", "ours", "--no-edit", "-m", "Keep " + tag + " on " + branch, tag},
			} {
				if err := w.gitIn(dir, args...); err != nil {
					return err
				}
			}
		}
		return w.gitIn(dir, "checkout", "-q", root)
	})
	if err != nil {
		return err
	}
	for _, dir := range w.made {
		if err := w.renameTemplate(dir, named, w.template); err != nil {
			return err
		}
	}
	return nil
}

// renameTemplate has the record of the project dir name the template to,
// where it named from, and commits it.
func (w *world) renameTemplate(dir, from, to string) error {
	p := filepath.Join(w.path(dir), ".itos-template.yaml")
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if n := strings.Count(string(data), from); n != 1 {
		return fmt.Errorf("the record of %s names %s %d times, not once:\n%s", dir, from, n, data)
	}
	if err := os.WriteFile(p, []byte(strings.Replace(string(data), from, to, 1)), 0o644); err != nil {
		return err
	}
	if err := w.gitIn(w.path(dir), "add", "--", ".itos-template.yaml"); err != nil {
		return err
	}
	return w.commit(dir, "Name the template where its tags moved")
}

// HEAD and the index.

// headOf is the repository dir's HEAD, the ref it names and every ref
// with its commit.
func (w *world) headOf(dir string) (string, error) {
	var parts []string
	for _, args := range [][]string{
		{"rev-parse", "HEAD"},
		{"rev-parse", "--symbolic-full-name", "HEAD"},
		{"for-each-ref", "--format=%(refname) %(objectname)"},
	} {
		out, err := w.gitOut(w.path(dir), args...)
		if err != nil {
			return "", err
		}
		parts = append(parts, out)
	}
	return strings.Join(parts, "\n"), nil
}

func (w *world) saveHead(dir string) error {
	head, err := w.headOf(dir)
	if err != nil {
		return err
	}
	if w.heads == nil {
		w.heads = map[string]string{}
	}
	w.heads[dir] = head
	return nil
}

// headUnchanged is whether dir's HEAD names the same ref and commit as
// when it was saved, and every ref is where it was, none added.
func (w *world) headUnchanged(dir string) error {
	before, ok := w.heads[dir]
	if !ok {
		return fmt.Errorf("no HEAD of %s was saved", dir)
	}
	after, err := w.headOf(dir)
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("the HEAD or the refs of %s changed, from\n%s\nto\n%s\n%s", dir, before, after, w.report())
	}
	return nil
}

// statusEntry is a path git status lists: its index's letter, its working
// tree's, and the path.
type statusEntry struct {
	index, tree byte
	path        string
}

// status is what git status lists in dir, each untracked file too, every
// rename as a removal and an addition.
func (w *world) status(dir string, paths ...string) ([]statusEntry, error) {
	cmd := exec.Command(gitBin(), append([]string{"status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=all", "--"}, paths...)...)
	cmd.Dir = w.path(dir)
	cmd.Env = w.env()
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status in %s: %v", dir, err)
	}
	out := string(raw)
	var entries []statusEntry
	for _, e := range strings.Split(out, "\x00") {
		if len(e) > 3 {
			entries = append(entries, statusEntry{e[0], e[1], e[3:]})
		}
	}
	return entries, nil
}

// stagedOnly is whether dir's index changes exactly the paths listed from
// HEAD, and its working tree nothing from the index, no untracked file
// either.
func (w *world) stagedOnly(dir, list string) error {
	entries, err := w.status(dir)
	if err != nil {
		return err
	}
	var staged []string
	for _, e := range entries {
		if e.tree != ' ' {
			return fmt.Errorf("%s has %s unstaged (%c%c)\n%s", dir, e.path, e.index, e.tree, w.report())
		}
		staged = append(staged, e.path)
	}
	want := quotedList(list)
	slices.Sort(want)
	slices.Sort(staged)
	if !slices.Equal(staged, want) {
		return fmt.Errorf("%s has %q staged, not %q\n%s", dir, staged, want, w.report())
	}
	return nil
}

// stagedAs is whether dir's index adds, removes or modifies file from HEAD,
// as kind says, and its working tree holds it as the index does.
func (w *world) stagedAs(dir, file, kind string) error {
	letter := map[string]byte{"added": 'A', "removed": 'D', "modified": 'M'}[kind]
	entries, err := w.status(dir, file)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.path == file && e.index == letter && e.tree == ' ' {
			return nil
		}
	}
	return fmt.Errorf("%s does not have %s staged as %s: git status says %v\n%s", dir, file, kind, entries, w.report())
}

// conflictMarkers is whether file holds a conflict as git marks one: a line
// <<<<<<<, the project's side holding ours, a line =======, the release's
// side holding theirs, a line >>>>>>>; with merge.conflictStyle diff3 the
// base's side, after a line |||||||, is not the project's. A line ending in
// \r\n is read as one in \n.
func (w *world) conflictMarkers(file, ours, theirs string) error {
	lines, data, err := w.lines(file)
	if err != nil {
		return err
	}
	start := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "<<<<<<< ") })
	middle := slices.Index(lines, "=======")
	end := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, ">>>>>>> ") })
	if start < 0 || middle < start || end < middle {
		return fmt.Errorf("%s holds no conflict as git marks one:\n%s", file, data)
	}
	project := lines[start+1 : middle]
	if base := slices.IndexFunc(project, func(l string) bool { return strings.HasPrefix(l, "||||||| ") }); base >= 0 {
		project = project[:base]
	}
	if !strings.Contains(strings.Join(project, "\n"), ours) || !strings.Contains(strings.Join(lines[middle+1:end], "\n"), theirs) {
		return fmt.Errorf("%s's conflict is not the project's %q against the release's %q:\n%s", file, ours, theirs, data)
	}
	return nil
}

// lines are file's lines, a CRLF read as LF, as git may write them on
// Windows.
func (w *world) lines(file string) ([]string, []byte, error) {
	data, err := os.ReadFile(w.path(file))
	if err != nil {
		return nil, nil, fmt.Errorf("%v\n%s", err, w.report())
	}
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), data, nil
}

// hasLine is whether file holds line as one of its lines, whole.
func (w *world) hasLine(file, line string) error {
	lines, data, err := w.lines(file)
	if err != nil {
		return err
	}
	if !slices.Contains(lines, line) {
		return fmt.Errorf("%s has no line %q:\n%s", file, line, data)
	}
	return nil
}

// conflictMarker is a line git's conflict markers start: seven or more of
// one marker character, then the end of the line or a space and a label.
var conflictMarker = regexp.MustCompile(`^(<{7,}|\|{7,}|={7,}|>{7,})( |$)`)

// objectID is a run of hex digits as long as a git object id, SHA-1's 40
// or SHA-256's 64.
var objectID = regexp.MustCompile(`[0-9a-f]{40}`)

// namesNoObjectID is whether no conflict marker line of file names a git
// object id, as a label of git merge-tree's arguments would.
func (w *world) namesNoObjectID(file string) error {
	lines, data, err := w.lines(file)
	if err != nil {
		return err
	}
	for _, l := range lines {
		if conflictMarker.MatchString(l) && objectID.MatchString(l) {
			return fmt.Errorf("%s's marker line %q names a git object id:\n%s", file, l, data)
		}
	}
	return nil
}

// unmerged is whether dir's index holds file unmerged, at the stages want
// names and no other, no stage 0 among them, each stage's blob holding the
// text want gives it: 1 the base, 2 the project's, 3 the release's.
func (w *world) unmerged(dir, file string, want map[string]string) error {
	out, err := w.gitOut(w.path(dir), "ls-files", "--stage", "-z", "--", file)
	if err != nil {
		return err
	}
	stages := map[string]string{}
	for _, e := range strings.Split(out, "\x00") {
		meta, p, ok := strings.Cut(e, "\t")
		fields := strings.Fields(meta)
		if ok && p == file && len(fields) == 3 {
			stages[fields[2]] = fields[1]
		}
	}
	var got []string
	for stage := range stages {
		got = append(got, stage)
	}
	var wanted []string
	for stage := range want {
		wanted = append(wanted, stage)
	}
	slices.Sort(got)
	slices.Sort(wanted)
	if !slices.Equal(got, wanted) {
		return fmt.Errorf("the index of %s holds %s at the stages %q, not %q\n%s", dir, file, got, wanted, w.report())
	}
	for stage, text := range want {
		blob, err := w.gitOut(w.path(dir), "cat-file", "blob", stages[stage])
		if err != nil {
			return err
		}
		if !strings.Contains(blob, text) {
			return fmt.Errorf("the index of %s holds %s at stage %s as %q, not holding %q", dir, file, stage, blob, text)
		}
	}
	return nil
}

// The --json object.

// updateConflicts is whether standard output is one JSON object of schema
// 1 whose conflicts are exactly the paths listed, in that order.
func (w *world) updateConflicts(list string) error {
	var u struct {
		Schema    int `json:"schema"`
		Conflicts []struct {
			Path string `json:"path"`
		} `json:"conflicts"`
	}
	dec := json.NewDecoder(strings.NewReader(w.stdout))
	if err := dec.Decode(&u); err != nil {
		return fmt.Errorf("standard output is no JSON object: %v\n%s", err, w.report())
	}
	if dec.More() {
		return fmt.Errorf("standard output holds more than one JSON value\n%s", w.report())
	}
	var paths []string
	for _, c := range u.Conflicts {
		paths = append(paths, c.Path)
	}
	if u.Schema != 1 || !slices.Equal(paths, quotedList(list)) {
		return fmt.Errorf("the object's schema is %d and its conflicts %q, not 1 and %q\n%s", u.Schema, paths, list, w.report())
	}
	return nil
}
