package demo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestListKinds_CoversEveryAdvertisedResource holds the invariant that made
// the --demo startup crash possible: any resource DiscoverAPIResources can
// see via APIResourceLists must have a matching ListKind entry, or the fake
// dynamic client panics instead of erroring on List. This guards against a
// future resource being added to APIResourceLists without its ListKind
// counterpart.
func TestListKinds_CoversEveryAdvertisedResource(t *testing.T) {
	kinds := ListKinds()

	for _, list := range APIResourceLists() {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if !assert.NoError(t, err, "unparseable GroupVersion %q", list.GroupVersion) {
			continue
		}
		for _, r := range list.APIResources {
			gvr := schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: r.Name}
			assert.Contains(t, kinds, gvr, "APIResourceLists advertises %s but ListKinds has no registration for it", gvr)
		}
	}
}
