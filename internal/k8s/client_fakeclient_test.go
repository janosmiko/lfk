package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/model"
)

// newFakeClient creates a Client with injected fake clientset and dynamic client.
func newFakeClient(cs *k8sfake.Clientset, dc *dynamicfake.FakeDynamicClient) *Client {
	return &Client{
		testClientset: cs,
		testDynClient: dc,
	}
}

// --- GetSecretData ---

func TestGetSecretData(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		Data: map[string][]byte{
			"password": []byte("s3cret"),
			"username": []byte("admin"),
		},
	}
	cs := k8sfake.NewClientset(secret)
	c := newFakeClient(cs, nil)

	result, err := c.GetSecretData(context.Background(), "", "default", "my-secret")
	require.NoError(t, err)
	assert.Equal(t, []string{"password", "username"}, result.Keys)
	assert.Equal(t, "s3cret", result.Data["password"])
	assert.Equal(t, "admin", result.Data["username"])
}

func TestGetSecretData_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	_, err := c.GetSecretData(context.Background(), "", "default", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting secret")
}

// --- UpdateSecretData ---

func TestUpdateSecretData(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		Data:       map[string][]byte{"old-key": []byte("old-value")},
	}
	cs := k8sfake.NewClientset(secret)
	c := newFakeClient(cs, nil)

	err := c.UpdateSecretData("", "default", "my-secret", map[string]string{
		"new-key": "new-value",
	})
	require.NoError(t, err)

	// Verify the update took effect.
	updated, err := cs.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []byte("new-value"), updated.Data["new-key"])
	_, hasOld := updated.Data["old-key"]
	assert.False(t, hasOld, "old key should be removed")
}

func TestUpdateSecretData_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	err := c.UpdateSecretData("", "default", "missing", map[string]string{"k": "v"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting secret for update")
}

// --- GetConfigMapData ---

func TestGetConfigMapData(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
		Data: map[string]string{
			"config.yaml": "key: value",
			"app.conf":    "setting=1",
		},
	}
	cs := k8sfake.NewClientset(cm)
	c := newFakeClient(cs, nil)

	result, err := c.GetConfigMapData(context.Background(), "", "default", "my-cm")
	require.NoError(t, err)
	assert.Equal(t, []string{"app.conf", "config.yaml"}, result.Keys)
	assert.Equal(t, "key: value", result.Data["config.yaml"])
}

func TestGetConfigMapData_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	_, err := c.GetConfigMapData(context.Background(), "", "default", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting configmap")
}

// --- UpdateConfigMapData ---

func TestUpdateConfigMapData(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
		Data:       map[string]string{"old": "data"},
	}
	cs := k8sfake.NewClientset(cm)
	c := newFakeClient(cs, nil)

	err := c.UpdateConfigMapData("", "default", "my-cm", map[string]string{
		"new": "data",
	})
	require.NoError(t, err)

	updated, err := cs.CoreV1().ConfigMaps("default").Get(context.Background(), "my-cm", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "data", updated.Data["new"])
	_, hasOld := updated.Data["old"]
	assert.False(t, hasOld)
}

func TestUpdateConfigMapData_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	err := c.UpdateConfigMapData("", "default", "missing", map[string]string{"k": "v"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting configmap for update")
}

// --- GetPodResourceRequests ---

func TestGetPodResourceRequests(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "app",
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
				},
				{
					Name: "sidecar",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)

	cpuReq, cpuLim, memReq, memLim, err := c.GetPodResourceRequests(context.Background(), "", "default", "my-pod")
	require.NoError(t, err)
	assert.Equal(t, int64(150), cpuReq) // 100m + 50m
	assert.Equal(t, int64(700), cpuLim) // 500m + 200m
	assert.Equal(t, int64(128*1024*1024+64*1024*1024), memReq)
	assert.Equal(t, int64(256*1024*1024+128*1024*1024), memLim)
}

func TestGetPodResourceRequests_NoResources(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "bare-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app"}},
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)

	cpuReq, cpuLim, memReq, memLim, err := c.GetPodResourceRequests(context.Background(), "", "default", "bare-pod")
	require.NoError(t, err)
	assert.Equal(t, int64(0), cpuReq)
	assert.Equal(t, int64(0), cpuLim)
	assert.Equal(t, int64(0), memReq)
	assert.Equal(t, int64(0), memLim)
}

func TestGetPodResourceRequests_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	_, _, _, _, err := c.GetPodResourceRequests(context.Background(), "", "default", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting pod")
}

// --- TriggerCronJob ---

func TestTriggerCronJob(t *testing.T) {
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cron", Namespace: "default"},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers:    []corev1.Container{{Name: "worker", Image: "busybox"}},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(cronJob)
	c := newFakeClient(cs, nil)

	jobName, err := c.TriggerCronJob(context.Background(), "", "default", "my-cron")
	require.NoError(t, err)
	assert.Contains(t, jobName, "my-cron-manual-")

	// Verify the job was created.
	jobs, err := cs.BatchV1().Jobs("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, jobs.Items, 1)
	assert.Equal(t, "busybox", jobs.Items[0].Spec.Template.Spec.Containers[0].Image)
}

func TestTriggerCronJob_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	_, err := c.TriggerCronJob(context.Background(), "", "default", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting cronjob")
}

// --- GetContainers ---

func TestGetContainers(t *testing.T) {
	alwaysRestart := corev1.ContainerRestartPolicyAlways
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "init-db", Image: "busybox:latest"},
				{Name: "sidecar-init", Image: "envoy:latest", RestartPolicy: &alwaysRestart},
			},
			Containers: []corev1.Container{
				{Name: "app", Image: "myapp:v1"},
			},
		},
		Status: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{
				{Name: "init-db", Ready: false, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed"}}},
				{Name: "sidecar-init", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
			},
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)

	items, err := c.GetContainers(context.Background(), "", "default", "my-pod")
	require.NoError(t, err)
	assert.Len(t, items, 3)
	// Init containers come first.
	assert.Equal(t, "init-db", items[0].Name)
	assert.Equal(t, "sidecar-init", items[1].Name)
	assert.Equal(t, "app", items[2].Name)
}

func TestGetContainers_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	_, err := c.GetContainers(context.Background(), "", "default", "missing")
	require.Error(t, err)
}

// --- GetContainerPorts ---

