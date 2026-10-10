package updating

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/template"
	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

const manifestText = `version: 2
stacks:
  - name: sh
features:
  - name: extra
    stack: sh
questions:
  - name: name
    literal: acme-widget
    question: Name?
    case_forms: true
  - name: owner
    literal: Acme Corp
    question: Owner?
    default: Nobody
`

// recorded is the record of a project made from acme's release v1.0.0.
func recorded() project.Record {
	return project.Record{
		Version:  project.RecordVersion,
		Template: "/home/you/acme",
		Release:  "v1.0.0",
		Stack:    "sh",
		Features: []string{"extra"},
		Answers:  answer.Set{"name": "blue-fox", "owner": "Blue Corp"},
		Commits:  map[string]string{"main": "tag main/v1.0.0", "stack/sh": "tag stack/sh/v1.0.0", "sh/extra": "tag sh/extra/v1.0.0"},
	}
}

// recordFile is r as the file new writes.
func recordFile(t *testing.T, r project.Record) []byte {
	t.Helper()
	f, err := r.File()
	if err != nil {
		t.Fatal(err)
	}
	return f.Data
}

// clean is a clean project at its top, HEAD holding its record r and
// README.md.
func clean(t *testing.T, r project.Record) *porttest.Project {
	t.Helper()
	return &porttest.Project{
		State: port.CheckoutState{Repository: true, WorkTree: true, Top: "/home/you/made", Committed: true},
		Head: map[string]port.File{
			project.RecordFile: {Path: project.RecordFile, Mode: 0o644, Data: recordFile(t, r)},
			"README.md":        {Path: "README.md", Mode: 0o644, Data: []byte("# blue-fox\n")},
		},
	}
}

func TestOpenReadsTheRecordAtHead(t *testing.T) {
	s, err := Open(clean(t, recorded()), answer.Given{"new=1", "malformed"})
	if err != nil {
		t.Fatal(err)
	}
	want := recorded()
	if s.Record.Template != want.Template || s.Record.Release != want.Release || !maps.Equal(s.Record.Commits, want.Commits) {
		t.Errorf("the record read is %+v", s.Record)
	}
}

func TestOpenRefusesWhatIsNoCleanProjectWithARecord(t *testing.T) {
	for name, c := range map[string]struct {
		change func(*porttest.Project)
		want   func(error) bool
	}{
		"no repository": {func(p *porttest.Project) { p.State = port.CheckoutState{Why: "not a git repository"} }, func(err error) bool {
			var e *NotRepository
			return errors.As(err, &e) && e.Why == "not a git repository"
		}},
		"no working tree": {func(p *porttest.Project) { p.State.WorkTree = false }, func(err error) bool {
			var e *NoWorkTree
			return errors.As(err, &e)
		}},
		"no commit": {func(p *porttest.Project) { p.State.Committed = false }, func(err error) bool {
			var e *NoCommit
			return errors.As(err, &e) && e.Top == "/home/you/made"
		}},
		"dirty": {func(p *porttest.Project) { p.State.Changed = []string{"README.md", "new.txt"} }, func(err error) bool {
			var e *Dirty
			return errors.As(err, &e) && slices.Equal(e.Paths, []string{"README.md", "new.txt"})
		}},
		"no record": {func(p *porttest.Project) { delete(p.Head, project.RecordFile) }, func(err error) bool {
			var e *NoRecord
			return errors.As(err, &e) && e.File == project.RecordFile
		}},
		"a record that cannot be read": {func(p *porttest.Project) {
			p.Head[project.RecordFile] = port.File{Path: project.RecordFile, Data: []byte("version: 9\n")}
		}, func(err error) bool {
			var e *project.BadRecord
			return errors.As(err, &e)
		}},
		"HEAD that cannot be read": {func(p *porttest.Project) { p.Fail = errors.New("broken") }, func(err error) bool {
			return err != nil && err.Error() == "broken"
		}},
	} {
		p := clean(t, recorded())
		c.change(p)
		s, err := Open(p, nil)
		if s != nil || !c.want(err) {
			t.Errorf("with %s, Open = %v, %v", name, s, err)
		}
	}
}

