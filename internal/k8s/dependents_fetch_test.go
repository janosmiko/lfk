package k8s

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestDependentKindsFor(t *testing.T) {
	tests := []struct {
		name         string
		ownerKind    string
		ownerName    string
		ownerSel     string
		wantKnown    bool
		wantSelector map[string]string // resource -> expected label selector
	}{
		{
			name:      "unknown kind",
			ownerKind: "ConfigMap",
		},
		{
			name:      "deployment narrows both child lists",
			ownerKind: "Deployment",
			ownerName: "web",
			ownerSel:  "app=web",
			wantKnown: true,
			wantSelector: map[string]string{
				"replicasets": "app=web",
				"pods":        "app=web",
			},
		},
		{
			name:      "statefulset leaves claims unnarrowed",
			ownerKind: "StatefulSet",
			ownerName: "db",
			ownerSel:  "app=db",
			wantKnown: true,
			wantSelector: map[string]string{
				"controllerrevisions":    "app=db",
				"pods":                   "app=db",
				"persistentvolumeclaims": "",
			},
		},
		{
			name:      "cronjob has no selector to narrow with",
			ownerKind: "CronJob",
			ownerName: "nightly",
			wantKnown: true,
			wantSelector: map[string]string{
				"jobs": "",
				"pods": "",
			},
		},
		{
			name:      "service narrows by the endpointslice label",
			ownerKind: "Service",
			ownerName: "api",
			wantKnown: true,
			wantSelector: map[string]string{
				"endpointslices": serviceNameLabel + "=api",
			},
		},
		{
			name:      "a deployment with no selector lists the namespace",
			ownerKind: "Deployment",
			ownerName: "web",
			wantKnown: true,
			wantSelector: map[string]string{
				"replicasets": "",
				"pods":        "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kinds, known := DependentKindsFor(tt.ownerKind, tt.ownerName, tt.ownerSel)
			if known != tt.wantKnown {
				t.Fatalf("known = %v, want %v", known, tt.wantKnown)
			}
			if !known {
				return
			}
			got := make(map[string]string, len(kinds))
			for _, k := range kinds {
				got[k.GVR.Resource] = k.Selector
			}
			for resource, want := range tt.wantSelector {
				if got[resource] != want {
					t.Errorf("%s selector = %q, want %q", resource, got[resource], want)
				}
			}
			if len(got) != len(tt.wantSelector) {
				t.Errorf("kinds = %v, want exactly %v", got, tt.wantSelector)
			}
		})
	}
}

func TestDependentKindsForDoesNotMutateTheTable(t *testing.T) {
	if _, ok := DependentKindsFor("Deployment", "web", "app=web"); !ok {
		t.Fatal("Deployment should be known")
	}
	kinds, _ := DependentKindsFor("Deployment", "other", "")
	for _, k := range kinds {
		if k.Selector != "" {
			t.Fatalf("%s kept a selector from an earlier call: %q", k.GVR.Resource, k.Selector)
		}
	}
}

func TestHasDependentKinds(t *testing.T) {
	if !HasDependentKinds("Deployment") {
		t.Error("Deployment should be walkable")
	}
	if HasDependentKinds("ConfigMap") {
		t.Error("ConfigMap should not be walkable")
	}
}

func TestMergeDependentKinds(t *testing.T) {
	got := MergeDependentKinds([]string{"Deployment", "ReplicaSet", "CronJob", "ConfigMap"})

	seen := make(map[string]int)
	for _, k := range got {
		seen[k.GVR.Resource]++
		if k.Selector != "" {
			t.Errorf("%s is scoped, but a bulk list must cover every target", k.GVR.Resource)
		}
	}
	if seen["pods"] != 1 {
		t.Errorf("pods listed %d times, want once", seen["pods"])
	}
	if seen["replicasets"] != 1 || seen["jobs"] != 1 {
		t.Errorf("merged kinds = %v, want one replicasets and one jobs", seen)
	}
}

func TestDependentRefsFrom(t *testing.T) {
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{podsGVR: "PodList"},
		podUnstructured("owned", "u-owned", "owner-uid"),
		podUnstructured("free", "u-free", ""),
	)

	refs, err := dependentRefsFrom(t.Context(), dyn, "default",
		[]DependentKind{{GVR: podsGVR, Kind: "Pod"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("refs = %+v, want only the owned pod", refs)
	}
	if refs[0].UID != "u-owned" || refs[0].OwnerUIDs[0] != "owner-uid" {
		t.Errorf("refs[0] = %+v, want the owned pod and its owner", refs[0])
	}
}

func TestDependentRefsFromPassesTheSelectorToTheServer(t *testing.T) {
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{podsGVR: "PodList"})

	var got string
	dyn.PrependReactor("list", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		got = a.(k8stesting.ListAction).GetListRestrictions().Labels.String()
		return false, nil, nil
	})

	if _, err := dependentRefsFrom(t.Context(), dyn, "default",
		[]DependentKind{{GVR: podsGVR, Kind: "Pod", Selector: "app=web"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "app=web" {
		t.Errorf("server saw selector %q, want %q", got, "app=web")
	}
}

func podUnstructured(name, uid, ownerUID string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "default",
			"uid":       uid,
		},
	}}
	if ownerUID != "" {
		obj.SetOwnerReferences([]metav1.OwnerReference{{
			APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "rs", UID: types.UID(ownerUID),
		}})
	}
	return obj
}

func TestDependentRefsFromRefusesAClusterWideList(t *testing.T) {
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{podsGVR: "PodList"})

	dyn.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		t.Fatal("an empty namespace reached the server as a cluster-wide list")
		return false, nil, nil
	})

	if _, err := dependentRefsFrom(t.Context(), dyn, "",
		[]DependentKind{{GVR: podsGVR, Kind: "Pod"}}); err == nil {
		t.Fatal("expected an error for an empty namespace")
	}
}
