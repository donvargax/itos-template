package cli

import (
	"testing"

	"github.com/alecthomas/kong"
)

// The scenarios (@ID-CLI-04 to 09) hold the usage errors by their exit code
// and the flag they name. These hold what no scenario reads: each sentence
// whole, a switch's --no- pair, a flag that may repeat, and a switch's
// environment variable, which no scenario sets.

// line is a command line of every kind of flag Flags reads.
type line struct {
	Folder   string   `arg:"" optional:""`
	Stack    string   `placeholder:"STACK"`
	Answer   []string `sep:"none"`
	Defaults bool     `negatable:"" env:"ITOS_TEMPLATE_TEST_DEFAULTS"`
	Quiet    bool
}

// parse is args parsed as line, through Flags, and the error kong gives.
func parse(t *testing.T, args ...string) (line, error) {
	t.Helper()
	var l line
	parser, err := kong.New(&l, append(Flags(), kong.Name("itos-template"), kong.Exit(func(int) { t.Fatal("kong exited") }))...)
	if err != nil {
		t.Fatal(err)
	}
	_, err = parser.Parse(args)
	return l, err
}

func TestAFlagGivenNoValueOrAFlagForItIsRefusedNamingIt(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--stack"}, "--stack: needs a value, as --stack STACK"},
		{[]string{"--answer"}, "--answer: needs a value, as --answer ANSWER"},
		{[]string{"--stack", "--quiet"}, "--stack: needs a value, and --quiet is a flag, never its value; to give it as the value, write --stack=--quiet"},
		{[]string{"--answer", "-h"}, "--answer: needs a value, and -h is a flag, never its value; to give it as the value, write --answer=-h"},
	}
	for _, c := range cases {
		if _, err := parse(t, c.args...); err == nil || err.Error() != c.want {
			t.Errorf("%v: got %v, want %q", c.args, err, c.want)
		}
	}
}

func TestAValueGivenAsAFlagsOwnIsTaken(t *testing.T) {
	l, err := parse(t, "made", "--stack=--quiet", "--answer", "a=1", "--answer=--b")
	if err != nil {
		t.Fatal(err)
	}
	if l.Folder != "made" || l.Stack != "--quiet" || len(l.Answer) != 2 || l.Answer[0] != "a=1" || l.Answer[1] != "--b" {
		t.Errorf("got %+v", l)
	}
}

func TestASwitchGivenAValueIsRefusedNamingItAndItsPair(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--defaults=yes"}, "--defaults: a switch takes no value; give --defaults or --no-defaults alone"},
		{[]string{"--no-defaults=false"}, "--defaults: a switch takes no value; give --defaults or --no-defaults alone"},
		{[]string{"--quiet=true"}, "--quiet: a switch takes no value; give --quiet alone"},
	}
	for _, c := range cases {
		if _, err := parse(t, c.args...); err == nil || err.Error() != c.want {
			t.Errorf("%v: got %v, want %q", c.args, err, c.want)
		}
	}
}

func TestAOnceOnlyFlagGivenTwiceIsRefusedNamingIt(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--stack", "go", "--stack=python"}, "--stack: given more than once; give it once"},
		{[]string{"--defaults", "--no-defaults"}, "--defaults or --no-defaults: given more than once; give it once"},
		{[]string{"--quiet", "made", "--quiet"}, "--quiet: given more than once; give it once"},
	}
	for _, c := range cases {
		if _, err := parse(t, c.args...); err == nil || err.Error() != c.want {
			t.Errorf("%v: got %v, want %q", c.args, err, c.want)
		}
	}
}

func TestSwitchesGivenAloneAreTaken(t *testing.T) {
	l, err := parse(t, "--quiet", "--no-defaults")
	if err != nil {
		t.Fatal(err)
	}
	if !l.Quiet || l.Defaults {
		t.Errorf("got %+v", l)
	}
}

func TestASwitchTakesItsEnvironmentVariablesValue(t *testing.T) {
	for value, want := range map[string]bool{"yes": true, "false": false} {
		t.Setenv("ITOS_TEMPLATE_TEST_DEFAULTS", value)
		l, err := parse(t)
		if err != nil {
			t.Fatalf("%s: %v", value, err)
		}
		if l.Defaults != want {
			t.Errorf("%s: got %v, want %v", value, l.Defaults, want)
		}
	}
	t.Setenv("ITOS_TEMPLATE_TEST_DEFAULTS", "true")
	l, err := parse(t, "--no-defaults")
	if err != nil || l.Defaults {
		t.Errorf("--no-defaults over the variable: got %+v, %v", l, err)
	}
}
