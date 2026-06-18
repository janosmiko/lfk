package ui

import "testing"

func TestLogTopDefaultProfile_ValidatesAndDefaults(t *testing.T) {
	orig := ConfigLogTopDefaultProfile
	defer func() { ConfigLogTopDefaultProfile = orig }()

	cases := map[string]string{
		"traefik-json": "traefik-json",
		"json":         "json",
		"bogus":        "auto", // invalid falls back
	}
	for in, want := range cases {
		v := in
		applyConfigOptions(configFile{LogTopDefaultProfile: &v})
		if ConfigLogTopDefaultProfile != want {
			t.Errorf("input %q -> %q, want %q", in, ConfigLogTopDefaultProfile, want)
		}
	}
}
