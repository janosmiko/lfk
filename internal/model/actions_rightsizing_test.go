package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRightsizingActionPresent(t *testing.T) {
	for _, kind := range []string{"Pod", "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob"} {
		t.Run(kind, func(t *testing.T) {
			items := ActionsForKind(kind)
			found := false
			for _, it := range items {
				if it.Label == "Right-sizing" {
					found = true
					break
				}
			}
			assert.True(t, found, "%q must offer the Right-sizing action item", kind)
		})
	}
}

func TestRightsizingActionAbsentForUnsupported(t *testing.T) {
	for _, kind := range []string{"Service", "ConfigMap", "Secret", "Node", "PersistentVolumeClaim"} {
		t.Run(kind, func(t *testing.T) {
			items := ActionsForKind(kind)
			for _, it := range items {
				assert.NotEqual(t, "Right-sizing", it.Label,
					"%q has no pod template — Right-sizing must be hidden", kind)
			}
		})
	}
}
