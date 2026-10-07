package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos-template/internal/newproject"
	"github.com/donvargax/itos-template/internal/prompt"
)

// newCmd is itos-template new: a project made from a template's branch heads
// (decisions 1, 2, 8 to 12).
type newCmd struct {
	Template string   `arg:"" help:"The template: anything git clone takes, a path or a URL."`
	Folder   string   `arg:"" help:"The project's folder: one that does not exist, or an empty one."`
	Stack    string   `help:"The stack to render, by the name the template's manifest gives it. Asked on a terminal when not given." placeholder:"STACK"`
	Feature  []string `help:"A feature to merge onto the stack, by its name (cli) or its branch (go/cli); once for each. The features a feature needs are never added unasked." placeholder:"FEATURE" sep:"none"`
	Answer   []string `help:"An answer to one of the template's questions, as name=answer; once for each. An answer not given is asked on a terminal." placeholder:"NAME=ANSWER" sep:"none"`
	Defaults bool     `help:"Take a missing answer's default instead of asking or refusing." negatable:"" env:"ITOS_TEMPLATE_DEFAULTS"`
	JSON     bool     `name:"json" help:"Print the result as one JSON object on stdout." negatable:"" env:"ITOS_TEMPLATE_JSON"`
}

// Help is new's detail in its --help: its JSON and its exit codes
// (docs/CLI.md, rules 10 and 14).
func (c *newCmd) Help() string {
	return `Renders the template's stack branch merged with the chosen features' branches, replaces each literal its manifest (itos-template.yaml) lists by its answer, in file contents and names and in every case form, and commits the render, with its record (.itos-template.yaml), as the new git repository's first commit. Nothing is written to the folder until every check has passed.

--json prints {"schema":1,"ok":true,"folder","template","stack","features":[…],"answers":{…},"commits":{"<branch>":"<sha>"},"commit":"<sha>"}, or {"schema":1,"ok":false,"problems":[{"rule","message"}]}.

Exit codes: 0 made; 1 refused: the folder has files in it, a feature of another stack, a feature whose needed feature is not chosen, branches that do not merge cleanly; 2 a usage or manifest error: an unknown stack or feature, an answer missing (without a terminal) or not one its question takes; 3 git cannot reach the template, or cannot run; 70 an internal error.

Examples:
  itos-template new ../acme made --stack go --feature cli --answer name=blue-fox
  itos-template new https://github.com/you/template.git made --stack go --defaults

Report issues at https://github.com/donvargax/itos-template/issues.`
}

func (c *newCmd) run(in io.Reader, stdout, stderr io.Writer, terminal bool) int {
	opts := newproject.Options{
		Template: c.Template,
		Folder:   c.Folder,
		Stack:    c.Stack,
		Features: c.Feature,
		Answers:  c.Answer,
		Defaults: c.Defaults,
	}
	if terminal {
		opts.Asker = prompt.NewLines(in, stderr)
	}
	result, err := newproject.Make(opts)
	if err != nil {
		var f *newproject.Failure
		if !errors.As(err, &f) {
			f = &newproject.Failure{Code: exitInternal, Problems: []newproject.Problem{{Rule: "internal", Message: err.Error()}}}
		}
		if c.JSON {
			printJSON(stdout, stderr, failure(f.Problems))
		}
		for _, p := range f.Problems {
			fail(stderr, errors.New(p.Message))
		}
		return f.Code
	}
	if c.JSON {
		printJSON(stdout, stderr, struct {
			Schema int  `json:"schema"`
			OK     bool `json:"ok"`
			*newproject.Result
		}{1, true, result})
		return exitOK
	}
	features := "no features"
	if len(result.Features) > 0 {
		features = "the features " + strings.Join(result.Features, ", ")
	}
	_, _ = fmt.Fprintf(stdout, "Made %s from %s: the stack %s, %s.\n", result.Folder, result.Template, result.Stack, features)
	return exitOK
}

// failure is the --json object of a failure (docs/CLI.md, rule 29).
func failure(problems []newproject.Problem) any {
	return struct {
		Schema   int                  `json:"schema"`
		OK       bool                 `json:"ok"`
		Problems []newproject.Problem `json:"problems"`
	}{1, false, problems}
}

func printJSON(stdout, stderr io.Writer, v any) {
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fail(stderr, err)
	}
}
