package template

import "testing"

func TestSystemOfTellsWindowsFromEveryOtherSystem(t *testing.T) {
	for goos, want := range map[string]System{"windows": Windows, "linux": Unix, "darwin": Unix, "freebsd": Unix} {
		if got := SystemOf(goos); got != want {
			t.Errorf("SystemOf(%q) = %v, not %v", goos, got, want)
		}
	}
}

// The token every case holds is the scenarios' fake one: none here is a
// real credential.
func TestRecordedCutsAURLsCredentialAndKeepsTheRest(t *testing.T) {
	cases := []struct{ name, given, want string }{
		{"a user and a token", "https://x-access-token:ghp_EXAMPLETOKENNOTREAL@example.invalid/acme.git", "https://example.invalid/acme.git"},
		{"a token as the user", "https://ghp_EXAMPLETOKENNOTREAL@github.com/you/template.git", "https://github.com/you/template.git"},
		{"ssh with a user", "ssh://git@host:22/you/template.git", "ssh://host:22/you/template.git"},
		{"a scheme holding a digit, +, - and .", "git+s-s.h2://you:secret@host/acme.git", "git+s-s.h2://host/acme.git"},
	}
	for _, sys := range []System{Unix, Windows} {
		for _, c := range cases {
			got, cut := Recorded(c.given, "/work", sys)
			if got != c.want || !cut {
				t.Errorf("%s on %v: Recorded = %q, %v, not %q, true", c.name, sys, got, cut, c.want)
			}
		}
	}
}

func TestRecordedKeepsAsGivenWhatHoldsNoCredentialAndNamesNoRelativePath(t *testing.T) {
	cases := []struct {
		name, given string
		sys         System
	}{
		{"a URL with no userinfo", "https://github.com/you/template.git", Unix},
		{"a URL on Windows", "https://github.com/you/template.git", Windows},
		{"a file URL", "file:///home/you/acme", Unix},
		{"a file URL on Windows", "file:///C:/Users/you/acme", Windows},
		{"an scp-like name", "git@github.com:you/template.git", Unix},
		{"an scp-like name on Windows", "git@github.com:you/template.git", Windows},
		{"an scp-like name with no user", "host:acme", Unix},
		{"an scp-like name with no host", ":acme", Unix},
		{"an absolute path", "/home/you/acme", Unix},
		{"a drive on Unix, an scp-like name there", `C:\acme`, Unix},
		{"a drive and a backslash", `C:\Users\you\acme`, Windows},
		{"a drive and a slash", "C:/Users/you/acme", Windows},
		{"a drive and no separator, relative to that drive's folder", "D:acme", Windows},
		{"a share", `\\server\share\acme`, Windows},
		{"a share written with slashes", "//server/share/acme", Windows},
	}
	for _, c := range cases {
		got, cut := Recorded(c.given, `C:\work`, c.sys)
		if c.sys == Unix {
			got, cut = Recorded(c.given, "/work", c.sys)
		}
		if got != c.given || cut {
			t.Errorf("%s: Recorded(%q) = %q, %v, not as given", c.name, c.given, got, cut)
		}
	}
}

