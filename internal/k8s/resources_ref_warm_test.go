package k8s

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// countingDyn is a dynamic.Interface that only answers Get. The fake dynamic
// client serializes every call behind one mutex, so it cannot show whether
// lookups overlap.
type countingDyn struct {
	dynamic.Interface
	onGet func()
}

type countingNamespaceable struct {
	dynamic.NamespaceableResourceInterface
	onGet func()
}

type countingResource struct {
	dynamic.ResourceInterface
	onGet func()
}

func (d countingDyn) Resource(schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return countingNamespaceable{onGet: d.onGet}
}

func (n countingNamespaceable) Namespace(string) dynamic.ResourceInterface {
	return countingResource{onGet: n.onGet}
}

func (r countingResource) Get(_ context.Context, name string, _ metav1.GetOptions, _ ...string) (*unstructured.Unstructured, error) {
	r.onGet()
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
}

func podWithSecretRef(name, secret string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": name, "namespace": "default"},
		"spec": map[string]any{
			"automountServiceAccountToken": false,
			"volumes": []any{
				map[string]any{"name": "v", "secret": map[string]any{"secretName": secret}},
			},
		},
	}}
}

// TestRefChecker_WarmResolvesRefsInParallel guards the tree build's slowest
// step: a release with hundreds of distinct refs used to pay one serial
// round trip each.
func TestRefChecker_WarmResolvesRefsInParallel(t *testing.T) {
	var mu sync.Mutex
	var inFlight, peak, total int

	dyn := countingDyn{onGet: func() {
		mu.Lock()
		inFlight++
		total++
		if inFlight > peak {
			peak = inFlight
		}
		mu.Unlock()

		time.Sleep(20 * time.Millisecond)

		mu.Lock()
		inFlight--
		mu.Unlock()
	}}

	pods := make([]unstructured.Unstructured, 0, 10)
	for i := range 10 {
		pods = append(pods, podWithSecretRef("pod-"+strconv.Itoa(i), "secret-"+strconv.Itoa(i)))
	}

	rc := newRefChecker(t.Context(), dyn, "default")
	rc.warm(pods)

	mu.Lock()
	gotPeak, gotTotal := peak, total
	mu.Unlock()
	assert.GreaterOrEqual(t, gotPeak, 2, "refs must be resolved in parallel")
	assert.Equal(t, 10, gotTotal, "one lookup per distinct ref")

	// Warmed answers are served from the cache, and NotFound still means missing.
	assert.False(t, rc.exists("Secret", "secret-3"))
	mu.Lock()
	afterExists := total
	mu.Unlock()
	assert.Equal(t, 10, afterExists, "a warmed ref costs no second lookup")
}

// TestRefChecker_WarmSkipsOptionalRefs keeps the prefetch as narrow as the
// check it feeds: only required refs can ever be flagged missing.
func TestRefChecker_WarmSkipsOptionalRefs(t *testing.T) {
	var mu sync.Mutex
	var total int

	dyn := countingDyn{onGet: func() {
		mu.Lock()
		total++
		mu.Unlock()
	}}

	pod := podWithSecretRef("pod-0", "required-secret")
	spec, _ := pod.Object["spec"].(map[string]any)
	spec["volumes"] = append(spec["volumes"].([]any), map[string]any{
		"name":      "opt",
		"configMap": map[string]any{"name": "optional-cm", "optional": true},
	})

	rc := newRefChecker(t.Context(), dyn, "default")
	rc.warm([]unstructured.Unstructured{pod})

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, total)
}
