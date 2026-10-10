// The steps of check.feature: fixture templates whose manifest a scenario
// changes, check run in a template's folder, and check's report, read
// strictly in the format docs/manifest.md gives it ("What check reports"):
// a report the format does not describe fails the step reading it, so the
// format cannot drift from what the document says.
package features

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

func (w *world) checkSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the template "([^"]*)" whose feature "([^"]*)" of the stack "([^"]*)" has the check "([^"]*)"$`, w.templateWithFeatureCheck)
	sc.Step(`^the template "([^"]*)" whose manifest lists the stack "([^"]*)" with the features (".*") as unsupported$`, w.templateWithUnsupported)
	sc.Step(`^the template "([^"]*)" whose branch "([^"]*)" holds the file "([^"]*)" with the line "([^"]*)"$`, w.templateWithFileLine)
	sc.Step(`^the template "([^"]*)" whose root has, after its own, the check "([^"]*)" marked as scanning "([^"]*)"$`, w.templateWithMarkedCheck)
	sc.Step(`^the template "([^"]*)" whose root has a check that fails where a render holds "([^"]*)"$`, w.templateWithTextCheck)

	// Split before {template} is expanded, so a path with a space stays one
	// argument.
	sc.Step(`^itos-template runs in the template's folder with "([^"]*)"$`, func(args string) error {
		if w.templateDir == "" {
			return errors.New("no template in this scenario")
		}
		fields := strings.Fields(args)
		for i, f := range fields {
			fields[i] = w.expand(f)
		}
		return w.run(w.templateDir, w.bin, fields...)
	})
	sc.Step(`^a clone "([^"]*)" of the template, only its default branch local$`, w.cloneOfTemplate)
	sc.Step(`^the clone "([^"]*)" has its HEAD detached$`, w.cloneDetached)
	sc.Step(`^the clone "([^"]*)" has its HEAD detached, no local branch and no record of origin's HEAD$`, w.cloneAsPullRequest)
	sc.Step(`^the clone "([^"]*)" has an origin that cannot be reached$`, w.cloneWithUnreachableOrigin)
	sc.Step(`^the clone "([^"]*)" has an origin whose HEAD names no branch$`, w.cloneWithDetachedOrigin)
	sc.Step(`^a bare clone "([^"]*)" of the template$`, w.bareCloneOfTemplate)
	sc.Step(`^itos-template runs in the folder "([^"]*)" with "([^"]*)"$`, func(folder, args string) error {
		fields := strings.Fields(args)
		for i, f := range fields {
			fields[i] = w.expand(f)
		}
		return w.run(w.path(folder), w.bin, fields...)
	})

	sc.Step(`^its report says "([^"]*)" passed$`, w.reportSaysPassed)
	sc.Step(`^its report says "([^"]*)" failed at "([^"]*)"$`, w.reportSaysFailedAt)
	sc.Step(`^its report shows the checks of "([^"]*)" in order: (".*")$`, w.reportShowsChecks)
	sc.Step(`^its report says "([^"]*)" failed with the leftover "([^"]*)" at "([^"]*)"$`, w.reportSaysLeftover)
	sc.Step(`^its report does not name "([^"]*)"$`, w.reportDoesNotName)
	sc.Step(`^its report says "([^"]*)"$`, w.reportSays)
	sc.Step(`^its report names no other combination$`, w.reportNamesNoOther)
	sc.Step(`^its report names no combination$`, w.reportNamesNone)
}

// The fixture templates a scenario changes.

// templateWithFeatureCheck is the fixture template name, its manifest on the
// root branch giving the feature of the stack one more check: the words of
// check, split at spaces.
func (w *world) templateWithFeatureCheck(name, feature, stack, check string) error {
	key := fmt.Sprintf("%s whose feature %s of %s has the check %q", name, feature, stack, check)
	return w.changedTemplate(name, key, func(top *yaml.Node) error {
		features := mappingValue(top, "features")
		if features == nil || features.Kind != yaml.SequenceNode {
			return errors.New("the manifest lists no features")
		}
		for _, f := range features.Content {
			if scalarValue(f, "name") != feature || scalarValue(f, "stack") != stack {
				continue
			}
			checks := mappingValue(f, "checks")
			if checks == nil {
				checks = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
				f.Content = append(f.Content, scalar("checks"), checks)
			}
			words := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
			for _, word := range strings.Fields(check) {
				words.Content = append(words.Content, scalar(word))
			}
			checks.Content = append(checks.Content, words)
			return nil
		}
		return fmt.Errorf("the manifest has no feature %s of the stack %s", feature, stack)
	})
}

