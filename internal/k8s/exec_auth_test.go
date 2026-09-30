package k8s

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const execKubeconfig = `apiVersion: v1
kind: Config
current-context: ex
clusters:
- name: ex
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: ex
  context:
    cluster: ex
    user: ex
users:
- name: ex
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: kubelogin
`

func TestRestConfigForContext_ExecPluginNeverInteractive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(execKubeconfig), 0o600))
	c, err := NewClient(path, nil, true, nil)
	require.NoError(t, err)

	cfg, err := c.restConfigForContext("ex")
	require.NoError(t, err)
	require.NotNil(t, cfg.ExecProvider)
	assert.Equal(t, clientcmdapi.NeverExecInteractiveMode, cfg.ExecProvider.InteractiveMode)
}

func TestIsExecPluginExitError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"exit code", errors.New("getting credentials: exec: executable kubelogin failed with exit code 1"), true},
		{"not found", errors.New("getting credentials: exec: executable kubelogin not found"), false},
		{"interactive", errors.New("exec plugin cannot support interactive mode"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsExecPluginExitError(tt.err))
		})
	}
}
