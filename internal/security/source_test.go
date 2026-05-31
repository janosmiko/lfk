package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeverityOrdering(t *testing.T) {
	assert.True(t, SeverityCritical > SeverityHigh)
	assert.True(t, SeverityHigh > SeverityMedium)
	assert.True(t, SeverityMedium > SeverityLow)
	assert.True(t, SeverityLow > SeverityUnknown)
}

func TestResourceRefKey(t *testing.T) {
	r := ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}
	assert.Equal(t, "prod/Deployment/api", r.Key())
}

func TestResourceRefKeyWithoutContainer(t *testing.T) {
	r := ResourceRef{Namespace: "prod", Kind: "Pod", Name: "api-abc", Container: "main"}
	// Key() intentionally omits container for per-resource aggregation.
	assert.Equal(t, "prod/Pod/api-abc", r.Key())
}

func TestFindingZeroValueSafe(t *testing.T) {
	f := Finding{}
	assert.Equal(t, SeverityUnknown, f.Severity)
	assert.Empty(t, f.Labels)
}
