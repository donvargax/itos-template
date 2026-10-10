// The steps of a template's releases (new.feature's and check.feature's
// releases scenarios, decision 36): the fixture "acme released", its
// variants, and what a made project's record says of the release it was
// rendered from.
package features

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

// versions is a step's list of versions, "v1.0.0", "v1.1.0" and
// "v1.2.0-rc.1", each quoted, joined by commas and a last "and", or one
// alone.
const versions = `("[^"]*"(?:(?:, | and )"[^"]*")*)`

func (w *world) releaseSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the template "([^"]*)" released as `+versions+`$`, func(name, list string) error {
		return w.releasedTemplate(name, quotedList(list), "", nil)
	})
	sc.Step(`^the template "([^"]*)" released as `+versions+`, without the tag "([^"]*)"$`, func(name, list, tag string) error {
		return w.releasedTemplate(name, quotedList(list), fmt.Sprintf("without the tag %q", tag), func(dir string) error {
			return w.gitIn(dir, "tag", "-d", tag)
		})
	})
	sc.Step(`^the template "([^"]*)" released as `+versions+`, whose root check fails unless release\.txt holds "([^"]*)"$`, w.releasedWithReleaseCheck)

	sc.Step(`^the record in "([^"]*)" names the release "([^"]*)"$`, w.recordNamesRelease)
	sc.Step(`^the record in "([^"]*)" names no release$`, w.recordNamesNoRelease)
}

// releasedTemplate is the fixture template name released as each of
// versions, in order (release), then every branch moved one commit past the
// last, release.txt holding heads; then changed by change, when it is given,
// which variant names in the template's key.
func (w *world) releasedTemplate(name string, versions []string, variant string, change func(dir string) error) error {
	key := fmt.Sprintf("%s released as %q", name, versions)
	if variant != "" {
		key += ", " + variant
	}
	return w.useTemplate(key, func(dir string) error {
		src := filepath.Join(w.root, "features", "testdata", name)
		if err := w.buildTemplate(src, dir); err != nil {
			return err
		}
		if err := w.release(src, dir, versions); err != nil {
			return err
		}
		if change == nil {
			return nil
		}
		return change(dir)
	})
}

// releasedWithReleaseCheck is the fixture template name, its manifest on the
// root branch giving the root one more check, which fails unless the
// render's release.txt holds text, before it is released as each of
// versions, so every release's manifest and the heads' have the check.
func (w *world) releasedWithReleaseCheck(name, list, text string) error {
	versions := quotedList(list)
	key := fmt.Sprintf("%s released as %q, whose root check fails unless release.txt holds %q", name, versions, text)
	return w.useTemplate(key, func(dir string) error {
		src := filepath.Join(w.root, "features", "testdata", name)
		if err := w.buildTemplate(src, dir); err != nil {
			return err
		}
		err := w.changeManifest(dir, key, func(top *yaml.Node) error {
			checks := mappingValue(top, "checks")
			if checks == nil || checks.Kind != yaml.SequenceNode {
				return errors.New("the manifest has no checks of the root")
			}
			checks.Content = append(checks.Content, &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle, Content: []*yaml.Node{
				scalar("sh"), scalar("-c"), scalar(releaseCheck(text)),
			}})
			return nil
		})
		if err != nil {
			return err
		}
		return w.release(src, dir, versions)
	})
}

// releaseCheck is the sh script releasedWithReleaseCheck's check runs: it
// exits 0 when the render's release.txt holds text, its line ending aside,
// and 1, saying what it holds, when it holds another. text is named by its
// bytes in octal, as textCheck names its text, so no shell reads it.
func releaseCheck(text string) string {
	return `t=$(printf '` + octal(text) + `'); ` +
		`r=$(tr -d '\r' < release.txt); ` +
		`[ "$r" = "$t" ] && exit 0; ` +
		`printf 'release.txt holds %s\n' "$r"; exit 1`
}

// release releases the template in dir, built from the testdata folder src,
// as each of versions, in order: release.txt holding the version committed
// on the root branch, the root merged down into every branch, each branch
// merging the one it starts from in branches.txt's order (the stacks before
// their features, the order new merges), and every branch tagged
// <branch>/<version>. Then every branch is moved one commit past the last
// release the same way, release.txt holding heads, untagged. The root
// branch is checked out at the end, so it stays the default branch.
func (w *world) release(src, dir string, versions []string) error {
	branches, err := branchList(src)
	if err != nil {
		return err
	}
	for _, v := range append(versions[:len(versions):len(versions)], "heads") {
		if err := w.mergeDown(dir, branches, v); err != nil {
			return err
		}
		if v == "heads" {
			break
		}
		for _, b := range branches {
			if err := w.gitIn(dir, "tag", b.name+"/"+v, "refs/heads/"+b.name); err != nil {
				return err
			}
		}
	}
	return w.gitIn(dir, "checkout", "-q", branches[0].name)
}

// mergeDown commits release.txt holding text on the root branch of the
// template in dir, then merges each branch's start into it, in the order
// branches lists them.
func (w *world) mergeDown(dir string, branches []branch, text string) error {
	if err := w.gitIn(dir, "checkout", "-q", branches[0].name); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "release.txt"), []byte(text+"\n"), 0o644); err != nil {
		return err
	}
	if err := w.gitIn(dir, "add", "--", "release.txt"); err != nil {
		return err
	}
	if err := w.gitIn(dir, "commit", "-q", "-m", "Release "+text); err != nil {
		return err
	}
	for _, b := range branches[1:] {
		if err := w.gitIn(dir, "checkout", "-q", b.name); err != nil {
			return err
		}
		if err := w.gitIn(dir, "merge", "-q", "--no-ff", "--no-edit", "-m", "Merge "+b.from+" into "+b.name, b.from); err != nil {
			return err
		}
	}
	return nil
}

// The record's release.

// recordNamesRelease is whether the record names the release version.
func (w *world) recordNamesRelease(dir, version string) error {
	r, err := w.record(dir)
	if err != nil {
		return err
	}
	if r.Release == nil {
		return fmt.Errorf("the record names no release, not %q\n%s", version, w.report())
	}
	if *r.Release != version {
		return fmt.Errorf("the record names the release %q, not %q", *r.Release, version)
	}
	return nil
}

// recordNamesNoRelease is whether the record has no release key at all.
func (w *world) recordNamesNoRelease(dir string) error {
	r, err := w.record(dir)
	if err != nil {
		return err
	}
	if r.Release != nil {
		return fmt.Errorf("the record names the release %q", *r.Release)
	}
	return nil
}
