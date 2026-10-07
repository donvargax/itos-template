// Command itos-template makes new projects from a template that is itself a
// real, working project, and keeps them up to date with it (PLAN.md). It runs
// alone as itos-template, and as itos template, an itos extension.
//
// kong declares the command line (decision 4, docs/CLI.md): the flags and
// commands are the fields of cli, each flag with its environment variable and
// each switch with its --no- pair. The main output goes to stdout; logs,
// structured, and errors go to stderr.
//
// main only assembles: each command is a slice (decision 16), its kong
// struct, its use case and its output in a package of its own (newproject,
// check), and main parses the command line, runs the one it names and exits
// with the code the slice returns. A usage error, which kong finds before
// any slice runs, is the one failure main reports itself.
package main

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/donvargax/itos-template/internal/check"
	"github.com/donvargax/itos-template/internal/newproject"
	"github.com/donvargax/itos-template/internal/problem"
	"github.com/donvargax/itos-template/internal/prompt"
	"github.com/donvargax/itos-template/internal/version"
)

// cli is the command line: its flags and commands.
type cli struct {
	// An action, not a switch: it prints and exits, so it has no --no- pair
	// and no environment variable.
	Version kong.VersionFlag `help:"Print the version and exit."`

	New   newproject.Command `cmd:"" help:"Make a project from a template."`
	Check check.Command      `cmd:"" help:"Render every combination a template allows and run its checks."`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, prompt.Terminal()))
}

// run parses args and runs what they name, returning the exit code; it asks
// questions on in only when terminal says stdin and stdout are one. kong
// exits by itself, 0, after printing the help.
func run(args []string, in io.Reader, stdout, stderr io.Writer, terminal bool) int {
	slog.SetDefault(logger(stderr))
	var c cli
	parser, err := kong.New(&c,
		kong.Name("itos-template"),
		kong.Description("Make projects from a template that is a real project, and keep them up to date with it."),
		kong.Writers(stdout, stderr),
		// docs/CLI.md, rule 11: the first line is itos-template <version>.
		kong.Vars{"version": "itos-template " + version.Version()},
	)
	if err != nil {
		return problem.Internal(err).Report(stdout, stderr, false)
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		// kong's own code for a usage error is 80; docs/CLI.md's is 2. With
		// --json, the failure's object too (rule 29).
		return problem.New(problem.CodeUsage, "usage", "%v", err).Report(stdout, stderr, wantsJSON(args))
	}
	if command := strings.Fields(ctx.Command()); len(command) > 0 {
		switch command[0] {
		case "new":
			return c.New.Run(in, stdout, stderr, terminal)
		case "check":
			return c.Check.Run(stdout, stderr)
		}
	}
	if err := ctx.PrintUsage(false); err != nil {
		return problem.Internal(err).Report(stdout, stderr, false)
	}
	return 0
}

// wantsJSON is whether args ask for --json before any --, as a command line
// kong could not parse is read.
func wantsJSON(args []string) bool {
	json := false
	for _, a := range args {
		switch a {
		case "--":
			return json
		case "--json", "--json=true":
			json = true
		case "--no-json", "--json=false":
			json = false
		}
	}
	return json
}

// logger writes structured logs to w: JSON when w is not a terminal, text
// when it is, warnings and errors only, so a run is quiet by default.
func logger(w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	if f, ok := w.(*os.File); ok {
		if info, err := f.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return slog.New(slog.NewTextHandler(w, opts))
		}
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}
