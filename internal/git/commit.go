package git

import (
	"errors"
	"os"
	"strings"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Committer is the git that makes a project's folder a repository and
// commits the render as its first commit: the project's port.Committer.
type Committer struct{}

var _ port.Committer = Committer{}

// Identity is nil when git knows who commits, else a *NoIdentity, or a
// *Missing when git cannot be run. It asks git in a temporary folder, a
// repository of no one's, so only the person's config and environment
// answer, as they do for a new project's first commit.
func (Committer) Identity() error {
	for _, v := range []string{"GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT"} {
		_, err := run(os.TempDir(), "var", v)
		var failed *Failed
		if errors.As(err, &failed) {
			return &NoIdentity{Err: failed}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Commit makes folder a git repository and commits everything in it, the
// files of executables recorded executable, as its first commit by by, or
// by whoever git's config names when by is nil. A commit git refuses is a
// *port.Refused.
func (Committer) Commit(folder, message string, executables []string, by *port.Identity) (string, error) {
	var env []string
	if by != nil {
		env = []string{
			"GIT_AUTHOR_NAME=" + by.Name, "GIT_AUTHOR_EMAIL=" + by.Email,
			"GIT_COMMITTER_NAME=" + by.Name, "GIT_COMMITTER_EMAIL=" + by.Email,
		}
	}
	g := command{dir: folder, env: env}
	steps := [][]string{
		{"init", "-q"},
		// The render is committed as written, whatever core.autocrlf says, and
		// every file of it, whatever its .gitignore leaves out: the template
		// holds them.
		{"-c", "core.autocrlf=false", "-c", "core.safecrlf=false", "add", "--all", "--force", "--", "."},
	}
	if len(executables) > 0 {
		// Where the file system has no execute bit (windows), git takes it from
		// here, as the template records it.
		steps = append(steps, append([]string{"update-index", "--chmod=+x", "--"}, executables...))
	}
	for _, args := range steps {
		if _, err := g.output(args...); err != nil {
			return "", err
		}
	}
	// git cleans the message as it cleans any given with -m, whatever the
	// person's commit.cleanup says: set to strip, it would drop each line
	// starting with #, and the message the template's commit rules judge
	// would differ from one machine to another (bug-3).
	if _, err := g.output("commit", "-q", "--cleanup=whitespace", "-m", message); err != nil {
		return "", &port.Refused{Err: err}
	}
	sha, err := g.output("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(sha)), nil
}
