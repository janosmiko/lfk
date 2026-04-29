package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/model"
)

// --- preferredCRDVersion ---

func TestPreferredCRDVersion(t *testing.T) {
	tests := []struct {
		name string
		spec map[string]any
		obj  map[string]any
		want string
	}{
		{
			name: "picks storage+served version",
			spec: map[string]any{
				"versions": []any{
					map[string]any{"name": "v1alpha1", "served": true, "storage": false},
					map[string]any{"name": "v1", "served": true, "storage": true},
				},
			},
			obj:  map[string]any{},
			want: "v1",
		},
		{
			name: "first served when no storage",
			spec: map[string]any{
				"versions": []any{
					map[string]any{"name": "v1beta1", "served": true, "storage": false},
					map[string]any{"name": "v1alpha1", "served": true, "storage": false},
				},
			},
			obj:  map[string]any{},
			want: "v1beta1",
		},
		{
			name: "storage takes priority over first served",
			spec: map[string]any{
				"versions": []any{
					map[string]any{"name": "v1alpha1", "served": true, "storage": false},
					map[string]any{"name": "v2", "served": true, "storage": true},
					map[string]any{"name": "v1", "served": true, "storage": false},
				},
			},
			obj:  map[string]any{},
			want: "v2",
		},
		{
			name: "skips non-served versions",
			spec: map[string]any{
				"versions": []any{
					map[string]any{"name": "v1alpha1", "served": false, "storage": false},
					map[string]any{"name": "v1", "served": true, "storage": false},
				},
			},
			obj:  map[string]any{},
			want: "v1",
		},
		{
			name: "skips entries with empty name",
			spec: map[string]any{
				"versions": []any{
					map[string]any{"name": "", "served": true, "storage": true},
					map[string]any{"name": "v1", "served": true, "storage": false},
				},
			},
			obj:  map[string]any{},
			want: "v1",
		},
		{
			name: "skips non-map entries",
			spec: map[string]any{
				"versions": []any{
					"not-a-map",
					map[string]any{"name": "v1", "served": true, "storage": false},
				},
			},
			obj:  map[string]any{},
			want: "v1",
		},
		{
			name: "falls back to status.storedVersions",
			spec: map[string]any{},
			obj: map[string]any{
				"status": map[string]any{
					"storedVersions": []any{"v1beta2", "v1beta1"},
				},
			},
			want: "v1beta2",
		},
		{
			name: "storedVersions skips empty first element",
			spec: map[string]any{},
			obj: map[string]any{
				"status": map[string]any{
					"storedVersions": []any{"", "v1"},
				},
			},
			want: "v1",
		},
		{
			name: "defaults to v1 when nothing found",
			spec: map[string]any{},
			obj:  map[string]any{},
			want: "v1",
		},
		{
			name: "defaults to v1 with empty versions list",
			spec: map[string]any{
				"versions": []any{},
			},
			obj:  map[string]any{},
			want: "v1",
		},
		{
			name: "defaults to v1 when all versions have empty names",
			spec: map[string]any{
				"versions": []any{
					map[string]any{"name": "", "served": true, "storage": true},
				},
			},
			obj: map[string]any{
				"status": map[string]any{
					"storedVersions": []any{},
				},
			},
			want: "v1",
		},
		{
			name: "storage must also be served",
			spec: map[string]any{
				"versions": []any{
					map[string]any{"name": "v2", "served": false, "storage": true},
					map[string]any{"name": "v1", "served": true, "storage": false},
				},
			},
			obj:  map[string]any{},
			want: "v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preferredCRDVersion(tt.spec, tt.obj)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- containerStateString ---

func TestContainerStateString(t *testing.T) {
	tests := []struct {
		name       string
		ready      bool
		waiting    *corev1.ContainerStateWaiting
		running    *corev1.ContainerStateRunning
		terminated *corev1.ContainerStateTerminated
		want       string
	}{
		{
			name:  "running and ready",
			ready: true,
			running: &corev1.ContainerStateRunning{
				StartedAt: metav1.Now(),
			},
			want: "Running",
		},
		{
			name:  "running but not ready",
			ready: false,
			running: &corev1.ContainerStateRunning{
				StartedAt: metav1.Now(),
			},
			want: "NotReady",
		},
		{
			name:    "waiting",
			waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
			want:    "Waiting",
		},
		{
			name:       "terminated with Completed",
			terminated: &corev1.ContainerStateTerminated{Reason: "Completed", ExitCode: 0},
			want:       "Completed",
		},
		{
			name:       "terminated with other reason",
			terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 1},
			want:       "Terminated",
		},
		{
			name:       "terminated with empty reason",
			terminated: &corev1.ContainerStateTerminated{ExitCode: 137},
			want:       "Terminated",
		},
		{
			name: "all nil returns Unknown",
			want: "Unknown",
		},
		{
			name:    "running takes priority over waiting",
			ready:   true,
			running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()},
			waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"},
			want:    "Running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containerStateString(tt.ready, tt.waiting, tt.running, tt.terminated)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- extractContainerNotReadyReason ---

func TestExtractContainerNotReadyReason(t *testing.T) {
	tests := []struct {
		name              string
		containerStatuses []any
		want              string
	}{
		{
			name: "waiting with CrashLoopBackOff",
			containerStatuses: []any{
				map[string]any{
					"ready": false,
					"state": map[string]any{
						"waiting": map[string]any{
							"reason": "CrashLoopBackOff",
						},
					},
				},
			},
			want: "CrashLoopBackOff",
		},
		{
			name: "waiting with ImagePullBackOff",
			containerStatuses: []any{
				map[string]any{
					"ready": false,
					"state": map[string]any{
						"waiting": map[string]any{
							"reason": "ImagePullBackOff",
						},
					},
				},
			},
			want: "ImagePullBackOff",
		},
		{
			name: "terminated with OOMKilled",
			containerStatuses: []any{
				map[string]any{
					"ready": false,
					"state": map[string]any{
						"terminated": map[string]any{
							"reason": "OOMKilled",
						},
					},
				},
			},
			want: "OOMKilled",
		},
		{
			name: "skips ready containers",
			containerStatuses: []any{
				map[string]any{
					"ready": true,
					"state": map[string]any{
						"waiting": map[string]any{
							"reason": "ShouldBeSkipped",
						},
					},
				},
				map[string]any{
					"ready": false,
					"state": map[string]any{
						"waiting": map[string]any{
							"reason": "ErrImagePull",
						},
					},
				},
			},
			want: "ErrImagePull",
		},
		{
			name: "returns first not-ready container reason",
			containerStatuses: []any{
				map[string]any{
					"ready": false,
					"state": map[string]any{
						"waiting": map[string]any{
							"reason": "CrashLoopBackOff",
						},
					},
				},
				map[string]any{
					"ready": false,
					"state": map[string]any{
						"terminated": map[string]any{
							"reason": "OOMKilled",
						},
					},
				},
			},
			want: "CrashLoopBackOff",
		},
		{
			name:              "empty container statuses",
			containerStatuses: []any{},
			want:              "",
		},
		{
			name:              "nil container statuses",
			containerStatuses: nil,
			want:              "",
		},
		{
			name: "non-map entry is skipped",
			containerStatuses: []any{
				"not-a-map",
				map[string]any{
					"ready": false,
					"state": map[string]any{
						"waiting": map[string]any{
							"reason": "ContainerCreating",
						},
					},
				},
			},
			want: "ContainerCreating",
		},
		{
			name: "no state field returns empty",
			containerStatuses: []any{
				map[string]any{
					"ready": false,
				},
			},
			want: "",
		},
		{
			name: "nil state returns empty",
			containerStatuses: []any{
				map[string]any{
					"ready": false,
					"state": nil,
				},
			},
			want: "",
		},
		{
			name: "waiting with empty reason skips to terminated",
			containerStatuses: []any{
				map[string]any{
					"ready": false,
					"state": map[string]any{
						"waiting": map[string]any{
							"reason": "",
						},
						"terminated": map[string]any{
							"reason": "Error",
						},
					},
				},
			},
			want: "Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractContainerNotReadyReason(tt.containerStatuses)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- buildContainerItem ---

func TestBuildContainerItem(t *testing.T) {
	t.Run("regular container running and ready", func(t *testing.T) {
		startedAt := metav1.NewTime(time.Now().Add(-10 * time.Minute))
		c := corev1.Container{
			Name:  "app",
			Image: "nginx:latest",
			Ports: []corev1.ContainerPort{
				{Name: "http", ContainerPort: 80, Protocol: corev1.ProtocolTCP},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		}
		statuses := []corev1.ContainerStatus{
			{
				Name:         "app",
				Ready:        true,
				RestartCount: 0,
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{StartedAt: startedAt},
				},
			},
		}

		item := buildContainerItem(c, statuses, false, false)

		assert.Equal(t, "app", item.Name)
		assert.Equal(t, "Container", item.Kind)
		assert.Equal(t, "nginx:latest", item.Extra)
		assert.Equal(t, "Containers", item.Category)
		assert.Equal(t, "Running", item.Status)
		assert.Equal(t, "true", item.Ready)
		assert.Equal(t, "0", item.Restarts)
		assert.False(t, item.CreatedAt.IsZero())

		// Check resource columns.
		colMap := columnsToMap(item.Columns)
		assert.Equal(t, "100m", colMap["CPU Request"])
		assert.Equal(t, "128Mi", colMap["Memory Request"])
		assert.Equal(t, "500m", colMap["CPU Limit"])
		assert.Equal(t, "256Mi", colMap["Memory Limit"])
		assert.Equal(t, "http:80/TCP", colMap["Ports"])
	})

	t.Run("init container", func(t *testing.T) {
		c := corev1.Container{Name: "init-db", Image: "busybox"}
		item := buildContainerItem(c, nil, true, false)

		assert.Equal(t, "Init Containers", item.Category)
		assert.Equal(t, "Init", item.Status)
	})

	t.Run("sidecar container", func(t *testing.T) {
		c := corev1.Container{Name: "envoy", Image: "envoyproxy/envoy:v1.28"}
		item := buildContainerItem(c, nil, false, true)

		assert.Equal(t, "Sidecar Containers", item.Category)
		assert.Equal(t, "Waiting", item.Status)
	})

	t.Run("container with waiting state", func(t *testing.T) {
		c := corev1.Container{Name: "app", Image: "myapp:v1"}
		statuses := []corev1.ContainerStatus{
			{
				Name:  "app",
				Ready: false,
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "CrashLoopBackOff",
						Message: "back-off 5m0s restarting failed container",
					},
				},
				RestartCount: 5,
			},
		}

		item := buildContainerItem(c, statuses, false, false)

		assert.Equal(t, "CrashLoopBackOff", item.Status)
		assert.Equal(t, "false", item.Ready)
		assert.Equal(t, "5", item.Restarts)

		colMap := columnsToMap(item.Columns)
		assert.Equal(t, "CrashLoopBackOff", colMap["Reason"])
		assert.Equal(t, "back-off 5m0s restarting failed container", colMap["Message"])
	})

	t.Run("container with terminated state", func(t *testing.T) {
		c := corev1.Container{Name: "worker", Image: "worker:v2"}
		statuses := []corev1.ContainerStatus{
			{
				Name:  "worker",
				Ready: false,
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Reason:   "OOMKilled",
						Message:  "memory limit exceeded",
						ExitCode: 137,
					},
				},
				RestartCount: 3,
			},
		}

		item := buildContainerItem(c, statuses, false, false)

		assert.Equal(t, "OOMKilled", item.Status)
		colMap := columnsToMap(item.Columns)
		assert.Equal(t, "OOMKilled", colMap["Reason"])
		assert.Equal(t, "memory limit exceeded", colMap["Message"])
		assert.Equal(t, "137", colMap["Exit Code"])
	})

	t.Run("container with last termination state", func(t *testing.T) {
		c := corev1.Container{Name: "app", Image: "app:v1"}
		statuses := []corev1.ContainerStatus{
			{
				Name:  "app",
				Ready: true,
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()},
				},
				LastTerminationState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Reason:   "OOMKilled",
						ExitCode: 137,
					},
				},
				RestartCount: 2,
			},
		}

		item := buildContainerItem(c, statuses, false, false)

		assert.Equal(t, "Running", item.Status)
		colMap := columnsToMap(item.Columns)
		assert.Equal(t, "OOMKilled", colMap["Last Terminated"])
		assert.Equal(t, "137", colMap["Last Exit Code"])
	})

	t.Run("no matching status uses defaults", func(t *testing.T) {
		c := corev1.Container{Name: "app", Image: "app:v1"}
		statuses := []corev1.ContainerStatus{
			{
				Name:  "other-container",
				Ready: true,
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()},
				},
			},
		}

		item := buildContainerItem(c, statuses, false, false)

		assert.Equal(t, "Waiting", item.Status)
		assert.Empty(t, item.Ready)
		assert.Empty(t, item.Restarts)
	})

	t.Run("multiple ports formatted correctly", func(t *testing.T) {
		c := corev1.Container{
			Name:  "app",
			Image: "app:v1",
			Ports: []corev1.ContainerPort{
				{Name: "http", ContainerPort: 80, Protocol: corev1.ProtocolTCP},
				{ContainerPort: 443, Protocol: corev1.ProtocolTCP},
				{Name: "metrics", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
			},
		}

		item := buildContainerItem(c, nil, false, false)

		colMap := columnsToMap(item.Columns)
		assert.Equal(t, "http:80/TCP, 443/TCP, metrics:9090/TCP", colMap["Ports"])
	})

	t.Run("last termination with zero exit code omits exit code column", func(t *testing.T) {
		c := corev1.Container{Name: "app", Image: "app:v1"}
		statuses := []corev1.ContainerStatus{
			{
				Name:  "app",
				Ready: true,
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()},
				},
				LastTerminationState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Reason:   "Completed",
						ExitCode: 0,
					},
				},
			},
		}

		item := buildContainerItem(c, statuses, false, false)

		colMap := columnsToMap(item.Columns)
		assert.Equal(t, "Completed", colMap["Last Terminated"])
		_, hasExitCode := colMap["Last Exit Code"]
		assert.False(t, hasExitCode, "should not include Last Exit Code for exit code 0")
	})
}

// columnsToMap converts a slice of KeyValue to a map for easier assertion.
// When duplicate keys exist, the last value wins.
func columnsToMap(cols []model.KeyValue) map[string]string {
	m := make(map[string]string, len(cols))
	for _, kv := range cols {
		m[kv.Key] = kv.Value
	}
	return m
}

// --- extractStatus (Suspended / FluxCD) ---

func TestExtractStatus_Suspended(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"suspend": true,
		},
		"status": map[string]any{},
	}
	assert.Equal(t, "Suspended", extractStatus(obj))
}

func TestExtractStatus_NotSuspended(t *testing.T) {
	// suspend is false, no conditions, no phase, should return empty.
	obj := map[string]any{
		"spec": map[string]any{
			"suspend": false,
		},
		"status": map[string]any{},
	}
	assert.Equal(t, "", extractStatus(obj))
}

func TestExtractStatus_ConditionsWithInvalidEntry(t *testing.T) {
	obj := map[string]any{
		"status": map[string]any{
			"conditions": []any{
				"not-a-map",
				map[string]any{
					"type":   "Ready",
					"status": "True",
				},
			},
		},
	}
	// Non-map entry should be skipped; last valid condition is "Ready".
	assert.Equal(t, "Ready", extractStatus(obj))
}
