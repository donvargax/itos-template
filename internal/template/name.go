package template

import (
	"path"
	"strings"
)

// System is how the machine new runs on writes a path, which decides how a
// template's name is told a path and how a relative one is made absolute:
// Windows, with its drives (C:\), its shares (\\server\share) and \ beside /,
// or Unix, every other system, with / alone. It is a value (decision 26), so
// the tests judge both on any machine; only a thin caller reads it from
// runtime.GOOS, through SystemOf.
type System int

const (
	// Unix is every system but Windows: a path's one separator is /.
	Unix System = iota
	// Windows writes a path with \ or /, from a drive or a share.
	Windows
)

// SystemOf is the System of the GOOS goos.
func SystemOf(goos string) System {
	if goos == "windows" {
		return Windows
	}
	return Unix
}

// Recorded is the template named name as a made project records it, new run
// in the folder dir, an absolute path, on sys (record-name): a name update
// can reach again from anywhere on the machine, holding no credential. cut
// is whether a credential was left out.
//
// Each kind of name is told as git tells it, so what is recorded names what
// git cloned. A URL (scheme://…) is kept with its userinfo cut by Redact,
// bug-4's rule: the credential was the person's to give git, and update
// reaches the template through git's credential helper (decision 9). A path
// relative to dir is made absolute against it, written as sys writes a path.
// An absolute path is kept as given, and so is an scp-like name
// (git@host:path), which carries a user, never a secret.
func Recorded(name, dir string, sys System) (recorded string, cut bool) {
	switch {
	case isURL(name):
		recorded = Redact(name)
		return recorded, recorded != name
	case sys.isPath(name):
		return sys.absolute(name, dir), false
	default:
		return name, false
	}
}

// isURL is whether git reads name as a URL (git's is_url): a scheme, a
// letter then letters, digits, +, - or ., followed by ://.
func isURL(name string) bool {
	scheme, _, found := strings.Cut(name, "://")
	if !found || scheme == "" {
		return false
	}
	for i, r := range scheme {
		if !isLetter(r) && (i == 0 || !isDigit(r) && !strings.ContainsRune("+-.", r)) {
			return false
		}
	}
	return true
}

func isLetter(r rune) bool { return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' }
func isDigit(r rune) bool  { return '0' <= r && r <= '9' }

// isPath is whether git, on s, reads name, no URL, as a local path rather
// than an scp-like host:path (git's url_is_local_not_ssh): it holds no
// colon, or a / before its first, or on Windows it starts with a drive.
func (s System) isPath(name string) bool {
	colon := strings.IndexByte(name, ':')
	slash := strings.IndexByte(name, '/')
	return colon < 0 || slash >= 0 && slash < colon || s == Windows && hasDrive(name)
}

// absolute is the path name, relative to the folder dir or already
// absolute, as an absolute path written as s writes one.
//
// On Unix a path starting with / is absolute; any other is joined to dir,
// cleaned of its . and .. as go's filepath.Abs cleans one.
//
// On Windows a share (\\server\share\acme) is absolute, and so is a path
// starting with a drive: C:\acme, or C:acme, relative to that drive's own
// current folder, which new cannot know, and git resolves itself. A path
// starting with one separator (\acme) starts at dir's drive or share; any
// other is joined to dir. The path so made is cleaned and written with \,
// as Windows writes it, whichever separators the name was given with.
func (s System) absolute(name, dir string) string {
	if s == Unix {
		if strings.HasPrefix(name, "/") {
			return name
		}
		return path.Join(dir, name)
	}
	switch {
	case isShare(name) || hasDrive(name):
		return name
	case name != "" && isSeparator(name[0]):
		return windowsPath(volume(dir), name)
	default:
		v := volume(dir)
		return windowsPath(v, dir[len(v):]+`\`+name)
	}
}

// windowsPath is the path from the volume v, rest, which starts with a
// separator, cleaned and written with \, the volume's too.
func windowsPath(v, rest string) string {
	return strings.ReplaceAll(v+path.Clean(strings.ReplaceAll(rest, `\`, "/")), "/", `\`)
}

// volume is the drive (C:) or the share (\\server\share) a Windows path
// starts from, or "" for one that starts from neither.
func volume(p string) string {
	if hasDrive(p) {
		return p[:2]
	}
	if !isShare(p) {
		return ""
	}
	server := strings.IndexAny(p[2:], `\/`)
	if server < 0 {
		return p
	}
	share := strings.IndexAny(p[2+server+1:], `\/`)
	if share < 0 {
		return p
	}
	return p[:2+server+1+share]
}

// hasDrive is whether a Windows path starts with a drive: a letter and a
// colon (git's has_dos_drive_prefix).
func hasDrive(p string) bool { return len(p) >= 2 && isLetter(rune(p[0])) && p[1] == ':' }

// isShare is whether a Windows path starts with two separators, a share's
// \\server\share.
func isShare(p string) bool { return len(p) >= 2 && isSeparator(p[0]) && isSeparator(p[1]) }

func isSeparator(c byte) bool { return c == '\\' || c == '/' }