func TestGetContainerPorts(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "app",
					Ports: []corev1.ContainerPort{
						{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
						{Name: "metrics", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
					},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)

	ports, err := c.GetContainerPorts(context.Background(), "", "default", "my-pod")
	require.NoError(t, err)
	assert.Len(t, ports, 2)
	assert.Equal(t, int32(8080), ports[0].ContainerPort)
	assert.Equal(t, "http", ports[0].Name)
}

// --- GetServicePorts ---

func TestGetServicePorts(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
				{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	cs := k8sfake.NewClientset(svc)
	c := newFakeClient(cs, nil)

	ports, err := c.GetServicePorts(context.Background(), "", "default", "my-svc")
	require.NoError(t, err)
	assert.Len(t, ports, 2)
	assert.Equal(t, int32(80), ports[0].ContainerPort)
}

// --- GetDeploymentPorts ---

func TestGetDeploymentPorts(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "my-deploy", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(dep)
	c := newFakeClient(cs, nil)

	ports, err := c.GetDeploymentPorts(context.Background(), "", "default", "my-deploy")
	require.NoError(t, err)
	assert.Len(t, ports, 1)
	assert.Equal(t, int32(8080), ports[0].ContainerPort)
}

// --- GetStatefulSetPorts ---

func TestGetStatefulSetPorts(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sts", Namespace: "default"},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "sts"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "sts"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "db",
							Ports: []corev1.ContainerPort{
								{Name: "pg", ContainerPort: 5432, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(sts)
	c := newFakeClient(cs, nil)

	ports, err := c.GetStatefulSetPorts(context.Background(), "", "default", "my-sts")
	require.NoError(t, err)
	assert.Len(t, ports, 1)
	assert.Equal(t, int32(5432), ports[0].ContainerPort)
}

// --- GetDaemonSetPorts ---

func TestGetDaemonSetPorts(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "my-ds", Namespace: "default"},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "ds"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "ds"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "agent",
							Ports: []corev1.ContainerPort{
								{Name: "metrics", ContainerPort: 9100, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(ds)
	c := newFakeClient(cs, nil)

	ports, err := c.GetDaemonSetPorts(context.Background(), "", "default", "my-ds")
	require.NoError(t, err)
	assert.Len(t, ports, 1)
	assert.Equal(t, int32(9100), ports[0].ContainerPort)
}

// --- GetNamespaces ---

func TestGetNamespaces(t *testing.T) {
	ns1 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
	ns2 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
	cs := k8sfake.NewClientset(ns1, ns2)
	c := newFakeClient(cs, nil)

	items, err := c.GetNamespaces(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, items, 2)
	// Should be sorted by name.
	assert.Equal(t, "default", items[0].Name)
	assert.Equal(t, "kube-system", items[1].Name)
	assert.Equal(t, "Active", items[0].Status)
}

// --- ListServiceAccounts ---

func TestListServiceAccounts(t *testing.T) {
	sa1 := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "deployer", Namespace: "default"}}
	sa2 := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "admin", Namespace: "default"}}
	cs := k8sfake.NewClientset(sa1, sa2)
	c := newFakeClient(cs, nil)

	names, err := c.ListServiceAccounts(context.Background(), "", "default")
	require.NoError(t, err)
	assert.Len(t, names, 2)
	assert.Equal(t, "admin", names[0]) // sorted
	assert.Equal(t, "deployer", names[1])
}

func TestListServiceAccounts_AllNamespaces(t *testing.T) {
	sa1 := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa1", Namespace: "ns1"}}
	sa2 := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa2", Namespace: "ns2"}}
	cs := k8sfake.NewClientset(sa1, sa2)
	c := newFakeClient(cs, nil)

	names, err := c.ListServiceAccounts(context.Background(), "", "")
	require.NoError(t, err)
	assert.Len(t, names, 2)
	// When namespace is empty, names include namespace prefix.
	assert.Contains(t, names[0], "/")
}

// --- ListRBACSubjects ---

func TestListRBACSubjects(t *testing.T) {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-binding"},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "alice"},
			{Kind: "Group", Name: "devs"},
			{Kind: "ServiceAccount", Name: "deploy-sa", Namespace: "default"},
		},
	}
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "ns-binding", Namespace: "production"},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "bob"},
			{Kind: "ServiceAccount", Name: "ci-sa"},
		},
	}
	cs := k8sfake.NewClientset(crb, rb)
	c := newFakeClient(cs, nil)

	subjects, err := c.ListRBACSubjects(context.Background(), "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(subjects), 4)
	// Check ordering: Users first, then Groups, then ServiceAccounts.
	kinds := make([]string, 0, len(subjects))
	for _, s := range subjects {
		kinds = append(kinds, s.Kind)
	}
	// Verify ordering is correct (Users come before Groups come before SAs).
	foundGroup := false
	foundSA := false
	for _, k := range kinds {
		if k == "Group" {
			foundGroup = true
		}
		if k == "ServiceAccount" {
			foundSA = true
		}
		if foundGroup {
			assert.NotEqual(t, "User", k, "User should not appear after Group")
		}
		if foundSA {
			assert.NotEqual(t, "Group", k, "Group should not appear after ServiceAccount")
		}
	}
}

// --- GetDeploymentRevisions ---

func TestGetDeploymentRevisions(t *testing.T) {
	uid := k8stypes.UID("deploy-uid")
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "my-deploy", Namespace: "default", UID: uid},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "myapp:v2"}}},
			},
		},
	}
	rs1 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-deploy-abc", Namespace: "default",
			Annotations:     map[string]string{"deployment.kubernetes.io/revision": "1"},
			OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "my-deploy", UID: uid}},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "myapp:v1"}}},
			},
		},
		Status: appsv1.ReplicaSetStatus{Replicas: 0},
	}
	rs2 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-deploy-def", Namespace: "default",
			Annotations:     map[string]string{"deployment.kubernetes.io/revision": "2"},
			OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "my-deploy", UID: uid}},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "myapp:v2"}}},
			},
		},
		Status: appsv1.ReplicaSetStatus{Replicas: 1},
	}
	// Unowned RS should be excluded.
	rsOther := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "other-rs", Namespace: "default",
			Annotations: map[string]string{"deployment.kubernetes.io/revision": "1"},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "other"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "other"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "other:v1"}}},
			},
		},
	}
	cs := k8sfake.NewClientset(dep, rs1, rs2, rsOther)
	c := newFakeClient(cs, nil)

	revisions, err := c.GetDeploymentRevisions(context.Background(), "", "default", "my-deploy")
	require.NoError(t, err)
	assert.Len(t, revisions, 2)
	// Sorted by revision descending.
	assert.Equal(t, int64(2), revisions[0].Revision)
	assert.Equal(t, int64(1), revisions[1].Revision)
	assert.Equal(t, []string{"myapp:v2"}, revisions[0].Images)
	assert.Equal(t, []string{"myapp:v1"}, revisions[1].Images)
}

func TestGetDeploymentRevisions_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	_, err := c.GetDeploymentRevisions(context.Background(), "", "default", "missing")
	require.Error(t, err)
}

// ScaleResource tests with fake clients are in client_operations_test.go
// (GetScale/UpdateScale are sub-resource operations that the simple fake
// doesn't support out of the box).

// --- RestartResource with fake ---

func TestRestartResource_DeploymentWithFake(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "my-deploy", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app:v1"}}},
			},
		},
	}
	cs := k8sfake.NewClientset(dep)
	c := newFakeClient(cs, nil)

	err := c.RestartResource("", "default", "my-deploy", "Deployment")
	require.NoError(t, err)
}

func TestRestartResource_StatefulSetWithFake(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sts", Namespace: "default"},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "pg:15"}}},
			},
		},
	}
	cs := k8sfake.NewClientset(sts)
	c := newFakeClient(cs, nil)

	err := c.RestartResource("", "default", "my-sts", "StatefulSet")
	require.NoError(t, err)
}

func TestRestartResource_DaemonSetWithFake(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "my-ds", Namespace: "default"},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "agent", Image: "agent:v1"}}},
			},
		},
	}
	cs := k8sfake.NewClientset(ds)
	c := newFakeClient(cs, nil)

	err := c.RestartResource("", "default", "my-ds", "DaemonSet")
	require.NoError(t, err)
}

