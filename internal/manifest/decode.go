package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

// Decode reads data, a file of JSON data written as YAML, into v, strictly
// (docs/CONFIG.md rule 1, decision 22): the manifest's reader, and the
// record's once a command reads it. It refuses what JSON's data model
// cannot say, so a file from any template git can clone builds no object
// and expands nothing: a tag (a custom one, or a core one written out), an
// anchor, an alias, a merge key, a second document, a key that is not a
// string and a key given twice. It refuses a key v does not have, too.
//
// The non-specific tag ! is the one tag taken. Rule 1 refuses what JSON's
// data model cannot say, not every YAML spelling, and the reader takes
// what is only spelling, never reaching the data (quotes, comments, block
// or flow style); ! only marks a plain scalar as no other tag's, building
// no object and expanding nothing. yaml.v3 drops it as it parses, so a
// value tagged ! reads as if the ! were not there.
//
// Every problem found is returned, each a sentence naming what was found
// and its line, so an author fixes them in one go; none when v holds the
// data. The file is walked as yaml.v3's nodes before anything is decoded
// into v, so an alias is never expanded.
func Decode(data []byte, v any) []string {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return []string{"it is empty"}
		}
		return []string{err.Error()}
	}
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	strict(&doc, add)
	var next yaml.Node
	switch err := dec.Decode(&next); {
	case err == nil:
		add("line %d: a second document follows the first: a file is one document, so remove the --- and what follows it", next.Line)
	case !errors.Is(err, io.EOF):
		add("a second document follows the first: a file is one document, so remove the --- and what follows it (%v)", err)
	}
	if len(problems) > 0 {
		return problems
	}
	dec = yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// strict adds a problem for each thing of n and the nodes under it that
// JSON cannot say, in the order of the file. An alias is named and not
// followed.
func strict(n *yaml.Node, add func(string, ...any)) {
	if n.Style&yaml.TaggedStyle != 0 {
		add("line %d: the tag %s: JSON has no tags, so write the value without it", n.Line, n.Tag)
	}
	if n.Anchor != "" {
		add("line %d: the anchor &%s: JSON has no anchors or aliases, so write the value out where it is used", n.Line, n.Anchor)
	}
	switch n.Kind {
	case yaml.AliasNode:
		add("line %d: the alias *%s: JSON has no anchors or aliases, so write the value out where it is used", n.Line, n.Value)
		return
	case yaml.MappingNode:
		seen := map[string]int{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			switch {
			case k.Kind == yaml.ScalarNode && k.Tag == "!!merge":
				add("line %d: the merge key <<: JSON has no merge keys, so write the keys out in the mapping", k.Line)
			case k.Kind != yaml.ScalarNode:
				add("line %d: a key is a list or a mapping: JSON's keys are strings", k.Line)
			case k.ShortTag() != "!!str":
				add("line %d: the key %s is not a string: JSON's keys are strings, so quote it", k.Line, k.Value)
			default:
				if first, twice := seen[k.Value]; twice {
					add("line %d: the key %s is given twice, first at line %d: give it once", k.Line, k.Value, first)
				} else {
					seen[k.Value] = k.Line
				}
			}
		}
	}
	for _, c := range n.Content {
		strict(c, add)
	}
}
