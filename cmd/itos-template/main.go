// Command itos-template makes new projects from a template that is itself a
// real, working project, and keeps them up to date with it (PLAN.md). It runs
// alone as itos-template, and as itos template, an itos extension.
//
// kong declares the command line (decision 4, docs/CLI.md): the flags and
// commands are the fields of cli, each flag with its environment variable and
// each switch with its --no- pair. The main output goes to stdout; logs,
// structured, and errors go to stderr.
//
// main only assembles: each command is a slice (decisions 16 and 17), its
// kong struct and its handler in a package of its own (newproject, check),
// and main parses the command line, runs the one it names through the UI
// the slices share (internal/cli) and exits with the code the slice
// returns. A usage error, which kong finds before any slice runs, is the
// one failure main reports itself, through that UI; internal/cli's Flags
// hold kong to the rules on flags it does not hold by itself.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/donvargax/itos-template/internal/check"
	"github.com/donvargax/itos-template/internal/cli"
	"github.com/donvargax/itos-template/internal/newproject"
	"github.com/donvargax/itos-template/internal/prompt"
	"github.com/donvargax/itos-template/internal/version"
)

// commandLine is the command line: its flags and commands.
type commandLine struct {
	// An action, not a switch: it prints and exits, so it has no --no- pair
	// and no environment variable.
	ShowVersion kong.VersionFlag `name:"version" help:"Print the version and the commit it was built from, and exit."`

	New        newproject.CLI `cmd:"" help:"Make a project from a template."`
	Check      check.CLI      `cmd:"" help:"Render every combination a template allows and run its checks."`
	Completion cli.Completion `cmd:"" help:"Print a shell completion script."`
	Complete   struct{}       `cmd:"" name:"__complete" hidden:""`
	// docs/CLI.md, rule 11: version beside --version, printing the same.
	Version struct{} `cmd:"" help:"Print the version and the commit it was built from."`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, prompt.Terminal()))
}

// run parses args and runs what they name, returning the exit code; it asks
// questions on in only when terminal says stdin and stdout are one. kong
// exits by itself, 0, after printing the help.
func run(args []string, in io.Reader, stdout, stderr io.Writer, terminal bool) int {
	slog.SetDefault(logger(stderr))
	ui := &cli.UI{In: in, Stdout: stdout, Stderr: stderr, Terminal: terminal}
	var c commandLine
	parser, err := kong.New(&c, append(cli.Flags(),
		kong.Name("itos-template"),
		kong.Description("Make projects from a template that is a real project, and keep them up to date with it."),
		kong.Writers(stdout, stderr),
		// docs/CLI.md, rule 11: itos-template <version>, then the commit.
		kong.Vars{"version": version.Text("itos-template")},
	)...)
	if err != nil {
		// A model kong refuses is a bug, reported as one (rule 31).
		return ui.Fail(err, wantsJSON(args))
	}
	if len(args) > 0 && args[0] == "__complete" {
		for _, line := range cli.Complete(parser.Model, args[1:]) {
			_, _ = fmt.Fprintln(stdout, line)
		}
		return 0
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		// With --json, the failure's object too (rule 29).
		return ui.Usage(err, wantsJSON(args))
	}
	// kong refuses a command line that names no command, so there is one.
	switch command, _, _ := strings.Cut(ctx.Command(), " "); command {
	case "new":
		return c.New.Run(ui)
	case "check":
		return c.Check.Run(ui)
	case "completion":
		return c.Completion.Run(ui)
	case "version":
		// What --version prints, as kong prints it.
		_, _ = fmt.Fprintln(stdout, parser.Model.Vars()["version"])
		return 0
	}
	// A command kong took that has no case here: a bug (rule 31).
	return ui.Fail(fmt.Errorf("no case runs the command %q", ctx.Command()), wantsJSON(args))
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
