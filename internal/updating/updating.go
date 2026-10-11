// Package updating is a made or adopted project brought to a newer release
// of its template (decision 3, features/update.feature): the release its
// record names is rendered again from the record's branch commits, the
// target release with the record's stack, features and answers, and git's
// merge takes the difference into the project, the old render the base
// (decision 38). The renders are package template's, made as new makes
// one; the paths the record declines are left out of both, so no update
// brings them (GitHub issue #3).
//
// It reaches the project only through a port.Project: HEAD's record read,
// the merge written to the index and the working tree, never committed and
// never moving HEAD (decision 37), a conflict left as git merge leaves one.
// It keeps no state of its own, so the person undoes an update with git
// reset --hard, which is why the project must be clean before one.
//
// It is domain (decision 17): its tests run it against fakes of the port.
// Its refusals are a sealed set (Error), plain domain types carrying no
// exit code or wording.
package updating

import (
	"errors"
	"maps"
	"slices"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/donvargax/itos-template/internal/answer"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/template"
	"github.com/donvargax/itos-template/internal/template/port"
)

// Start is the project an update starts from: its record, read from HEAD,
// and the record's file as HEAD holds it, the base the new record merges
// over.
type Start struct {
	Record project.Record
	file   []byte
}

// Open looks at the project p was opened in, and reads its record, before
// anything is rendered or written. It refuses a folder in no git
// repository (*NotRepository), one with no working tree (*NoWorkTree), a
// repository with no commit (*NoCommit), a dirty index or working tree,
// untracked files git does not ignore included (*Dirty), and a HEAD holding
// no record (*NoRecord) or one that cannot be read (project.BadRecord). An
// answer of answers to a question the record answers already is an
// *Answered, every one named: changing an answer is not update's.
func Open(p port.Project, answers answer.Given) (*Start, error) {
	state, err := p.Look()
	switch {
	case err != nil:
		return nil, err
	case !state.Repository:
		return nil, &NotRepository{Why: state.Why}
	case !state.WorkTree:
		return nil, &NoWorkTree{}
	case !state.Committed:
		return nil, &NoCommit{Top: state.Top}
	case len(state.Changed) > 0:
		return nil, &Dirty{Paths: state.Changed}
	}
	data, ok, err := p.Committed(project.RecordFile)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &NoRecord{File: project.RecordFile}
	}
	r, err := project.ReadRecord(data)
	if err != nil {
		return nil, err
	}
	var answered []error
	for _, kv := range answers {
		name, _, _ := strings.Cut(kv, "=")
		if recorded, ok := r.Answers[name]; ok {
			answered = append(answered, &Answered{Name: name, Answer: recorded})
		}
	}
	if len(answered) > 0 {
		return nil, errors.Join(answered...)
	}
	return &Start{Record: r, file: data}, nil
}

// Target has t render the release the update brings s's project to: the
// one to names, a pre-release too, else t's newest (template.UseNewest), a
// *NoRelease when it has none. It is false when that release is the one
// the record names, so there is nothing to update; an *Older when it is
// older, since update only goes forward. A record of the branch heads,
// which names no release, is behind every release.
func (s *Start) Target(t *template.Template, to string) (bool, error) {
	if to != "" {
		if err := t.UseRelease(to); err != nil {
			return false, err
		}
	} else {
		ok, err := t.UseNewest()
		if err != nil {
			return false, err
		}
		if !ok {
			return false, &NoRelease{}
		}
	}
	from := s.Record.Release
	switch {
	case from == "":
		return true, nil
	case from == t.Release():
		return false, nil
	case semver.Compare(t.Release(), from) < 0:
		return false, &Older{Recorded: from, Target: t.Release()}
	}
	return true, nil
}

// Given are the answers the target is rendered with, as a command line
// gives them: the record's answer to each question m asks, then answers,
// those the person gave, each to a question the record does not answer
// (Open refused the others). A question m asks and neither answers is
// answered as new answers one: asked on a terminal, or its default taken.
func (s *Start) Given(m *manifest.Manifest, answers answer.Given) answer.Given {
	var given answer.Given
	for _, name := range slices.Sorted(maps.Keys(s.Record.Answers)) {
		if _, asked := m.Question(name); asked {
			given = append(given, name+"="+s.Record.Answers[name])
		}
	}
	return append(given, answers...)
}

// Release is a render's release, as an update names the two it merges: its
// version, absent for the branch heads, and each branch's commit.
type Release struct {
	Version string            `json:"release,omitempty"`
	Commits map[string]string `json:"commits"`
}

// Update is what updating found and did: the release the project's record
// named (From) and the one it was brought to (To), what the merge changed
// from HEAD, sorted by path, the new record among them, and what it left
// conflicted.
type Update struct {
	From      *Release        `json:"from"`
	To        *Release        `json:"to"`
	Changed   []port.Change   `json:"changed"`
	Conflicts []port.Unmerged `json:"conflicts"`
}

