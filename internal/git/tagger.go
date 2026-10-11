package git

import (
	"errors"
	"strings"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Tagger is the repository a release is cut in, the port.Tagger: the
// checkout or clone of a template that release runs in, read at its top,
// whatever folder of it that is. It writes nothing but the tags it is told
// to make and delete: no branch, no index, no working tree.
type Tagger struct {
	dir string // the folder it was opened in
	top string // its top, once Top found it
}

var _ port.Tagger = (*Tagger)(nil)

// OpenTagger is the repository the folder dir is in, not looked at yet.
func OpenTagger(dir string) *Tagger { return &Tagger{dir: dir} }

// Top is the repository's top, as Top gives the working folder's: a
// *port.NoRepository, with git's reason, when git finds none.
func (g *Tagger) Top() (string, error) {
	top, err := topIn(g.dir)
	var failed *Failed
	if errors.As(err, &failed) {
		return "", &port.NoRepository{Why: failed.Error()}
	}
	g.top = top
	return top, err
}

// HasOrigin is whether git's config names a URL for the remote origin.
func (g *Tagger) HasOrigin() (bool, error) {
	_, err := run(g.top, "config", "--get", "remote.origin.url")
	if exitCode(err) == 1 {
		return false, nil
	}
	return err == nil, err
}

// DefaultBranch is the branch origin's HEAD names, whatever is checked out
// (originsHead): release's root.
func (g *Tagger) DefaultBranch() (string, bool) { return originsHead(g.top) }

// Heads are the commits refs/heads/<branch> and refs/remotes/origin/<branch>
// name, each "" when there is no such ref.
func (g *Tagger) Heads(branch string) (string, string, error) {
	var heads [2]string
	for i, ref := range []string{"refs/heads/", "refs/remotes/origin/"} {
		out, err := run(g.top, "rev-parse", "--verify", "--quiet", ref+branch+"^{commit}")
		if exitCode(err) == 1 {
			continue
		}
		if err != nil {
			return "", "", err
		}
		heads[i] = strings.TrimSpace(string(out))
	}
	return heads[0], heads[1], nil
}

// IsAncestor is whether commit is of or one of its ancestors, as git
// merge-base --is-ancestor says: exit 0 it is, 1 it is not.
func (g *Tagger) IsAncestor(commit, of string) (bool, error) {
	return (&Repo{dir: g.top}).IsAncestor(commit, of)
}

// Tag makes name an annotated tag of commit with git tag -a, which refuses
// a tag there already.
func (g *Tagger) Tag(name, commit, message string) error {
	_, err := run(g.top, "tag", "-a", "-m", message, name, commit)
	return err
}

// Untag deletes the tag name with git tag -d.
func (g *Tagger) Untag(name string) error {
	_, err := run(g.top, "tag", "-d", name)
	return err
}
