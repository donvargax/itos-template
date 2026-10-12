// The fixture templates of the scenarios, new.feature's and
// check.feature's: git repositories the steps build from testdata, each once
// a run, and the variants a scenario names, each one more commit on the
// fixture. No command a scenario runs changes one, as new and check only
// clone it.
package features

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

// The fixture templates.

var (
	templatesMu  sync.Mutex
	templates    = map[string]string{}
	templatesDir string
)

// removeTemplates removes the fixture templates the run built.
func removeTemplates() {
	if templatesDir != "" {
		_ = os.RemoveAll(templatesDir)
	}
}

// theTemplate builds the fixture template name from testdata, once a run:
// no command a scenario runs changes it, as new and check only clone it.
func (w *world) theTemplate(name string) error {
	return w.useTemplate(name, func(dir string) error {
		return w.buildTemplate(filepath.Join(w.root, "features", "testdata", name), dir)
	})
}

// templateInFolder is the fixture template name built in a folder named
// folder (with /), so its path holds what folder holds, a space or a letter
// outside ASCII, and {template} names it there.
func (w *world) templateInFolder(name, folder string) error {
	key := fmt.Sprintf("%s in a folder named %q", name, folder)
	if err := w.useTemplate(key, func(dir string) error {
		return w.buildTemplate(filepath.Join(w.root, "features", "testdata", name), filepath.Join(dir, filepath.FromSlash(folder)))
	}); err != nil {
		return err
	}
	w.templateDir = filepath.Join(w.templateDir, filepath.FromSlash(folder))
	w.template = filepath.ToSlash(w.templateDir)
	return nil
}

// templateWithLiteral is the fixture template name, its manifest on the root
// branch giving the question named question the literal literal, the rest
// of the question as the fixture has it.
func (w *world) templateWithLiteral(name, question, literal string) error {
	key := fmt.Sprintf("%s whose question %s has the literal %q", name, question, literal)
	return w.changedTemplate(name, key, func(top *yaml.Node) error {
		questions := mappingValue(top, "questions")
		if questions == nil || questions.Kind != yaml.SequenceNode {
			return errors.New("the manifest lists no questions")
		}
		for _, q := range questions.Content {
			if scalarValue(q, "name") != question {
				continue
			}
			value := mappingValue(q, "literal")
			if value == nil {
				return fmt.Errorf("the question %s has no literal", question)
			}
			*value = *scalar(literal)
			return nil
		}
		return fmt.Errorf("the manifest has no question %s", question)
	})
}

// templateWithoutPattern is the fixture template name, its manifest on the
// root branch giving the question named question no pattern, so it takes
// any answer, the rest of the question as the fixture has it.
func (w *world) templateWithoutPattern(name, question string) error {
	key := fmt.Sprintf("%s whose question %s has no pattern", name, question)
	return w.changedTemplate(name, key, func(top *yaml.Node) error {
		questions := mappingValue(top, "questions")
		if questions == nil || questions.Kind != yaml.SequenceNode {
			return errors.New("the manifest lists no questions")
		}
		for _, q := range questions.Content {
			if scalarValue(q, "name") != question {
				continue
			}
			for i := 0; i+1 < len(q.Content); i += 2 {
				if q.Content[i].Value == "pattern" {
					q.Content = slices.Delete(q.Content, i, i+2)
					return nil
				}
			}
			return fmt.Errorf("the question %s has no pattern", question)
		}
		return fmt.Errorf("the manifest has no question %s", question)
	})
}

// templateWithFiles is the fixture template name with one more commit on its
// branch branch, adding the files listed (with /), each holding its own path
// and a line ending.
func (w *world) templateWithFiles(name, branch, files string) error {
	var added [][2]string
	for _, file := range quotedList(files) {
		added = append(added, [2]string{file, file + "\n"})
	}
	return w.templateWithBranchFiles(name, branch, fmt.Sprintf("the files %q", quotedList(files)), added)
}

