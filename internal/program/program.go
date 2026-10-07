// Package program is the infra that runs the programs a template's checks
// name (decision 17): check's port.Runner. It imports no package of ours
// but the ports it implements.
//
// A check is a list of words run with no shell (os/exec), so it means the
// same on every system: no quoting, no pipes, no variables, no globs.
package program

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"

	"github.com/donvargax/itos-template/internal/template/port"
)

// Runner runs programs in Env, the environment each is given whole: the
// check's port.Runner.
type Runner struct {
	Env []string
}

var _ port.Runner = Runner{}

// Run runs words in dir, with no shell, its standard input empty, and
// returns what it wrote on its standard output and error, as they came, and
// whether it exited 0. A program that cannot be started fails, its output
// the reason.
func (r Runner) Run(dir string, words []string) ([]byte, bool) {
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Dir = dir
	cmd.Env = r.Env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err == nil {
		return out.Bytes(), true
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		fmt.Fprintf(&out, "cannot run %s: %v\n", words[0], err)
	}
	return out.Bytes(), false
}
