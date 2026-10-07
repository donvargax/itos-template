// Command itos-template makes new projects from a template that is itself a
// real, working project, and keeps them up to date with it (PLAN.md). It runs
// alone as itos-template, and as itos template, an itos extension.
//
// kong declares the command line (decision 4, docs/CLI.md): the flags and
// commands are the fields of cli, each flag with its environment variable and
// each switch with its --no- pair. The main output goes to stdout; logs,
// structured, and errors go to stderr.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
)

// Exit codes (docs/CLI.md, "Exit codes").
const (
	exitOK       = 0
	exitUsage    = 2
	exitInternal = 70
)

// cli is the command line: its flags and commands.
type cli struct{}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses args and runs what they name, returning the exit code. kong
// exits by itself, 0, after printing the help.
func run(args []string, stdout, stderr io.Writer) int {
	slog.SetDefault(logger(stderr))
	var c cli
	parser, err := kong.New(&c,
		kong.Name("itos-template"),
		kong.Description("Make projects from a template that is a real project, and keep them up to date with it."),
		kong.Writers(stdout, stderr),
	)
	if err != nil {
		fail(stderr, err)
		return exitInternal
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		// kong's own code for a usage error is 80; docs/CLI.md's is 2.
		fail(stderr, err)
		return exitUsage
	}
	if ctx.Command() == "" {
		if err := ctx.PrintUsage(false); err != nil {
			fail(stderr, err)
			return exitInternal
		}
	}
	return exitOK
}

// fail writes err on stderr as a line for people, itos-template: first
// (docs/CLI.md, rule 32). A stderr that cannot be written leaves nowhere else
// to say so.
func fail(stderr io.Writer, err error) {
	_, _ = fmt.Fprintf(stderr, "itos-template: %v\n", err)
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
