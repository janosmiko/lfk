package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// mapAt returns the whole labels/annotations block so a test can assert both
// halves: the key is gone AND an author-written sibling survived.
func mapAt(t *testing.T, doc string, path ...string) map[string]any {
	t.Helper()
	var obj map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(doc), &obj))
	var cur any = obj
	for _, key := range path {
		m, ok := cur.(map[string]any)
		require.Truef(t, ok, "path %v: %q is not a mapping", path, key)
		cur, ok = m[key]
		require.Truef(t, ok, "path %v: %q missing", path, key)
	}
	m, ok := cur.(map[string]any)
	require.Truef(t, ok, "path %v does not resolve to a mapping", path)
	return m
}

// helmDeploymentYAML carries one example of every category this file guards,
// alongside an author-written sibling in the same block. The values are copied
// from real cluster objects (Helm-managed Rancher / Argo CD workloads).
const helmDeploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: prod
  finalizers:
    - foregroundDeletion
    - example.com/custom-finalizer
  labels:
    app: web
    my-team/owner: platform
    app.kubernetes.io/name: web
    app.kubernetes.io/component: server
    pod-template-hash: 58f99fb59b
    batch.kubernetes.io/controller-uid: abc-123
    app.kubernetes.io/managed-by: Helm
    helm.sh/chart: argo-cd-10.3.0
    heritage: Helm
    release: rancher
    chart: rancher-2.14.2
  annotations:
    team: payments
    kubernetes.io/ingress.class: nginx
    meta.helm.sh/release-name: rancher
    meta.helm.sh/release-namespace: cattle-system
    cni.projectcalico.org/podIP: 10.0.0.5/32
    field.cattle.io/publicEndpoints: '[{"addresses":["10.0.0.5"]}]'
spec:
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
        pod-template-hash: 58f99fb59b
        app.kubernetes.io/managed-by: Helm
      annotations:
        team: payments
        cni.projectcalico.org/podIP: 10.0.0.5/32
    spec:
      containers:
        - name: web
          image: nginx:1.27
`

// TestStripToTemplate_DropsFinalizers_KeepsAuthorMetadata: a template carrying
// a finalizer creates an object that cannot be deleted until some controller
// removes it — and never, if that controller is absent in the target cluster.
func TestStripToTemplate_DropsFinalizers_KeepsAuthorMetadata(t *testing.T) {
	out, err := StripToTemplateWith(helmDeploymentYAML, DefaultTemplateStripSet())
	require.NoError(t, err)

	md := mapAt(t, out, "metadata")
	assert.NotContains(t, md, "finalizers", "finalizers must not survive into a template")
	assert.Equal(t, "web", md["name"], "the author-written name must survive")
}

// TestStripToTemplate_DropsControllerLabels_KeepsAuthorLabels: pod-template-hash
// lands inside spec.template.metadata.labels too, so a template that keeps it
// starts the new Deployment with a stale hash its controller will fight.
func TestStripToTemplate_DropsControllerLabels_KeepsAuthorLabels(t *testing.T) {
	out, err := StripToTemplateWith(helmDeploymentYAML, DefaultTemplateStripSet())
	require.NoError(t, err)

	top := mapAt(t, out, "metadata", "labels")
	assert.NotContains(t, top, "pod-template-hash")
	assert.NotContains(t, top, "batch.kubernetes.io/controller-uid")
	assert.Equal(t, "web", top["app"], "an author-written label must survive")
	assert.Equal(t, "platform", top["my-team/owner"], "an author-written label must survive")

	tmpl := mapAt(t, out, "spec", "template", "metadata", "labels")
	assert.NotContains(t, tmpl, "pod-template-hash", "the pod template carries the stale hash")
	assert.Equal(t, "web", tmpl["app"], "the selector-matching label must survive")
}

// TestStripToTemplate_DropsHelmOwnership_KeepsRecommendedLabels: an object
// claiming a Helm release it is not in may be adopted by the next `helm
// upgrade` or deleted by `helm uninstall`.
func TestStripToTemplate_DropsHelmOwnership_KeepsRecommendedLabels(t *testing.T) {
	out, err := StripToTemplateWith(helmDeploymentYAML, DefaultTemplateStripSet())
	require.NoError(t, err)

	labels := mapAt(t, out, "metadata", "labels")
	for _, key := range []string{"app.kubernetes.io/managed-by", "helm.sh/chart", "heritage", "release", "chart"} {
		assert.NotContainsf(t, labels, key, "Helm ownership label %q must not survive", key)
	}
	assert.Equal(t, "web", labels["app.kubernetes.io/name"], "a recommended label must survive")
	assert.Equal(t, "server", labels["app.kubernetes.io/component"], "a recommended label must survive")

	annotations := mapAt(t, out, "metadata", "annotations")
	assert.NotContains(t, annotations, "meta.helm.sh/release-name")
	assert.NotContains(t, annotations, "meta.helm.sh/release-namespace")
	assert.Equal(t, "payments", annotations["team"], "an author-written annotation must survive")
}

// TestStripToTemplate_KeepsNonHelmManagedBy: the deny-list is value-sensitive.
// A managed-by naming any other tool is the author's own statement of ownership.
func TestStripToTemplate_KeepsNonHelmManagedBy(t *testing.T) {
	const doc = `apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
  labels:
    app.kubernetes.io/managed-by: kustomize
    release: canary
data:
  key: value
`
	out, err := StripToTemplateWith(doc, DefaultTemplateStripSet())
	require.NoError(t, err)

	labels := mapAt(t, out, "metadata", "labels")
	assert.Equal(t, "kustomize", labels["app.kubernetes.io/managed-by"],
		"managed-by goes only when its value is Helm")
	assert.Equal(t, "canary", labels["release"],
		"release goes only on an object that carries a Helm signal")
}

// TestStripToTemplate_DropsVendorRuntimeAnnotations_KeepsAuthorAnnotations:
// these hold live runtime state — a pod IP, a set of public endpoint addresses —
// that is a lie the moment the template is applied anywhere else.
func TestStripToTemplate_DropsVendorRuntimeAnnotations_KeepsAuthorAnnotations(t *testing.T) {
	out, err := StripToTemplateWith(helmDeploymentYAML, DefaultTemplateStripSet())
	require.NoError(t, err)

	top := mapAt(t, out, "metadata", "annotations")
	assert.NotContains(t, top, "cni.projectcalico.org/podIP")
	assert.NotContains(t, top, "field.cattle.io/publicEndpoints")
	assert.Equal(t, "payments", top["team"], "an author-written annotation must survive")
	assert.Equal(t, "nginx", top["kubernetes.io/ingress.class"],
		"a kubernetes.io/ annotation the author wrote must survive a deny-list strip")

	tmpl := mapAt(t, out, "spec", "template", "metadata", "annotations")
	assert.NotContains(t, tmpl, "cni.projectcalico.org/podIP")
	assert.Equal(t, "payments", tmpl["team"], "an author-written annotation must survive")
}
