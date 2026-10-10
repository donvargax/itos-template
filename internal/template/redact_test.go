package template

import (
	"strings"
	"testing"
	"unicode"

	"pgregory.net/rapid"
)

// The token every case holds is the scenarios' fake one: none here is a
// real credential.
func TestRedactLeavesOutAURLsUserinfo(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"a user and a token", "https://x-access-token:ghp_EXAMPLETOKENNOTREAL@127.0.0.1:1/acme.git", "https://127.0.0.1:1/acme.git"},
		{"a user alone, a token as GitHub takes one", "https://ghp_EXAMPLETOKENNOTREAL@github.com/you/template.git", "https://github.com/you/template.git"},
		{"a password holding @, : and %", "https://you:p@ss:w%rd@host/acme.git", "https://host/acme.git"},
		{"a bad port", "https://you:secret@host:9x9/acme.git", "https://host:9x9/acme.git"},
		{"no path", "https://you:secret@host", "https://host"},
		{"a query after the host", "https://you:secret@host?ref=a@b", "https://host?ref=a@b"},
		{"a fragment after the host", "https://you:secret@host#a@b", "https://host#a@b"},
		{"ssh", "ssh://git@host:22/you/template.git", "ssh://host:22/you/template.git"},
		{"git", "git://you@host/template.git", "git://host/template.git"},
		{"a URL nested in another's query", "https://host/?from=https://you:secret@other/x", "https://host/?from=https://other/x"},
		{
			"two URLs in one text, as template-unreachable has them",
			"git cannot reach the template https://you:secret@127.0.0.1:1/acme.git: fatal: unable to access 'https://you:secret@127.0.0.1:1/acme.git/': refused",
			"git cannot reach the template https://127.0.0.1:1/acme.git: fatal: unable to access 'https://127.0.0.1:1/acme.git/': refused",
		},
		{"a URL its text starts with, its scheme cut off", "://you:secret@host/acme.git", "://host/acme.git"},
		{"a URL with no userinfo", "https://github.com/you/template.git", "https://github.com/you/template.git"},
		{"an @ in the path", "https://host/@scope/pkg", "https://host/@scope/pkg"},
		{"an @ in a path after an empty authority", "file:///home/you@work/template", "file:///home/you@work/template"},
		{"an @ after the space ending a URL", "the template https://host: git says you@host", "the template https://host: git says you@host"},
		{"an @ after a space outside ASCII ending a URL", "https://host\u00a0you@host", "https://host\u00a0you@host"},
		{"no URL at all", "the template ../acme has no itos-template.yaml", "the template ../acme has no itos-template.yaml"},
		{"an scp-like name, a user and no secret", "git cannot reach the template git@github.com:you/template.git", "git cannot reach the template git@github.com:you/template.git"},
		{"nothing", "", ""},
	}
	for _, c := range cases {
		if got := Redact(c.text); got != c.want {
			t.Errorf("%s: Redact(%q) = %q, not %q", c.name, c.text, got, c.want)
		}
	}
}

// Whatever a URL's userinfo holds but the bytes that end its authority, the
// redaction leaves all of it out and keeps what follows its last @, the
// host and the path, unchanged, whatever bytes the path holds.
func TestRedactNeverKeepsTheUserinfo(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		userinfo := anyBytes(1).Filter(func(s string) bool {
			return !strings.ContainsAny(s, "/?#") && strings.IndexFunc(s, unicode.IsSpace) < 0
		}).Draw(t, "userinfo")
		path := anyBytes(0).Filter(func(s string) bool {
			return !strings.Contains(s, "://")
		}).Draw(t, "path")
		kept := "127.0.0.1:1/" + path
		got := Redact("https://" + userinfo + "@" + kept)
		if got != "https://"+kept {
			t.Fatalf("Redact left the userinfo %q as %q", userinfo, got)
		}
	})
}

// anyBytes are strings of any bytes, valid UTF-8 or not, at least least
// of them.
func anyBytes(least int) *rapid.Generator[string] {
	return rapid.Map(rapid.SliceOfN(rapid.Byte(), least, 40), func(b []byte) string { return string(b) })
}