// --- GetPodSelector ---

func TestGetPodSelector_Deployment(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "my-deploy", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test", "env": "prod"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test", "env": "prod"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app:v1"}}},
			},
		},
	}
	cs := k8sfake.NewClientset(dep)
	c := newFakeClient(cs, nil)

	sel, err := c.GetPodSelector(context.Background(), "", "default", "Deployment", "my-deploy")
	require.NoError(t, err)
	assert.Contains(t, sel, "app=test")
	assert.Contains(t, sel, "env=prod")
}

func TestGetPodSelector_StatefulSet(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sts", Namespace: "default"},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "db"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "pg:15"}}},
			},
		},
	}
	cs := k8sfake.NewClientset(sts)
	c := newFakeClient(cs, nil)

	sel, err := c.GetPodSelector(context.Background(), "", "default", "StatefulSet", "my-sts")
	require.NoError(t, err)
	assert.Equal(t, "app=db", sel)
}

func TestGetPodSelector_DaemonSet(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "my-ds", Namespace: "default"},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "agent"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "agent"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "agent", Image: "agent:v1"}}},
			},
		},
	}
	cs := k8sfake.NewClientset(ds)
	c := newFakeClient(cs, nil)

	sel, err := c.GetPodSelector(context.Background(), "", "default", "DaemonSet", "my-ds")
	require.NoError(t, err)
	assert.Equal(t, "app=agent", sel)
}

func TestGetPodSelector_Job(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "my-job", Namespace: "default"},
		Spec: batchv1.JobSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"job-name": "my-job"}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{{Name: "worker", Image: "worker:v1"}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	cs := k8sfake.NewClientset(job)
	c := newFakeClient(cs, nil)

	sel, err := c.GetPodSelector(context.Background(), "", "default", "Job", "my-job")
	require.NoError(t, err)
	assert.Equal(t, "job-name=my-job", sel)
}

func TestGetPodSelector_Service(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "web"},
			Ports:    []corev1.ServicePort{{Port: 80}},
		},
	}
	cs := k8sfake.NewClientset(svc)
	c := newFakeClient(cs, nil)

	sel, err := c.GetPodSelector(context.Background(), "", "default", "Service", "my-svc")
	require.NoError(t, err)
	assert.Equal(t, "app=web", sel)
}

func TestGetPodSelector_UnknownKind(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	sel, err := c.GetPodSelector(context.Background(), "", "default", "ConfigMap", "my-cm")
	require.NoError(t, err)
	assert.Empty(t, sel)
}

func TestGetPodSelector_NoSelector(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "headless", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80}}},
	}
	cs := k8sfake.NewClientset(svc)
	c := newFakeClient(cs, nil)

	sel, err := c.GetPodSelector(context.Background(), "", "default", "Service", "headless")
	require.NoError(t, err)
	assert.Empty(t, sel)
}

// --- GetHelmReleases ---

func TestGetHelmReleases(t *testing.T) {
	now := time.Now()
	s1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "sh.helm.release.v1.myapp.v1",
			Namespace:         "default",
			Labels:            map[string]string{"owner": "helm", "name": "myapp", "status": "deployed", "version": "1"},
			CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Hour)),
		},
	}
	s2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "sh.helm.release.v1.myapp.v2",
			Namespace:         "default",
			Labels:            map[string]string{"owner": "helm", "name": "myapp", "status": "deployed", "version": "2"},
			CreationTimestamp: metav1.NewTime(now),
		},
	}
	s3 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "sh.helm.release.v1.other.v1",
			Namespace:         "default",
			Labels:            map[string]string{"owner": "helm", "name": "other", "status": "superseded", "version": "1"},
			CreationTimestamp: metav1.NewTime(now.Add(-2 * time.Hour)),
		},
	}
	cs := k8sfake.NewClientset(s1, s2, s3)
	c := newFakeClient(cs, nil)

	items, err := c.GetHelmReleases(context.Background(), "", "default")
	require.NoError(t, err)
	assert.Len(t, items, 2)
	// Should find myapp (latest v2) and other (v1).
	names := []string{items[0].Name, items[1].Name}
	assert.Contains(t, names, "myapp")
	assert.Contains(t, names, "other")
}

func TestGetHelmReleases_NoNamespace(t *testing.T) {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sh.helm.release.v1.app.v1",
			Namespace: "prod",
			Labels:    map[string]string{"owner": "helm", "name": "app", "status": "deployed", "version": "1"},
		},
	}
	cs := k8sfake.NewClientset(s)
	c := newFakeClient(cs, nil)

	items, err := c.GetHelmReleases(context.Background(), "", "")
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "prod", items[0].Namespace)
}

func TestGetHelmReleases_SkipsNoName(t *testing.T) {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "helm-secret-noname",
			Namespace: "default",
			Labels:    map[string]string{"owner": "helm", "status": "deployed"},
		},
	}
	cs := k8sfake.NewClientset(s)
	c := newFakeClient(cs, nil)

	items, err := c.GetHelmReleases(context.Background(), "", "default")
	require.NoError(t, err)
	assert.Len(t, items, 0)
}

// --- GetHelmReleaseYAML ---

func TestGetHelmReleaseYAML(t *testing.T) {
	now := time.Now()
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "sh.helm.release.v1.myapp.v1",
			Namespace:         "default",
			Labels:            map[string]string{"owner": "helm", "name": "myapp", "status": "deployed", "version": "1"},
			CreationTimestamp: metav1.NewTime(now),
		},
	}
	cs := k8sfake.NewClientset(s)
	c := newFakeClient(cs, nil)

	yamlStr, err := c.GetHelmReleaseYAML(context.Background(), "", "default", "myapp")
	require.NoError(t, err)
	assert.Contains(t, yamlStr, "name: myapp")
	assert.Contains(t, yamlStr, "status: deployed")
}

func TestGetHelmReleaseYAML_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	_, err := c.GetHelmReleaseYAML(context.Background(), "", "default", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no helm release found")
}

// --- GetHelmReleaseYAML with multiple versions ---

func TestGetHelmReleaseYAML_PicksLatest(t *testing.T) {
	old := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "sh.helm.release.v1.myapp.v1",
			Namespace:         "default",
			Labels:            map[string]string{"owner": "helm", "name": "myapp", "status": "superseded", "version": "1"},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
		},
	}
	newer := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "sh.helm.release.v1.myapp.v2",
			Namespace:         "default",
			Labels:            map[string]string{"owner": "helm", "name": "myapp", "status": "deployed", "version": "2"},
			CreationTimestamp: metav1.NewTime(time.Now()),
		},
	}
	cs := k8sfake.NewClientset(old, newer)
	c := newFakeClient(cs, nil)

	yamlStr, err := c.GetHelmReleaseYAML(context.Background(), "", "default", "myapp")
	require.NoError(t, err)
	assert.Contains(t, yamlStr, "version: \"2\"")
}

// --- GetPodStartupAnalysis ---

