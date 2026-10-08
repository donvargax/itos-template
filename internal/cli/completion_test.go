package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

type completionModel struct {
	Verbose bool `name:"verbose" negatable:""`

	Search struct {
		Path    string `arg:""`
		Target  string `arg:"" optional:""`
		Mode    string `name:"mode" enum:"fast,slow" short:"m" required:""`
		Output  string `name:"output" short:"o"`
		Enabled bool   `name:"enabled" negatable:""`
		Secret  bool   `name:"secret" hidden:""`
	} `cmd:""`

	Inspect struct {
		Path string `arg:""`
	} `cmd:"" aliases:"i"`

	Shell struct {
		Name string `arg:"" enum:"bash,zsh,fish,powershell" required:""`
	} `cmd:""`

	Hidden struct{} `cmd:"" hidden:""`
}

func completionModelForTest(t *testing.T) *kong.Application {
	t.Helper()
	var model completionModel
	app, err := kong.New(&model, kong.Name("test"))
	if err != nil {
		t.Fatal(err)
	}
	return app.Model
}

func TestCompleteUsesKongCommandAndFlagModel(t *testing.T) {
	model := completionModelForTest(t)
	tests := []struct {
		name  string
		words []string
		want  []string
	}{
		{name: "top-level commands omit hidden commands", want: []string{"inspect", "search", "shell", ":none"}},
		{name: "command prefix", words: []string{"s"}, want: []string{"search", "shell", ":none"}},
		{name: "command flags", words: []string{"search", "src", "dst", "--mo"}, want: []string{"--mode", ":none"}},
		{name: "short flags", words: []string{"search", "src", "dst", "-m"}, want: []string{"-m", ":none"}},
		{name: "enum flag value after equals", words: []string{"search", "src", "dst", "--mode=f"}, want: []string{"fast", ":none"}},
		{name: "non-enum switch value is not treated as enum completion", words: []string{"search", "src", "dst", "--enabled=x"}, want: []string{":none"}},
		{name: "non-enum flag value in next word offers no values", words: []string{"search", "--output", ""}, want: []string{":none"}},
		{name: "enum flag value in next word", words: []string{"search", "--mode", "f"}, want: []string{"fast", ":none"}},
		{name: "flag awaiting a value", words: []string{"search", "--mode", ""}, want: []string{"fast", "slow", ":none"}},
		{name: "short flag awaiting a value", words: []string{"search", "-m", ""}, want: []string{"fast", "slow", ":none"}},
		{name: "consumed flag value leaves the positional available", words: []string{"search", "--mode", "fast", ""}, want: []string{":files"}},
		{name: "parent flag after command arguments", words: []string{"search", "src", "dst", "--verb"}, want: []string{"--verbose", ":none"}},
		{name: "negatable switches", words: []string{"search", "src", "dst", "--no-"}, want: []string{"--no-enabled", "--no-verbose", ":none"}},
		{name: "hidden flags are omitted", words: []string{"search", "src", "dst", "--sec"}, want: []string{":none"}},
		{name: "first positional argument uses files", words: []string{"search", ""}, want: []string{":files"}},
		{name: "end of options leaves a positional to files", words: []string{"search", "--", ""}, want: []string{":files"}},
		{name: "end of options keeps a dash-prefixed word positional", words: []string{"search", "--", "--mode"}, want: []string{":files"}},
		{name: "end of options after positionals has no fallback", words: []string{"search", "src", "dst", "--", ""}, want: []string{":none"}},
		{name: "positional enum values", words: []string{"shell", "p"}, want: []string{"powershell", ":none"}},
		{name: "command aliases follow the model", words: []string{"i", ""}, want: []string{":files"}},
		{name: "unknown complete flag falls through to positional", words: []string{"search", "--unknown", ""}, want: []string{":files"}},
		{name: "no positional remains", words: []string{"search", "src", "dst", ""}, want: []string{":none"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Complete(model, test.words); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Complete(%q) = %q, want %q", test.words, got, test.want)
			}
		})
	}
}

func TestCompletionRunRejectsUnsupportedShell(t *testing.T) {
	var stdout, stderr strings.Builder
	ui := &UI{Stdout: &stdout, Stderr: &stderr}
	if code := (Completion{Shell: "tcsh"}).Run(ui); code != CodeUsage {
		t.Fatalf("Run exit = %d, want %d", code, CodeUsage)
	}
	if !strings.Contains(stderr.String(), "tcsh") {
		t.Fatalf("Run error = %q, want shell name", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("Run wrote unexpected standard output %q", stdout.String())
	}
}
