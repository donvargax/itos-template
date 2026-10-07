// Package release reads what the release cut counts (T-5): a commit's
// Conventional Commits type, whether it is marked as breaking, which tag
// names the last release, and what moved beneath the binary between two
// commits (Build and Moved, decision 23). tools/bin/release-version computes
// the next version with it, and tools/bin/release-notes picks the range it
// describes with it, so both read a commit and pick the last release from
// one copy.
//
// Harvested from itos's internal/release (its releasable.go, T-088 and bug
// 20), the part its release tools import: a module cannot import another's
// internal package. A release is the highest tag vX.Y.Z, three numbers and
// nothing else; a commit the cut counts is a feat, a fix, or one of any type
// marked as breaking. Build and Moved are this repository's own (T-15): with
// no such commit, a binary built from other modules or another toolchain
// still cuts a patch.
package release

import (
	"regexp"
	"strings"
)

var (
	// typed is a Conventional Commits header's type, scope and !.
	typed = regexp.MustCompile(`^([a-zA-Z]+)(\([^)]*\))?(!)?: `)
	// releaseTag is a release's tag, vX.Y.Z, and its three numbers.
	releaseTag = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)
)

// Type is the Conventional Commits type of a commit's header, "" when the
// header has none.
func Type(message string) string {
	header, _, _ := strings.Cut(strings.TrimSpace(message), "\n")
	if t := typed.FindStringSubmatch(header); t != nil {
		return t[1]
	}
	return ""
}

// Breaking says whether a commit's message is marked as breaking: a !
// before its header's colon, or a BREAKING-CHANGE: or BREAKING CHANGE:
// footer in its last paragraph.
func Breaking(message string) bool {
	header, rest, _ := strings.Cut(strings.TrimSpace(message), "\n")
	if t := typed.FindStringSubmatch(header); t != nil && t[3] == "!" {
		return true
	}
	if strings.TrimSpace(rest) == "" {
		return false
	}
	paragraphs := strings.Split(strings.TrimSpace(rest), "\n\n")
	for _, line := range strings.Split(paragraphs[len(paragraphs)-1], "\n") {
		if strings.HasPrefix(line, "BREAKING-CHANGE:") || strings.HasPrefix(line, "BREAKING CHANGE:") {
			return true
		}
	}
	return false
}

// IsTag says whether the tag names a release: v followed by three
// dot-separated numbers and nothing else. A prerelease is never cut, so a tag
// with a prerelease or build metadata names none; a number with leading zeros
// counts.
func IsTag(tag string) bool { return releaseTag.MatchString(tag) }

// Newest is the highest of the tags that name a release, by their numbers,
// "" when none does. Two spellings of one version (v1.0.0, v01.0.0) go to the
// shorter, then the later by name, so the pick does not hang on the order the
// tags are listed in.
func Newest(tags []string) string {
	newest := ""
	for _, t := range tags {
		if IsTag(t) && (newest == "" || compareTags(t, newest) > 0) {
			newest = t
		}
	}
	return newest
}

// compareTags orders two release tags by their numbers, each compared as a
// number however long, then a tie as Newest breaks it.
func compareTags(a, b string) int {
	x, y := releaseTag.FindStringSubmatch(a), releaseTag.FindStringSubmatch(b)
	for i := 1; i <= 3; i++ {
		if c := compareNumbers(x[i], y[i]); c != 0 {
			return c
		}
	}
	if len(a) != len(b) {
		return len(b) - len(a)
	}
	return strings.Compare(a, b)
}

// compareNumbers orders two runs of digits by the numbers they write.
func compareNumbers(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}
