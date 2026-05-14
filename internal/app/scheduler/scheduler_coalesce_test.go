package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSig_EqualByFields(t *testing.T) {
	a := Sig{KubeContext: "c1", Kind: KindResourceList, Name: "List Pods", Target: "default", Gen: 7}
	b := Sig{KubeContext: "c1", Kind: KindResourceList, Name: "List Pods", Target: "default", Gen: 7}
	assert.Equal(t, a, b)
}

// TestSig_NameInSignature is a regression test for the bug where
// "List Deployments" (the main resource list refresh) and "List Deployment
// children" (the owned-pods preview) had identical Sigs and coalesced each
// other under watch-tick refresh, leaving the right-pane children pane stuck
// empty for Deployment / StatefulSet / DaemonSet hovers.
func TestSig_NameInSignature(t *testing.T) {
	mainList := Sig{KubeContext: "c1", Kind: KindResourceList, Name: "List Deployments", Target: "c1 / argo", Gen: 7}
	childList := Sig{KubeContext: "c1", Kind: KindResourceList, Name: "List Deployment children", Target: "c1 / argo", Gen: 7}
	assert.NotEqual(t, mainList, childList,
		"main resource list and owned-children preview must NOT share a coalesce Sig")
}

func TestSig_DifferByAnyField(t *testing.T) {
	base := Sig{KubeContext: "c1", Kind: KindResourceList, Name: "List Pods", Target: "default", Gen: 7}
	cases := []struct {
		name string
		s    Sig
	}{
		{"different context", Sig{KubeContext: "c2", Kind: KindResourceList, Name: "List Pods", Target: "default", Gen: 7}},
		{"different kind", Sig{KubeContext: "c1", Kind: KindMetrics, Name: "List Pods", Target: "default", Gen: 7}},
		{"different name", Sig{KubeContext: "c1", Kind: KindResourceList, Name: "List Pod children", Target: "default", Gen: 7}},
		{"different target", Sig{KubeContext: "c1", Kind: KindResourceList, Name: "List Pods", Target: "kube-system", Gen: 7}},
		{"different gen", Sig{KubeContext: "c1", Kind: KindResourceList, Name: "List Pods", Target: "default", Gen: 8}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEqual(t, base, tc.s)
		})
	}
}

func TestSig_MutationsNeverCoalesce(t *testing.T) {
	a := Sig{KubeContext: "c1", Kind: KindMutation, Target: "delete pod foo", Gen: 1}
	assert.True(t, a.NeverCoalesce(), "Mutation Kind must opt out of coalescing")

	b := Sig{KubeContext: "c1", Kind: KindResourceList, Target: "default", Gen: 1}
	assert.False(t, b.NeverCoalesce(), "Read Kinds must allow coalescing")
}
