package k8s

import (
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/janosmiko/lfk/internal/model"
)

// --- populateResourceDetailsExt: PodCertificateRequest ---

func TestPopulateResourceDetailsExt_PodCertificateRequest(t *testing.T) {
	tests := []struct {
		name     string
		spec     map[string]any
		status   map[string]any
		wantCols map[string]string
	}{
		{
			name: "pod, signer, and issued/expiry columns",
			spec: map[string]any{
				"podName":    "web-1",
				"signerName": "example.com/signer",
			},
			status: map[string]any{
				"notBefore": "2026-01-01T00:00:00Z",
				"notAfter":  "2026-01-02T00:00:00Z",
			},
			wantCols: map[string]string{
				"Pod":     "web-1",
				"Signer":  "example.com/signer",
				"Issued":  "2026-01-01T00:00:00Z",
				"Expires": "2026-01-02T00:00:00Z",
			},
		},
		{
			name: "no status yields no issued/expiry columns",
			spec: map[string]any{
				"podName":    "web-1",
				"signerName": "example.com/signer",
			},
			status: nil,
			wantCols: map[string]string{
				"Pod":    "web-1",
				"Signer": "example.com/signer",
			},
		},
		{
			name:   "no spec yields no pod/signer columns",
			spec:   nil,
			status: map[string]any{"notBefore": "2026-01-01T00:00:00Z"},
			wantCols: map[string]string{
				"Issued": "2026-01-01T00:00:00Z",
			},
		},
		{
			name:     "nil spec and status produce no columns",
			spec:     nil,
			status:   nil,
			wantCols: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetailsExt(ti, map[string]any{"spec": tt.spec, "status": tt.status}, "PodCertificateRequest", tt.status, tt.spec)

			colMap := columnsToMap(ti.Columns)
			if tt.wantCols == nil {
				assert.Empty(t, colMap)
			} else {
				assert.Equal(t, tt.wantCols, colMap)
			}
		})
	}
}

// --- populateResourceDetailsExt: ClusterTrustBundle ---

func pemCertBlock(t *testing.T, payload string) string {
	t.Helper()
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte(payload)}))
}

func TestPopulateResourceDetailsExt_ClusterTrustBundle(t *testing.T) {
	twoCerts := pemCertBlock(t, "cert-one") + pemCertBlock(t, "cert-two")
	oneCert := pemCertBlock(t, "cert-one")

	tests := []struct {
		name     string
		spec     map[string]any
		wantCols map[string]string
	}{
		{
			name: "signer set with two certificates",
			spec: map[string]any{
				"signerName":  "example.com/signer",
				"trustBundle": twoCerts,
			},
			wantCols: map[string]string{
				"Signer":       "example.com/signer",
				"Certificates": "2",
			},
		},
		{
			name: "empty signer means anonymous bundle, still reported",
			spec: map[string]any{
				"signerName":  "",
				"trustBundle": oneCert,
			},
			wantCols: map[string]string{
				"Signer":       "",
				"Certificates": "1",
			},
		},
		{
			name: "malformed PEM counts zero certificates",
			spec: map[string]any{
				"signerName":  "example.com/signer",
				"trustBundle": "not a pem bundle",
			},
			wantCols: map[string]string{
				"Signer":       "example.com/signer",
				"Certificates": "0",
			},
		},
		{
			name:     "nil spec produces no columns",
			spec:     nil,
			wantCols: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetailsExt(ti, map[string]any{"spec": tt.spec}, "ClusterTrustBundle", nil, tt.spec)

			colMap := columnsToMap(ti.Columns)
			if tt.wantCols == nil {
				assert.Empty(t, colMap)
			} else {
				assert.Equal(t, tt.wantCols, colMap)
			}
		})
	}
}

// --- GetResources end to end: PodCertificateRequest ---

var podCertificateRequestGVR = schema.GroupVersionResource{
	Group: "certificates.k8s.io", Version: "v1", Resource: "podcertificaterequests",
}

func TestGetResources_PodCertificateRequestColumns(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		podCertificateRequestGVR: "PodCertificateRequestList",
	}
	pcr := &unstructured.Unstructured{}
	pcr.SetUnstructuredContent(map[string]any{
		"apiVersion": "certificates.k8s.io/v1",
		"kind":       "PodCertificateRequest",
		"metadata":   map[string]any{"name": "pcr-1", "namespace": "default"},
		"spec": map[string]any{
			"podName":    "web-1",
			"signerName": "example.com/signer",
		},
		"status": map[string]any{
			"notBefore": "2026-01-01T00:00:00Z",
			"notAfter":  "2026-01-02T00:00:00Z",
		},
	})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, pcr)
	c := newFakeClient(nil, dyn)

	rt := model.ResourceTypeEntry{
		Kind: "PodCertificateRequest", APIGroup: "certificates.k8s.io", APIVersion: "v1",
		Resource: "podcertificaterequests", Namespaced: true,
	}
	items, err := c.GetResources(t.Context(), "test-ctx", "default", rt, false)
	require.NoError(t, err)
	require.Len(t, items, 1)

	colMap := columnsToMap(items[0].Columns)
	assert.Equal(t, "web-1", colMap["Pod"])
	assert.Equal(t, "example.com/signer", colMap["Signer"])
	assert.Equal(t, "2026-01-01T00:00:00Z", colMap["Issued"])
	assert.Equal(t, "2026-01-02T00:00:00Z", colMap["Expires"])
}

// --- GetResources end to end: ClusterTrustBundle ---

var clusterTrustBundleGVR = schema.GroupVersionResource{
	Group: "certificates.k8s.io", Version: "v1", Resource: "clustertrustbundles",
}

func TestGetResources_ClusterTrustBundleColumns(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		clusterTrustBundleGVR: "ClusterTrustBundleList",
	}
	ctb := &unstructured.Unstructured{}
	ctb.SetUnstructuredContent(map[string]any{
		"apiVersion": "certificates.k8s.io/v1",
		"kind":       "ClusterTrustBundle",
		"metadata":   map[string]any{"name": "example.com:signer:v1"},
		"spec": map[string]any{
			"signerName":  "example.com/signer",
			"trustBundle": pemCertBlock(t, "cert-one") + pemCertBlock(t, "cert-two"),
		},
	})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, ctb)
	c := newFakeClient(nil, dyn)

	rt := model.ResourceTypeEntry{
		Kind: "ClusterTrustBundle", APIGroup: "certificates.k8s.io", APIVersion: "v1",
		Resource: "clustertrustbundles", Namespaced: false,
	}
	items, err := c.GetResources(t.Context(), "test-ctx", "", rt, false)
	require.NoError(t, err)
	require.Len(t, items, 1)

	colMap := columnsToMap(items[0].Columns)
	assert.Equal(t, "example.com/signer", colMap["Signer"])
	assert.Equal(t, "2", colMap["Certificates"])
}
