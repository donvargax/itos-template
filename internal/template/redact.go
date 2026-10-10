package template

import (
	"strings"
	"unicode"
)

// Redact is text with the userinfo of every URL in it left out (bug-4): a
// template named https://user:token@host/path, or git's own line naming it,
// prints as https://host/path, and the rest of the text is kept byte for
// byte. A credential in the name the person gave is theirs to give git,
// never ours to print, on a terminal or in a CI log. The userinfo is left
// out, not shown as ***@, because git's own fatal line leaves it out too,
// so the two URLs one message holds read the same.
//
// It is cut by hand, by RFC 3986's rule for a URL's authority: after each
// "://", everything up to the last @ before the first /, ?, # or space, or
// the text's end. net/url is not used (the person's call, 2026-10-09): it
// refuses the malformed URLs that most need redacting, a mistyped password
// holding a bare % or a bad port, and re-escapes what it keeps. The URL
// ends at a space, as a word of a sentence does. An scp-like name
// (git@host:path) holds no "://": it carries a user, not a secret, and is
// kept as it is.
func Redact(text string) string {
	var b strings.Builder
	for {
		i := strings.Index(text, "://")
		if i < 0 {
			b.WriteString(text)
			return b.String()
		}
		b.WriteString(text[:i+len("://")])
		text = text[i+len("://"):]
		end := strings.IndexFunc(text, endsAuthority)
		if end < 0 {
			end = len(text)
		}
		text = text[strings.LastIndex(text[:end], "@")+1:]
	}
}

// endsAuthority is whether r ends a URL's authority: the start of its path,
// its query or its fragment, or a space, which ends the URL.
func endsAuthority(r rune) bool {
	return strings.ContainsRune("/?#", r) || unicode.IsSpace(r)
}
