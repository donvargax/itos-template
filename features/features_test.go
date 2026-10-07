// The features: every *.feature file in this folder, run by godog through go
// test against the itos-template binary built once a run from this tree, each
// scenario a subtest of TestFeatures. Harvested from itos's
// (github.com/donvargax/itos, features/features_test.go).
//
//	go test ./features -count=1                        every live scenario
//	go test ./features -count=1 -scenarios=<regexp>    the live scenarios with a
//	                                                   tag the expression matches
//
// godog's own tag filter takes exact tags joined by commas; itos's run
// templates join the IDs they select with |, as a regular expression
// (itos.yaml's tests.scenario.run), so -scenarios takes that expression and
// is turned here into godog's filter. A scenario tagged @wip never runs.
package features

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/cucumber/godog"
)

var scenarios = flag.String("scenarios", "", "run only the live scenarios with a tag this regular expression matches")

// stampedVersion is the version the binary under test is stamped with, as a
// release's build stamps the release's, so a scenario can tell the stamp
// from the version a build without one says.
const stampedVersion = "1.2.3-features"

// versionSymbol is the variable the build stamps (-ldflags -X).
const versionSymbol = "github.com/donvargax/itos-template/internal/version.stamp"

func TestFeatures(t *testing.T) {
	filter, err := tagFilter(*scenarios, ".")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(removeCallerPath)
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	bin, err := build(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	suite := godog.TestSuite{
		Name:                "itos-template",
		ScenarioInitializer: func(sc *godog.ScenarioContext) { initializeScenario(sc, root, bin) },
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"."},
			Tags:     filter,
			Strict:   true,
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("a scenario failed")
	}
}

// build builds cmd/itos-template from the module at root into dir, stamped
// with stampedVersion, once for every scenario of the run: itos-template, or
// itos-template.exe on windows, where a program is found by its extension.
// No VCS stamping, so the build runs no git of the caller's.
func build(root, dir string) (string, error) {
	name := "itos-template"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false",
		"-ldflags", "-X "+versionSymbol+"="+stampedVersion,
		"-o", bin, "./cmd/itos-template")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("building itos-template: %v\n%s", err, out)
	}
	return bin, nil
}

// godog's tag filter for a selection: every live scenario without one, else
// the live scenarios carrying a tag the expression matches. An expression
// that matches no tag is an error, so a selection never runs nothing.
func tagFilter(selection, dir string) (string, error) {
	if selection == "" {
		return "~@wip", nil
	}
	pattern, err := regexp.Compile(selection)
	if err != nil {
		return "", err
	}
	tags, err := featureTags(dir)
	if err != nil {
		return "", err
	}
	var each []string
	for _, tag := range tags {
		if pattern.MatchString(tag) {
			each = append(each, tag+"&&~@wip")
		}
	}
	if len(each) == 0 {
		return "", &noMatch{selection}
	}
	return strings.Join(each, ","), nil
}

type noMatch struct{ selection string }

func (e *noMatch) Error() string {
	return "no scenario has a tag that " + e.selection + " matches"
}

// Every tag written in the feature files under dir, once each, sorted.
func featureTags(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.feature"))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		lines := bufio.NewScanner(f)
		for lines.Scan() {
			line := strings.TrimSpace(lines.Text())
			if !strings.HasPrefix(line, "@") {
				continue
			}
			for _, word := range strings.Fields(line) {
				if strings.HasPrefix(word, "#") {
					break
				}
				if strings.HasPrefix(word, "@") {
					seen[word] = true
				}
			}
		}
		_ = f.Close()
		if err := lines.Err(); err != nil {
			return nil, err
		}
	}
	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags, nil
}
