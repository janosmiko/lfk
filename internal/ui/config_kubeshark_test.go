package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLoadConfig_KubesharkNamespace covers the kubeshark.namespace flow
// end-to-end at the config layer:
//
//   - explicit override takes effect,
//   - empty value (kubeshark: {} with no namespace) leaves the running
//     value alone — `if cfg.Kubeshark.Namespace != ""` guards a typo
//     `kubeshark:` block from accidentally clobbering an env-set override,
//   - unset (no kubeshark: block) leaves the default in place.
func TestLoadConfig_KubesharkNamespace(t *testing.T) {
	orig := ConfigKubesharkNamespace
	t.Cleanup(func() { ConfigKubesharkNamespace = orig })

	tests := []struct {
		name       string
		yaml       string
		startValue string
		want       string
	}{
		{
			name:       "explicit namespace overrides the default",
			yaml:       "kubeshark:\n  namespace: trafcap\n",
			startValue: DefaultKubesharkNamespace,
			want:       "trafcap",
		},
		{
			name:       "empty namespace leaves the start value",
			yaml:       "kubeshark:\n  namespace: \"\"\n",
			startValue: "preset",
			want:       "preset",
		},
		{
			name:       "missing kubeshark block leaves the start value",
			yaml:       "colorscheme: dracula\n",
			startValue: DefaultKubesharkNamespace,
			want:       DefaultKubesharkNamespace,
		},
		{
			// Whitespace-only would silently overwrite the default with " ",
			// then the K8s API rejects the namespace lookup with a confusing
			// error. Treat as unset.
			name:       "whitespace-only namespace is treated as unset",
			yaml:       "kubeshark:\n  namespace: \"   \"\n",
			startValue: "preset",
			want:       "preset",
		},
		{
			// Surrounding whitespace on a real value is trimmed.
			name:       "namespace with surrounding whitespace is trimmed",
			yaml:       "kubeshark:\n  namespace: \"  trafcap  \"\n",
			startValue: DefaultKubesharkNamespace,
			want:       "trafcap",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ConfigKubesharkNamespace = tc.startValue

			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			LoadConfig(path)
			assert.Equal(t, tc.want, ConfigKubesharkNamespace)
		})
	}
}
