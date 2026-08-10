package k8s

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

// allowVerbs makes the fake clientset answer SelfSubjectAccessReview with
// Allowed=true for the listed verbs and false for every other verb.
func allowVerbs(cs *k8sfake.Clientset, verbs ...string) {
	allowed := make(map[string]bool, len(verbs))
	for _, v := range verbs {
		allowed[v] = true
	}
	cs.PrependReactor("create", "selfsubjectaccessreviews",
		func(action clienttesting.Action) (bool, runtime.Object, error) {
			create, ok := action.(clienttesting.CreateAction)
			if !ok {
				return false, nil, nil
			}
			sar, ok := create.GetObject().(*authorizationv1.SelfSubjectAccessReview)
			if !ok {
				return false, nil, nil
			}
			out := sar.DeepCopy()
			out.Status.Allowed = allowed[sar.Spec.ResourceAttributes.Verb]
			return true, out, nil
		})
}

func TestPermissionQueryKey(t *testing.T) {
	assert.Equal(t, "delete:pods", PermissionQuery{Resource: "pods", Verb: "delete"}.Key())
	assert.Equal(t, "create:pods/exec", PermissionQuery{Resource: "pods", Subresource: "exec", Verb: "create"}.Key())
	assert.Equal(t, "delete:deployments.apps", PermissionQuery{Group: "apps", Resource: "deployments", Verb: "delete"}.Key())
	assert.Equal(t, "update:deployments.apps/scale",
		PermissionQuery{Group: "apps", Resource: "deployments", Subresource: "scale", Verb: "update"}.Key())
}

func TestPermissionQueryKey_GroupsDoNotCollide(t *testing.T) {
	// A CRD may borrow a core resource name, and the two verdicts are not
	// interchangeable.
	core := PermissionQuery{Resource: "widgets", Verb: "delete"}
	crd := PermissionQuery{Group: "example.com", Resource: "widgets", Verb: "delete"}
	assert.NotEqual(t, core.Key(), crd.Key())
}

func TestCheckPermissions_KeepsBothGroupsApart(t *testing.T) {
	cs := k8sfake.NewClientset()
	cs.PrependReactor("create", "selfsubjectaccessreviews",
		func(action clienttesting.Action) (bool, runtime.Object, error) {
			sar := action.(clienttesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
			out := sar.DeepCopy()
			// Only the CRD group is allowed.
			out.Status.Allowed = sar.Spec.ResourceAttributes.Group == "example.com"
			return true, out, nil
		})
	c := newFakeClient(cs, nil)

	got, err := c.CheckPermissions(t.Context(), "ctx", "default", []PermissionQuery{
		{Resource: "widgets", Verb: "delete"},
		{Group: "example.com", Resource: "widgets", Verb: "delete"},
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{
		"delete:widgets":             false,
		"delete:widgets.example.com": true,
	}, got)
}

func TestCheckPermissions_AnswersEveryQuery(t *testing.T) {
	cs := k8sfake.NewClientset()
	allowVerbs(cs, "get", "create")
	c := newFakeClient(cs, nil)

	got, err := c.CheckPermissions(t.Context(), "ctx", "default", []PermissionQuery{
		{Resource: "pods", Verb: "delete"},
		{Resource: "pods", Verb: "get"},
		{Resource: "pods", Subresource: "exec", Verb: "create"},
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{
		"delete:pods":      false,
		"get:pods":         true,
		"create:pods/exec": true,
	}, got)
}

func TestCheckPermissions_DeduplicatesQueries(t *testing.T) {
	cs := k8sfake.NewClientset()
	allowVerbs(cs, "delete")
	c := newFakeClient(cs, nil)

	got, err := c.CheckPermissions(t.Context(), "ctx", "default", []PermissionQuery{
		{Resource: "pods", Verb: "delete"},
		{Resource: "pods", Verb: "delete"},
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"delete:pods": true}, got)

	reviews := 0
	for _, a := range cs.Actions() {
		if a.GetVerb() == "create" && a.GetResource().Resource == "selfsubjectaccessreviews" {
			reviews++
		}
	}
	assert.Equal(t, 1, reviews, "a repeated query must cost one review, not two")
}

func TestCheckPermissions_SendsNamespaceAndGroup(t *testing.T) {
	cs := k8sfake.NewClientset()
	var spec authorizationv1.SelfSubjectAccessReviewSpec
	cs.PrependReactor("create", "selfsubjectaccessreviews",
		func(action clienttesting.Action) (bool, runtime.Object, error) {
			sar := action.(clienttesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
			spec = sar.Spec
			out := sar.DeepCopy()
			out.Status.Allowed = true
			return true, out, nil
		})
	c := newFakeClient(cs, nil)

	_, err := c.CheckPermissions(t.Context(), "ctx", "kube-system", []PermissionQuery{
		{Group: "apps", Resource: "deployments", Verb: "patch"},
	})
	require.NoError(t, err)
	require.NotNil(t, spec.ResourceAttributes)
	assert.Equal(t, "kube-system", spec.ResourceAttributes.Namespace)
	assert.Equal(t, "apps", spec.ResourceAttributes.Group)
	assert.Equal(t, "deployments", spec.ResourceAttributes.Resource)
	assert.Equal(t, "patch", spec.ResourceAttributes.Verb)
}

func TestCheckPermissions_NoQueries(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	got, err := c.CheckPermissions(t.Context(), "ctx", "default", nil)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Empty(t, cs.Actions())
}

func TestCheckPermissions_ErrorIsReported(t *testing.T) {
	cs := k8sfake.NewClientset()
	cs.PrependReactor("create", "selfsubjectaccessreviews",
		func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, assert.AnError
		})
	c := newFakeClient(cs, nil)

	_, err := c.CheckPermissions(t.Context(), "ctx", "default", []PermissionQuery{
		{Resource: "pods", Verb: "delete"},
	})
	require.Error(t, err)
}

func TestCheckPermissions_CapsQueryCount(t *testing.T) {
	cs := k8sfake.NewClientset()
	allowVerbs(cs, "get")
	c := newFakeClient(cs, nil)

	queries := make([]PermissionQuery, maxPermissionQueries+5)
	for i := range queries {
		// Every subresource is distinct, or dedupe would cut the set below
		// the cap and the cap itself would go untested.
		queries[i] = PermissionQuery{Resource: "pods", Subresource: fmt.Sprintf("sub-%d", i), Verb: "get"}
	}
	got, err := c.CheckPermissions(t.Context(), "ctx", "default", queries)
	require.NoError(t, err)
	assert.Len(t, got, maxPermissionQueries)
}