// templateWithUnsupported is the fixture template name, its manifest on the
// root branch listing the stack with exactly the features as unsupported.
func (w *world) templateWithUnsupported(name, stack, features string) error {
	list := quotedList(features)
	key := fmt.Sprintf("%s whose manifest lists %s with %q as unsupported", name, stack, list)
	return w.changedTemplate(name, key, func(top *yaml.Node) error {
		unsupported := mappingValue(top, "unsupported")
		if unsupported == nil {
			unsupported = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
			top.Content = append(top.Content, scalar("unsupported"), unsupported)
		}
		names := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, f := range list {
			names.Content = append(names.Content, scalar(f))
		}
		unsupported.Content = append(unsupported.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			scalar("stack"), scalar(stack),
			scalar("features"), names,
		}})
		return nil
	})
}

// templateWithMarkedCheck is the fixture template name, its manifest on the
// root branch of version 3, the root's checks followed by one more in the
// long form: the words of check, split at spaces, marked as scanning scan.
func (w *world) templateWithMarkedCheck(name, check, scan string) error {
	key := fmt.Sprintf("%s whose root has the check %q marked as scanning %q", name, check, scan)
	return w.changedTemplate(name, key, func(top *yaml.Node) error {
		version := mappingValue(top, "version")
		if version == nil {
			return errors.New("the manifest has no version")
		}
		*version = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "3"}
		checks := mappingValue(top, "checks")
		if checks == nil || checks.Kind != yaml.SequenceNode {
			return errors.New("the manifest has no checks of the root")
		}
		words := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, word := range strings.Fields(check) {
			words.Content = append(words.Content, scalar(word))
		}
		scans := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle, Content: []*yaml.Node{scalar(scan)}}
		checks.Content = append(checks.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			scalar("run"), words,
			scalar("scans"), scans,
		}})
		return nil
	})
}

// templateWithTextCheck is the fixture template name, its manifest on the
// root branch giving the root one more check, which fails where a render
// holds text: in a file git tracks there (git grep) or in a commit's
// message (git log). The CLI runs a check with no shell, so it is sh -c and
// a script, which sh is on every platform's runner. The report prints a
// check's words, so the script names text by its bytes in octal, which
// printf turns back into it: text itself is never in a word, and so in no
// line of the report the scenario reads, and the octal digits in single
// quotes are a word no shell reads specially, whatever text holds.
func (w *world) templateWithTextCheck(name, text string) error {
	key := fmt.Sprintf("%s whose root has a check that fails where a render holds %q", name, text)
	return w.changedTemplate(name, key, func(top *yaml.Node) error {
		checks := mappingValue(top, "checks")
		if checks == nil || checks.Kind != yaml.SequenceNode {
			return errors.New("the manifest has no checks of the root")
		}
		checks.Content = append(checks.Content, &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle, Content: []*yaml.Node{
			scalar("sh"), scalar("-c"), scalar(textCheck(text)),
		}})
		return nil
	})
}

// textCheck is the sh script templateWithTextCheck's check runs: it exits 1,
// saying where, when the render in the folder it runs in holds text in a
// tracked file or in a commit's message, and 0 when it holds it in neither.
func textCheck(text string) string {
	var octal strings.Builder
	for _, b := range []byte(text) {
		fmt.Fprintf(&octal, `\%03o`, b)
	}
	return `t=$(printf '` + octal.String() + `'); ` +
		`if git grep -q -F -e "$t"; then echo 'a tracked file holds the text'; exit 1; fi; ` +
		`case "$(git log --format=%B)" in *"$t"*) echo 'a commit message holds the text'; exit 1;; esac`
}

// changedTemplate builds, once a run for each key, the fixture template name
// with one more commit on its root branch: the manifest there, its top
// mapping changed by change. The manifest is read from the root branch
// alone, so the other branches keep the fixture's.
func (w *world) changedTemplate(name, key string, change func(top *yaml.Node) error) error {
	return w.useTemplate(key, func(dir string) error {
		if err := w.buildTemplate(filepath.Join(w.root, "features", "testdata", name), dir); err != nil {
			return err
		}
		file := filepath.Join(dir, "itos-template.yaml")
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return err
		}
		if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
			return errors.New("the manifest is not a mapping")
		}
		if err := change(doc.Content[0]); err != nil {
			return err
		}
		var out bytes.Buffer
		enc := yaml.NewEncoder(&out)
		enc.SetIndent(2)
		if err := enc.Encode(&doc); err != nil {
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}
		if err := os.WriteFile(file, out.Bytes(), 0o644); err != nil {
			return err
		}
		return w.gitIn(dir, "commit", "-q", "-a", "-m", "Change the manifest: "+key)
	})
}

