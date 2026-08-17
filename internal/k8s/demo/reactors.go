package demo

import (
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

// reactingClientset is the subset of *k8sfake.Clientset (via its embedded
// testing.Fake) AddRBACReactors needs. Declared here instead of importing
// k8sfake so the reactor logic does not depend on the fake package's full
// surface.
type reactingClientset interface {
	PrependReactor(verb, resource string, reaction clienttesting.ReactionFunc)
}

// AddRBACReactors wires SelfSubjectAccessReview and SelfSubjectRulesReview
// reactors that report the demo user as fully authorized. Neither review
// type is a tracked object, so the fake clientset has no built-in reactor
// for them. Create falls through to a zero-value response (Allowed: false,
// no resource rules), which hides every action in the UI. See
// internal/k8s/client_rbac.go CheckRBAC and GetSelfRulesAs for the exact
// request/response shape the app reads.
func AddRBACReactors(cs reactingClientset) {
	cs.PrependReactor("create", "selfsubjectaccessreviews", reactSelfSubjectAccessReview)
	cs.PrependReactor("create", "selfsubjectrulesreviews", reactSelfSubjectRulesReview)
}

func reactSelfSubjectAccessReview(action clienttesting.Action) (bool, runtime.Object, error) {
	create, ok := action.(clienttesting.CreateAction)
	if !ok {
		return false, nil, nil
	}
	sar, ok := create.GetObject().(*authorizationv1.SelfSubjectAccessReview)
	if !ok {
		return false, nil, nil
	}
	out := sar.DeepCopy()
	out.Status = authorizationv1.SubjectAccessReviewStatus{
		Allowed: true,
		Reason:  "demo cluster grants all actions",
	}
	return true, out, nil
}

func reactSelfSubjectRulesReview(action clienttesting.Action) (bool, runtime.Object, error) {
	create, ok := action.(clienttesting.CreateAction)
	if !ok {
		return false, nil, nil
	}
	review, ok := create.GetObject().(*authorizationv1.SelfSubjectRulesReview)
	if !ok {
		return false, nil, nil
	}
	out := review.DeepCopy()
	out.Status = authorizationv1.SubjectRulesReviewStatus{
		ResourceRules: []authorizationv1.ResourceRule{
			{Verbs: []string{"*"}, APIGroups: []string{"*"}, Resources: []string{"*"}},
		},
		NonResourceRules: []authorizationv1.NonResourceRule{
			{Verbs: []string{"*"}, NonResourceURLs: []string{"*"}},
		},
	}
	return true, out, nil
}
