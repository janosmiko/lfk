package heuristic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/security"
)

func podWith(container corev1.Container) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "api-abc"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{container}},
	}
}

func boolPtr(b bool) *bool    { return &b }
func int64Ptr(i int64) *int64 { return &i }

func TestSourceMetadata(t *testing.T) {
	s := New()
	assert.Equal(t, "heuristic", s.Name())
	assert.Equal(t, []security.Category{security.CategoryMisconfig}, s.Categories())
	ok, err := s.IsAvailable(context.Background(), "")
	assert.NoError(t, err)
	assert.True(t, ok, "heuristic is always available")
}

// TestAllChecksRegistered verifies every expected check is wired into the
// allChecks list the Source iterates in Fetch (implemented in Task B8). It
// runs each check against an empty pod/container to confirm the signatures
// line up and the slice is non-nil.
func TestAllChecksRegistered(t *testing.T) {
	assert.Len(t, allChecks, 6, "allChecks should contain the six B1-B4 checks")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c"}}},
	}
	for i, fn := range allChecks {
		assert.NotNil(t, fn, "allChecks[%d] must not be nil", i)
		// Each check must accept an empty pod/container without panicking.
		_ = fn(pod, pod.Spec.Containers[0])
	}
}

func TestCheckPrivileged(t *testing.T) {
	cases := []struct {
		name      string
		container corev1.Container
		want      int
		wantSev   security.Severity
	}{
		{"privileged true", corev1.Container{
			Name: "c", SecurityContext: &corev1.SecurityContext{Privileged: boolPtr(true)},
		}, 1, security.SeverityCritical},
		{"privileged false", corev1.Container{
			Name: "c", SecurityContext: &corev1.SecurityContext{Privileged: boolPtr(false)},
		}, 0, security.SeverityUnknown},
		{"no security context", corev1.Container{Name: "c"}, 0, security.SeverityUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := podWith(tc.container)
			findings := checkPrivileged(pod, tc.container)
			assert.Len(t, findings, tc.want)
			if tc.want == 1 {
				assert.Equal(t, tc.wantSev, findings[0].Severity)
				assert.Equal(t, security.CategoryMisconfig, findings[0].Category)
				assert.Equal(t, "privileged", findings[0].Labels["check"])
			}
		})
	}
}

func TestCheckHostNamespaces(t *testing.T) {
	cases := []struct {
		name    string
		spec    corev1.PodSpec
		wantIDs []string
	}{
		{"hostPID", corev1.PodSpec{HostPID: true, Containers: []corev1.Container{{Name: "c"}}}, []string{"host_pid"}},
		{"hostNetwork", corev1.PodSpec{HostNetwork: true, Containers: []corev1.Container{{Name: "c"}}}, []string{"host_network"}},
		{"hostIPC", corev1.PodSpec{HostIPC: true, Containers: []corev1.Container{{Name: "c"}}}, []string{"host_ipc"}},
		{"all three", corev1.PodSpec{
			HostPID: true, HostNetwork: true, HostIPC: true,
			Containers: []corev1.Container{{Name: "c"}},
		}, []string{"host_pid", "host_network", "host_ipc"}},
		{"none", corev1.PodSpec{Containers: []corev1.Container{{Name: "c"}}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
				Spec:       tc.spec,
			}
			findings := checkHostNamespaces(pod, pod.Spec.Containers[0])
			gotIDs := make([]string, 0, len(findings))
			for _, f := range findings {
				gotIDs = append(gotIDs, f.Labels["check"])
				assert.Equal(t, security.SeverityHigh, f.Severity)
			}
			assert.ElementsMatch(t, tc.wantIDs, gotIDs)
		})
	}
}

func TestCheckHostPath(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c"}},
			Volumes: []corev1.Volume{
				{Name: "etc", VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/etc"},
				}},
				{Name: "data", VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
			},
		},
	}
	findings := checkHostPath(pod, pod.Spec.Containers[0])
	assert.Len(t, findings, 1)
	assert.Equal(t, security.SeverityHigh, findings[0].Severity)
	assert.Equal(t, "host_path", findings[0].Labels["check"])
	assert.Contains(t, findings[0].Summary, "/etc")
}

func TestCheckReadOnlyRootFilesystem(t *testing.T) {
	writable := corev1.Container{Name: "c"}
	explicitFalse := corev1.Container{Name: "c", SecurityContext: &corev1.SecurityContext{
		ReadOnlyRootFilesystem: boolPtr(false),
	}}
	readOnly := corev1.Container{Name: "c", SecurityContext: &corev1.SecurityContext{
		ReadOnlyRootFilesystem: boolPtr(true),
	}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}

	assert.Len(t, checkReadOnlyRootFilesystem(pod, writable), 1)
	assert.Len(t, checkReadOnlyRootFilesystem(pod, explicitFalse), 1)
	assert.Len(t, checkReadOnlyRootFilesystem(pod, readOnly), 0)
}

func TestCheckRunAsRoot(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}
	cases := []struct {
		name      string
		pod       corev1.PodSecurityContext
		container corev1.SecurityContext
		want      int
	}{
		{"no context -> flag", corev1.PodSecurityContext{}, corev1.SecurityContext{}, 1},
		{"runAsUser:0 -> flag", corev1.PodSecurityContext{}, corev1.SecurityContext{RunAsUser: int64Ptr(0)}, 1},
		{"runAsUser:1000 -> clean", corev1.PodSecurityContext{}, corev1.SecurityContext{RunAsUser: int64Ptr(1000)}, 0},
		{"pod runAsNonRoot:true -> clean", corev1.PodSecurityContext{RunAsNonRoot: boolPtr(true)}, corev1.SecurityContext{}, 0},
		{"container runAsNonRoot:true -> clean", corev1.PodSecurityContext{}, corev1.SecurityContext{RunAsNonRoot: boolPtr(true)}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := pod.DeepCopy()
			p.Spec.SecurityContext = &tc.pod
			c := corev1.Container{Name: "c", SecurityContext: &tc.container}
			p.Spec.Containers = []corev1.Container{c}
			findings := checkRunAsRoot(p, c)
			assert.Len(t, findings, tc.want)
		})
	}
}

func TestCheckAllowPrivilegeEscalation(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}
	cases := []struct {
		name string
		sc   *corev1.SecurityContext
		want int
	}{
		{"nil context -> flag", nil, 1},
		{"unset -> flag", &corev1.SecurityContext{}, 1},
		{"true -> flag", &corev1.SecurityContext{AllowPrivilegeEscalation: boolPtr(true)}, 1},
		{"false -> clean", &corev1.SecurityContext{AllowPrivilegeEscalation: boolPtr(false)}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := corev1.Container{Name: "c", SecurityContext: tc.sc}
			findings := checkAllowPrivilegeEscalation(pod, c)
			assert.Len(t, findings, tc.want)
		})
	}
}
