package caseform

import (
	"slices"
	"testing"
)

func TestForms(t *testing.T) {
	cases := map[string][]string{
		"acme-widget":    {"acme-widget", "acme_widget", "acmeWidget", "AcmeWidget", "ACME_WIDGET"},
		"blue-fox":       {"blue-fox", "blue_fox", "blueFox", "BlueFox", "BLUE_FOX"},
		"my-http-server": {"my-http-server", "my_http_server", "myHttpServer", "MyHttpServer", "MY_HTTP_SERVER"},
		"acme-v2":        {"acme-v2", "acme_v2", "acmeV2", "AcmeV2", "ACME_V2"},
		"2fa-code":       {"2fa-code", "2fa_code", "2faCode", "2faCode", "2FA_CODE"},
		"1-2":            {"1-2", "1_2", "12", "12", "1_2"},
		"acme":           {"acme", "acme", "acme", "Acme", "ACME"},
	}
	for kebab, want := range cases {
		words, err := Parse(kebab)
		if err != nil {
			t.Fatalf("Parse(%q): %v", kebab, err)
		}
		if got := words.Forms(); !slices.Equal(got, want) {
			t.Errorf("Parse(%q).Forms() = %q, want %q", kebab, got, want)
		}
	}
}

// The words are never guessed: anything but lowercase words joined by
// single dashes is refused, not split.
func TestParseRefusesWhatIsNotKebab(t *testing.T) {
	for _, s := range []string{"", "Blue_Fox", "blueFox", "blue_fox", "blue--fox", "-blue", "blue-", "blue fox", "blué"} {
		if words, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) = %q, want an error", s, words)
		}
	}
}
