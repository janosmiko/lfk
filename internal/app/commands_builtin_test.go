package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCov80IsKubectlCommandDirect(t *testing.T) {
	assert.True(t, isKubectlCommand("kubectl get pods"))
	assert.True(t, isKubectlCommand("kubectl"))
	assert.True(t, isKubectlCommand("get pods"))
	assert.False(t, isKubectlCommand("helm install"))
}

func TestCov80ShellQuote(t *testing.T) {
	assert.Equal(t, "'hello'", shellQuote("hello"))
	assert.Equal(t, "'he'\"'\"'llo'", shellQuote("he'llo"))
}

func TestCovLoadEventTimeline(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-pod", "default", "Pod", model.ResourceTypeEntry{})
	cmd := m.loadEventTimeline()
	msg := execCmd(t, cmd)
	result, ok := msg.(eventTimelineMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
}

func TestCovCheckRBAC(t *testing.T) {
	m := baseModelWithFakeClient()
	rt := model.ResourceTypeEntry{
		Kind:     "Pod",
		APIGroup: "",
		Resource: "pods",
	}
	m = withActionCtx(m, "my-pod", "default", "Pod", rt)
	cmd := m.checkRBAC()
	msg := execCmd(t, cmd)
	result, ok := msg.(rbacCheckMsg)
	require.True(t, ok)
	assert.Equal(t, "Pod", result.kind)
	assert.Equal(t, "pods", result.resource)
}

func TestCovLoadCanISAList(t *testing.T) {
	m := baseModelWithFakeClient()
	cmd := m.loadCanISAList()
	msg := execCmd(t, cmd)
	result, ok := msg.(canISAListMsg)
	require.True(t, ok)
	// No SAs exist in the fake client.
	assert.NoError(t, result.err)
}

func TestCovLoadPodStartup(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "nginx"}}},
	}
	m := baseModelWithFakeClient(pod)
	m = withActionCtx(m, "my-pod", "default", "Pod", model.ResourceTypeEntry{})
	cmd := m.loadPodStartup()
	msg := execCmd(t, cmd)
	result, ok := msg.(podStartupMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
}

func TestCovLoadAlertsReturnsCmd(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-pod", "default", "Pod", model.ResourceTypeEntry{})
	cmd := m.loadAlerts()
	// Just verify command is returned. Executing would hit nil pointers in
	// the alerts code that tries Prometheus port-forwarding.
	assert.NotNil(t, cmd)
}

func TestCovLoadNetworkPolicy(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-netpol", "default", "NetworkPolicy", model.ResourceTypeEntry{})
	cmd := m.loadNetworkPolicy()
	msg := execCmd(t, cmd)
	result, ok := msg.(netpolLoadedMsg)
	require.True(t, ok)
	// No NetworkPolicy exists; expect error.
	assert.Error(t, result.err)
}

func TestCovLoadContainerPorts(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "nginx",
					Ports: []corev1.ContainerPort{
						{Name: "http", ContainerPort: 80, Protocol: corev1.ProtocolTCP},
					},
				},
			},
		},
	}
	m := baseModelWithFakeClient(pod)
	m = withActionCtx(m, "my-pod", "default", "Pod", model.ResourceTypeEntry{})
	cmd := m.loadContainerPorts()
	msg := execCmd(t, cmd)
	result, ok := msg.(containerPortsLoadedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
	require.Len(t, result.ports, 1)
	assert.Equal(t, int32(80), result.ports[0].ContainerPort)
}

func TestCovLoadContainerPortsService(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	m := baseModelWithFakeClient(svc)
	m = withActionCtx(m, "my-svc", "default", "Service", model.ResourceTypeEntry{})
	cmd := m.loadContainerPorts()
	msg := execCmd(t, cmd)
	result, ok := msg.(containerPortsLoadedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
}

func TestCovLoadContainerPortsUnsupportedKind(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-job", "default", "Job", model.ResourceTypeEntry{})
	cmd := m.loadContainerPorts()
	msg := execCmd(t, cmd)
	result, ok := msg.(containerPortsLoadedMsg)
	require.True(t, ok)
	assert.Error(t, result.err)
	assert.Contains(t, result.err.Error(), "unsupported kind")
}

func TestCovScheduleDescribeRefresh(t *testing.T) {
	cmd := scheduleDescribeRefresh()
	assert.NotNil(t, cmd)
}

func TestCovOpenInBrowser(t *testing.T) {
	cmd := openInBrowser("https://example.com")
	assert.NotNil(t, cmd)
	// We don't execute because it would open a real browser.
}

func TestCovOpenIngressInBrowserNoSelection(t *testing.T) {
	m := baseModelCov()
	m.middleItems = nil
	ret, cmd := m.openIngressInBrowser()
	model := ret.(Model)
	assert.True(t, model.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovOpenIngressInBrowserNoURL(t *testing.T) {
	m := baseModelCov()
	m = withMiddleItem(m, model.Item{Name: "my-ingress"})
	ret, cmd := m.openIngressInBrowser()
	result := ret.(Model)
	assert.True(t, result.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovOpenIngressInBrowserWithURL(t *testing.T) {
	m := baseModelCov()
	item := model.Item{
		Name: "my-ingress",
		Columns: []model.KeyValue{
			{Key: "__ingress_url", Value: "https://example.com"},
		},
	}
	m = withMiddleItem(m, item)
	ret, cmd := m.openIngressInBrowser()
	result := ret.(Model)
	assert.True(t, result.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovWaitForStderrNil(t *testing.T) {
	m := baseModelCov()
	m.stderrChan = nil
	cmd := m.waitForStderr()
	assert.Nil(t, cmd)
}
