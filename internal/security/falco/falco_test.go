package falco

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/security"
)

func TestSourceName(t *testing.T) {
	assert.Equal(t, "falco", New().Name())
}

func TestSourceCategories(t *testing.T) {
	cats := New().Categories()
	assert.Contains(t, cats, security.CategoryPolicy)
}

func TestIsAvailableNilClient(t *testing.T) {
	ok, err := New().IsAvailable(context.Background(), "ctx")
	assert.False(t, ok)
	assert.NoError(t, err)
}

func TestFetchNilClient(t *testing.T) {
	findings, err := New().Fetch(context.Background(), "ctx", "")
	assert.Nil(t, findings)
	assert.NoError(t, err)
}

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  security.Severity
	}{
		{"EMERGENCY", security.SeverityCritical},
		{"ALERT", security.SeverityCritical},
		{"CRITICAL", security.SeverityCritical},
		{"ERROR", security.SeverityHigh},
		{"WARNING", security.SeverityMedium},
		{"NOTICE", security.SeverityLow},
		{"INFORMATIONAL", security.SeverityLow},
		{"DEBUG", security.SeverityLow},
		{"unknown", security.SeverityUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, parseSeverity(tt.input))
		})
	}
}

func TestParseEvent(t *testing.T) {
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "prod",
			UID:       types.UID("abc-123"),
			Annotations: map[string]string{
				"falco.org/rule":     "Terminal Shell in Container",
				"falco.org/priority": "WARNING",
				"falco.org/output":   "A shell was spawned in container",
			},
		},
		Reason:  "FalcoAlert",
		Message: "A shell was spawned in container",
		Type:    "Warning",
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      "web-abc",
			Namespace: "prod",
		},
	}

	findings := parseEvent(ev)
	require.Len(t, findings, 1)
	f := findings[0]
	assert.Equal(t, "falco", f.Source)
	assert.Equal(t, security.SeverityMedium, f.Severity)
	assert.Equal(t, "Terminal Shell in Container", f.Title)
	assert.Equal(t, "Pod", f.Resource.Kind)
	assert.Equal(t, "web-abc", f.Resource.Name)
	assert.Equal(t, "prod", f.Resource.Namespace)
	assert.Equal(t, "A shell was spawned in container", f.Summary)
	assert.Equal(t, "Terminal Shell in Container", f.Labels["rule"])
}

func TestParseEventMinimal(t *testing.T) {
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			UID:       types.UID("xyz"),
		},
		Reason:  "SomeRule",
		Message: "something happened",
		Type:    "Warning",
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Deployment",
			Name:      "api",
			Namespace: "default",
		},
	}

	findings := parseEvent(ev)
	require.Len(t, findings, 1)
	assert.Equal(t, "SomeRule", findings[0].Labels["rule"])
	assert.Equal(t, security.SeverityMedium, findings[0].Severity)
}

func TestParseLogLine(t *testing.T) {
	line := []byte(`{"output":"15:30:00.000 Warning Terminal shell in container (user=root container=app pod=web-abc ns=prod)","priority":"Warning","rule":"Terminal shell in container","time":"2026-04-14T15:30:00.000Z","source":"syscall","tags":["container","shell","mitre_execution"],"output_fields":{"k8s.ns.name":"prod","k8s.pod.name":"web-abc","container.name":"app"}}`)

	findings := parseLogLine(line, "")
	require.Len(t, findings, 1)
	f := findings[0]
	assert.Equal(t, "falco", f.Source)
	assert.Equal(t, security.SeverityMedium, f.Severity)
	assert.Equal(t, "Terminal shell in container", f.Title)
	assert.Equal(t, "Pod", f.Resource.Kind)
	assert.Equal(t, "web-abc", f.Resource.Name)
	assert.Equal(t, "prod", f.Resource.Namespace)
	assert.Equal(t, "app", f.Resource.Container)
	assert.Contains(t, f.Summary, "Terminal shell in container")
	assert.Equal(t, "Terminal shell in container", f.Labels["rule"])
	assert.Equal(t, "true", f.Labels["tag:mitre_execution"])
}

func TestParseLogLineNamespaceFilter(t *testing.T) {
	line := []byte(`{"output":"alert","priority":"Error","rule":"test","time":"2026-01-01T00:00:00Z","output_fields":{"k8s.ns.name":"kube-system","k8s.pod.name":"etcd"}}`)

	// Matching namespace.
	findings := parseLogLine(line, "kube-system")
	assert.Len(t, findings, 1)

	// Non-matching namespace.
	findings = parseLogLine(line, "prod")
	assert.Len(t, findings, 0)
}

func TestParseLogLineInvalidJSON(t *testing.T) {
	assert.Nil(t, parseLogLine([]byte("not json"), ""))
	assert.Nil(t, parseLogLine([]byte(`{"rule":""}`), ""))
	assert.Nil(t, parseLogLine([]byte(`{"some":"info log"}`), ""))
}

func TestFetchWithFakeClient(t *testing.T) {
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "prod",
			Name:      "falco-alert-1",
			UID:       types.UID("uid-1"),
			Annotations: map[string]string{
				"falco.org/rule":     "Unexpected Outbound Connection",
				"falco.org/priority": "ERROR",
			},
		},
		Reason:            "FalcoAlert",
		Message:           "Outbound connection to suspicious IP",
		Type:              "Warning",
		ReportingInstance: "falcosidekick",
		Source:            corev1.EventSource{Component: "falcosidekick"},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod", Name: "api-xyz", Namespace: "prod",
		},
	}

	// Falco DaemonSet pod (needed for IsAvailable check).
	falcoPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "falco",
			Name:      "falco-node1",
			Labels:    map[string]string{"app.kubernetes.io/name": "falco"},
		},
	}
	client := kubefake.NewSimpleClientset(ev, falcoPod)
	s := NewWithClient(client)

	// IsAvailable: checks for pods with the falco label.
	ok, err := s.IsAvailable(context.Background(), "ctx")
	require.NoError(t, err)
	assert.True(t, ok)

	findings, err := s.Fetch(context.Background(), "ctx", "prod")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(findings), 1)
	assert.Equal(t, "Unexpected Outbound Connection", findings[0].Title)
}