func TestGetPodStartupAnalysis(t *testing.T) {
	now := time.Now()
	created := now.Add(-30 * time.Second)
	scheduled := now.Add(-28 * time.Second)
	initialized := now.Add(-25 * time.Second)
	containersReady := now.Add(-20 * time.Second)
	ready := now.Add(-18 * time.Second)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "my-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "init-db", Image: "busybox"},
			},
			Containers: []corev1.Container{
				{Name: "app", Image: "myapp:v1"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: "PodScheduled", LastTransitionTime: metav1.NewTime(scheduled)},
				{Type: "Initialized", LastTransitionTime: metav1.NewTime(initialized)},
				{Type: "ContainersReady", LastTransitionTime: metav1.NewTime(containersReady)},
				{Type: "Ready", LastTransitionTime: metav1.NewTime(ready)},
			},
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "init-db",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{StartedAt: metav1.NewTime(scheduled), FinishedAt: metav1.NewTime(initialized)}},
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "app",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(initialized)}},
				},
			},
		},
	}

	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)

	info, err := c.GetPodStartupAnalysis(context.Background(), "", "default", "my-pod")
	require.NoError(t, err)
	assert.Equal(t, "my-pod", info.PodName)
	assert.Equal(t, "default", info.Namespace)
	assert.Greater(t, len(info.Phases), 0)
	// Total time should be roughly ready - created.
	assert.InDelta(t, 12*time.Second, info.TotalTime, float64(time.Second))

	// Check that we have scheduling phase.
	var foundScheduling bool
	for _, p := range info.Phases {
		if p.Name == "Scheduling" {
			foundScheduling = true
			assert.Equal(t, "completed", p.Status)
		}
	}
	assert.True(t, foundScheduling)
}

func TestGetPodStartupAnalysis_PendingPod(t *testing.T) {
	created := time.Now().Add(-10 * time.Second)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pending-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "app:v1"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			// No conditions at all (still scheduling).
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)

	info, err := c.GetPodStartupAnalysis(context.Background(), "", "default", "pending-pod")
	require.NoError(t, err)
	assert.Equal(t, "pending-pod", info.PodName)
	// Should have scheduling phase in-progress.
	require.Greater(t, len(info.Phases), 0)
	assert.Equal(t, "Scheduling", info.Phases[0].Name)
	assert.Equal(t, "in-progress", info.Phases[0].Status)
}

func TestGetPodStartupAnalysis_WithImagePullEvents(t *testing.T) {
	now := time.Now()
	created := now.Add(-30 * time.Second)
	pullStart := now.Add(-25 * time.Second)
	pullEnd := now.Add(-20 * time.Second)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pull-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "myapp:latest"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: "PodScheduled", LastTransitionTime: metav1.NewTime(created.Add(time.Second))},
				{Type: "ContainersReady", LastTransitionTime: metav1.NewTime(now.Add(-15 * time.Second))},
				{Type: "Ready", LastTransitionTime: metav1.NewTime(now.Add(-14 * time.Second))},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(pullEnd)}}},
			},
		},
	}

	// Image pull events.
	pullingEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "pull-pod.pulling", Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{Name: "pull-pod", Kind: "Pod"},
		Reason:         "Pulling",
		Message:        `Pulling image "myapp:latest"`,
		LastTimestamp:  metav1.NewTime(pullStart),
	}
	pulledEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "pull-pod.pulled", Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{Name: "pull-pod", Kind: "Pod"},
		Reason:         "Pulled",
		Message:        `Successfully pulled image "myapp:latest"`,
		LastTimestamp:  metav1.NewTime(pullEnd),
	}

	cs := k8sfake.NewClientset(pod, pullingEvent, pulledEvent)
	c := newFakeClient(cs, nil)

	info, err := c.GetPodStartupAnalysis(context.Background(), "", "default", "pull-pod")
	require.NoError(t, err)

	var foundImagePull bool
	for _, p := range info.Phases {
		if p.Name == "Image Pull" {
			foundImagePull = true
			assert.Equal(t, "completed", p.Status)
			assert.InDelta(t, 5*time.Second, p.Duration, float64(time.Second))
		}
	}
	assert.True(t, foundImagePull)
}

func TestGetPodStartupAnalysis_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	_, err := c.GetPodStartupAnalysis(context.Background(), "", "default", "missing")
	require.Error(t, err)
}

func TestGetPodStartupAnalysis_InitContainerRunning(t *testing.T) {
	now := time.Now()
	created := now.Add(-60 * time.Second)
	scheduled := now.Add(-58 * time.Second)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "init-running-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "init-slow"},
				{Name: "init-waiting"},
			},
			Containers: []corev1.Container{{Name: "app"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{Type: "PodScheduled", LastTransitionTime: metav1.NewTime(scheduled)},
			},
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "init-slow",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(scheduled)}},
				},
				{
					Name:  "init-waiting",
					State: corev1.ContainerState{},
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "app",
					State: corev1.ContainerState{},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)

	info, err := c.GetPodStartupAnalysis(context.Background(), "", "default", "init-running-pod")
	require.NoError(t, err)
	assert.Equal(t, "init-running-pod", info.PodName)
	// Should have init container phases.
	var foundInitRunning, foundInitUnknown bool
	for _, p := range info.Phases {
		if p.Name == "  init: init-slow" {
			foundInitRunning = true
			assert.Equal(t, "in-progress", p.Status)
		}
		if p.Name == "  init: init-waiting" {
			foundInitUnknown = true
			assert.Equal(t, "unknown", p.Status)
		}
	}
	assert.True(t, foundInitRunning)
	assert.True(t, foundInitUnknown)
}

func TestGetPodStartupAnalysis_TerminatedContainer(t *testing.T) {
	now := time.Now()
	created := now.Add(-120 * time.Second)
	scheduled := now.Add(-118 * time.Second)
	containersReady := now.Add(-110 * time.Second)
	ready := now.Add(-109 * time.Second)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "terminated-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "worker"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			Conditions: []corev1.PodCondition{
				{Type: "PodScheduled", LastTransitionTime: metav1.NewTime(scheduled)},
				{Type: "ContainersReady", LastTransitionTime: metav1.NewTime(containersReady)},
				{Type: "Ready", LastTransitionTime: metav1.NewTime(ready)},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "worker",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							StartedAt:  metav1.NewTime(scheduled),
							FinishedAt: metav1.NewTime(containersReady),
						},
					},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)

	info, err := c.GetPodStartupAnalysis(context.Background(), "", "default", "terminated-pod")
	require.NoError(t, err)
	assert.Equal(t, "terminated-pod", info.PodName)
}

func TestGetPodStartupAnalysis_ContainersReadyNoReady(t *testing.T) {
	now := time.Now()
	created := now.Add(-30 * time.Second)
	scheduled := now.Add(-28 * time.Second)
	containersReady := now.Add(-20 * time.Second)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "readiness-waiting",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: "PodScheduled", LastTransitionTime: metav1.NewTime(scheduled)},
				{Type: "ContainersReady", LastTransitionTime: metav1.NewTime(containersReady)},
				// Ready condition is missing - readiness probes still running.
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "app",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(scheduled)}},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)

	info, err := c.GetPodStartupAnalysis(context.Background(), "", "default", "readiness-waiting")
	require.NoError(t, err)
	// Should have readiness probes in-progress.
	var foundReadiness bool
	for _, p := range info.Phases {
		if p.Name == "Readiness Probes" {
			foundReadiness = true
			assert.Equal(t, "in-progress", p.Status)
		}
	}
	assert.True(t, foundReadiness)
}

