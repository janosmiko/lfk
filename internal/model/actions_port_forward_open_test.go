package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPortForwardAndOpenActionPresentForService(t *testing.T) {
	var found bool
	for _, it := range ActionsForKind("Service") {
		if it.Label == "Port Forward & Open" {
			found = true
			assert.Equal(t, "O", it.Key)
			assert.NotEmpty(t, it.Description)
			break
		}
	}
	assert.True(t, found, "Service must offer the Port Forward & Open action item")
}

func TestPortForwardAndOpenActionAbsentForNonService(t *testing.T) {
	for _, kind := range []string{"Pod", "Deployment", "StatefulSet", "DaemonSet", "Ingress"} {
		t.Run(kind, func(t *testing.T) {
			for _, it := range ActionsForKind(kind) {
				assert.NotEqual(t, "Port Forward & Open", it.Label,
					"%q must not offer the Port Forward & Open action item", kind)
			}
		})
	}
}
