package manifest

import "strings"

// Shell is the words as a POSIX shell (sh, bash, zsh) reads them back, the
// step as new prints it for the person to paste: each word quoted (Quote),
// joined by one space, so what the shell runs is the words themselves, not
// its reading of them. Our own quoting, not al.essio.dev/pkg/shellescape,
// whose Quote is the same few lines: the person's call, 2026-10-09, proved
// by a property running the real sh.
func (w Words) Shell() string {
	quoted := make([]string, len(w))
	for i, word := range w {
		quoted[i] = Quote(word)
	}
	return strings.Join(quoted, " ")
}

// Quote is word as a POSIX shell reads it back as one word: as it is when
// it is made only of ASCII letters, digits and @%+=:,./_-, which no shell
// reads specially, two single quotes when it is empty, and otherwise in
// single quotes, inside which a shell reads every byte as it is but the
// quote itself, so each ' is written '"'"': the quotes closed, a ' in
// double quotes, the quotes opened again.
func Quote(word string) string {
	if word == "" {
		return "''"
	}
	if strings.IndexFunc(word, needsQuoting) < 0 {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'"'"'`) + "'"
}

// needsQuoting is whether a shell may read r specially: anything but an
// ASCII letter, a digit and @%+=:,./_-.
func needsQuoting(r rune) bool {
	switch {
	case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
		return false
	}
	return !strings.ContainsRune("@%+=:,./_-", r)
}