func TestGetPodStartupAnalysis_InProgressImagePull(t *testing.T) {
	now := time.Now()
	created := now.Add(-10 * time.Second)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pulling-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "large-image:latest"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{Type: "PodScheduled", LastTransitionTime: metav1.NewTime(created.Add(time.Second))},
			},
		},
	}
	pullingEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "pulling-pod.pulling", Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{Name: "pulling-pod", Kind: "Pod"},
		Reason:         "Pulling",
		Message:        `Pulling image "large-image:latest"`,
		LastTimestamp:  metav1.NewTime(created.Add(2 * time.Second)),
	}

	cs := k8sfake.NewClientset(pod, pullingEvent)
	c := newFakeClient(cs, nil)

	info, err := c.GetPodStartupAnalysis(context.Background(), "", "default", "pulling-pod")
	require.NoError(t, err)

	var foundImagePull bool
	for _, p := range info.Phases {
		if p.Name == "Image Pull" {
			foundImagePull = true
			assert.Equal(t, "in-progress", p.Status)
		}
	}
	assert.True(t, foundImagePull)
}

// --- Dynamic client tests using fake ---

func newFakeDynClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "pods"}:                                      "PodList",
			{Group: "", Version: "v1", Resource: "namespaces"}:                                "NamespaceList",
			{Group: "", Version: "v1", Resource: "secrets"}:                                   "SecretList",
			{Group: "", Version: "v1", Resource: "configmaps"}:                                "ConfigMapList",
			{Group: "", Version: "v1", Resource: "services"}:                                  "ServiceList",
			{Group: "", Version: "v1", Resource: "events"}:                                    "EventList",
			{Group: "", Version: "v1", Resource: "resourcequotas"}:                            "ResourceQuotaList",
			{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}:                    "PersistentVolumeClaimList",
			{Group: "apps", Version: "v1", Resource: "deployments"}:                           "DeploymentList",
			{Group: "apps", Version: "v1", Resource: "replicasets"}:                           "ReplicaSetList",
			{Group: "apps", Version: "v1", Resource: "statefulsets"}:                          "StatefulSetList",
			{Group: "apps", Version: "v1", Resource: "daemonsets"}:                            "DaemonSetList",
			{Group: "batch", Version: "v1", Resource: "jobs"}:                                 "JobList",
			{Group: "batch", Version: "v1", Resource: "cronjobs"}:                             "CronJobList",
			{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}:          "NetworkPolicyList",
			{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:                   "PodMetricsList",
			{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}:                  "NodeMetricsList",
			{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}:             "ApplicationList",
			{Group: "argoproj.io", Version: "v1alpha1", Resource: "workflows"}:                "WorkflowList",
			{Group: "argoproj.io", Version: "v1alpha1", Resource: "cronworkflows"}:            "CronWorkflowList",
			{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories"}:   "GitRepositoryList",
			{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"}: "KustomizationList",
			{Group: "helm.toolkit.fluxcd.io", Version: "v2beta1", Resource: "helmreleases"}:   "HelmReleaseList",
			{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}:               "CertificateList",
		}, objects...)
}

// --- GetResources: Event ordering ---

// TestGetResources_EventsSortedByLastSeen verifies that GetResources orders
// Event items so that the most-recently-observed event sits at the top. The
// fixture deliberately inverts firstTimestamp vs lastTimestamp ordering — a
// sort keyed on first-seen (CreatedAt) would put "old-incident-recurring" at
// the bottom even though its lastTimestamp is the newest, which is exactly
// the bug we're guarding against.
func TestGetResources_EventsSortedByLastSeen(t *testing.T) {
	mkEvent := func(name, first, last string) *unstructured.Unstructured {
		return &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Event",
				"metadata": map[string]any{
					"name":      name,
					"namespace": "default",
				},
				"type":           "Warning",
				"reason":         "Backoff",
				"message":        "container failed",
				"firstTimestamp": first,
				"lastTimestamp":  last,
				"count":          int64(1),
				"involvedObject": map[string]any{
					"kind": "Pod",
					"name": name,
				},
			},
		}
	}

	// Three events. firstTimestamp ascending order is A, B, C. But
	// lastTimestamp ordering puts A on top because the same incident keeps
	// recurring — A's most recent observation is the newest.
	eventA := mkEvent("old-incident-recurring", "2026-04-10T08:00:00Z", "2026-04-10T12:00:00Z")
	eventB := mkEvent("recent-once-only", "2026-04-10T11:00:00Z", "2026-04-10T11:00:00Z")
	eventC := mkEvent("middle-once-only", "2026-04-10T10:00:00Z", "2026-04-10T10:00:00Z")

	dc := newFakeDynClient(eventA, eventB, eventC)
	c := newFakeClient(nil, dc)

	items, err := c.GetResources(context.Background(), "", "default", model.ResourceTypeEntry{
		Kind: "Event", APIGroup: "", APIVersion: "v1", Resource: "events", Namespaced: true,
	})
	require.NoError(t, err)
	require.Len(t, items, 3)

	got := []string{items[0].Name, items[1].Name, items[2].Name}
	want := []string{"old-incident-recurring", "recent-once-only", "middle-once-only"}
	assert.Equal(t, want, got,
		"events must be ordered newest-LastSeen first, not newest-CreatedAt first")
}

// --- DeleteResource ---

func TestDeleteResource_Namespaced(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetName("my-cm")
	obj.SetNamespace("default")
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})

	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	err := c.DeleteResource("", "default", model.ResourceTypeEntry{
		APIGroup: "", APIVersion: "v1", Resource: "configmaps", Namespaced: true,
	}, "my-cm")
	require.NoError(t, err)

	// Verify it's gone.
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	_, err = dc.Resource(gvr).Namespace("default").Get(context.Background(), "my-cm", metav1.GetOptions{})
	require.Error(t, err)
}

func TestDeleteResource_ClusterScoped(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetName("my-ns")
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"})

	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	err := c.DeleteResource("", "", model.ResourceTypeEntry{
		APIGroup: "", APIVersion: "v1", Resource: "namespaces", Namespaced: false,
	}, "my-ns")
	require.NoError(t, err)
}

// --- ResizePVC ---

func TestResizePVC(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "PersistentVolumeClaim",
			"metadata": map[string]any{
				"name":      "my-pvc",
				"namespace": "default",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"requests": map[string]any{
						"storage": "10Gi",
					},
				},
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	err := c.ResizePVC("", "default", "my-pvc", "20Gi")
	require.NoError(t, err)
}

// --- GetResourceYAML ---

func TestGetResourceYAML_Namespaced(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "my-cm",
				"namespace": "default",
			},
			"data": map[string]any{
				"key": "value",
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	yamlStr, err := c.GetResourceYAML(context.Background(), "", "default", model.ResourceTypeEntry{
		APIGroup: "", APIVersion: "v1", Resource: "configmaps", Namespaced: true,
	}, "my-cm")
	require.NoError(t, err)
	assert.Contains(t, yamlStr, "kind: ConfigMap")
	assert.Contains(t, yamlStr, "key: value")
}

