package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartMultiLogStream_AllFail_ReportsStatusAndKeepsMode(t *testing.T) {
	m := basePush80Model()
	m.mode = modeExplorer
	t.Setenv("KUBECTL_BIN", "/nonexistent/kubectl")

	items := []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod"},
		{Name: "pod-2", Namespace: "default", Kind: "Pod"},
		{Name: "pod-3", Namespace: "default", Kind: "Pod"},
	}

	updated, cmd := m.startMultiLogStream(items)
	mdl, ok := updated.(*Model)
	require.True(t, ok)

	assert.Equal(t, modeExplorer, mdl.mode, "must not enter the log viewer when every stream fails")
	assert.True(t, mdl.statusMessageErr)
	assert.Contains(t, mdl.statusMessage, "3")
	require.NotNil(t, cmd)
}
