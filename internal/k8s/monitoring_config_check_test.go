package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func TestUnmatchedMonitoringKeys(t *testing.T) {
	contexts := []string{"launch-dev", "smartcv-dev"}
	tests := []struct {
		name string
		cfg  map[string]model.MonitoringConfig
		want []string
	}{
		{name: "nil config", cfg: nil, want: nil},
		{name: "only _global", cfg: map[string]model.MonitoringConfig{"_global": {}}, want: nil},
		{name: "all keys match", cfg: map[string]model.MonitoringConfig{"launch-dev": {}, "smartcv-dev": {}}, want: nil},
		{
			name: "one key matches no context",
			cfg:  map[string]model.MonitoringConfig{"_global": {}, "vm-cluster": {}, "launch-dev": {}},
			want: []string{"vm-cluster"},
		},
		{
			name: "unmatched keys are sorted",
			cfg:  map[string]model.MonitoringConfig{"zeta": {}, "alpha": {}},
			want: []string{"alpha", "zeta"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, UnmatchedMonitoringKeys(tt.cfg, contexts))
		})
	}
}

func TestClient_WarnUnmatchedMonitoringKeys_UsesDisplayNames(t *testing.T) {
	// Issue #705: a user keyed the entry "vm-cluster" while the kubeconfig
	// context was "launch-dev", so the _global prefix applied silently.
	tmp := t.TempDir()
	path := filepath.Join(tmp, "kubeconfig.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`apiVersion: v1
kind: Config
current-context: launch-dev
clusters:
- name: c
  cluster:
    server: https://c.example.test:6443
    insecure-skip-tls-verify: true
contexts:
- name: launch-dev
  context:
    cluster: c
    user: u
users:
- name: u
  user:
    token: x
`), 0o600))
	t.Setenv("KUBECONFIG", path)

	client, err := NewClient("", nil, true, nil)
	require.NoError(t, err)

	prev := model.ConfigMonitoring
	t.Cleanup(func() { model.ConfigMonitoring = prev })
	model.ConfigMonitoring = map[string]model.MonitoringConfig{
		"_global":    {},
		"vm-cluster": {},
		"launch-dev": {},
	}

	assert.Equal(t, []string{"vm-cluster"}, client.WarnUnmatchedMonitoringKeys())
}