func TestGetResourceYAML_ClusterScoped(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name": "my-ns",
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	yamlStr, err := c.GetResourceYAML(context.Background(), "", "", model.ResourceTypeEntry{
		APIGroup: "", APIVersion: "v1", Resource: "namespaces", Namespaced: false,
	}, "my-ns")
	require.NoError(t, err)
	assert.Contains(t, yamlStr, "kind: Namespace")
}

func TestGetResourceYAML_HelmVirtualType(t *testing.T) {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sh.helm.release.v1.myapp.v1",
			Namespace: "default",
			Labels:    map[string]string{"owner": "helm", "name": "myapp", "status": "deployed", "version": "1"},
		},
	}
	cs := k8sfake.NewClientset(s)
	c := newFakeClient(cs, nil)

	yamlStr, err := c.GetResourceYAML(context.Background(), "", "default", model.ResourceTypeEntry{
		APIGroup: "_helm", Resource: "releases",
	}, "myapp")
	require.NoError(t, err)
	assert.Contains(t, yamlStr, "name: myapp")
}

func TestGetResourceYAML_PortForwardVirtualType(t *testing.T) {
	c := newFakeClient(nil, nil)

	yamlStr, err := c.GetResourceYAML(context.Background(), "", "default", model.ResourceTypeEntry{
		APIGroup: "_portforward",
	}, "any")
	require.NoError(t, err)
	assert.Empty(t, yamlStr)
}

// --- GetPodYAML ---
// GetPodYAML delegates to GetResourceYAML with Namespaced=false (the
// ResourceTypeEntry zero value), so the test validates that code path.
// The underlying GetResourceYAML is tested directly above with both
// namespaced and cluster-scoped objects.

// --- GetLabelAnnotationData ---

func TestGetLabelAnnotationData_Namespaced(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":        "my-cm",
				"namespace":   "default",
				"labels":      map[string]any{"app": "test", "env": "prod"},
				"annotations": map[string]any{"note": "important"},
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	result, err := c.GetLabelAnnotationData(context.Background(), "", model.ResourceTypeEntry{
		APIGroup: "", APIVersion: "v1", Resource: "configmaps", Namespaced: true,
	}, "default", "my-cm")
	require.NoError(t, err)
	assert.Equal(t, "test", result.Labels["app"])
	assert.Equal(t, "prod", result.Labels["env"])
	assert.Equal(t, "important", result.Annotations["note"])
	assert.Equal(t, []string{"app", "env"}, result.LabelKeys)
	assert.Equal(t, []string{"note"}, result.AnnotKeys)
}

func TestGetLabelAnnotationData_ClusterScoped(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name":   "my-ns",
				"labels": map[string]any{"env": "staging"},
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	result, err := c.GetLabelAnnotationData(context.Background(), "", model.ResourceTypeEntry{
		APIGroup: "", APIVersion: "v1", Resource: "namespaces", Namespaced: false,
	}, "", "my-ns")
	require.NoError(t, err)
	assert.Equal(t, "staging", result.Labels["env"])
	assert.NotNil(t, result.Annotations) // should be initialized even if nil
}

func TestGetLabelAnnotationData_NoLabelsOrAnnotations(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "bare-cm",
				"namespace": "default",
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	result, err := c.GetLabelAnnotationData(context.Background(), "", model.ResourceTypeEntry{
		APIGroup: "", APIVersion: "v1", Resource: "configmaps", Namespaced: true,
	}, "default", "bare-cm")
	require.NoError(t, err)
	assert.NotNil(t, result.Labels)
	assert.NotNil(t, result.Annotations)
	assert.Empty(t, result.LabelKeys)
}

// --- UpdateLabelAnnotationData ---

func TestUpdateLabelAnnotationData_Namespaced(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":        "my-cm",
				"namespace":   "default",
				"labels":      map[string]any{"old-label": "val", "keep": "yes"},
				"annotations": map[string]any{"old-annot": "val"},
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	err := c.UpdateLabelAnnotationData(context.Background(), "", model.ResourceTypeEntry{
		APIGroup: "", APIVersion: "v1", Resource: "configmaps", Namespaced: true,
	}, "default", "my-cm",
		map[string]string{"keep": "yes", "new-label": "added"},
		map[string]string{"new-annot": "added"},
	)
	require.NoError(t, err)
}

func TestUpdateLabelAnnotationData_ClusterScoped(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name":   "my-ns",
				"labels": map[string]any{"env": "old"},
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	err := c.UpdateLabelAnnotationData(context.Background(), "", model.ResourceTypeEntry{
		APIGroup: "", APIVersion: "v1", Resource: "namespaces", Namespaced: false,
	}, "", "my-ns",
		map[string]string{"env": "new"},
		map[string]string{},
	)
	require.NoError(t, err)
}

// --- FindResourcesWithFinalizer ---

func TestFindResourcesWithFinalizer(t *testing.T) {
	obj1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":              "cm-with-finalizer",
				"namespace":         "default",
				"finalizers":        []any{"my.finalizer.io/cleanup"},
				"creationTimestamp": "2026-01-01T00:00:00Z",
			},
		},
	}
	obj2 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "cm-no-finalizer",
				"namespace": "default",
			},
		},
	}
	dc := newFakeDynClient(obj1, obj2)
	c := newFakeClient(nil, dc)

	rts := []model.ResourceTypeEntry{
		{APIGroup: "", APIVersion: "v1", Resource: "configmaps", Kind: "ConfigMap", Namespaced: true},
	}

	results, err := c.FindResourcesWithFinalizer(context.Background(), "", "default", "finalizer", rts)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "cm-with-finalizer", results[0].Name)
	assert.Equal(t, "my.finalizer.io/cleanup", results[0].Matched)
}

