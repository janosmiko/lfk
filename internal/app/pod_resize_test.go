package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func podResizeRawFixture() map[string]any {
	return map[string]any{
		"metadata": map[string]any{
			"name":            "my-pod",
			"resourceVersion": "999",
		},
		"spec": map[string]any{
			"resources": map[string]any{
				"requests": map[string]any{"cpu": "50m"},
			},
			"containers": []any{
				map[string]any{
					"name": "web",
					"resources": map[string]any{
						"requests": map[string]any{"cpu": "200m", "memory": "256Mi"},
						"limits":   map[string]any{"cpu": "1"},
					},
					"resizePolicy": []any{
						map[string]any{"resourceName": "cpu", "restartPolicy": "RestartContainer"},
						map[string]any{"resourceName": "memory", "restartPolicy": "NotRequired"},
					},
				},
				map[string]any{
					"name": "sidecar",
					"resources": map[string]any{
						"requests": map[string]any{"cpu": "10m"},
					},
				},
			},
		},
	}
}

func TestBuildPodResizeState_PrefillsFromRaw(t *testing.T) {
	st := buildPodResizeState(podResizeRawFixture())

	assert.Equal(t, "my-pod", st.name)
	assert.Equal(t, "999", st.resourceVersion)
	require.Len(t, st.containers, 2)

	web := st.containers[0]
	assert.Equal(t, "web", web.name)
	assert.Equal(t, "200m", web.cpuReq.Value)
	assert.Equal(t, "1", web.cpuLim.Value)
	assert.Equal(t, "256Mi", web.memReq.Value)
	assert.Equal(t, "", web.memLim.Value)
	assert.Equal(t, [4]string{"200m", "1", "256Mi", ""}, web.orig)

	sidecar := st.containers[1]
	assert.Equal(t, "sidecar", sidecar.name)
	assert.Equal(t, "10m", sidecar.cpuReq.Value)

	require.Len(t, st.podLevel, 1)
	assert.Equal(t, "cpu", st.podLevel[0].Key)
	assert.Equal(t, "50m", st.podLevel[0].Value)
}

func TestBuildPodResizeState_FlagsRestartContainer(t *testing.T) {
	st := buildPodResizeState(podResizeRawFixture())
	assert.Equal(t, []string{"web"}, st.restartWarn)
}

func TestBuildPodResizeState_NilRaw(t *testing.T) {
	st := buildPodResizeState(nil)
	assert.Empty(t, st.containers)
	assert.Empty(t, st.restartWarn)
}

func TestParsePodResizeForm_RejectsBadQuantity(t *testing.T) {
	st := buildPodResizeState(podResizeRawFixture())
	st.containers[0].cpuReq.Set("12x")

	_, err := parsePodResizeForm(st)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "web")
	assert.Contains(t, err.Error(), "cpu")
}

func TestParsePodResizeForm_OnlyChangedContainers(t *testing.T) {
	st := buildPodResizeState(podResizeRawFixture())
	st.containers[0].cpuReq.Set("500m")

	specs, err := parsePodResizeForm(st)
	require.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, "web", specs[0].Name)
	assert.Equal(t, "500m", specs[0].CPURequest)
	assert.Equal(t, "1", specs[0].CPULimit)
	assert.Equal(t, "256Mi", specs[0].MemRequest)
	assert.Equal(t, "", specs[0].MemLimit)
}

func TestParsePodResizeForm_RejectsClearingASetField(t *testing.T) {
	st := buildPodResizeState(podResizeRawFixture())
	st.containers[0].cpuReq.Set("")

	_, err := parsePodResizeForm(st)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "web")
	assert.Contains(t, err.Error(), "cpu request")
	assert.Contains(t, err.Error(), "cannot be cleared")
}

func TestParsePodResizeForm_AllowsAnEmptyFieldToStayEmpty(t *testing.T) {
	st := buildPodResizeState(podResizeRawFixture())
	// web.memLim starts empty (see fixture); changing cpuReq keeps memLim
	// in the diff without ever setting it.
	st.containers[0].cpuReq.Set("500m")

	specs, err := parsePodResizeForm(st)
	require.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, "", specs[0].MemLimit)
}

func TestParsePodResizeForm_NoChanges(t *testing.T) {
	st := buildPodResizeState(podResizeRawFixture())

	specs, err := parsePodResizeForm(st)
	require.NoError(t, err)
	assert.Empty(t, specs)
}
