package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The logs are held here: a run is quiet by default, and nothing in the
// template logs below a warning yet, so no scenario sees a log line, nor
// stderr on a terminal (docs/template-contents.md, CLI).

// A warning to a writer that is not a terminal is one JSON object, with its
// level, its message and its attributes.
func TestLoggerWritesAWarningAsOneJSONObject(t *testing.T) {
	var buf bytes.Buffer
	logger(&buf).Warn("could not remove a folder", "path", "/tmp/x", "tries", 3)

	var line map[string]any
	dec := json.NewDecoder(&buf)
	if err := dec.Decode(&line); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if dec.More() {
		t.Fatalf("more than one object: %q", buf.String())
	}
	want := map[string]any{"level": "WARN", "msg": "could not remove a folder", "path": "/tmp/x", "tries": 3.0}
	for k, v := range want {
		if line[k] != v {
			t.Errorf("%s = %v, want %v", k, line[k], v)
		}
	}
}

// An error is logged too.
func TestLoggerWritesAnError(t *testing.T) {
	var buf bytes.Buffer
	logger(&buf).Error("broken")
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil || line["level"] != "ERROR" {
		t.Errorf("logged %q", buf.String())
	}
}

// Below a warning, nothing: a run is quiet by default.
func TestLoggerWritesNothingBelowAWarning(t *testing.T) {
	var buf bytes.Buffer
	l := logger(&buf)
	l.Info("starting")
	l.Debug("details")
	if buf.Len() != 0 {
		t.Errorf("logged %q", buf.String())
	}
}

// A terminal, a character device as os.DevNull is, gets text.
func TestLoggerWritesTextToATerminal(t *testing.T) {
	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if h := logger(f).Handler(); !isText(h) {
		t.Errorf("handler %T, want *slog.TextHandler", h)
	}
}

// A file that is not a terminal, as stderr redirected to one, gets JSON.
func TestLoggerWritesJSONToAFile(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "log"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if h := logger(f).Handler(); isText(h) {
		t.Errorf("handler %T, want *slog.JSONHandler", h)
	}
}

// isText is whether h is the text handler, and not the JSON one.
func isText(h slog.Handler) bool {
	_, ok := h.(*slog.TextHandler)
	return ok
}

// A command line kong could not parse still asks for --json, or not: the
// last of --json and --no-json before any -- decides, and none means no. No
// scenario gives a usage error every one of these ways.
func TestWantsJSONReadsTheLastJSONFlagBeforeDashDash(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"new", "--bogus"}, false},
		{[]string{"new", "--json"}, true},
		{[]string{"--json=true", "new"}, true},
		{[]string{"--json", "--no-json"}, false},
		{[]string{"--json", "--json=false"}, false},
		{[]string{"--no-json", "--json"}, true},
		{[]string{"--", "--json"}, false},
		{[]string{"--json", "--", "--no-json"}, true},
	} {
		if got := wantsJSON(c.args); got != c.want {
			t.Errorf("wantsJSON(%q) = %v, want %v", c.args, got, c.want)
		}
	}
}

func TestRunWithNoArgumentsReturnsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, strings.NewReader(""), &stdout, &stderr, false); code != 2 {
		t.Fatalf("run with no arguments returned %d, want usage exit 2", code)
	}
	if stderr.Len() == 0 {
		t.Fatal("run with no arguments did not explain the usage error")
	}
}