func TestFindResourcesWithFinalizer_SkipsVirtualTypes(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	rts := []model.ResourceTypeEntry{
		{APIGroup: "_helm", Resource: "releases"},
		{APIGroup: "_portforward", Resource: "portforwards"},
	}

	results, err := c.FindResourcesWithFinalizer(context.Background(), "", "", "test", rts)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestFindResourcesWithFinalizer_CaseInsensitive(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":       "cm1",
				"namespace":  "default",
				"finalizers": []any{"MyFinalizer.IO/Cleanup"},
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	rts := []model.ResourceTypeEntry{
		{APIGroup: "", APIVersion: "v1", Resource: "configmaps", Kind: "ConfigMap", Namespaced: true},
	}

	results, err := c.FindResourcesWithFinalizer(context.Background(), "", "default", "myfinalizer", rts)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

// --- RemoveFinalizerFromResource ---

func TestRemoveFinalizerFromResource(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":       "cm1",
				"namespace":  "default",
				"finalizers": []any{"keep.this/finalizer", "remove.this/finalizer"},
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	match := FinalizerMatch{
		Name:       "cm1",
		Namespace:  "default",
		Kind:       "ConfigMap",
		APIGroup:   "",
		APIVersion: "v1",
		Resource:   "configmaps",
		Namespaced: true,
		Matched:    "remove.this/finalizer",
	}

	err := c.RemoveFinalizerFromResource(context.Background(), "", match)
	require.NoError(t, err)
}

func TestRemoveFinalizerFromResource_ClusterScoped(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name":       "stuck-ns",
				"finalizers": []any{"kubernetes"},
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	match := FinalizerMatch{
		Name:       "stuck-ns",
		Kind:       "Namespace",
		APIGroup:   "",
		APIVersion: "v1",
		Resource:   "namespaces",
		Namespaced: false,
		Matched:    "kubernetes",
	}

	err := c.RemoveFinalizerFromResource(context.Background(), "", match)
	require.NoError(t, err)
}

// --- GetResourceEvents ---

func TestGetResourceEvents(t *testing.T) {
	event1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Event",
			"metadata": map[string]any{
				"name":      "ev1",
				"namespace": "default",
			},
			"involvedObject": map[string]any{
				"name": "my-deploy",
				"kind": "Deployment",
			},
			"type":          "Normal",
			"reason":        "ScalingReplicaSet",
			"message":       "Scaled up replica set my-deploy-abc to 1",
			"lastTimestamp": "2026-03-22T10:00:00Z",
			"count":         int64(1),
			"source":        map[string]any{"component": "deployment-controller"},
		},
	}
	// Prefix-match event (pod owned by deployment).
	event2 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Event",
			"metadata": map[string]any{
				"name":      "ev2",
				"namespace": "default",
			},
			"involvedObject": map[string]any{
				"name": "my-deploy-abc-xyz",
				"kind": "Pod",
			},
			"type":          "Warning",
			"reason":        "BackOff",
			"message":       "Back-off restarting failed container",
			"lastTimestamp": "2026-03-22T11:00:00Z",
			"count":         int64(5),
		},
	}
	// Unrelated event.
	event3 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Event",
			"metadata": map[string]any{
				"name":      "ev3",
				"namespace": "default",
			},
			"involvedObject": map[string]any{
				"name": "other-deploy",
				"kind": "Deployment",
			},
			"type":   "Normal",
			"reason": "ScalingReplicaSet",
		},
	}

	dc := newFakeDynClient(event1, event2, event3)
	c := newFakeClient(nil, dc)

	events, err := c.GetResourceEvents(context.Background(), "", "default", "my-deploy", "Deployment")
	require.NoError(t, err)
	assert.Len(t, events, 2)
	// Most recent first.
	assert.Equal(t, "BackOff", events[0].Reason)
	assert.Equal(t, "ScalingReplicaSet", events[1].Reason)
	assert.Equal(t, "deployment-controller", events[1].Source)
}

func TestGetResourceEvents_NoEvents(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	events, err := c.GetResourceEvents(context.Background(), "", "default", "my-deploy", "Deployment")
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestGetResourceEvents_EventTimeAndCreationFallback(t *testing.T) {
	// Event with eventTime instead of lastTimestamp.
	ev := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Event",
			"metadata": map[string]any{
				"name":              "ev-eventtime",
				"namespace":         "default",
				"creationTimestamp": "2026-03-22T08:00:00Z",
			},
			"involvedObject": map[string]any{
				"name": "my-pod",
				"kind": "Pod",
			},
			"type":      "Normal",
			"reason":    "Scheduled",
			"eventTime": "2026-03-22T09:00:00Z",
		},
	}
	dc := newFakeDynClient(ev)
	c := newFakeClient(nil, dc)

	events, err := c.GetResourceEvents(context.Background(), "", "default", "my-pod", "Pod")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	// Should use eventTime.
	assert.False(t, events[0].Timestamp.IsZero())
}

func TestGetResourceEvents_ReportingComponentFallback(t *testing.T) {
	ev := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Event",
			"metadata": map[string]any{
				"name":      "ev-reporting",
				"namespace": "default",
			},
			"involvedObject": map[string]any{
				"name": "my-pod",
				"kind": "Pod",
			},
			"type":               "Normal",
			"reason":             "Created",
			"reportingComponent": "kubelet",
			"lastTimestamp":      "2026-03-22T10:00:00Z",
		},
	}
	dc := newFakeDynClient(ev)
	c := newFakeClient(nil, dc)

	events, err := c.GetResourceEvents(context.Background(), "", "default", "my-pod", "Pod")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "kubelet", events[0].Source)
}

// --- GetPodsUsingPVC ---

func TestGetPodsUsingPVC(t *testing.T) {
	pod1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]any{"name": "pod-using-pvc", "namespace": "default"},
			"spec": map[string]any{
				"volumes": []any{
					map[string]any{
						"name": "data",
						"persistentVolumeClaim": map[string]any{
							"claimName": "my-pvc",
						},
					},
				},
			},
		},
	}
	pod2 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]any{"name": "pod-no-pvc", "namespace": "default"},
			"spec": map[string]any{
				"volumes": []any{
					map[string]any{
						"name":     "config",
						"emptyDir": map[string]any{},
					},
				},
			},
		},
	}
	dc := newFakeDynClient(pod1, pod2)
	c := newFakeClient(nil, dc)

	names, err := c.GetPodsUsingPVC(context.Background(), "", "default", "my-pvc")
	require.NoError(t, err)
	assert.Equal(t, []string{"pod-using-pvc"}, names)
}

func TestGetPodsUsingPVC_NoPods(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	names, err := c.GetPodsUsingPVC(context.Background(), "", "default", "my-pvc")
	require.NoError(t, err)
	assert.Empty(t, names)
}

// TestGetOwnedResources_PersistentVolumeClaim verifies the lazy
// replacement for the former eager "Used By" column: requesting owned
// resources for a PVC returns Pod items for every pod that mounts it,
// ready to render in the right-pane preview. The earlier design
// performed this lookup for *every* PVC during the list fetch (N+1);
// the test here exercises the on-demand path that runs only when the
// user selects a specific PVC.
func TestGetOwnedResources_PersistentVolumeClaim(t *testing.T) {
	mounting := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]any{"name": "mounter", "namespace": "default"},
			"spec": map[string]any{
				"volumes": []any{
					map[string]any{
						"name": "data",
						"persistentVolumeClaim": map[string]any{
							"claimName": "target-pvc",
						},
					},
				},
			},
		},
	}
	unrelated := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]any{"name": "unrelated", "namespace": "default"},
			"spec": map[string]any{
				"volumes": []any{
					map[string]any{
						"name":     "tmp",
						"emptyDir": map[string]any{},
					},
				},
			},
		},
	}
	dc := newFakeDynClient(mounting, unrelated)
	c := newFakeClient(nil, dc)

	items, err := c.GetOwnedResources(context.Background(), "", "default", "PersistentVolumeClaim", "target-pvc")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "mounter", items[0].Name)
	assert.Equal(t, "Pod", items[0].Kind)
	assert.Equal(t, "default", items[0].Namespace)
}

// --- PatchLabels ---

func TestPatchLabels_Namespaced(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "my-cm",
				"namespace": "default",
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	err := c.PatchLabels(context.Background(), "", "default", "my-cm", gvr, map[string]any{
		"env": "prod",
	})
	require.NoError(t, err)
}

func TestPatchLabels_ClusterScoped(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name": "my-ns",
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	err := c.PatchLabels(context.Background(), "", "", "my-ns", gvr, map[string]any{
		"env": "prod",
	})
	require.NoError(t, err)
}