// templateWithFileLine is the fixture template name with one more commit on
// its branch branch, adding the file file (with /) holding the line line.
func (w *world) templateWithFileLine(name, branch, file, line string) error {
	return w.templateWithBranchFiles(name, branch, fmt.Sprintf("the file %q with the line %q", file, line), [][2]string{{file, line + "\n"}})
}

// templateWithBranchFiles is the fixture template name with one more commit
// on its branch branch, adding each file of files, its path (with /) and
// its contents; what names the files in the template's key. The commit is
// built from git's objects, never from a checkout, so a file reaches the
// branch even where the disk cannot hold its name: windows drops a trailing
// period or space from a name, and git there refuses such a path in its
// index. The working tree stays on the root branch, the template's default.
func (w *world) templateWithBranchFiles(name, branch, what string, files [][2]string) error {
	key := fmt.Sprintf("%s whose branch %s holds %s", name, branch, what)
	return w.useTemplate(key, func(dir string) error {
		if err := w.buildTemplate(filepath.Join(w.root, "features", "testdata", name), dir); err != nil {
			return err
		}
		ref := "refs/heads/" + branch
		parent, err := w.gitOut(dir, "rev-parse", "--verify", ref)
		if err != nil {
			return err
		}
		blobs := map[string]string{}
		for _, file := range files {
			blob, err := w.gitInput(dir, file[1], "hash-object", "-w", "--stdin")
			if err != nil {
				return err
			}
			blobs[file[0]] = strings.TrimSpace(blob)
		}
		tree, err := w.treeWith(dir, parent+"^{tree}", blobs)
		if err != nil {
			return err
		}
		commit, err := w.gitInput(dir, "Add "+what+"\n", "commit-tree", tree, "-p", parent)
		if err != nil {
			return err
		}
		return w.gitIn(dir, "update-ref", ref, strings.TrimSpace(commit), parent)
	})
}

// treeWith writes the tree that is the tree base (any name git reads as a
// tree, or "" for none) with each file of blobs, its path (with /) below the
// tree, its blob that one, and returns its name. A file keeps the mode of
// the one it replaces, so an executable stays one; a new one is not
// executable. It reads and writes trees with ls-tree and mktree, which take
// a name as it is, whatever the disk could hold.
func (w *world) treeWith(dir, base string, blobs map[string]string) (string, error) {
	type entry struct{ mode, kind, object string }
	entries := map[string]entry{}
	if base != "" {
		list, err := w.gitInput(dir, "", "ls-tree", "-z", base)
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(strings.TrimSuffix(list, "\x00"), "\x00") {
			if line == "" {
				continue
			}
			meta, name, ok := strings.Cut(line, "\t")
			fields := strings.Fields(meta)
			if !ok || len(fields) != 3 {
				return "", fmt.Errorf("git ls-tree %s: %q is not a tree entry", base, line)
			}
			entries[name] = entry{fields[0], fields[1], fields[2]}
		}
	}
	below := map[string]map[string]string{}
	for p, blob := range blobs {
		first, rest, nested := strings.Cut(p, "/")
		if !nested {
			mode := "100644"
			if old, ok := entries[first]; ok && old.kind == "blob" {
				mode = old.mode
			}
			entries[first] = entry{mode, "blob", blob}
			continue
		}
		if below[first] == nil {
			below[first] = map[string]string{}
		}
		below[first][rest] = blob
	}
	for name, files := range below {
		sub := ""
		if old, ok := entries[name]; ok && old.kind == "tree" {
			sub = old.object
		}
		tree, err := w.treeWith(dir, sub, files)
		if err != nil {
			return "", err
		}
		entries[name] = entry{"040000", "tree", tree}
	}
	var input strings.Builder
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		e := entries[name]
		fmt.Fprintf(&input, "%s %s %s\t%s\x00", e.mode, e.kind, e.object, name)
	}
	tree, err := w.gitInput(dir, input.String(), "mktree", "-z")
	return strings.TrimSpace(tree), err
}