func TestRecordedMakesARelativePathAbsoluteAsTheSystemWritesOne(t *testing.T) {
	cases := []struct {
		name, given, dir string
		sys              System
		want             string
	}{
		{"a sibling", "../acme", "/home/you/work", Unix, "/home/you/acme"},
		{"a folder in it", "acme", "/home/you", Unix, "/home/you/acme"},
		{"a dot", "./acme/", "/home/you", Unix, "/home/you/acme"},
		{"a folder in it, a colon after the /", "acme/a:b", "/home/you", Unix, "/home/you/acme/a:b"},
		{"a / before a colon", "../a:b", "/home/you/work", Unix, "/home/you/a:b"},
		{"a backslash, a name's character on Unix", `..\acme`, "/home/you", Unix, `/home/you/..\acme`},
		{"a sibling with backslashes", `..\acme`, `C:\Users\you\work`, Windows, `C:\Users\you\acme`},
		{"a sibling with slashes", "../acme", `C:\Users\you\work`, Windows, `C:\Users\you\acme`},
		{"a folder in it", "acme", `C:\Users\you`, Windows, `C:\Users\you\acme`},
		{"a dot", `.\acme\`, `C:\Users\you`, Windows, `C:\Users\you\acme`},
		{"above the drive's top", `..\..\acme`, `C:\work`, Windows, `C:\acme`},
		{"from the drive's top", `\acme`, `C:\Users\you`, Windows, `C:\acme`},
		{"from the drive's top, with a slash", "/acme", `D:\Users\you`, Windows, `D:\acme`},
		{"from the drive's top, a colon after its /", "/a:b", `C:\work`, Windows, `C:\a:b`},
		{"in a share", `..\acme`, `\\server\share\you\work`, Windows, `\\server\share\you\acme`},
		{"above a share's top", `..\..\acme`, `\\server\share\work`, Windows, `\\server\share\acme`},
		{"from a share's top", `\acme`, `\\server\share\you`, Windows, `\\server\share\acme`},
		{"from a share's top, the folder the share", `acme`, `\\server\share`, Windows, `\\server\share\acme`},
		{"a share written with slashes", "acme", "//server/share/you", Windows, `\\server\share\you\acme`},
		// check-here: the repository check runs in, a dot from its top as
		// git gives it, with / on Windows too.
		{"the folder itself", ".", "/home/you/acme", Unix, "/home/you/acme"},
		{"the folder itself, given with slashes", ".", "C:/Users/you/acme", Windows, `C:\Users\you\acme`},
		{"the folder itself, a share given with slashes", ".", "//server/share/acme", Windows, `\\server\share\acme`},
	}
	for _, c := range cases {
		got, cut := Recorded(c.given, c.dir, c.sys)
		if got != c.want || cut {
			t.Errorf("%s on %v: Recorded(%q, %q) = %q, %v, not %q", c.name, c.sys, c.given, c.dir, got, cut, c.want)
		}
	}
}

func TestVolumeIsTheDriveOrTheShareAPathStartsFrom(t *testing.T) {
	for p, want := range map[string]string{
		`C:\Users\you`:           "C:",
		"c:":                     "c:",
		`\\server\share\you`:     `\\server\share`,
		"//server/share/you":     "//server/share",
		`\\server\share`:         `\\server\share`,
		`\\server`:               `\\server`,
		`\\`:                     `\\`,
		`\\\share\you`:           `\\\share`,
		`\\server\\you`:          `\\server\`,
		`\Users\you`:             "",
		"":                       "",
		`1:\no drive is a digit`: "",
	} {
		if got := volume(p); got != want {
			t.Errorf("volume(%q) = %q, not %q", p, got, want)
		}
	}
}

func TestIsURLIsGitsRule(t *testing.T) {
	for name, want := range map[string]bool{
		"https://host/acme":   true,
		"a1+.-://host":        true,
		"1a://host":           false,
		"://host":             false,
		"a_b://host":          false,
		"a:b://host":          false,
		"/tmp/a://b":          false,
		"git@host:acme":       false,
		"../acme":             false,
		"Z://":                true,
		"https:/host/acme":    false,
		"host:path://in-path": false,
	} {
		if got := isURL(name); got != want {
			t.Errorf("isURL(%q) = %v, not %v", name, got, want)
		}
	}
}

func TestIsLetterAndIsDigitTakeASCIIsRangesEndsIncluded(t *testing.T) {
	for r, want := range map[rune]bool{'a': true, 'z': true, 'A': true, 'Z': true, '`': false, '{': false, '@': false, '[': false, '0': false, '\u00e9': false} {
		if got := isLetter(r); got != want {
			t.Errorf("isLetter(%q) = %v, not %v", r, got, want)
		}
	}
	for r, want := range map[rune]bool{'0': true, '9': true, '/': false, ':': false, 'a': false} {
		if got := isDigit(r); got != want {
			t.Errorf("isDigit(%q) = %v, not %v", r, got, want)
		}
	}
}
