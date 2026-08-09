package k8s

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// TestPatch_SendsFieldManagerOnTheWire proves the manager name reaches the API
// server as the fieldManager query parameter, and the agent reaches it as the
// User-Agent header. Asserting the option struct alone would not show that
// client-go forwards either one.
func TestPatch_SendsFieldManagerOnTheWire(t *testing.T) {
	var gotQuery, gotAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("fieldManager")
		gotAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Node","metadata":{"name":"n1"}}`))
	}))
	t.Cleanup(server.Close)

	cs, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL, UserAgent: UserAgent()})
	require.NoError(t, err)

	_, err = cs.CoreV1().Nodes().Patch(
		t.Context(), "n1", k8stypes.MergePatchType,
		[]byte(`{"spec":{"unschedulable":true}}`),
		metav1.PatchOptions{FieldManager: FieldManager()},
	)
	require.NoError(t, err)

	assert.Equal(t, FieldManager(), gotQuery, "fieldManager query parameter")
	assert.Equal(t, UserAgent(), gotAgent, "User-Agent header")
	assert.Contains(t, gotAgent, "lfk/")
}

// TestRestConfigForContext_CarriesTheUserAgent proves the wiring point sets the
// agent, so every client built from the config inherits it.
func TestRestConfigForContext_CarriesTheUserAgent(t *testing.T) {
	c := newCacheTestClient(t)

	cfg, err := c.restConfigForContext("plain")
	require.NoError(t, err)

	assert.Equal(t, UserAgent(), cfg.UserAgent)
}
