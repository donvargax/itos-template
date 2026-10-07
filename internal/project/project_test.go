package project

import (
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// The record is read as it is written: update and adopt read the type new
// writes.
func TestARecordReadsAsItIsWritten(t *testing.T) {
	r := Record{
		Version:  RecordVersion,
		Template: "../acme",
		Stack:    "go",
		Features: []string{"cli"},
		Answers:  map[string]string{"name": "blue-fox", "module": "example.com/blue/fox"},
		Commits:  map[string]string{"main": "9a2e", "stack/go": "7b44", "go/cli": "3f1c"},
	}
	data, err := r.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, recordHeader+"version: 1\ntemplate: ../acme\nstack: go\nfeatures:\n  - cli\nanswers:\n") {
		t.Errorf("the record is\n%s", text)
	}
	var read Record
	if err := yaml.Unmarshal(data, &read); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read, r) {
		t.Errorf("read %+v, wrote %+v", read, r)
	}
}
