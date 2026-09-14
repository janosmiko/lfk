package k8s

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const releaseTestManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
`

// helmReleaseFixture builds n retained revisions of one release plus the
// matching metadata objects, mirroring what helm leaves in a namespace.
func helmReleaseFixture(t *testing.T, release string, n int) ([]runtime.Object, []*metav1.PartialObjectMetadata) {
	t.Helper()
	base := time.Now().Add(-time.Duration(n) * time.Hour)

	secrets := make([]runtime.Object, 0, n)
	metas := make([]*metav1.PartialObjectMetadata, 0, n)
	for i := 1; i <= n; i++ {
		version := strconv.Itoa(i)
		name := "sh.helm.release.v1." + release + ".v" + version
		created := base.Add(time.Duration(i) * time.Minute)
		blob := makeHelmBlobWithManifest(
			release, "app", "1.0.0", "1.0.0", "deployed", "Upgrade complete", releaseTestManifest, i,
		)
		secrets = append(secrets, newFakeHelmReleaseSecret(t, name, release, "deployed", version, blob, created))
		metas = append(metas, &metav1.PartialObjectMetadata{
			APIVersion:        "v1",
			Kind:              "Secret",
			Name:              name,
			Namespace:         "default",
			CreationTimestamp: metav1.Time{Time: created},
			Labels: map[string]string{
				"owner":   "helm",
				"name":    release,
				"status":  "deployed",
				"version": version,
			},
		})
	}
	return secrets, metas
}

func countSecretVerbs(actions []k8stesting.Action) map[string]int {
	counts := map[string]int{}
	for _, a := range actions {
		if a.GetResource().Resource != "secrets" {
			continue
		}
		counts[a.GetVerb()]++
	}
	return counts
}

// TestGetHelmManagedResources_ReadsOnlyLatestReleaseSecret guards the cost of
// the resource tree: every retained revision carries the full rendered
// manifest, so a typed LIST downloads all ten to read one.
func TestGetHelmManagedResources_ReadsOnlyLatestReleaseSecret(t *testing.T) {
	secrets, metas := helmReleaseFixture(t, "rel", 10)
	cs := k8sfake.NewClientset(secrets...)
	c := newFakeClient(cs, nil)
	c.injectedMetaClient = newFakeMetaClient(metas...)

	items, err := c.getHelmManagedResources(t.Context(), "", "default", "rel")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Deployment", items[0].Kind)

	verbs := countSecretVerbs(cs.Actions())
	assert.Zero(t, verbs["list"], "release secrets must not be listed in full")
	assert.Equal(t, 1, verbs["get"], "only the latest revision is fetched")

	var fetched string
	for _, a := range cs.Actions() {
		if g, ok := a.(k8stesting.GetAction); ok && a.GetResource().Resource == "secrets" {
			fetched = g.GetName()
		}
	}
	assert.Equal(t, "sh.helm.release.v1.rel.v10", fetched)
}

// TestGetHelmReleaseYAML_ReadsLabelsFromMetadata covers the summary path,
// which needs labels only and never touches the release blob.
func TestGetHelmReleaseYAML_ReadsLabelsFromMetadata(t *testing.T) {
	secrets, metas := helmReleaseFixture(t, "rel", 10)
	cs := k8sfake.NewClientset(secrets...)
	c := newFakeClient(cs, nil)
	c.injectedMetaClient = newFakeMetaClient(metas...)

	out, err := c.GetHelmReleaseYAML(t.Context(), "", "default", "rel")
	require.NoError(t, err)
	assert.Contains(t, out, `version: "10"`)
	assert.Contains(t, out, "secret: sh.helm.release.v1.rel.v10")

	assert.Empty(t, countSecretVerbs(cs.Actions()), "the summary needs labels only")
}

// TestFindLatestHelmReleaseSecret_FallsBackWithoutMetadata keeps the typed
// path working for callers with no metadata client.
func TestFindLatestHelmReleaseSecret_FallsBackWithoutMetadata(t *testing.T) {
	secrets, _ := helmReleaseFixture(t, "rel", 3)
	cs := k8sfake.NewClientset(secrets...)

	secret, ok := findLatestHelmReleaseSecret(t.Context(), cs, nil, "default", "rel")
	require.True(t, ok)
	assert.Equal(t, "sh.helm.release.v1.rel.v3", secret.Name)
	assert.NotEmpty(t, secret.Data["release"])
}
