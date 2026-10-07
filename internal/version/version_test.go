package version

import "testing"

// With nothing stamped, a build of a checkout, as go test's is, says the dev
// version; the acceptance harness always stamps, so this is where that is
// shown.
func TestVersionUnstamped(t *testing.T) {
	saved := stamp
	t.Cleanup(func() { stamp = saved })
	stamp = ""
	if got := Version(); got != Dev {
		t.Errorf("Version() with nothing stamped = %q, want %q", got, Dev)
	}
}

// The stamp, when there is one, is the version.
func TestVersionStamped(t *testing.T) {
	saved := stamp
	t.Cleanup(func() { stamp = saved })
	stamp = "1.4.0"
	if got := Version(); got != "1.4.0" {
		t.Errorf("Version() stamped 1.4.0 = %q", got)
	}
}

// A module version go install records is read without its v; none is Dev.
func TestFromModule(t *testing.T) {
	for in, want := range map[string]string{"v0.6.0": "0.6.0", "(devel)": Dev, "": Dev} {
		if got := fromModule(in); got != want {
			t.Errorf("fromModule(%q) = %q, want %q", in, got, want)
		}
	}
}