// cloneOfTemplate clones the scenario's template into the folder name of the
// scratch repository as a CI checkout has it: git clone's own, the default
// branch the one local branch, every other only origin's remote-tracking
// branch.
func (w *world) cloneOfTemplate(name string) error {
	if w.templateDir == "" {
		return errors.New("no template in this scenario")
	}
	return w.git("clone", "-q", "--", w.template, name)
}

// cloneDetached detaches the HEAD of the clone name at the commit it is on,
// as a checkout of a commit rather than a branch leaves it.
func (w *world) cloneDetached(name string) error {
	return w.gitIn(w.path(name), "checkout", "-q", "--detach")
}

// cloneAsPullRequest leaves the clone name as actions/checkout leaves a pull
// request's checkout: its HEAD detached, no local branch, and no
// refs/remotes/origin/HEAD, so only origin itself can say which branch is
// the root.
func (w *world) cloneAsPullRequest(name string) error {
	dir := w.path(name)
	if err := w.cloneDetached(name); err != nil {
		return err
	}
	branches, err := w.gitOut(dir, "for-each-ref", "--format=%(refname)", "refs/heads/")
	if err != nil {
		return err
	}
	for _, ref := range strings.Fields(branches) {
		if err := w.gitIn(dir, "update-ref", "-d", ref); err != nil {
			return err
		}
	}
	return w.gitIn(dir, "remote", "set-head", "origin", "-d")
}

// cloneWithUnreachableOrigin points the clone name's origin at a folder that
// does not exist, so asking origin anything (git ls-remote) fails at once,
// with no network.
func (w *world) cloneWithUnreachableOrigin(name string) error {
	gone := filepath.ToSlash(w.path("no-such-origin.git"))
	return w.gitIn(w.path(name), "remote", "set-url", "origin", gone)
}

// cloneWithDetachedOrigin points the clone name's origin at a bare copy of
// the template whose HEAD is detached at the commit it named, so origin
// answers git ls-remote but names no branch as its HEAD. The fixture
// template itself is never changed.
func (w *world) cloneWithDetachedOrigin(name string) error {
	origin := name + "-origin.git"
	if err := w.git("clone", "-q", "--bare", "--", w.template, origin); err != nil {
		return err
	}
	dir := w.path(origin)
	commit, err := w.gitOut(dir, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if err := w.gitIn(dir, "update-ref", "--no-deref", "HEAD", commit); err != nil {
		return err
	}
	return w.gitIn(w.path(name), "remote", "set-url", "origin", filepath.ToSlash(dir))
}

// bareCloneOfTemplate clones the scenario's template bare into the folder
// name of the scratch repository: every branch local, HEAD the root branch.
func (w *world) bareCloneOfTemplate(name string) error {
	if w.templateDir == "" {
		return errors.New("no template in this scenario")
	}
	return w.git("clone", "-q", "--bare", "--", w.template, name)
}

// mappingValue is the value of key in the mapping m, or nil.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func scalarValue(m *yaml.Node, key string) string {
	if v := mappingValue(m, key); v != nil && v.Kind == yaml.ScalarNode {
		return v.Value
	}
	return ""
}

