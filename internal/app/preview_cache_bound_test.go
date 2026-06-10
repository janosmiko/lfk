package app

import (
	"fmt"
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The secret-preview cache holds decoded Secret plaintext keyed ctx/ns/name. It
// grew for the whole session, retaining every secret ever hovered (a leak and a
// needless plaintext-in-heap exposure). It must stay bounded.
func TestSecretPreviewCacheBounded(t *testing.T) {
	m := baseModelCov()
	m.secretPreviewCache = make(map[string]*model.SecretData)

	for i := range secretPreviewCacheCap * 3 {
		mm := m.updatePreviewSecretDataLoaded(previewSecretDataLoadedMsg{
			gen:  m.requestGen,
			ctx:  "ctx",
			ns:   "ns",
			name: fmt.Sprintf("secret-%d", i),
			data: &model.SecretData{},
		})
		m = mm
	}

	assert.LessOrEqual(t, len(m.secretPreviewCache), secretPreviewCacheCap,
		"secret cache must stay within its cap, not grow unbounded")
	require.NotEmpty(t, m.secretPreviewCache, "cap eviction must not wipe everything to zero")
}

func TestServiceEndpointsCacheBounded(t *testing.T) {
	m := baseModelCov()
	m.serviceEndpointsCache = nil

	for i := range serviceEndpointsCacheCap * 3 {
		mm := m.updatePreviewServiceEndpointsLoaded(previewServiceEndpointsLoadedMsg{
			gen:  m.requestGen,
			ctx:  "ctx",
			ns:   "ns",
			name: fmt.Sprintf("svc-%d", i),
			data: &k8s.ServiceEndpoints{},
		})
		m = mm
	}

	assert.LessOrEqual(t, len(m.serviceEndpointsCache), serviceEndpointsCacheCap)
}
