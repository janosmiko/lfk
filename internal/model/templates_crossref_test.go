package model

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// parseTemplatesByName returns each builtin template's YAML parsed into a
// generic map, keyed by template Name.
func parseTemplatesByName(t *testing.T) map[string]map[string]any {
	t.Helper()
	out := make(map[string]map[string]any)
	for _, tmpl := range BuiltinTemplates() {
		var parsed map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(tmpl.YAML), &parsed), "template %q", tmpl.Name)
		out[tmpl.Name] = parsed
	}
	return out
}

// dig walks a nested map/slice structure by string keys (maps) and returns the
// value at the path, or nil if any step is missing.
func dig(v any, path ...string) any {
	for _, key := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[key]
	}
	return v
}

// TestTemplatesCrossReferencesResolve verifies that the templates a user is
// expected to combine actually reference each other correctly: the default
// Pod + Service + Ingress must compose out of the box, and the other
// cross-referencing templates (NetworkPolicy, ServiceMonitor, HPA, RBAC
// bindings) must point at the right targets.
func TestTemplatesCrossReferencesResolve(t *testing.T) {
	tpl := parseTemplatesByName(t)

	// All basic workloads share one generic app label so the Service (and the
	// other glue templates) compose with whichever one the user creates.
	const sharedApp = "my-app"

	require.Equal(t, sharedApp, dig(tpl["Pod"], "metadata", "labels", "app"), "Pod app label")

	// Each controller's metadata label, selector, and pod-template label must
	// all use the shared app label (and agree with each other).
	for _, kind := range []string{"Deployment", "ReplicaSet", "StatefulSet", "DaemonSet"} {
		require.Equal(t, sharedApp, dig(tpl[kind], "metadata", "labels", "app"), "%s metadata label", kind)
		require.Equal(t, sharedApp, dig(tpl[kind], "spec", "selector", "matchLabels", "app"), "%s selector", kind)
		require.Equal(t, sharedApp, dig(tpl[kind], "spec", "template", "metadata", "labels", "app"), "%s pod-template label", kind)
	}

	podApp := sharedApp

	// Service must select the shared app label.
	svcSelectorApp := dig(tpl["Service"], "spec", "selector", "app")
	require.Equal(t, sharedApp, svcSelectorApp,
		"Service selector must match the shared workload app label")

	// Service must be labeled so the ServiceMonitor can select it.
	svcLabelApp := dig(tpl["Service"], "metadata", "labels", "app")
	require.Equal(t, sharedApp, svcLabelApp, "Service must carry the shared app label")

	// Pod containerPort, Service port, and targetPort line up, and the Service
	// port is named so the ServiceMonitor endpoint can reference it.
	svcPort := dig(tpl["Service"], "spec", "ports").([]any)[0].(map[string]any)
	require.EqualValues(t, 80, svcPort["port"])
	require.EqualValues(t, 80, svcPort["targetPort"])
	svcPortName := svcPort["name"]
	require.NotEmpty(t, svcPortName, "Service port must be named so a ServiceMonitor can reference it")

	// Ingress backend must point at the Service by name and port.
	svcName := dig(tpl["Service"], "metadata", "name")
	rules := dig(tpl["Ingress"], "spec", "rules").([]any)
	backend := dig(rules[0], "http", "paths").([]any)[0].(map[string]any)["backend"]
	require.Equal(t, svcName, dig(backend, "service", "name"), "Ingress backend service name")
	require.EqualValues(t, 80, dig(backend, "service", "port", "number"), "Ingress backend service port")

	// NetworkPolicy must target the Pod.
	npApp := dig(tpl["NetworkPolicy"], "spec", "podSelector", "matchLabels", "app")
	require.Equal(t, podApp, npApp, "NetworkPolicy podSelector must match the Pod")

	// ServiceMonitor must select the Service (by its label) and scrape a port
	// that actually exists on that Service (by name).
	smApp := dig(tpl["ServiceMonitor"], "spec", "selector", "matchLabels", "app")
	require.Equal(t, svcLabelApp, smApp, "ServiceMonitor selector must match the Service label")
	smPort := dig(tpl["ServiceMonitor"], "spec", "endpoints").([]any)[0].(map[string]any)["port"]
	require.Equal(t, svcPortName, smPort,
		"ServiceMonitor endpoint port must reference the Service's named port")

	// HPA must target the Deployment by name.
	deployName := dig(tpl["Deployment"], "metadata", "name")
	require.Equal(t, deployName, dig(tpl["HorizontalPodAutoscaler"], "spec", "scaleTargetRef", "name"),
		"HPA scaleTargetRef must name the Deployment")

	// RoleBinding must reference the Role and ServiceAccount, in the same namespace.
	roleName := dig(tpl["Role"], "metadata", "name")
	saName := dig(tpl["ServiceAccount"], "metadata", "name")
	require.Equal(t, roleName, dig(tpl["RoleBinding"], "roleRef", "name"), "RoleBinding roleRef")
	rbSub := dig(tpl["RoleBinding"], "subjects").([]any)[0].(map[string]any)
	require.Equal(t, saName, rbSub["name"], "RoleBinding subject SA name")
	require.Equal(t, "NAMESPACE", rbSub["namespace"], "RoleBinding subject namespace placeholder")

	// ClusterRoleBinding must reference the ClusterRole and ServiceAccount,
	// using the NAMESPACE placeholder so the SA resolves where it is created.
	crName := dig(tpl["ClusterRole"], "metadata", "name")
	require.Equal(t, crName, dig(tpl["ClusterRoleBinding"], "roleRef", "name"), "ClusterRoleBinding roleRef")
	crbSub := dig(tpl["ClusterRoleBinding"], "subjects").([]any)[0].(map[string]any)
	require.Equal(t, saName, crbSub["name"], "ClusterRoleBinding subject SA name")
	require.Equal(t, "NAMESPACE", crbSub["namespace"],
		"ClusterRoleBinding subject namespace must use the NAMESPACE placeholder, matching the ServiceAccount template")
}