func TestOpenRefusesOneAnswerToAQuestionTheRecordAnswers(t *testing.T) {
	_, err := Open(clean(t, recorded()), answer.Given{"name=red-cat"})
	var a *Answered
	if !errors.As(err, &a) || a.Name != "name" || a.Answer != "blue-fox" {
		t.Errorf("Open = %v", err)
	}
}

func TestOpenRefusesEveryAnswerToAQuestionTheRecordAnswers(t *testing.T) {
	_, err := Open(clean(t, recorded()), answer.Given{"name=red-cat", "owner=Red Corp", "new=1"})
	var named []string
	for _, e := range err.(interface{ Unwrap() []error }).Unwrap() {
		var a *Answered
		if !errors.As(e, &a) {
			t.Fatalf("%v is no *Answered", e)
		}
		named = append(named, a.Name+"="+a.Answer)
	}
	if !slices.Equal(named, []string{"name=blue-fox", "owner=Blue Corp"}) {
		t.Errorf("the answers refused are %q", named)
	}
}

// acme is the fixture template released as each of versions, every
// branch's tag a tree of its own holding release.txt, and its heads past
// the last.
func acme(versions ...string) *porttest.Repository {
	main := fstest.MapFS{
		manifest.File: {Data: []byte(manifestText)},
		"README.md":   {Data: []byte("# acme-widget by Acme Corp\n")},
		"release.txt": {Data: []byte("heads\n")},
	}
	sh := maps.Clone(main)
	sh["bin/acme-widget"] = &fstest.MapFile{Data: []byte("#!/bin/sh\n"), Mode: porttest.Executable}
	extra := maps.Clone(sh)
	extra["extra.txt"] = &fstest.MapFile{Data: []byte("extra\n")}
	repo := &porttest.Repository{Root: "main", Branches: map[string]fstest.MapFS{"main": main, "stack/sh": sh, "sh/extra": extra}, Tagged: map[string]porttest.Tag{}}
	for _, v := range versions {
		for b, tree := range repo.Branches {
			tagged := maps.Clone(tree)
			tagged["release.txt"] = &fstest.MapFile{Data: []byte(v + "\n")}
			repo.Tagged[b+"/"+v] = porttest.Tag{Tree: tagged, On: b}
		}
	}
	return repo
}