// --- PatchAnnotations ---

func TestPatchAnnotations_Namespaced(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "my-cm",
				"namespace": "default",
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	err := c.PatchAnnotations(context.Background(), "", "default", "my-cm", gvr, map[string]any{
		"note": "important",
	})
	require.NoError(t, err)
}

func TestPatchAnnotations_ClusterScoped(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name": "my-ns",
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	err := c.PatchAnnotations(context.Background(), "", "", "my-ns", gvr, map[string]any{
		"note": "important",
	})
	require.NoError(t, err)
}

// --- GetNamespaceQuotas ---

func TestGetNamespaceQuotas(t *testing.T) {
	quota := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ResourceQuota",
			"metadata": map[string]any{
				"name":      "my-quota",
				"namespace": "default",
			},
			"spec": map[string]any{
				"hard": map[string]any{
					"pods":   "10",
					"cpu":    "4",
					"memory": "8Gi",
				},
			},
			"status": map[string]any{
				"used": map[string]any{
					"pods":   "5",
					"cpu":    "2",
					"memory": "4Gi",
				},
			},
		},
	}
	dc := newFakeDynClient(quota)
	c := newFakeClient(nil, dc)

	quotas, err := c.GetNamespaceQuotas(context.Background(), "", "default")
	require.NoError(t, err)
	assert.Len(t, quotas, 1)
	assert.Equal(t, "my-quota", quotas[0].Name)
	assert.GreaterOrEqual(t, len(quotas[0].Resources), 3)

	// Check one resource.
	for _, r := range quotas[0].Resources {
		if r.Name == "pods" {
			assert.Equal(t, "10", r.Hard)
			assert.Equal(t, "5", r.Used)
			assert.InDelta(t, 50.0, r.Percent, 1.0)
		}
	}
}

func TestGetNamespaceQuotas_NoStatus(t *testing.T) {
	quota := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ResourceQuota",
			"metadata": map[string]any{
				"name":      "new-quota",
				"namespace": "default",
			},
			"spec": map[string]any{
				"hard": map[string]any{
					"pods": "10",
				},
			},
		},
	}
	dc := newFakeDynClient(quota)
	c := newFakeClient(nil, dc)

	quotas, err := c.GetNamespaceQuotas(context.Background(), "", "default")
	require.NoError(t, err)
	assert.Len(t, quotas, 1)
	assert.Len(t, quotas[0].Resources, 1)
	assert.Equal(t, "0", quotas[0].Resources[0].Used)
}

func TestGetNamespaceQuotas_AllNamespaces(t *testing.T) {
	quota := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ResourceQuota",
			"metadata": map[string]any{
				"name":      "global-quota",
				"namespace": "production",
			},
			"spec": map[string]any{
				"hard": map[string]any{"pods": "100"},
			},
		},
	}
	dc := newFakeDynClient(quota)
	c := newFakeClient(nil, dc)

	quotas, err := c.GetNamespaceQuotas(context.Background(), "", "")
	require.NoError(t, err)
	assert.Len(t, quotas, 1)
}

// --- GetPodMetrics_NotAvailable ---

func TestGetPodMetrics_NotAvailable(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	_, err := c.GetPodMetrics(context.Background(), "", "default", "my-pod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metrics API unavailable")
}

// --- metricsGVR ---

func TestMetricsGVR(t *testing.T) {
	c := &Client{}
	gvrs := c.metricsGVR("pods")
	assert.Len(t, gvrs, 2)
	assert.Equal(t, "metrics.k8s.io", gvrs[0].Group)
	assert.Equal(t, "v1beta1", gvrs[0].Version)
	assert.Equal(t, "pods", gvrs[0].Resource)
	assert.Equal(t, "v1", gvrs[1].Version)
}

// --- getPodsForService ---

func TestGetPodsForService(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "web"},
			Ports:    []corev1.ServicePort{{Port: 80}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-pod",
			Namespace: "default",
			Labels:    map[string]string{"app": "web"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	cs := k8sfake.NewClientset(svc, pod)
	c := newFakeClient(cs, nil)

	items, err := c.getPodsForService(context.Background(), "", "default", "my-svc")
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "web-pod", items[0].Name)
}

func TestGetPodsForService_NoSelector(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "headless", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 80}},
		},
	}
	cs := k8sfake.NewClientset(svc)
	c := newFakeClient(cs, nil)

	items, err := c.getPodsForService(context.Background(), "", "default", "headless")
	require.NoError(t, err)
	assert.Nil(t, items)
}

// --- buildPodTree ---

func TestBuildPodTree(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init"}},
			Containers:     []corev1.Container{{Name: "app"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			InitContainerStatuses: []corev1.ContainerStatus{
				{Name: "init", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed"}}},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
			},
		},
	}
	cs := k8sfake.NewClientset(pod)
	dc := newFakeDynClient()
	c := newFakeClient(cs, dc)

	root := &model.ResourceNode{Name: "my-pod", Kind: "Pod", Namespace: "default"}
	err := c.buildPodTree(context.Background(), "", "default", "my-pod", root)
	require.NoError(t, err)
	assert.Equal(t, "Running", root.Status)
	assert.Len(t, root.Children, 2) // init + app containers
}

// --- getHelmManagedResources ---

func TestGetHelmManagedResources(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myapp-deploy",
			Namespace: "default",
			Labels:    map[string]string{"app.kubernetes.io/instance": "myapp"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app:v1"}}},
			},
		},
		Status: appsv1.DeploymentStatus{AvailableReplicas: 1, ReadyReplicas: 1},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myapp-svc",
			Namespace: "default",
			Labels:    map[string]string{"app.kubernetes.io/instance": "myapp"},
		},
		Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80}}},
	}
	cs := k8sfake.NewClientset(dep, svc)
	c := newFakeClient(cs, nil)

	items, err := c.getHelmManagedResources(context.Background(), "", "default", "myapp")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(items), 2)
}

// --- getPodsOnNode (via dynamic) ---

func TestGetOwnedResources_DefaultReturnsNil(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	items, err := c.GetOwnedResources(context.Background(), "", "default", "ConfigMap", "my-cm")
	require.NoError(t, err)
	assert.Nil(t, items)
}

func TestGetOwnedResources_CronJob(t *testing.T) {
	job := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "batch/v1",
			"kind":       "Job",
			"metadata": map[string]any{
				"name":      "my-cron-12345",
				"namespace": "default",
				"ownerReferences": []any{
					map[string]any{
						"kind": "CronJob",
						"name": "my-cron",
					},
				},
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{"type": "Complete", "status": "True"},
				},
			},
		},
	}
	dc := newFakeDynClient(job)
	c := newFakeClient(nil, dc)

	items, err := c.GetOwnedResources(context.Background(), "", "default", "CronJob", "my-cron")
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "my-cron-12345", items[0].Name)
}

// --- GetResourceTree ---

func TestGetResourceTree_UnknownKind(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	root, err := c.GetResourceTree(context.Background(), "", "default", "ConfigMap", "my-cm")
	require.NoError(t, err)
	assert.Equal(t, "my-cm", root.Name)
}

// helper
//
//go:fix inline
func int32Ptr(i int32) *int32 { return new(i) }
