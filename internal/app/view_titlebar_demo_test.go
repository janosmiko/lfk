package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// renderTitleBar must show a DEMO badge when the model is running against
// the --demo in-memory cluster, and must omit it otherwise — acceptance
// criterion 6.
func TestRenderTitleBar_DemoBadge(t *testing.T) {
	baseNav := model.NavigationState{
		Context: "my-cluster",
		Level:   model.LevelResources,
		ResourceType: model.ResourceTypeEntry{
			DisplayName: "Pods",
		},
	}

	t.Run("shown in demo mode", func(t *testing.T) {
		m := basePush80Model()
		m.nav = baseNav
		m.demoMode = true
		stripped := stripANSI(m.renderTitleBar())
		assert.Contains(t, stripped, "DEMO")
	})

	t.Run("omitted outside demo mode", func(t *testing.T) {
		m := basePush80Model()
		m.nav = baseNav
		stripped := stripANSI(m.renderTitleBar())
		assert.NotContains(t, stripped, "DEMO")
	})
}