func scalar(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

// The report.

// checkReport is check's report as the steps read it: each combination in
// the order it names them, and the count its last line gives.
type checkReport struct {
	combinations []combinationReport
	passed       int
}

type combinationReport struct {
	name        string
	passed      bool
	notRendered bool
	leftovers   []leftoverLine
	checks      []checkLine
}

type leftoverLine struct {
	text string // the text found
	at   string // where: the path, or the path, a colon and the line
}

type checkLine struct {
	status string // passed, failed or skipped
	check  string // the check as it ran, the answers in place
}

var (
	combinationLine = regexp.MustCompile(`^([a-z0-9][a-z0-9._-]*(?: \+ [a-z0-9][a-z0-9._-]*)*): (passed|failed)$`)
	checkStatusLine = regexp.MustCompile(`^  (passed|failed|skipped): (\S.*)$`)
	notRenderedLine = "  not rendered"
	outputLine      = regexp.MustCompile(`^    \|(?: .*)?$`)
	summaryLine     = regexp.MustCompile(`^(\d+) of (\d+) combinations? passed\.$`)
	leftoverPrefix  = "  leftover: "
	lineNumber      = regexp.MustCompile(`^(.+):([1-9][0-9]*)$`)
)

// parseLeftover reads a leftover's line after its prefix: the text found,
// in double quotes as Go writes a string, then " at " and the path, a colon
// and the line, for one in a file's contents, or " in the path " and the
// path, for one in a path; the path written as a check's word is.
func parseLeftover(rest string) (leftoverLine, error) {
	quoted, err := strconv.QuotedPrefix(rest)
	if err != nil {
		return leftoverLine{}, errors.New("does not give the text found in double quotes")
	}
	text, _ := strconv.Unquote(quoted)
	rest = rest[len(quoted):]
	word := func(s string) (string, error) {
		if !strings.HasPrefix(s, `"`) {
			if s == "" || strings.ContainsAny(s, " \"'\\") {
				return "", errors.New("writes a path that needs double quotes without them")
			}
			return s, nil
		}
		q, err := strconv.QuotedPrefix(s)
		if err != nil || q != s {
			return "", errors.New("writes a path in double quotes that do not close it")
		}
		u, _ := strconv.Unquote(q)
		return u, nil
	}
	switch {
	case strings.HasPrefix(rest, " at "):
		m := lineNumber.FindStringSubmatch(rest[len(" at "):])
		if m == nil {
			return leftoverLine{}, errors.New("is at no path and line")
		}
		path, err := word(m[1])
		if err != nil {
			return leftoverLine{}, err
		}
		return leftoverLine{text: text, at: path + ":" + m[2]}, nil
	case strings.HasPrefix(rest, " in the path "):
		path, err := word(rest[len(" in the path "):])
		if err != nil {
			return leftoverLine{}, err
		}
		return leftoverLine{text: text, at: path}, nil
	}
	return leftoverLine{}, errors.New("says neither where in a file nor in which path")
}

// parseReport reads standard output as check's report, strictly: one block
// per combination, its name and whether it passed, then its checks, each
// passed, failed or skipped, a failed one followed by its output (lines
// starting with "    |"), or "not rendered" followed by why; then an empty
// line and the count of combinations that passed. A line the format does
// not describe, or a report that contradicts itself, is an error.
func parseReport(stdout string) (*checkReport, error) {
	text := strings.ReplaceAll(stdout, "\r\n", "\n")
	if !strings.HasSuffix(text, "\n") {
		return nil, fmt.Errorf("the report does not end with a line ending")
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) < 1 {
		return nil, errors.New("the report is empty")
	}
	m := summaryLine.FindStringSubmatch(lines[len(lines)-1])
	if m == nil {
		return nil, fmt.Errorf("the report's last line, %q, is not the count of combinations that passed", lines[len(lines)-1])
	}
	r := &checkReport{}
	r.passed, _ = strconv.Atoi(m[1])
	total, _ := strconv.Atoi(m[2])
	body := lines[:len(lines)-1]
	if len(body) > 0 {
		if body[len(body)-1] != "" {
			return nil, errors.New("the report's count is not after an empty line")
		}
		body = body[:len(body)-1]
	}
	var c *combinationReport
	output := false // whether the line before may be followed by output
	for i, line := range body {
		bad := func(why string) error { return fmt.Errorf("the report's line %d, %q, %s", i+1, line, why) }
		if m := combinationLine.FindStringSubmatch(line); m != nil {
			r.combinations = append(r.combinations, combinationReport{name: m[1], passed: m[2] == "passed"})
			c = &r.combinations[len(r.combinations)-1]
			output = false
			continue
		}
		if c == nil {
			return nil, bad("comes before any combination")
		}
		switch m := checkStatusLine.FindStringSubmatch(line); {
		case strings.HasPrefix(line, leftoverPrefix):
			if c.notRendered || len(c.checks) > 0 {
				return nil, bad("names a leftover after the combination's checks, or of one not rendered")
			}
			l, err := parseLeftover(line[len(leftoverPrefix):])
			if err != nil {
				return nil, bad(err.Error())
			}
			c.leftovers = append(c.leftovers, l)
			output = false
		case m != nil:
			if c.notRendered {
				return nil, bad("names a check of a combination that was not rendered")
			}
			if n := len(c.checks); n > 0 && c.checks[n-1].status != "passed" && m[1] != "skipped" {
				return nil, bad("follows a check that did not pass, so is not skipped")
			}
			if m[1] == "skipped" && (len(c.checks) == 0 || c.checks[len(c.checks)-1].status == "passed") {
				return nil, bad("skips a check that no failed check stopped")
			}
			c.checks = append(c.checks, checkLine{status: m[1], check: m[2]})
			output = m[1] == "failed"
		case line == notRenderedLine:
			if c.notRendered || len(c.checks) > 0 {
				return nil, bad("says not rendered after the combination's checks")
			}
			c.notRendered = true
			output = true
		case outputLine.MatchString(line):
			if !output {
				return nil, bad("is output, but follows no failed check and no render that failed")
			}
		default:
			return nil, bad("is no line the report's format describes")
		}
	}
	passed := 0
	for _, c := range r.combinations {
		failed := c.notRendered || len(c.leftovers) > 0 || slices.ContainsFunc(c.checks, func(l checkLine) bool { return l.status != "passed" })
		if c.passed == failed {
			return nil, fmt.Errorf("the report says %s %s, but its checks say otherwise", c.name, map[bool]string{true: "passed", false: "failed"}[c.passed])
		}
		if c.passed {
			passed++
		}
	}
	if total != len(r.combinations) || passed != r.passed {
		return nil, fmt.Errorf("the report counts %d of %d combinations passed, but names %d, %d passed", r.passed, total, len(r.combinations), passed)
	}
	return r, nil
}

// combination is the combination name in the report, which the scenario
// then names: its report names no other combination is judged against the
// ones the scenario named.
func (w *world) combination(name string) (*combinationReport, error) {
	r, err := parseReport(w.stdout)
	if err != nil {
		return nil, fmt.Errorf("%v\n%s", err, w.report())
	}
	w.named = append(w.named, name)
	for i := range r.combinations {
		if r.combinations[i].name == name {
			return &r.combinations[i], nil
		}
	}
	return nil, fmt.Errorf("the report does not name %s\n%s", name, w.report())
}

func (w *world) reportSaysPassed(name string) error {
	c, err := w.combination(name)
	if err != nil {
		return err
	}
	if !c.passed {
		return fmt.Errorf("the report says %s failed\n%s", name, w.report())
	}
	return nil
}

func (w *world) reportSaysFailedAt(name, check string) error {
	c, err := w.combination(name)
	if err != nil {
		return err
	}
	if c.passed {
		return fmt.Errorf("the report says %s passed\n%s", name, w.report())
	}
	i := slices.IndexFunc(c.checks, func(l checkLine) bool { return l.status == "failed" })
	if i < 0 || c.checks[i].check != check {
		return fmt.Errorf("the report does not say %s failed at %q\n%s", name, check, w.report())
	}
	return nil
}

func (w *world) reportSaysLeftover(name, text, at string) error {
	c, err := w.combination(name)
	if err != nil {
		return err
	}
	if c.passed {
		return fmt.Errorf("the report says %s passed\n%s", name, w.report())
	}
	if !slices.Contains(c.leftovers, leftoverLine{text: text, at: at}) {
		return fmt.Errorf("the report does not say %s failed with the leftover %q at %s\n%s", name, text, at, w.report())
	}
	return nil
}

func (w *world) reportShowsChecks(name, list string) error {
	c, err := w.combination(name)
	if err != nil {
		return err
	}
	var got []string
	for _, l := range c.checks {
		got = append(got, l.check)
	}
	if want := quotedList(list); !slices.Equal(got, want) {
		return fmt.Errorf("the report shows the checks of %s as %q, not %q\n%s", name, got, want, w.report())
	}
	return nil
}

// reportSays is whether check's report, read as the format gives it, says
// text, a check's output included.
func (w *world) reportSays(text string) error {
	if _, err := parseReport(w.stdout); err != nil {
		return fmt.Errorf("%v\n%s", err, w.report())
	}
	if !strings.Contains(w.stdout, text) {
		return fmt.Errorf("the report does not say %q\n%s", text, w.report())
	}
	return nil
}

func (w *world) reportDoesNotName(name string) error {
	r, err := parseReport(w.stdout)
	if err != nil {
		return fmt.Errorf("%v\n%s", err, w.report())
	}
	if slices.ContainsFunc(r.combinations, func(c combinationReport) bool { return c.name == name }) {
		return fmt.Errorf("the report names %s\n%s", name, w.report())
	}
	return nil
}

func (w *world) reportNamesNoOther() error {
	r, err := parseReport(w.stdout)
	if err != nil {
		return fmt.Errorf("%v\n%s", err, w.report())
	}
	var others []string
	for _, c := range r.combinations {
		if !slices.Contains(w.named, c.name) {
			others = append(others, c.name)
		}
	}
	if len(others) > 0 {
		return fmt.Errorf("the report names %q too\n%s", others, w.report())
	}
	return nil
}

// reportNamesNone is whether standard output names no combination: empty,
// or a report of none.
func (w *world) reportNamesNone() error {
	if w.stdout == "" {
		return nil
	}
	r, err := parseReport(w.stdout)
	if err != nil {
		return fmt.Errorf("%v\n%s", err, w.report())
	}
	if len(r.combinations) > 0 {
		return fmt.Errorf("the report names %d combinations\n%s", len(r.combinations), w.report())
	}
	return nil
}
