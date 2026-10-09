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

func TestCompleteSingleCommandAndFlagWithoutShortName(t *testing.T) {
	var model struct {
		Only struct {
			Path string `arg:""`
			Name string `name:"name"`
		} `cmd:""`
	}
	app, err := kong.New(&model, kong.Name("test"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := Complete(app.Model, nil), []string{"only", ":none"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Complete(nil) = %q, want %q", got, want)
	}
	if got, want := Complete(app.Model, []string{"only", "-"}), []string{"--help", "--name", "-h", ":none"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Complete(only, -) = %q, want %q", got, want)
	}
}

func TestCompleteRecognizesRuneOneShortFlagFromKongModel(t *testing.T) {
	text := reflect.TypeOf("")
	command := reflect.StructOf([]reflect.StructField{
		{Name: "Mode", Type: text, Tag: `enum:"fast,slow" name:"mode" short:"\x01" required:""`},
	})
	modelType := reflect.StructOf([]reflect.StructField{
		{Name: "Search", Type: command, Tag: `cmd:""`},
	})
	app, err := kong.New(reflect.New(modelType).Interface(), kong.Name("test"))
	if err != nil {
		t.Fatalf("build Kong model with rune-one short flag: %v", err)
	}
	if got := app.Model.Node.Children[0].Flags[0].Short; got != 1 {
		t.Fatalf("Kong model short flag = %U, want U+0001", got)
	}

	words := []string{"search", "-" + string(rune(1)), "f"}
	if got, want := Complete(app.Model, words), []string{"fast", ":none"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Complete(%q) = %q, want %q", words, got, want)
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

func TestCompletionScriptsNameInstallLocations(t *testing.T) {
	for shell, location := range map[string]string{
		"bash":       "~/.bashrc",
		"zsh":        "~/.zshrc",
		"fish":       "~/.config/fish/completions/itos-template.fish",
		"powershell": "$PROFILE",
	} {
		t.Run(shell, func(t *testing.T) {
			if script := Scripts()[shell]; !strings.Contains(script, location) {
				t.Errorf("completion script does not name install location %q", location)
			}
		})
	}
}
