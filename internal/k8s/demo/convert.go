package demo

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// demoEpoch anchors every relative seed timestamp (pod start times, event
// firstSeen/lastSeen, node boot time) to one instant, so an object's fields
// stay internally consistent (e.g. an event never precedes the pod it
// describes) no matter when the demo cluster is loaded.
var demoEpoch = time.Now().UTC()

// mustToUnstructured converts a typed API object into the unstructured form
// the dynamic fake client serves. It panics on conversion failure, which can
// only happen here if one of the static builders below is malformed — a
// programming error the package's own tests catch immediately.
func mustToUnstructured(obj runtime.Object) *unstructured.Unstructured {
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		panic(fmt.Sprintf("demo: converting %T to unstructured: %v", obj, err))
	}
	return &unstructured.Unstructured{Object: m}
}

// managedField builds one metadata.managedFields entry, mirroring the shape
// kube-apiserver records for a real field manager: a manager name, the
// operation that wrote it, and structured FieldsV1 data naming the paths it
// owns.
func managedField(manager, operation, fieldsJSON string, at time.Time) metav1.ManagedFieldsEntry {
	fields := &metav1.FieldsV1{}
	fields.SetRawBytes([]byte(fieldsJSON))
	t := metav1.NewTime(at)
	return metav1.ManagedFieldsEntry{
		Manager:    manager,
		Operation:  metav1.ManagedFieldsOperationType(operation),
		Time:       &t,
		FieldsType: "FieldsV1",
		FieldsV1:   fields,
	}
}

func ptrTime(t time.Time) *metav1.Time {
	mt := metav1.NewTime(t)
	return &mt
}
