package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig_SecretLazyLoading(t *testing.T) {
	orig := ConfigSecretLazyLoading
	t.Cleanup(func() { ConfigSecretLazyLoading = orig })

	tests := []struct {
		name string
		yaml string
		// startValue is what ConfigSecretLazyLoading holds before LoadConfig
		// fires. Mirrors the real-world sequence where a fresh process starts
		// at the default (false) and the first LoadConfig call is supposed to
		// write the configured value.
		startValue bool
		want       bool
	}{
		{"explicit true overrides false default", "secret_lazy_loading: true\n", false, true},
		{"explicit false stays false", "secret_lazy_loading: false\n", false, false},
		{"explicit false overrides an externally-set true", "secret_lazy_loading: false\n", true, false},
		{"unset leaves default false", "colorscheme: dracula\n", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ConfigSecretLazyLoading = tc.startValue

			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			LoadConfig(path)
			assert.Equal(t, tc.want, ConfigSecretLazyLoading)
		})
	}
}
