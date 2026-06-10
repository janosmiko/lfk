package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestGetPodOS_SpecOSName(t *testing.T) {
	winOS := corev1.Windows
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "win-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{OS: &corev1.PodOS{Name: winOS}},
	}
	c := newFakeClient(k8sfake.NewClientset(pod), nil)

	got, err := c.GetPodOS(context.Background(), "ctx", "default", "win-pod")
	require.NoError(t, err)
	assert.Equal(t, "windows", got)
}

func TestGetPodOS_NodeSelectorFallback(t *testing.T) {
	// No spec.os; OS only discoverable via the kubernetes.io/os node selector.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "win-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeSelector: map[string]string{"kubernetes.io/os": "windows"}},
	}
	c := newFakeClient(k8sfake.NewClientset(pod), nil)

	got, err := c.GetPodOS(context.Background(), "ctx", "default", "win-pod")
	require.NoError(t, err)
	assert.Equal(t, "windows", got)
}

func TestGetPodOS_Linux(t *testing.T) {
	linuxOS := corev1.Linux
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "linux-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{OS: &corev1.PodOS{Name: linuxOS}},
	}
	c := newFakeClient(k8sfake.NewClientset(pod), nil)

	got, err := c.GetPodOS(context.Background(), "ctx", "default", "linux-pod")
	require.NoError(t, err)
	assert.Equal(t, "linux", got)
}

func TestGetPodOS_Unknown(t *testing.T) {
	// Neither spec.os nor a kubernetes.io/os selector — returns "".
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "plain-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{},
	}
	c := newFakeClient(k8sfake.NewClientset(pod), nil)

	got, err := c.GetPodOS(context.Background(), "ctx", "default", "plain-pod")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestGetPodOS_NotFound(t *testing.T) {
	c := newFakeClient(k8sfake.NewClientset(), nil)
	_, err := c.GetPodOS(context.Background(), "ctx", "default", "missing")
	assert.Error(t, err)
}