// gitInput is git run in the folder dir with input on its standard input,
// returning its standard output as it is.
func (w *world) gitInput(dir, input string, args ...string) (string, error) {
	cmd := exec.Command(gitBin(), args...)
	cmd.Dir = dir
	cmd.Env = w.env()
	cmd.Stdin = strings.NewReader(input)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return string(out), nil
}

// manifestVariants are the files of testdata/acme-manifests, each acme's
// manifest with the one change its step names.
var manifestVariants = map[string]string{
	"with the custom tag !foo before the literal of the question name":         "custom-tag.yaml",
	"with the anchor &shared on the stack go's checks and *shared in python's": "anchor.yaml",
	"with a merge key, <<, bringing the stack go's keys into python's":         "merge-key.yaml",
	"followed by a second document, after a line ---":                          "second-document.yaml",
	"with the key stacks given twice":                                          "key-twice.yaml",
	"written as JSON":                                                          "written-as-json.json",
}

// templateWithManifest is the fixture template name with one more commit
// on its root branch, its manifest there replaced by the variant of acme's
// the change names, as it is, byte for byte.
func (w *world) templateWithManifest(name, change string) error {
	file, ok := manifestVariants[change]
	if !ok {
		return fmt.Errorf("no variant of acme's manifest %s in testdata/acme-manifests", change)
	}
	key := fmt.Sprintf("%s whose manifest is acme's %s", name, change)
	return w.useTemplate(key, func(dir string) error {
		if err := w.buildTemplate(filepath.Join(w.root, "features", "testdata", name), dir); err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(w.root, "features", "testdata", "acme-manifests", file))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "itos-template.yaml"), data, 0o644); err != nil {
			return err
		}
		return w.gitIn(dir, "commit", "-q", "-a", "-m", "Change the manifest: "+key)
	})
}

// templateWithSubmodule is the fixture template name with one more commit
// on its branch branch, adding a submodule at path: a gitlink to the
// branch's own commit, as git records a submodule, with no .gitmodules,
// which nothing reads before a submodule is cloned.
func (w *world) templateWithSubmodule(name, branch, path string) error {
	key := fmt.Sprintf("%s whose branch %s holds a submodule at %s", name, branch, path)
	return w.useTemplate(key, func(dir string) error {
		if err := w.buildTemplate(filepath.Join(w.root, "features", "testdata", name), dir); err != nil {
			return err
		}
		root, err := w.gitOut(dir, "symbolic-ref", "--short", "HEAD")
		if err != nil {
			return err
		}
		sha, err := w.gitOut(dir, "rev-parse", "refs/heads/"+branch)
		if err != nil {
			return err
		}
		for _, args := range [][]string{
			{"checkout", "-q", branch},
			{"update-index", "--add", "--cacheinfo", "160000," + sha + "," + path},
			{"commit", "-q", "-m", "Add the submodule " + path},
			{"checkout", "-q", "-f", root},
		} {
			if err := w.gitIn(dir, args...); err != nil {
				return err
			}
		}
		return nil
	})
}

// templateWithoutManifest is the fixture template name with one more commit
// on its root branch, removing the manifest; the other branches keep it.
func (w *world) templateWithoutManifest(name string) error {
	return w.useTemplate(name+" whose root branch holds no manifest", func(dir string) error {
		if err := w.buildTemplate(filepath.Join(w.root, "features", "testdata", name), dir); err != nil {
			return err
		}
		if err := w.gitIn(dir, "rm", "-q", "--", "itos-template.yaml"); err != nil {
			return err
		}
		return w.gitIn(dir, "commit", "-q", "-m", "Remove the manifest")
	})
}

// templateWithKey is the fixture template name, its manifest on the root
// branch given the key key, which the format does not know, as a word.
func (w *world) templateWithKey(name, key string) error {
	return w.changedTemplate(name, fmt.Sprintf("%s whose manifest has the key %q", name, key), func(top *yaml.Node) error {
		top.Content = append(top.Content, scalar(key), scalar("red"))
		return nil
	})
}

