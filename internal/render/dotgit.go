package render

import "strings"

// This file refuses the spellings of .git that a file system resolves to the
// .git folder, so a rendered name cannot smuggle one into a checkout. It is
// written from the file systems' own naming rules, read from their vendors'
// documentation:
//
//   - NTFS and FAT compare names without regard to case, ignore trailing
//     spaces and periods, resolve 8.3 short names such as GIT~1 to their long
//     name, and read the part of a name after a colon as an alternate data
//     stream of the name before it (Microsoft, "Naming Files, Paths, and
//     Namespaces").
//   - HFS+ compares names without regard to case and ignores a fixed set of
//     zero-width and bidirectional code points (Apple, Technical Note TN1150,
//     "HFS Plus Volume Format").

// isDotGit reports whether name, a single path element, is a spelling that
// NTFS, FAT or HFS+ would resolve to the .git folder.
func isDotGit(name string) bool {
	return ntfsResolvesToDotGit(name) || hfsResolvesToDotGit(name)
}

// ntfsResolvesToDotGit reports whether NTFS or FAT would resolve name to
// .git. A colon starts an alternate data stream, so only the name before it
// is the file; trailing spaces and periods are not part of a name; and the
// result is .git in any case, or its 8.3 short name git~<digits>.
func ntfsResolvesToDotGit(name string) bool {
	base := name
	if colon := strings.IndexByte(base, ':'); colon >= 0 {
		base = base[:colon]
	}
	base = strings.TrimRight(base, ". ")
	return strings.EqualFold(base, ".git") || isDotGitShortName(base)
}

// isDotGitShortName reports whether base is an 8.3 short name NTFS would
// resolve to .git: git~ (in any case) and then a run of digits, as a short
// name numbers its collisions (GIT~1, GIT~2, …).
func isDotGitShortName(base string) bool {
	if len(base) <= len("git~") || !strings.EqualFold(base[:len("git~")], "git~") {
		return false
	}
	for _, c := range base[len("git~"):] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// hfsResolvesToDotGit reports whether HFS+ would resolve name to .git: the
// code points it ignores dropped, the rest is .git in any case.
func hfsResolvesToDotGit(name string) bool {
	var kept strings.Builder
	for _, r := range name {
		if !hfsIgnores[r] {
			kept.WriteRune(r)
		}
	}
	return strings.EqualFold(kept.String(), ".git")
}

// hfsIgnores is the set of code points HFS+ drops when it compares a name, so
// .git can be spelled with any of them between its characters and still open
// the .git folder. The set is the one Apple's Technical Note TN1150 lists as
// ignored by the HFS+ case-folding comparison.
var hfsIgnores = map[rune]bool{
	0x200c: true, // zero width non-joiner
	0x200d: true, // zero width joiner
	0x200e: true, // left-to-right mark
	0x200f: true, // right-to-left mark
	0x202a: true, // left-to-right embedding
	0x202b: true, // right-to-left embedding
	0x202c: true, // pop directional formatting
	0x202d: true, // left-to-right override
	0x202e: true, // right-to-left override
	0x206a: true, // inhibit symmetric swapping
	0x206b: true, // activate symmetric swapping
	0x206c: true, // inhibit arabic form shaping
	0x206d: true, // activate arabic form shaping
	0x206e: true, // national digit shapes
	0x206f: true, // nominal digit shapes
	0xfeff: true, // zero width no-break space
}
