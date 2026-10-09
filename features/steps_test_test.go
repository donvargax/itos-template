package features

import (
	"reflect"
	"testing"
)

func TestCommandWordsPreservesEmptyArgumentsAndWindowsPaths(t *testing.T) {
	got, err := commandWords(`__complete '' "C:\template folder\project"`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"__complete", "", `C:\template folder\project`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commandWords() = %#v, want %#v", got, want)
	}
}
