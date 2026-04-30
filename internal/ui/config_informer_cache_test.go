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
		{"explicit off", "informer_cache: off\n", InformerCacheAuto, InformerCacheOff},
		{"explicit auto", "informer_cache: auto\n", InformerCacheOff, InformerCacheAuto},
		{"explicit always", "informer_cache: always\n", InformerCacheOff, InformerCacheAlways},
		// Case-insensitivity is a small but real ergonomic win — config
		// files mixed with comments tend to drift in casing.
		{"uppercase Always normalised to lowercase", "informer_cache: ALWAYS\n", InformerCacheOff, InformerCacheAlways},
		// Unknown values must not silently change the active mode; a typo
		// in the config file should leave the start value untouched and
		// surface in the load-error path (which LoadConfig handles by
		// returning early on Unmarshal failure).
		{"unknown mode falls back to start value", "informer_cache: maybe\n", InformerCacheAuto, InformerCacheAuto},
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
