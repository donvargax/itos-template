package newproject

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/donvargax/itos-template/internal/cli"
	"github.com/donvargax/itos-template/internal/manifest"
	"github.com/donvargax/itos-template/internal/project"
)

// shown is what show writes for p and steps: its exit code, stdout and
// stderr.
func shown(p *project.Project, steps []manifest.Words, withJSON bool) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := show(&cli.UI{Stdout: &stdout, Stderr: &stderr}, p, steps, withJSON)
	return code, stdout.String(), stderr.String()
}

func made(features ...string) *project.Project {
	return &project.Project{Folder: "made", Record: project.Record{Template: "../acme", Stack: "go", Features: features}, Commit: "c0ffee"}
}

func TestShowSaysWhatWasMadeThenTheSetupStepsToRun(t *testing.T) {
	steps := []manifest.Words{{"itos", "init", "--agent-rules"}, {"sh", "-c", "go mod download && go vet ./..."}}
	code, stdout, stderr := shown(made(), steps, false)
	want := "Made made from ../acme: the stack go, no features.\n" +
		"itos-template ran none of the template's setup steps; run them in made:\n" +
		"itos init --agent-rules\n" +
		"sh -c 'go mod download && go vet ./...'\n"
	if code != 0 || stdout != want || stderr != "" {
		t.Errorf("show = %d\n%s---\n%s", code, stdout, stderr)
	}
}

func TestShowNamesTheFeaturesMade(t *testing.T) {
	for features, want := range map[string][]string{
		"Made made from ../acme: the stack go, the features cli.\n":      {"cli"},
		"Made made from ../acme: the stack go, the features cli, web.\n": {"cli", "web"},
	} {
		if _, stdout, _ := shown(made(want...), nil, false); stdout != features {
			t.Errorf("show printed %q, not %q", stdout, features)
		}
	}
}

func TestShowWithJSONPrintsTheObjectAloneAndTheStepsOnTheErrorOutput(t *testing.T) {
	code, stdout, stderr := shown(made("cli"), []manifest.Words{{"go", "mod", "download"}}, true)
	var got struct {
		Schema   int      `json:"schema"`
		OK       bool     `json:"ok"`
		Folder   string   `json:"folder"`
		Features []string `json:"features"`
		Commit   string   `json:"commit"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout)
	}
	if code != 0 || got.Schema != 1 || !got.OK || got.Folder != "made" || len(got.Features) != 1 || got.Commit != "c0ffee" {
		t.Errorf("show = %d, %+v", code, got)
	}
	if want := "itos-template ran none of the template's setup steps; run them in made:\ngo mod download\n"; stderr != want {
		t.Errorf("the error output is %q, not %q", stderr, want)
	}
}

func TestShowPrintsNothingAboutSetupWhenThereIsNoStep(t *testing.T) {
	for _, withJSON := range []bool{false, true} {
		_, stdout, stderr := shown(made(), nil, withJSON)
		if bytes.Contains([]byte(stdout+stderr), []byte("setup")) {
			t.Errorf("with --json %v, show printed\n%s---\n%s", withJSON, stdout, stderr)
		}
	}
}

// A credential cut from the template's URL is said on the error output
// alone, naming git's credential helper.
func TestCredentialLeftOutSaysSoOnTheErrorOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	credentialLeftOut(&cli.UI{Stdout: &stdout, Stderr: &stderr})("https://example.invalid/acme.git")
	want := "itos-template: the record names the template https://example.invalid/acme.git, the credential its URL held left out: itos-template update will reach the template through git's credential helper (git help credentials)\n"
	if stdout.String() != "" || stderr.String() != want {
		t.Errorf("credentialLeftOut printed\n%s---\n%s", stdout.String(), stderr.String())
	}
}
