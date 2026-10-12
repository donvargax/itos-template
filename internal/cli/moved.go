package cli

import "fmt"

// Moved is a command an earlier release had, kept only so that its old name
// exits 2 naming the command that replaced it (docs/CLI.md, rule 41). It
// runs nothing of the old command: declared hidden and passthrough, it is
// in neither the help nor completion, and it takes every word after it
// unparsed, the old command's flags included, to say which command
// replaced the one they asked for.
type Moved struct {
	Words []string `arg:"" optional:""`
}

// Gives is whether the words after the old command give flag, before any
// --.
func (m Moved) Gives(flag string) bool {
	for _, w := range m.Words {
		if w == "--" {
			return false
		}
		if w == flag {
			return true
		}
	}
	return false
}

// Run reports old, the command line an earlier release took, as a usage
// error naming now, the command that replaced it, and returns its exit
// code, 2: with withJSON the failure's object on stdout first (rule 29).
func (m Moved) Run(ui *UI, old, now string, withJSON bool) int {
	return ui.fail([]Problem{{Rule: "usage", Message: fmt.Sprintf("%s is now %s: run itos-template %s", old, now, now)}}, CodeUsage, withJSON)
}
