// Package gatekeeper reads OPA Gatekeeper Constraint CRDs and exposes
// the audit violations recorded on each Constraint's .status as
// security.Findings.
//
// Constraints are dynamic: every ConstraintTemplate the cluster admin
// installs creates a new CRD under constraints.gatekeeper.sh, so the
// adapter has to discover the GroupVersionResources at fetch time
// (via the Kubernetes Discovery API) before listing them.
package gatekeeper

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/janosmiko/lfk/internal/security"
)

// ConstraintsGroup is the API group every dynamically-generated
// Constraint CRD lives under.
const ConstraintsGroup = "constraints.gatekeeper.sh"

// ConstraintsVersion is the API version Gatekeeper has shipped Constraints
// under since v3.0. Newer versions (v1) are anticipated; for now we list
// only v1beta1 instances and accept that v1-only constraints would be
// missed until this constant is updated.
const ConstraintsVersion = "v1beta1"

// constraintTemplateGVRs are the parent-CRD availability probe targets,
// newest API version first. The probe avoids the Discovery API (which
// client-go doesn't expose with a context, so a hung API server would
// block the probe goroutine past the manager's 3s timeout). v1 has been
// GA since Gatekeeper 3.10; clusters older than that serve
// ConstraintTemplate as v1beta1 only, so the probe falls back to it
// before reporting Gatekeeper unavailable.
var constraintTemplateGVRs = []schema.GroupVersionResource{
	{Group: "templates.gatekeeper.sh", Version: "v1", Resource: "constrainttemplates"},
	{Group: "templates.gatekeeper.sh", Version: "v1beta1", Resource: "constrainttemplates"},
}

// Source is the gatekeeper SecuritySource implementation.
type Source struct {
	clientset kubernetes.Interface // Discovery() at fetch time lists constraint kinds
	dynClient dynamic.Interface    // probes ConstraintTemplate (IsAvailable) and lists constraint instances (Fetch)
}

// New returns a Source with no clients (Fetch returns nil, IsAvailable false).
func New() *Source { return &Source{} }

// NewWithClients returns a Source using the given clientset (for
// discovery) and dynamic client (for listing constraints).
func NewWithClients(clientset kubernetes.Interface, dynClient dynamic.Interface) *Source {
	return &Source{clientset: clientset, dynClient: dynClient}
}

// Name returns the stable identifier.
func (s *Source) Name() string { return "gatekeeper" }

// Categories returns the categories this source produces.
func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryPolicy, security.CategoryCompliance}
}

// IsAvailable returns true when the cluster serves the
// templates.gatekeeper.sh API group (the parent ConstraintTemplate
// CRD). Probing via the dynamic client honours the caller's context
// timeout, unlike client-go's Discovery() ServerResourcesForGroupVersion
// which has no Context-aware variant — a stalled API server there
// would block the manager's 3s probe timeout indefinitely and starve
// the rest of the security source probes.
func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	if s.dynClient == nil {
		return false, nil
	}
	for _, gvr := range constraintTemplateGVRs {
		_, err := s.dynClient.Resource(gvr).List(ctx, metav1.ListOptions{Limit: 1})
		if err == nil {
			return true, nil
		}
		// NotFound / no-such-resource → this API version isn't served;
		// try the next. Other errors propagate so the manager's probe
		// preserves the previous-known availability rather than briefly
		// hiding Gatekeeper on a transient failure.
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("gatekeeper availability probe: %w", err)
		}
	}
	// Every version returned NotFound → definitive "Gatekeeper not installed".
	return false, nil
}

// Fetch lists every Constraint instance the cluster currently knows
// about and converts each .status.violations entry into a Finding.
// Each kind's instance list is paginated (default 200 items per page)
// to keep per-response payloads bounded — Constraints carry full
// status.violations[] arrays and can balloon on busy clusters. The
// per-CRD listing failure mode is swallowed so a single broken kind
// doesn't black out the whole feed.
//
// namespace is ignored: Gatekeeper Constraints are cluster-scoped, but
// each violation carries its own (group, version, kind, namespace, name)
// — the namespace surface comes from there, not from the parent CR.
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	if s.dynClient == nil || s.clientset == nil {
		return nil, nil
	}
	apiList, err := discoverConstraintKinds(ctx, s.clientset)
	if err != nil {
		return nil, fmt.Errorf("discover gatekeeper constraints: %w", err)
	}
	var findings []security.Finding
	for _, r := range apiList {
		// Skip subresources (e.g., "/status", "/scale").
		if strings.Contains(r.Name, "/") {
			continue
		}
		gvr := schema.GroupVersionResource{
			Group:    ConstraintsGroup,
			Version:  ConstraintsVersion,
			Resource: r.Name,
		}
		// Constraints are cluster-scoped, so List with no namespace.
		list, err := security.ListPaginated(ctx, s.dynClient.Resource(gvr))
		if err != nil {
			// One bad CRD shouldn't kill the whole fetch — keep going
			// and let the caller see partial results.
			continue
		}
		for i := range list.Items {
			findings = append(findings, parseConstraint(&list.Items[i], r.Kind)...)
		}
	}
	return findings, nil
}

