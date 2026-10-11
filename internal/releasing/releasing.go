// Package releasing is a template release cut (decision 36,
// features/release.feature): one version tagged <branch>/<version> on every
// branch the template's manifest lists, the root included, at the commits
// check reads of them and only once check passes over every combination.
//
// It reaches the template's own repository, where the tags are made, only
// through a port.Tagger, and reads the template, copied from it as check
// copies the repository it runs in, through package template: its branch
// heads, its tags and its check are template's own, as new and check read
// them. Nothing is pushed (q-50): the tags stay in the repository, and
// Release.Push is the one command that publishes them all, the person's to
// run. The tags are annotated, each message naming the version (q-51).
//
// A release is for others to fetch, so the repository must have an origin,
// and every listed branch it holds a local head of must be at origin's head
// of it (q-53): a branch held only as origin's is origin's already. For the
// same reason its root is the template's default branch as origin names it,
// whatever branch the checkout is on (bug-7): the copy check reads is made
// with that root (Here.Root), never with the branch checked out. Each
// refusal comes before check runs, and a failing check makes no tag.
// Nothing is half-made: a tag git refuses midway takes back those made
// before it.
//
// It is domain (decision 17): its tests run it against fakes of the port.
// Its refusals are a sealed set (Error), plain domain types carrying no
// exit code or wording.
package releasing

import (
	"errors"
	"slices"

	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
	"github.com/donvargax/itos-template/internal/template"
	"github.com/donvargax/itos-template/internal/template/port"
)

// Here is the repository a release of version is cut in, looked at: its
// top, which the template is copied from, and its root, the branch origin's
// HEAD names, which the copy's HEAD names.
type Here struct {
	Top     string
	Root    string
	version string
	repo    port.Tagger
}

// Open judges version, refusing one that is not vX.Y.Z with an optional
// pre-release (*BadVersion), then looks at the repository r was opened in,
// refusing a folder in no repository (*NotRepository), a repository with no
// origin (*NoOrigin) and one whose origin names no default branch
// (*NoDefaultBranch), before anything of it is read.
func Open(r port.Tagger, version string) (*Here, error) {
	if !template.Cuttable(version) {
		return nil, &BadVersion{Version: version}
	}
	top, err := r.Top()
	var none *port.NoRepository
	if errors.As(err, &none) {
		return nil, &NotRepository{Why: none.Why}
	}
	if err != nil {
		return nil, err
	}
	origin, err := r.HasOrigin()
	if err != nil {
		return nil, err
	}
	if !origin {
		return nil, &NoOrigin{}
	}
	root, ok := r.DefaultBranch()
	if !ok {
		return nil, &NoDefaultBranch{}
	}
	return &Here{Top: top, Root: root, version: version, repo: r}, nil
}

// Tag is a tag of a release: its name, the branch it is on and the commit
// it names.
type Tag struct {
	Name   string `json:"name"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

// Plan is a release about to be cut: each tag it makes, in the order of the
// branches the manifest lists.
type Plan struct {
	version string
	repo    port.Tagger
	tags    []Tag
}

// Plan is the release of h's version of t, the template copied from h's
// repository, rendering its branch heads: a tag on each branch t's manifest
// lists, at the commit t renders it from (template.Head), the one check
// reads. A version a listed branch carries a tag of already is a *Used;
// a listed branch whose local head is not origin's, an *Unsynced, every
// one joined (errors.Join).
func (h *Here) Plan(t *template.Template) (*Plan, error) {
	used, err := t.Tagged(h.version)
	if err != nil {
		return nil, err
	}
	if len(used) > 0 {
		return nil, &Used{Tags: used}
	}
	var unsynced []error
	p := &Plan{version: h.version, repo: h.repo}
	for _, b := range t.Branches() {
		local, origin, err := h.repo.Heads(b)
		if err != nil {
			return nil, err
		}
		if local != "" && local != origin {
			how, err := h.how(local, origin)
			if err != nil {
				return nil, err
			}
			unsynced = append(unsynced, &Unsynced{Branch: b, How: how})
			continue
		}
		commit, err := t.Head(b)
		if err != nil {
			return nil, err
		}
		p.tags = append(p.tags, Tag{Name: template.TagOf(b, h.version), Branch: b, Commit: commit})
	}
	if len(unsynced) > 0 {
		return nil, errors.Join(unsynced...)
	}
	return p, nil
}

// how is how a branch's local head, local, differs from origin's, origin.
func (h *Here) how(local, origin string) (How, error) {
	if origin == "" {
		return Unpublished, nil
	}
	ahead, err := h.repo.IsAncestor(origin, local)
	if err != nil || ahead {
		return Ahead, err
	}
	behind, err := h.repo.IsAncestor(local, origin)
	if err != nil || behind {
		return Behind, err
	}
	return Diverged, nil
}

// Release is a release cut: its version and each tag made, in the order of
// the branches the manifest lists.
type Release struct {
	Version string `json:"version"`
	Tags    []Tag  `json:"tags"`
}

// Push is the command that publishes r, every tag in one atomic push to
// origin, each named by its whole ref so no branch or remote's name reads
// as it: the words of git push --atomic origin refs/tags/<tag>….
func (r *Release) Push() []string {
	words := []string{"git", "push", "--atomic", "origin"}
	for _, t := range r.Tags {
		words = append(words, "refs/tags/"+t.Name)
	}
	return words
}

// Cut checks t with answers as template.Check does, each result given to
// each as it is found, and only when every combination passed makes p's
// tags, annotated, each message naming the version. A combination that
// failed is a *Failed, and no tag is made. A tag the repository refuses is a
// *Refused, each tag made before it deleted first; one that cannot be is
// named in it. An error each returns stops the check and is returned.
func (p *Plan) Cut(t *template.Template, answers manifest.Answers, folders port.Folders, w project.Writer, run port.Runner, each func(template.Result) error) (*Release, error) {
	failed, all := 0, 0
	err := t.Check(answers, folders, w, run, func(r template.Result) error {
		all++
		if !r.Passed() {
			failed++
		}
		return each(r)
	})
	if err != nil {
		return nil, err
	}
	if failed > 0 {
		return nil, &Failed{Failed: failed, All: all}
	}
	message := "Template release " + p.version
	for i, tag := range p.tags {
		if err := p.repo.Tag(tag.Name, tag.Commit, message); err != nil {
			refused := &Refused{Tag: tag.Name, Err: err}
			for _, made := range slices.Backward(p.tags[:i]) {
				if p.repo.Untag(made.Name) != nil {
					refused.Left = append(refused.Left, made.Name)
				}
			}
			return nil, refused
		}
	}
	return &Release{Version: p.version, Tags: p.tags}, nil
}
