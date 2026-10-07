package render

import (
	"slices"
	"testing"

	"github.com/donvargax/itos-template/internal/caseform"
	"github.com/donvargax/itos-template/internal/template/port"
	"github.com/donvargax/itos-template/internal/template/port/porttest"
)

// acmeScan looks for the literal acme-widget, with case forms, and Acme
// Corp, without.
var acmeScan = NewScan([]caseform.Words{{"acme", "widget"}}, []string{"Acme Corp"})

func TestLeftoversFindTheWordsInAnySpellingInPathsAndLines(t *testing.T) {
	files := []port.File{
		{Path: "README.md", Mode: 0o644, Data: []byte("# Acme Widget\r\nsee acmewidget.example.com\rand ACME/WIDGET, Acme.widget\nacme widgets\n")},
		{Path: "docs/acme_Widget/notes.md", Mode: 0o644, Data: []byte("by Acme Corp, not acme corp\n")},
		{Path: "logo.bin", Mode: 0o644, Data: []byte("\x00acme widget")},
		{Path: "run", Mode: porttest.Link, Data: []byte("bin/AcmeWidget")},
		{Path: "acme-tool/widget.txt", Mode: 0o644, Data: []byte("acme, widget\nacme--widget\n")},
	}
	want := []Leftover{
		{"README.md", 1, "Acme Widget"},
		{"README.md", 2, "acmewidget"},
		{"README.md", 3, "ACME/WIDGET"},
		{"README.md", 3, "Acme.widget"},
		{"README.md", 4, "acme widget"},
		{"docs/acme_Widget/notes.md", 0, "acme_Widget"},
		{"docs/acme_Widget/notes.md", 1, "Acme Corp"},
		{"run", 1, "AcmeWidget"},
	}
	if got := acmeScan.Leftovers(files); !slices.Equal(got, want) {
		t.Errorf("Leftovers =\n%v\nnot\n%v", got, want)
	}
}

// A render whose files hold the literals in their five forms keeps none of
// them: every form is replaced by the answer's.
func TestARenderOfTheFiveFormsHasNoLeftover(t *testing.T) {
	files, err := Plan(tree(), all, acme)
	if err != nil {
		t.Fatal(err)
	}
	if got := NewScan([]caseform.Words{{"acme", "widget"}}, nil).Leftovers(files); len(got) != 0 {
		t.Errorf("Leftovers = %v", got)
	}
}
