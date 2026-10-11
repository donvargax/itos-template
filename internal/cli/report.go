package cli

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/template"
)

// check's report, in the format docs/manifest.md gives it ("What check
// reports"): what check prints, and release, which runs check's own
// domain code before it tags, prints the same.

// WarnUnscanned warns on stderr when no check of m is marked as scanning
// the renders for credentials.
func (u *UI) WarnUnscanned(m *manifest.Manifest) {
	if !m.Scans(manifest.Credentials) {
		u.Line("warning: no check of the template is marked as scanning its renders for credentials: mark the one that does with scans: [" + manifest.Credentials + "], in a check's long form (docs/manifest.md)")
	}
}

// ResultCode is the exit code r makes a check end with: 0 when it passed,
// else 1, or 3 or 70 when the environment or a defect of ours stopped its
// render.
func ResultCode(r template.Result) int {
	if r.Passed() {
		return 0
	}
	if r.Err != nil {
		if code := Code(r.Err); code == CodeEnvironment || code == CodeInternal {
			return code
		}
	}
	return CodeRefused
}

// Summary is the report's end: an empty line and the count of the
// combinations that passed of all.
func Summary(passed, all int) string {
	noun := "combinations"
	if all == 1 {
		noun = "combination"
	}
	return fmt.Sprintf("\n%d of %d %s passed.\n", passed, all, noun)
}

// Block is a combination's block of the report: its name and whether it
// passed, then either "not rendered" and why, or each literal its render
// kept and each check, passed, failed or skipped, a failed one followed by
// its output, each line of it after "    |". A leftover is the text found,
// in double quotes, then where: " at " and the path, a colon and the line,
// or " in the path " and the path, written as a check's word is.
func Block(r template.Result) string {
	var b strings.Builder
	status := "passed"
	if !r.Passed() {
		status = "failed"
	}
	fmt.Fprintf(&b, "%s: %s\n", r.Combination.Name(), status)
	if r.Err != nil {
		b.WriteString("  not rendered\n")
		writeLines(&b, template.Lines(Message(r.Err)))
	}
	for _, l := range r.Leftovers {
		path := manifest.Words{l.Path}.String()
		if l.Line == 0 {
			fmt.Fprintf(&b, "  leftover: %q in the path %s\n", l.Text, path)
		} else {
			fmt.Fprintf(&b, "  leftover: %q at %s:%d\n", l.Text, path, l.Line)
		}
	}
	for _, c := range r.Checks {
		switch c.Status {
		case template.Passed:
			fmt.Fprintf(&b, "  passed: %s\n", c.Check)
		case template.Failed:
			fmt.Fprintf(&b, "  failed: %s\n", c.Check)
			writeLines(&b, c.Lines())
		case template.Skipped:
			fmt.Fprintf(&b, "  skipped: %s\n", c.Check)
		}
	}
	return b.String()
}

// writeLines writes each line after "    |", a space before the line when
// it is not empty.
func writeLines(b *strings.Builder, lines []string) {
	for _, line := range lines {
		if line == "" {
			b.WriteString("    |\n")
			continue
		}
		b.WriteString("    | " + line + "\n")
	}
}