// Current is the update of a project already at the target release: From
// and To the record's, and nothing changed.
func (s *Start) Current() *Update {
	r := release(s.Record)
	return &Update{From: r, To: r, Changed: []port.Change{}, Conflicts: []port.Unmerged{}}
}

// Merge merges into the project p, as Open found it in s, the change from
// base, the render its record names, to target, the target release's
// render, whose record is next: the record is merged too, HEAD's the base,
// so the project ends with next as its record, staged with the rest. Each
// path the record declines is left out of both renders, so the release
// never changes, adds or removes it and the project keeps its own there,
// or its absence; next keeps the record's list. A conflict's markers name
// each render as label does, root the template's root branch. A merge that
// leaves conflicts is a *Conflicted, the Update returned with it.
func (s *Start) Merge(p port.Project, base, target []port.File, next project.Record, root string) (*Update, error) {
	base, target = s.Record.Keep(base), s.Record.Keep(target)
	next.Declined = s.Record.Declined
	file, err := next.File()
	if err != nil {
		return nil, err
	}
	base = append(base[:len(base):len(base)], port.File{Path: project.RecordFile, Mode: 0o644, Data: s.file})
	target = append(target[:len(target):len(target)], file)
	merged, err := p.Merge(base, target, port.Labels{Base: label(s.Record, root), Target: label(next, root)})
	if err != nil {
		return nil, err
	}
	u := &Update{From: release(s.Record), To: release(next), Changed: merged.Changes, Conflicts: merged.Conflicts}
	if u.Changed == nil {
		u.Changed = []port.Change{}
	}
	if u.Conflicts == nil {
		u.Conflicts = []port.Unmerged{}
	}
	if len(u.Conflicts) > 0 {
		paths := make([]string, len(u.Conflicts))
		for i, c := range u.Conflicts {
			paths[i] = c.Path
		}
		return u, &Conflicted{Release: next.Release, Paths: paths}
	}
	return u, nil
}

// abbrev is how many hex digits of a commit a label gives, as git
// abbreviates a commit at the least.
const abbrev = 7

// label is how a conflict's markers name the render of the record r, as a
// person reads it rather than by a tree's object id: "template" and the
// release, template v1.0.0, or, for a record of the branch heads, which
// names no release, the root's commit it names, abbreviated, root the
// template's root branch.
func label(r project.Record, root string) string {
	if r.Release != "" {
		return "template " + r.Release
	}
	commit := r.Commits[root]
	return "template " + commit[:min(len(commit), abbrev)]
}

func release(r project.Record) *Release {
	return &Release{Version: r.Release, Commits: r.Commits}
}

// Error is why a project cannot be updated, a sealed set (decision 17):
// internal/cli gives each kind its exit code and its line.
//
//sumtype:decl
type Error interface {
	error
	updatingError()
}

// NotRepository is a folder in no git repository, and why, as git says.
type NotRepository struct{ Why string }

// NoWorkTree is a git repository with no working tree: a bare one, or its
// .git folder.
type NoWorkTree struct{}

// NoCommit is a repository at Top whose HEAD names no commit.
type NoCommit struct{ Top string }

// Dirty is a project whose index or working tree changes Paths from HEAD,
// an untracked file git does not ignore among them.
type Dirty struct{ Paths []string }

// NoRecord is a project whose HEAD holds no record, File.
type NoRecord struct{ File string }

// Answered is an answer given to the question Name, which the record
// answers already, as Answer.
type Answered struct{ Name, Answer string }

// NoRelease is a template with no complete stable release to update to.
type NoRelease struct{}

// Older is a target release older than the one the record names.
type Older struct{ Recorded, Target string }

// Conflicted is a merge of the release Release that left conflicts in
// Paths.
type Conflicted struct {
	Release string
	Paths   []string
}

func (*NotRepository) updatingError() {}
func (*NoWorkTree) updatingError()    {}
func (*NoCommit) updatingError()      {}
func (*Dirty) updatingError()         {}
func (*NoRecord) updatingError()      {}
func (*Answered) updatingError()      {}
func (*NoRelease) updatingError()     {}
func (*Older) updatingError()         {}
func (*Conflicted) updatingError()    {}

func (e *NotRepository) Error() string { return "in no git repository: " + e.Why }
func (*NoWorkTree) Error() string      { return "a git repository with no working tree" }
func (e *NoCommit) Error() string      { return "no commit in " + e.Top }
func (e *Dirty) Error() string         { return "changes from HEAD in " + strings.Join(e.Paths, ", ") }
func (e *NoRecord) Error() string      { return "no " + e.File + " at HEAD" }
func (e *Answered) Error() string      { return "the record answers " + e.Name + " already" }
func (*NoRelease) Error() string       { return "the template has no release" }
func (e *Older) Error() string         { return e.Target + " is older than " + e.Recorded }
func (e *Conflicted) Error() string    { return "conflicts in " + strings.Join(e.Paths, ", ") }
