package template

import "testing"

// A refusal naming a stack or a feature the person gave and the template
// lacks quotes it with every character outside ASCII escaped, as
// package answer's do: Go prints a Hangul filler as itself under %q, and a
// right-to-left override reaching a terminal raw can turn the rest of the
// line around (bug-5).
func TestARefusalNamesWhatThePersonGaveEscaped(t *testing.T) {
	cases := []struct {
		err  Error
		want string
	}{
		{&UnknownStack{Name: "g\u3164o", Stacks: []string{"sh"}}, `no stack "g\u3164o"`},
		{&UnknownStack{Name: "g\u202eo", Stacks: []string{"sh"}}, `no stack "g\u202eo"`},
		{&UnknownFeature{Name: "c\u3164li", Stack: "sh"}, `no feature "c\u3164li"`},
		{&UnknownFeature{Name: "c\u202eli", Stack: "sh"}, `no feature "c\u202eli"`},
	}
	for _, c := range cases {
		if got := c.err.Error(); got != c.want {
			t.Errorf("%T reads %+q, not %+q", c.err, got, c.want)
		}
	}
}
