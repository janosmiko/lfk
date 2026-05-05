package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResolvePodsForWorkload_PodSelf(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "default"},
	})
	c := NewTestClient(cs, nil)
	pods, err := c.resolvePodsForWorkload(context.Background(), "test-ctx", "default", "Pod", "pod-a")
	assert.NoError(t, err)
	assert.Equal(t, []string{"pod-a"}, pods)
}

func TestResolvePodsForWorkload_DeploymentByLabels(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "frontend"}},
		},
	}
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-aaa", Namespace: "default", Labels: map[string]string{"app": "frontend"}},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-bbb", Namespace: "default", Labels: map[string]string{"app": "frontend"}},
	}
	other := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-zzz", Namespace: "default", Labels: map[string]string{"app": "backend"}},
	}
	cs := fake.NewSimpleClientset(dep, pod1, pod2, other)
	c := NewTestClient(cs, nil)
	pods, err := c.resolvePodsForWorkload(context.Background(), "test-ctx", "default", "Deployment", "frontend")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"frontend-aaa", "frontend-bbb"}, pods, "backend pod with non-matching labels must be excluded")
}

func TestResolvePodsForWorkload_StatefulSetByLabels(t *testing.T) {
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "db-0", Namespace: "default", Labels: map[string]string{"app": "db"}},
	}
	cs := fake.NewSimpleClientset(ss, pod)
	c := NewTestClient(cs, nil)
	pods, err := c.resolvePodsForWorkload(context.Background(), "test-ctx", "default", "StatefulSet", "db")
	assert.NoError(t, err)
	assert.Equal(t, []string{"db-0"}, pods)
}

func TestResolvePodsForWorkload_DaemonSetByLabels(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "node-exporter", Namespace: "default"},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "node-exporter"}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "node-exporter-x", Namespace: "default", Labels: map[string]string{"app": "node-exporter"}},
	}
	cs := fake.NewSimpleClientset(ds, pod)
	c := NewTestClient(cs, nil)
	pods, err := c.resolvePodsForWorkload(context.Background(), "test-ctx", "default", "DaemonSet", "node-exporter")
	assert.NoError(t, err)
	assert.Equal(t, []string{"node-exporter-x"}, pods)
}

func TestResolvePodsForWorkload_JobByLabels(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "report", Namespace: "default"},
		Spec: batchv1.JobSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"job-name": "report"}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "report-x", Namespace: "default", Labels: map[string]string{"job-name": "report"}},
	}
	cs := fake.NewSimpleClientset(job, pod)
	c := NewTestClient(cs, nil)
	pods, err := c.resolvePodsForWorkload(context.Background(), "test-ctx", "default", "Job", "report")
	assert.NoError(t, err)
	assert.Equal(t, []string{"report-x"}, pods)
}

func TestResolvePodsForWorkload_CronJobWalksJobs(t *testing.T) {
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "report", Namespace: "default", UID: "cj-uid"},
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "report-1", Namespace: "default", UID: "job-uid",
			OwnerReferences: []metav1.OwnerReference{{Kind: "CronJob", Name: "report", UID: "cj-uid"}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "report-1-pod", Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{Kind: "Job", Name: "report-1", UID: "job-uid"}},
		},
	}
	cs := fake.NewSimpleClientset(cj, job, pod)
	c := NewTestClient(cs, nil)
	pods, err := c.resolvePodsForWorkload(context.Background(), "test-ctx", "default", "CronJob", "report")
	assert.NoError(t, err)
	assert.Equal(t, []string{"report-1-pod"}, pods)
}

func TestResolvePodsForWorkload_UnsupportedKind(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := NewTestClient(cs, nil)
	_, err := c.resolvePodsForWorkload(context.Background(), "test-ctx", "default", "Service", "x")
	assert.Error(t, err, "kinds outside the in-scope set must error")
}
