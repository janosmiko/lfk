package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// A bare plural is ambiguous when two groups define the same Kind — kubectl
// resolves it to whichever group discovery returns first, which is how issue
// #562 hit the wrong CRD. The qualified "resource.group" form is unambiguous.
func TestKubectlResourceArg(t *testing.T) {
	tests := []struct {
		name string
		rt   model.ResourceTypeEntry
		want string
	}{
		{
			name: "core type stays bare",
			rt:   model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1"},
			want: "pods",
		},
		{
			name: "built-in group is qualified",
			rt:   model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", APIGroup: "apps", APIVersion: "v1"},
			want: "deployments.apps",
		},
		{
			name: "CRD group is qualified",
			rt:   model.ResourceTypeEntry{Kind: "MyKind", Resource: "mykinds", APIGroup: "foo.com", APIVersion: "v1"},
			want: "mykinds.foo.com",
		},
		{
			name: "same Kind in another group targets that group",
			rt:   model.ResourceTypeEntry{Kind: "MyKind", Resource: "mykinds", APIGroup: "bar.com", APIVersion: "v1"},
			want: "mykinds.bar.com",
		},
		{
			name: "empty resource degrades to the empty string",
			rt:   model.ResourceTypeEntry{Kind: "MyKind", APIGroup: "foo.com"},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, kubectlResourceArg(tt.rt))
		})
	}
}

// Two CRDs sharing a Kind must produce different kubectl targets — the whole
// point of the fix.
func TestKubectlResourceArg_SameKindDifferentGroups(t *testing.T) {
	foo := model.ResourceTypeEntry{Kind: "MyKind", Resource: "mykinds", APIGroup: "foo.com", APIVersion: "v1"}
	bar := model.ResourceTypeEntry{Kind: "MyKind", Resource: "mykinds", APIGroup: "bar.com", APIVersion: "v1"}

	assert.NotEqual(t, kubectlResourceArg(foo), kubectlResourceArg(bar))
}
