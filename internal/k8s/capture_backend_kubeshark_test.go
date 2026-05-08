package k8s

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestDetectKubeshark_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	dc := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	c := newFakeClient(cs, dc)
	info, err := c.DetectKubeshark(context.Background(), "test-ctx")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if info != nil {
		t.Errorf("info = %+v, want nil", info)
	}
}

func TestDetectKubeshark_Found(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "kubeshark-hub", Namespace: "kubeshark"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 80, Name: "http"}},
		},
	}
	cs := k8sfake.NewClientset(svc)
	dc := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	c := newFakeClient(cs, dc)
	info, err := c.DetectKubeshark(context.Background(), "test-ctx")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if info == nil {
		t.Fatal("info nil, want populated")
	}
	if info.Namespace != "kubeshark" || info.HubService != "kubeshark-hub" || info.HubPort != 80 {
		t.Errorf("info = %+v, want ns=kubeshark name=kubeshark-hub port=80", *info)
	}
}

// TestClient_KubesharkNamespace_OverrideAndDefault guards the SetKubesharkNamespace
// plumbing: an explicit non-empty override is returned by kubesharkNamespace();
// empty string falls back to the "kubeshark" default.
func TestClient_KubesharkNamespace_OverrideAndDefault(t *testing.T) {
	c := &Client{}
	if got := c.kubesharkNamespace(); got != "kubeshark" {
		t.Errorf("default = %q, want kubeshark", got)
	}
	c.SetKubesharkNamespace("trafcap")
	if got := c.kubesharkNamespace(); got != "trafcap" {
		t.Errorf("after override = %q, want trafcap", got)
	}
	c.SetKubesharkNamespace("")
	if got := c.kubesharkNamespace(); got != "kubeshark" {
		t.Errorf("after reset to empty = %q, want default kubeshark", got)
	}
}

// TestDetectKubeshark_HonoursOverrideNamespace ensures the override is the
// namespace actually probed by DetectKubeshark — not just the field that
// gets read.
func TestDetectKubeshark_HonoursOverrideNamespace(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "kubeshark-hub", Namespace: "trafcap"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80, Name: "http"}}},
	}
	cs := k8sfake.NewClientset(svc)
	dc := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	c := newFakeClient(cs, dc)
	c.SetKubesharkNamespace("trafcap")

	info, err := c.DetectKubeshark(context.Background(), "test-ctx")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if info == nil {
		t.Fatal("info nil — probe did not find Service in the override namespace")
	}
	if info.Namespace != "trafcap" {
		t.Errorf("info.Namespace = %q, want trafcap", info.Namespace)
	}
}

func TestKubesharkURL(t *testing.T) {
	got := kubesharkURL(8899, "my-pod")
	want := `http://localhost:8899/?q=name+%3D%3D+%22my-pod%22`
	if got != want {
		t.Errorf("URL = %s, want %s", got, want)
	}
}

func TestWaitForKubesharkPort_Timeout(t *testing.T) {
	mgr := NewPortForwardManager()
	t.Cleanup(mgr.StopAll)
	// No matching entry — must time out.
	_, err := waitForKubesharkPort(context.Background(), mgr, 999, 50*time.Millisecond, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestWaitForKubesharkPort_ContextCancel(t *testing.T) {
	mgr := NewPortForwardManager()
	t.Cleanup(mgr.StopAll)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := waitForKubesharkPort(ctx, mgr, 999, 5*time.Second, 10*time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
