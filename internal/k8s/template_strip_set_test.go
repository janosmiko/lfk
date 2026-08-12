package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withoutCategory returns the default strip set with one category turned off,
// which is what the overlay does when the user unticks a row.
func withoutCategory(c TemplateCategory) TemplateStripSet {
	set := DefaultTemplateStripSet()
	set[c] = false
	return set
}

const secretYAML = `apiVersion: v1
kind: Secret
metadata:
  name: db
  namespace: prod
type: Opaque
data:
  password: c3VwZXJzZWNyZXQ=
`

func TestStripToTemplateWith_DefaultSetMatchesStripToTemplate(t *testing.T) {
	want, err := StripToTemplate(helmDeploymentYAML)
	require.NoError(t, err)
	got, err := StripToTemplateWith(helmDeploymentYAML, DefaultTemplateStripSet())
	require.NoError(t, err)

	assert.Equal(t, want, got, "the default set is what the two-keystroke path already does")
}

func TestStripToTemplateWith_KeepsNamespace_WhenCategoryDisabled(t *testing.T) {
	out, err := StripToTemplateWith(helmDeploymentYAML, withoutCategory(TemplateNamespace))
	require.NoError(t, err)

	md := mapAt(t, out, "metadata")
	assert.Equal(t, "prod", md["namespace"])
	assert.NotContains(t, md, "finalizers", "a locked field goes regardless of the category set")
}

func TestStripToTemplateWith_StripsAllLabels_KeepsSelectorLabels(t *testing.T) {
	set := DefaultTemplateStripSet()
	set[TemplateLabels] = true
	out, err := StripToTemplateWith(helmDeploymentYAML, set)
	require.NoError(t, err)

	md := mapAt(t, out, "metadata")
	assert.NotContains(t, md, "labels", "every author label goes when the category is on")

	tmpl := mapAt(t, out, "spec", "template", "metadata", "labels")
	assert.Equal(t, "web", tmpl["app"],
		"a pod-template label the selector demands must survive, or the workload selects nothing")
	assert.Equal(t, "web", mapAt(t, out, "spec", "selector", "matchLabels")["app"])
}

func TestStripToTemplateWith_StripsAllAnnotations_KeepsSpec(t *testing.T) {
	set := DefaultTemplateStripSet()
	set[TemplateAnnotations] = true
	out, err := StripToTemplateWith(helmDeploymentYAML, set)
	require.NoError(t, err)

	assert.NotContains(t, mapAt(t, out, "metadata"), "annotations")
	assert.NotContains(t, mapAt(t, out, "spec", "template", "metadata"), "annotations")
	assert.Equal(t, "web", mapAt(t, out, "metadata")["name"], "the rest of the object is untouched")
}

func TestStripToTemplateWith_KeepsHelmMarkers_WhenCategoryDisabled(t *testing.T) {
	out, err := StripToTemplateWith(helmDeploymentYAML, withoutCategory(TemplateHelmOwnership))
	require.NoError(t, err)

	labels := mapAt(t, out, "metadata", "labels")
	assert.Equal(t, "Helm", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "rancher", mapAt(t, out, "metadata", "annotations")["meta.helm.sh/release-name"])
	assert.NotContains(t, labels, "pod-template-hash", "a locked label goes regardless of the category set")
}

func TestStripToTemplateWith_KeepsVendorAnnotations_WhenCategoryDisabled(t *testing.T) {
	out, err := StripToTemplateWith(helmDeploymentYAML, withoutCategory(TemplateVendorRuntime))
	require.NoError(t, err)

	annotations := mapAt(t, out, "metadata", "annotations")
	assert.Equal(t, "10.0.0.5/32", annotations["cni.projectcalico.org/podIP"])
	assert.NotContains(t, annotations, "meta.helm.sh/release-name",
		"an unrelated category is unaffected")
}

func TestStripToTemplateWith_KeepsSecretValues_WhenCategoryDisabled(t *testing.T) {
	set := withoutCategory(TemplateSecretValues)
	out, err := StripToTemplateWith(secretYAML, set)
	require.NoError(t, err)

	assert.Equal(t, "c3VwZXJzZWNyZXQ=", mapAt(t, out, "data")["password"])
	assert.False(t, TemplateRedactsValues("Secret", set),
		"the exporter must not promise a redaction it did not perform")
	assert.True(t, TemplateRedactsValues("Secret", DefaultTemplateStripSet()))
}

func TestStripToTemplateWith_EmptySet_StillStripsLockedFields(t *testing.T) {
	out, err := StripToTemplateWith(helmDeploymentYAML, TemplateStripSet{})
	require.NoError(t, err)

	md := mapAt(t, out, "metadata")
	assert.NotContains(t, md, "finalizers")
	assert.NotContains(t, mapAt(t, out, "metadata", "labels"), "pod-template-hash")
	assert.Equal(t, "prod", md["namespace"], "an unticked category is kept")
	assert.Equal(t, "payments", mapAt(t, out, "metadata", "annotations")["team"])
}
