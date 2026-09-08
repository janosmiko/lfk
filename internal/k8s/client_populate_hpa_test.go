package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestPopulateHPAReady_ScaledToZero(t *testing.T) {
	ti := &model.Item{}
	spec := map[string]any{
		"minReplicas": float64(0),
		"maxReplicas": float64(5),
	}
	status := map[string]any{
		"currentReplicas": float64(0),
		"desiredReplicas": float64(0),
		"conditions": []any{
			map[string]any{"type": "ScaledToZero", "status": "True", "reason": "ScalingLimited"},
		},
	}
	populateHPAReady(ti, status, spec)
	assert.Equal(t, "0/0 (0-5) (scaled to zero)", ti.Ready)
}

func TestPopulateHPAReady_ScaledToZeroConditionFalse(t *testing.T) {
	ti := &model.Item{}
	spec := map[string]any{
		"minReplicas": float64(0),
		"maxReplicas": float64(5),
	}
	status := map[string]any{
		"currentReplicas": float64(1),
		"desiredReplicas": float64(1),
		"conditions": []any{
			map[string]any{"type": "ScaledToZero", "status": "False"},
		},
	}
	populateHPAReady(ti, status, spec)
	assert.Equal(t, "1/1 (0-5)", ti.Ready)
}

func TestPopulateHPAReady_MinReplicasZeroNoConditions(t *testing.T) {
	ti := &model.Item{}
	spec := map[string]any{
		"minReplicas": float64(0),
		"maxReplicas": float64(5),
	}
	status := map[string]any{
		"currentReplicas": float64(2),
		"desiredReplicas": float64(2),
	}
	populateHPAReady(ti, status, spec)
	assert.Equal(t, "2/2 (0-5)", ti.Ready)
}
