package completion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompleteKubeContexts(t *testing.T) {
	kubeconfig := writeTempKubeconfig(t, `
apiVersion: v1
kind: Config
current-context: alpha
clusters:
  - name: c
    cluster:
      server: https://127.0.0.1
users:
  - name: u
    user:
      token: test
contexts:
  - name: alpha
    context:
      cluster: c
      user: u
  - name: beta
    context:
      cluster: c
      user: u
  - name: prod
    context:
      cluster: c
      user: u
`)

	got, err := completeKubeContexts(kubeconfig, nil, map[string]struct{}{"prod": {}}, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta"}, got)

	got, err = completeKubeContexts(kubeconfig, nil, nil, "b")
	require.NoError(t, err)
	assert.Equal(t, []string{"beta"}, got)
}

func TestCompleteUnionSets(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(config, []byte(`
union_sets:
  - name: staging-west
    namespace: cloud-cd
    contexts:
      - context: alpha
  - name: prod-east
    contexts:
      - context: beta
  - name: ""
    contexts:
      - context: ignored
  - name: prod-east
    contexts:
      - context: duplicate
`), 0o600))

	got, err := completeUnionSets(config, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"prod-east", "staging-west"}, got)

	got, err = completeUnionSets(config, "st")
	require.NoError(t, err)
	assert.Equal(t, []string{"staging-west"}, got)
}

func TestCompleteUnionSetsUsesDefaultConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	configDir := filepath.Join(dir, "lfk")
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(`
union_sets:
  - name: local-set
`), 0o600))

	got, err := completeUnionSets("", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"local-set"}, got)
}

func TestCompleteUnionSetsMapForm(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(config, []byte(`
union_sets:
  staging-west:
    contexts:
      - alpha
  prod-east:
    contexts:
      - beta
`), 0o600))

	got, err := completeUnionSets(config, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"prod-east", "staging-west"}, got)
}

func writeTempKubeconfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}
