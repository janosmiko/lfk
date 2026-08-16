package demo

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// GuardListPanics wraps a dynamic.Interface so a List call against an
// unregistered GVR degrades to an error instead of panicking. The fake
// dynamic client (dynamicfake.NewSimpleDynamicClientWithCustomListKinds)
// panics rather than errors when its ListKinds map is missing an entry. See
// ListKinds for the registration this guards against ever missing again.
// DiscoverAPIResources and similar calls run on a scheduler worker goroutine
// with nothing upstream able to recover a panic. A fixture gap here would
// otherwise crash the whole app instead of degrading one feature. This
// wrapper is demo-only: a real cluster's dynamic client talks to an API
// server and never panics on List. So production code paths never carry
// this guard.
func GuardListPanics(dyn dynamic.Interface) dynamic.Interface {
	return recoveringDynamicClient{Interface: dyn}
}

type recoveringDynamicClient struct {
	dynamic.Interface
}

func (r recoveringDynamicClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return recoveringNamespaceableResourceInterface{
		NamespaceableResourceInterface: r.Interface.Resource(gvr),
		gvr:                            gvr,
	}
}

type recoveringNamespaceableResourceInterface struct {
	dynamic.NamespaceableResourceInterface
	gvr schema.GroupVersionResource
}

func (r recoveringNamespaceableResourceInterface) Namespace(ns string) dynamic.ResourceInterface {
	return recoveringResourceInterface{
		ResourceInterface: r.NamespaceableResourceInterface.Namespace(ns),
		gvr:               r.gvr,
	}
}

func (r recoveringNamespaceableResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (list *unstructured.UnstructuredList, err error) {
	defer recoverListPanic(r.gvr, &err)
	return r.NamespaceableResourceInterface.List(ctx, opts)
}

type recoveringResourceInterface struct {
	dynamic.ResourceInterface
	gvr schema.GroupVersionResource
}

func (r recoveringResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (list *unstructured.UnstructuredList, err error) {
	defer recoverListPanic(r.gvr, &err)
	return r.ResourceInterface.List(ctx, opts)
}

// recoverListPanic converts a panic from the fake dynamic client's List
// into an error on the named return, identified by gvr for the caller.
func recoverListPanic(gvr schema.GroupVersionResource, err *error) {
	if p := recover(); p != nil {
		*err = fmt.Errorf("demo: listing %s: %v", gvr, p)
	}
}