// discoverConstraintKinds returns every API resource served under
// constraints.gatekeeper.sh/v1beta1, bounded by ctx. client-go's
// Discovery() ServerResourcesForGroupVersion has no Context-aware
// variant, so we run it on a goroutine and select on ctx.Done(); the
// goroutine may leak if the API server hangs forever, but the caller
// returns promptly and the manager can move on.
//
// A NotFound discovery error is treated as "Gatekeeper webhook
// installed but no ConstraintTemplates registered yet" rather than a
// failure: returning the error here would surface as a per-fetch
// "Application error" in the explorer and spam the log on every
// FetchAll. An empty list is the correct semantic — there are zero
// constraint kinds to query, so there are zero violations.
func discoverConstraintKinds(ctx context.Context, clientset kubernetes.Interface) ([]metav1.APIResource, error) {
	type result struct {
		list *metav1.APIResourceList
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		list, err := clientset.Discovery().ServerResourcesForGroupVersion(ConstraintsGroup + "/" + ConstraintsVersion)
		resCh <- result{list: list, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-resCh:
		if r.err != nil {
			if apierrors.IsNotFound(r.err) {
				return nil, nil
			}
			return nil, r.err
		}
		return r.list.APIResources, nil
	}
}

// parseConstraint walks a single Constraint object and converts every
// status.violations entry into a security.Finding. constraintKind is
// the Kubernetes Kind name of the parent constraint (e.g.,
// "K8sRequiredLabels"); we surface it on the Finding so users can
// distinguish "10 violations of K8sRequiredLabels" from "10 violations
// of K8sBlockNodeport" in the group view.
func parseConstraint(u *unstructured.Unstructured, constraintKind string) []security.Finding {
	violations, ok, _ := unstructured.NestedSlice(u.Object, "status", "violations")
	if !ok || len(violations) == 0 {
		return nil
	}
	constraintName := u.GetName()
	enforcement := defaultEnforcementAction(u)

	var findings []security.Finding
	for _, raw := range violations {
		v, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := v["kind"].(string)
		name, _ := v["name"].(string)
		ns, _ := v["namespace"].(string)
		message, _ := v["message"].(string)
		violationEnforcement, _ := v["enforcementAction"].(string)
		if violationEnforcement == "" {
			violationEnforcement = enforcement
		}
		findings = append(findings, security.Finding{
			ID: fmt.Sprintf("gatekeeper/%s/%s/%s/%s/%s",
				constraintKind, constraintName, ns, kind, name),
			Source:   "gatekeeper",
			Category: security.CategoryPolicy,
			Severity: severityFromEnforcement(violationEnforcement),
			Title:    constraintKind,
			Resource: security.ResourceRef{Namespace: ns, Kind: kind, Name: name},
			Summary:  message,
			Labels: map[string]string{
				"constraint_kind":    constraintKind,
				"constraint_name":    constraintName,
				"enforcement_action": violationEnforcement,
			},
		})
	}
	return findings
}

// defaultEnforcementAction reads spec.enforcementAction from the parent
// Constraint, returning "deny" when the field is unset (Gatekeeper's
// default behaviour).
func defaultEnforcementAction(u *unstructured.Unstructured) string {
	v, ok, _ := unstructured.NestedString(u.Object, "spec", "enforcementAction")
	if !ok || v == "" {
		return "deny"
	}
	return v
}

// severityFromEnforcement maps Gatekeeper's enforcementAction values to
// the explorer's coarse severity buckets so the SEC badge colour matches
// users' expectations: a "deny" violation is louder than a "dryrun" one
// even though Gatekeeper itself doesn't grade severities.
func severityFromEnforcement(action string) security.Severity {
	switch strings.ToLower(action) {
	case "deny":
		return security.SeverityHigh
	case "warn":
		return security.SeverityMedium
	case "dryrun":
		return security.SeverityLow
	}
	return security.SeverityMedium
}
