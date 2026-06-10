package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Helm release secrets are name-sorted (sh.helm.release.v1.<rel>.v1, .v10, .v2,
// ...), so list order puts low revisions first and "v10" before "v2". With equal
// CreationTimestamps (1s granularity collides on fast install/rollback or CI
// upgrades), a strict timestamp comparison keeps Items[0] — the OLDEST revision.
// The version label is the authoritative revision and must win.
func TestLatestHelmReleaseSecretPicksHighestRevision(t *testing.T) {
	ts := metav1.Now() // all identical → forces the tie-break onto the version label
	mk := func(rev string) corev1.Secret {
		return corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "sh.helm.release.v1.app.v" + rev,
				CreationTimestamp: ts,
				Labels:            map[string]string{"version": rev},
			},
		}
	}
	// Name-sorted order as the API returns it: v1, v10, v2.
	items := []corev1.Secret{mk("1"), mk("10"), mk("2")}

	latest := latestHelmReleaseSecret(items)

	assert.Equal(t, "10", latest.Labels["version"], "highest revision must win, not list order")
}
