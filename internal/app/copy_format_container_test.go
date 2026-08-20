package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleContainerPodYAML = `
apiVersion: v1
kind: Pod
metadata:
  name: web
  namespace: default
spec:
  initContainers:
    - name: init-db
      image: busybox:1.36
      command: ["sh", "-c", "wait-for-db"]
  containers:
    - name: app
      image: nginx:1.25
      ports:
        - containerPort: 80
    - name: sidecar
      image: envoyproxy/envoy:v1.30
  ephemeralContainers:
    - name: debugger
      image: busybox:1.36
`

func TestExtractContainerBlocksYAML_Single(t *testing.T) {
	out, err := ExtractContainerBlocksYAML(sampleContainerPodYAML, []string{"app"})
	require.NoError(t, err)
	assert.Contains(t, out, "name: app")
	assert.Contains(t, out, "image: nginx:1.25")
	assert.NotContains(t, out, "sidecar")
	assert.NotContains(t, out, "init-db")
}

func TestExtractContainerBlocksYAML_InitContainer(t *testing.T) {
	out, err := ExtractContainerBlocksYAML(sampleContainerPodYAML, []string{"init-db"})
	require.NoError(t, err)
	assert.Contains(t, out, "name: init-db")
	assert.Contains(t, out, "image: busybox:1.36")
}

func TestExtractContainerBlocksYAML_Multi(t *testing.T) {
	out, err := ExtractContainerBlocksYAML(sampleContainerPodYAML, []string{"app", "sidecar"})
	require.NoError(t, err)
	// Two YAML docs joined with ---
	assert.Equal(t, 1, strings.Count(out, "\n---\n"))
	assert.Contains(t, out, "name: app")
	assert.Contains(t, out, "name: sidecar")
}

func TestExtractContainerBlocksYAML_NotFound(t *testing.T) {
	_, err := ExtractContainerBlocksYAML(sampleContainerPodYAML, []string{"missing"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
	assert.Contains(t, err.Error(), "web") // Pod name from sampleContainerPodYAML
}

func TestExtractContainerBlocksYAML_EphemeralContainer(t *testing.T) {
	out, err := ExtractContainerBlocksYAML(sampleContainerPodYAML, []string{"debugger"})
	require.NoError(t, err)
	assert.Contains(t, out, "name: debugger")
	assert.Contains(t, out, "image: busybox:1.36")
}