// templateWithFirstCommit is the fixture template name, its manifest on the
// root branch of version 4 and giving the first commit the message header,
// a blank line and the footer footer, each line ending in a line feed: the
// whole message, as first_commit holds it (docs/manifest.md).
func (w *world) templateWithFirstCommit(name, header, footer string) error {
	key := fmt.Sprintf("%s whose manifest gives the first commit the message %q with the footer %q", name, header, footer)
	return w.templateWithMessage(name, key, header+"\n\n"+footer+"\n")
}

// templateWithFirstCommitBody is templateWithFirstCommit with the body line
// body, a paragraph of its own between the header and the footer.
func (w *world) templateWithFirstCommitBody(name, header, body, footer string) error {
	key := fmt.Sprintf("%s whose manifest gives the first commit the message %q with the body line %q and the footer %q", name, header, body, footer)
	return w.templateWithMessage(name, key, header+"\n\n"+body+"\n\n"+footer+"\n")
}

// templateWithMessage is the fixture template name, built once a run for
// key, its manifest on the root branch of version 4 and giving the first
// commit the message message.
func (w *world) templateWithMessage(name, key, message string) error {
	return w.changedTemplate(name, key, func(top *yaml.Node) error {
		version := mappingValue(top, "version")
		if version == nil {
			return errors.New("the manifest has no version")
		}
		*version = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "4"}
		top.Content = append(top.Content, scalar("first_commit"), scalar(message))
		return nil
	})
}

// templateWithSetup is the fixture template name, its manifest on the root
// branch of version 5, the root listing the setup step root, when it is
// given, and each stack of stacks the step it maps to: each step a list of
// words, the program first, as a check is (docs/manifest.md, "Setup
// steps"). The words are written as they are given, so a word may hold a
// space or a character no terminal should be shown.
func (w *world) templateWithSetup(name string, root []string, stacks map[string][]string) error {
	key := fmt.Sprintf("%s whose root lists the setup step %q and whose stacks list %q", name, root, stacks)
	steps := func(words []string) *yaml.Node {
		step := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, word := range words {
			step.Content = append(step.Content, scalar(word))
		}
		return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{step}}
	}
	return w.changedTemplate(name, key, func(top *yaml.Node) error {
		version := mappingValue(top, "version")
		if version == nil {
			return errors.New("the manifest has no version")
		}
		*version = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "5"}
		if root != nil {
			top.Content = append(top.Content, scalar("setup"), steps(root))
		}
		for stack, words := range stacks {
			list := mappingValue(top, "stacks")
			if list == nil || list.Kind != yaml.SequenceNode {
				return errors.New("the manifest lists no stacks")
			}
			i := slices.IndexFunc(list.Content, func(s *yaml.Node) bool { return scalarValue(s, "name") == stack })
			if i < 0 {
				return fmt.Errorf("the manifest has no stack %s", stack)
			}
			list.Content[i].Content = append(list.Content[i].Content, scalar("setup"), steps(words))
		}
		return nil
	})
}

// templateWithoutBranch is the fixture template name without its branch
// branch, which the manifest still names; the branches started from it
// stay.
func (w *world) templateWithoutBranch(name, branch string) error {
	return w.useTemplate(fmt.Sprintf("%s whose branch %s is missing", name, branch), func(dir string) error {
		if err := w.buildTemplate(filepath.Join(w.root, "features", "testdata", name), dir); err != nil {
			return err
		}
		return w.gitIn(dir, "branch", "-q", "-D", branch)
	})
}

// useTemplate makes the template the scenario's: the one built for key, or
// one build makes in a new folder, once a run for each key.
func (w *world) useTemplate(key string, build func(dir string) error) error {
	templatesMu.Lock()
	defer templatesMu.Unlock()
	dir, ok := templates[key]
	if !ok {
		if templatesDir == "" {
			var err error
			if templatesDir, err = os.MkdirTemp("", "itos-template-features-templates-"); err != nil {
				return err
			}
		}
		dir = filepath.Join(templatesDir, strconv.Itoa(len(templates)))
		if err := build(dir); err != nil {
			// The next template is built in the same folder: leave it none of
			// this one's files.
			_ = os.RemoveAll(dir)
			return fmt.Errorf("building the template %s: %w", key, err)
		}
		templates[key] = dir
	}
	w.templateDir = dir
	w.template = filepath.ToSlash(dir)
	return nil
}

