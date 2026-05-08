package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSig_EqualByFields(t *testing.T) {
	a := Sig{KubeContext: "c1", Kind: KindResourceList, Target: "default", Gen: 7}
	b := Sig{KubeContext: "c1", Kind: KindResourceList, Target: "default", Gen: 7}
	assert.Equal(t, a, b)
}

func TestSig_DifferByAnyField(t *testing.T) {
	base := Sig{KubeContext: "c1", Kind: KindResourceList, Target: "default", Gen: 7}
	cases := []struct {
		name string
		s    Sig
	}{
		{"different context", Sig{KubeContext: "c2", Kind: KindResourceList, Target: "default", Gen: 7}},
		{"different kind", Sig{KubeContext: "c1", Kind: KindMetrics, Target: "default", Gen: 7}},
		{"different target", Sig{KubeContext: "c1", Kind: KindResourceList, Target: "kube-system", Gen: 7}},
		{"different gen", Sig{KubeContext: "c1", Kind: KindResourceList, Target: "default", Gen: 8}},
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