func opened(t *testing.T, repo *porttest.Repository) *template.Template {
	t.Helper()
	tpl, err := template.Open("/home/you/acme", repo)
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

func TestTargetIsTheNewestReleaseOrTheOneNamedAndOnlyForward(t *testing.T) {
	for _, c := range []struct {
		name, from, to string
		newer          bool
		release        string
		older          bool
	}{
		{"the newest, newer", "v1.0.0", "", true, "v1.1.0", false},
		{"the newest, current", "v1.1.0", "", false, "v1.1.0", false},
		{"a pre-release named", "v1.1.0", "v1.2.0-rc.1", true, "v1.2.0-rc.1", false},
		{"the release named, current", "v1.0.0", "v1.0.0", false, "v1.0.0", false},
		{"an older release named", "v1.1.0", "v1.0.0", false, "v1.0.0", true},
		{"from the branch heads", "", "v1.0.0", true, "v1.0.0", false},
		{"semver's equal, written otherwise", "v1.0.0+old", "v1.0.0", true, "v1.0.0", false},
	} {
		r := recorded()
		r.Release = c.from
		tpl := opened(t, acme("v1.0.0", "v1.1.0", "v1.2.0-rc.1"))
		newer, err := (&Start{Record: r}).Target(tpl, c.to)
		var older *Older
		if newer != c.newer || errors.As(err, &older) != c.older || !c.older && err != nil || tpl.Release() != c.release {
			t.Errorf("%s: Target = %v, %v, the release %q", c.name, newer, err, tpl.Release())
		}
		if c.older && (older.Recorded != c.from || older.Target != c.to) {
			t.Errorf("%s: the refusal names %+v", c.name, older)
		}
	}
}

func TestTargetRefusesATemplateWithNoReleaseAndPassesOnWhatTheTemplateRefuses(t *testing.T) {
	s := &Start{Record: recorded()}
	newer, err := s.Target(opened(t, acme()), "")
	var none *NoRelease
	if !errors.As(err, &none) || newer {
		t.Errorf("a template with no release gives %v, %v", newer, err)
	}
	newer, err = s.Target(opened(t, acme("v1.0.0")), "v9.9.9")
	var unknown *template.NoRelease
	if !errors.As(err, &unknown) || unknown.Version != "v9.9.9" || newer {
		t.Errorf("a --to no release has gives %v, %v", newer, err)
	}
	broken := acme("v1.0.0")
	broken.NoTags = errors.New("no tags")
	if newer, err := s.Target(opened(t, broken), ""); err == nil || err.Error() != "no tags" || newer {
		t.Errorf("tags that cannot be read give %v, %v", newer, err)
	}
}

func TestGivenKeepsTheRecordsAnswersToTheQuestionsAskedThenThePersons(t *testing.T) {
	r := recorded()
	r.Answers["gone"] = "a question the release dropped"
	m, err := manifest.Parse([]byte(manifestText))
	if err != nil {
		t.Fatal(err)
	}
	got := (&Start{Record: r}).Given(m, answer.Given{"new=1"})
	if want := (answer.Given{"name=blue-fox", "owner=Blue Corp", "new=1"}); !slices.Equal(got, want) {
		t.Errorf("Given = %q, not %q", got, want)
	}
}

func TestCurrentNamesTheRecordsReleaseBothWaysAndNothingChanged(t *testing.T) {
	u := (&Start{Record: recorded()}).Current()
	if u.From.Version != "v1.0.0" || u.To != u.From || !maps.Equal(u.From.Commits, recorded().Commits) {
		t.Errorf("Current = %+v", u)
	}
	if u.Changed == nil || len(u.Changed) != 0 || u.Conflicts == nil || len(u.Conflicts) != 0 {
		t.Errorf("Current changes %v and conflicts %v, not two empty lists", u.Changed, u.Conflicts)
	}
}

// next is the record of the release v1.1.0.
func next() project.Record {
	r := recorded()
	r.Release = "v1.1.0"
	r.Commits = map[string]string{"main": "tag main/v1.1.0", "stack/sh": "tag stack/sh/v1.1.0", "sh/extra": "tag sh/extra/v1.1.0"}
	return r
}

func files(pairs ...string) []port.File {
	var list []port.File
	for i := 0; i+1 < len(pairs); i += 2 {
		list = append(list, port.File{Path: pairs[i], Mode: 0o644, Data: []byte(pairs[i+1])})
	}
	return list
}

func TestMergeTakesTheReleasesChangeKeepsTheProjectsAndStagesTheNewRecord(t *testing.T) {
	p := clean(t, recorded())
	p.Head["release.txt"] = port.File{Path: "release.txt", Mode: 0o644, Data: []byte("v1.0.0\n")}
	s, err := Open(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.Merge(p,
		files("README.md", "# acme\n", "release.txt", "v1.0.0\n"),
		files("README.md", "# acme\n", "release.txt", "v1.1.0\n", "added.txt", "added\n"),
		next())
	if err != nil {
		t.Fatal(err)
	}
	if got := string(p.Tree["README.md"].Data); got != "# blue-fox\n" {
		t.Errorf("the project's README.md is %q", got)
	}
	if got := string(p.Tree["release.txt"].Data); got != "v1.1.0\n" {
		t.Errorf("release.txt is %q", got)
	}
	if got := p.Tree[project.RecordFile].Data; string(got) != string(recordFile(t, next())) {
		t.Errorf("the record is\n%s", got)
	}
	want := []port.Change{{Path: project.RecordFile, Kind: port.Modified}, {Path: "added.txt", Kind: port.Added}, {Path: "release.txt", Kind: port.Modified}}
	if !slices.Equal(u.Changed, want) || u.Conflicts == nil || len(u.Conflicts) != 0 {
		t.Errorf("the update changed %v, conflicts %v", u.Changed, u.Conflicts)
	}
	if u.From.Version != "v1.0.0" || u.To.Version != "v1.1.0" || !maps.Equal(u.To.Commits, next().Commits) {
		t.Errorf("the update is from %+v to %+v", u.From, u.To)
	}
}

func TestMergeKeepsAProjectsChangeToItsRecord(t *testing.T) {
	r := recorded()
	r.Template = "/home/you/elsewhere/acme"
	p := clean(t, r)
	s, err := Open(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	n := next()
	n.Template = r.Template
	if _, err := s.Merge(p, nil, nil, n); err != nil {
		t.Fatal(err)
	}
	if got := p.Tree[project.RecordFile].Data; string(got) != string(recordFile(t, n)) {
		t.Errorf("the record is\n%s", got)
	}
}

func TestMergeLeavesAConflictRefusingWithTheUpdate(t *testing.T) {
	p := clean(t, recorded())
	p.Head["release.txt"] = port.File{Path: "release.txt", Mode: 0o644, Data: []byte("project\n")}
	s, err := Open(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.Merge(p, files("release.txt", "v1.0.0\n"), files("release.txt", "v1.1.0\n"), next())
	var conflicted *Conflicted
	if !errors.As(err, &conflicted) || conflicted.Release != "v1.1.0" || !slices.Equal(conflicted.Paths, []string{"release.txt"}) {
		t.Fatalf("Merge = %v", err)
	}
	if u == nil || !slices.Equal(u.Conflicts, []port.Unmerged{{Path: "release.txt"}}) {
		t.Errorf("the update is %+v", u)
	}
	if got := string(p.Tree[project.RecordFile].Data); got != string(recordFile(t, next())) {
		t.Errorf("a conflicted update's record is\n%s", got)
	}
}

func TestMergeOfNoChangeIsEmptyListsAndAFailurePassesOn(t *testing.T) {
	r := recorded()
	p := clean(t, r)
	s, err := Open(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.Merge(p, files("a", "1\n"), files("a", "1\n"), r)
	if err != nil || u.Changed == nil || len(u.Changed) != 0 || u.Conflicts == nil {
		t.Errorf("an update changing nothing is %+v, %v", u, err)
	}
	p.Fail = errors.New("cannot merge")
	if u, err := s.Merge(p, nil, nil, next()); u != nil || err == nil || err.Error() != "cannot merge" {
		t.Errorf("a merge that fails gives %v, %v", u, err)
	}
}

func TestEachRefusalSaysWhatItIs(t *testing.T) {
	for _, c := range []struct {
		err  Error
		want string
	}{
		{&NotRepository{Why: "fatal"}, "fatal"},
		{&NoWorkTree{}, "working tree"},
		{&NoCommit{Top: "/made"}, "/made"},
		{&Dirty{Paths: []string{"a", "b"}}, "a, b"},
		{&NoRecord{File: ".itos-template.yaml"}, ".itos-template.yaml"},
		{&Answered{Name: "name"}, "name"},
		{&NoRelease{}, "no release"},
		{&Older{Recorded: "v1.1.0", Target: "v1.0.0"}, "v1.0.0 is older than v1.1.0"},
		{&Conflicted{Paths: []string{"a", "b"}}, "a, b"},
	} {
		if !strings.Contains(c.err.Error(), c.want) {
			t.Errorf("%T says %q", c.err, c.err.Error())
		}
	}
}