// buildTemplate makes the git repository dir from the testdata folder src:
// its branches.txt lists each branch and the one it starts from, and each
// branch's files are the folder of its name, laid over what it starts from;
// a branch with no folder adds no file. The files its executables.txt
// lists, when it has one, every branch records as executable, whatever the
// checkout of testdata says: windows has no execute bit, and git there
// records what update-index says. The root branch is checked out at the
// end, so it is the template's default branch.
func (w *world) buildTemplate(src, dir string) error {
	branches, err := branchList(src)
	if err != nil {
		return err
	}
	executables, err := listFile(filepath.Join(src, "executables.txt"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	root := branches[0].name
	for i, b := range branches {
		branch := b.name
		if i == 0 {
			if err := w.gitIn(dir, "init", "-q", "-b", branch); err != nil {
				return err
			}
		} else if err := w.gitIn(dir, "checkout", "-q", "-b", branch, b.from); err != nil {
			return err
		}
		files := filepath.Join(src, filepath.FromSlash(branch))
		if _, err := os.Stat(files); err == nil {
			if err := copyOver(files, dir); err != nil {
				return err
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := w.gitIn(dir, "add", "-A"); err != nil {
			return err
		}
		if err := w.recordExecutable(dir, executables); err != nil {
			return err
		}
		if err := w.gitIn(dir, "commit", "-q", "--allow-empty", "-m", "Make "+branch); err != nil {
			return err
		}
	}
	return w.gitIn(dir, "checkout", "-q", root)
}

// branch is a line of a fixture's branches.txt: a branch and the branch it
// starts from, "" for the root.
type branch struct{ name, from string }

// branchList is the branches the testdata folder src's branches.txt lists,
// in its order, the root first: each branch after the one it starts from.
func branchList(src string) ([]branch, error) {
	list, err := os.ReadFile(filepath.Join(src, "branches.txt"))
	if err != nil {
		return nil, err
	}
	var branches []branch
	lines := bufio.NewScanner(bytes.NewReader(list))
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		switch {
		case len(branches) == 0 && len(fields) == 1:
			branches = append(branches, branch{name: fields[0]})
		case len(branches) > 0 && len(fields) == 2:
			branches = append(branches, branch{name: fields[0], from: fields[1]})
		default:
			return nil, fmt.Errorf("%s: %q is not a branch and the branch it starts from, the root alone first", filepath.Join(src, "branches.txt"), lines.Text())
		}
	}
	if err := lines.Err(); err != nil {
		return nil, err
	}
	if len(branches) == 0 {
		return nil, fmt.Errorf("%s lists no branch", filepath.Join(src, "branches.txt"))
	}
	return branches, nil
}

// copyOver copies every file under the folder src into the folder dir, at
// the same path, replacing a file dir already holds there, as a branch's
// files are laid over what it starts from (os.CopyFS replaces none).
func copyOver(src, dir string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		to := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		return os.WriteFile(to, data, 0o644)
	})
}

// recordExecutable records each of paths (with /) the repository dir holds
// as executable, in its index and, where the system has the bit, in its
// working tree, so the next git add keeps it.
func (w *world) recordExecutable(dir string, paths []string) error {
	for _, p := range paths {
		file := filepath.Join(dir, filepath.FromSlash(p))
		if _, err := os.Stat(file); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err := os.Chmod(file, 0o755); err != nil {
			return err
		}
		if err := w.gitIn(dir, "update-index", "--chmod=+x", "--", p); err != nil {
			return err
		}
	}
	return nil
}

// listFile is the lines of the file p, but empty ones and comments (#), or
// none when there is no such file.
func listFile(p string) ([]string, error) {
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines, nil
}
