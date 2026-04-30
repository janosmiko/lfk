package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLoadConfig_InformerCache verifies the bool-or-string union for the
// informer_cache key. The legacy `true`/`false` form must still parse so
// users who flipped the original boolean knob keep working; the string
// form ("off"/"auto"/"always") is the new way to spell each mode and must
// be honoured case-insensitively. Default — no key set — is "auto".
//
// Bad values must NOT silently mutate the active mode and must NOT poison
// the rest of the config — earlier behaviour discarded every key on a
// parse error, masking unrelated keybinding/colorscheme settings. The
// "typo doesn't nuke unrelated keys" subtest is the regression guard.
func TestLoadConfig_InformerCache(t *testing.T) {
	orig := ConfigInformerCacheMode
	t.Cleanup(func() { ConfigInformerCacheMode = orig })

	tests := []struct {
		name       string
		yaml       string
		startValue string
		want       string
	}{
		// Legacy bool form.
		{"legacy true maps to always", "informer_cache: true\n", InformerCacheAuto, InformerCacheAlways},
		{"legacy false maps to off", "informer_cache: false\n", InformerCacheAuto, InformerCacheOff},
		// String form, all three modes.
		{"explicit off", "informer_cache: \"off\"\n", InformerCacheAuto, InformerCacheOff},
		{"explicit auto", "informer_cache: auto\n", InformerCacheOff, InformerCacheAuto},
		{"explicit always", "informer_cache: always\n", InformerCacheOff, InformerCacheAlways},
		// Case-insensitivity is a small but real ergonomic win — config
		// files mixed with comments tend to drift in casing.
		{"uppercase Always normalised to lowercase", "informer_cache: ALWAYS\n", InformerCacheOff, InformerCacheAlways},
		// Whitespace tolerance: copy-paste mistakes shouldn't silently
		// fall through to "auto" on a trim mismatch.
		{"surrounding whitespace trimmed", "informer_cache: \"  always  \"\n", InformerCacheOff, InformerCacheAlways},
		// Unknown / unparseable values: warn-and-fallback. The active
		// mode stays at its start value; logger.Warn is fired (silent at
		// LoadConfig time but present for future Init reordering).
		{"unknown mode falls back to start value", "informer_cache: maybe\n", InformerCacheAuto, InformerCacheAuto},
		{"empty quoted string treated as unset", "informer_cache: \"\"\n", InformerCacheOff, InformerCacheOff},
		{"numeric value falls back to start", "informer_cache: 42\n", InformerCacheAuto, InformerCacheAuto},
		// YAML 1.1 boolean coercion: bare `off` / `on` / `yes` / `no`
		// resolve as bools, NOT as strings. Documented quirk; passing
		// quoted "off" goes through the string branch instead. The
		// matrix here pins the actual behaviour so a future YAML lib
		// upgrade that changes coercion rules trips this test.
		{"yaml 1.1 bare off coerces to bool false", "informer_cache: off\n", InformerCacheAuto, InformerCacheOff},
		{"yaml 1.1 bare on coerces to bool true", "informer_cache: on\n", InformerCacheOff, InformerCacheAlways},
		// Unrelated config keys should not touch the mode.
		{"unset leaves default auto", "colorscheme: dracula\n", InformerCacheAuto, InformerCacheAuto},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ConfigInformerCacheMode = tc.startValue

			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			LoadConfig(path)
			assert.Equal(t, tc.want, ConfigInformerCacheMode)
		})
	}
}

// TestLoadConfig_InformerCacheTypoDoesNotPoisonRestOfFile is the
// regression guard for the silent-discard behaviour: a typo'd
// informer_cache value used to abort yaml.Unmarshal and drop the entire
// config, which masked unrelated colorscheme / keybinding / etc. keys.
// After the warn-and-fallback fix, only informer_cache resets while
// every other key in the file lands as written.
func TestLoadConfig_InformerCacheTypoDoesNotPoisonRestOfFile(t *testing.T) {
	origMode := ConfigInformerCacheMode
	origScheme := ConfigDarkColorscheme
	t.Cleanup(func() {
		ConfigInformerCacheMode = origMode
		ConfigDarkColorscheme = origScheme
	})

	ConfigInformerCacheMode = InformerCacheAuto
	ConfigDarkColorscheme = ""

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := "informer_cache: maybe\ncolorscheme: \"dark:dracula\"\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	LoadConfig(path)

	assert.Equal(t, InformerCacheAuto, ConfigInformerCacheMode,
		"typo'd informer_cache must fall back to default")
	assert.Equal(t, "dracula", ConfigDarkColorscheme,
		"unrelated colorscheme key must still apply — earlier behaviour silently dropped it")
}
