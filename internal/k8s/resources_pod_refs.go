package k8s

import (
	"context"

	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/model"
)

// refKey identifies a unique reference target within a Pod's namespace.
type refKey struct {
	kind, name string
}

// refEntry tracks a unique pod reference plus whether any reference site is
// non-optional. Required wins on dedup — if the same Secret is referenced once
// as required and once as optional, we treat it as required so a missing
// object surfaces as MissingRef.
type refEntry struct {
	kind, name string
	required   bool
}

// existsFn reports whether a namespaced object of the given kind+name exists.
// Returning true on uncertain errors (RBAC, network) avoids false-flagging.
type existsFn func(kind, name string) bool

// appendPodRefs walks a Pod's spec and appends Secret/ConfigMap/PVC/ServiceAccount
// child nodes for every distinct reference. Refs are deduped by (kind, name) and
// emitted in a stable order: ServiceAccount, ConfigMap, Secret, PersistentVolumeClaim.
//
// podObj is the unstructured pod representation (same shape consumed by
// appendContainerNodes). namespace is the pod's namespace — Secrets, ConfigMaps,
// PVCs, and ServiceAccounts are all namespace-scoped, so cross-namespace refs
// can't occur via env/volume/SA fields.
//
// If exists is non-nil, required refs whose target is reported missing get
// Status=model.MissingRefStatus. Optional refs (env.valueFrom.*.optional=true,
// envFrom.*.optional=true, volume.*.optional=true) are never flagged. A nil
// exists skips the check entirely (used by tests and callers that don't need it).
//
// At large scale (e.g. a Deployment with N replicas) the same Secret is emitted
// under each Pod, but treeCache hands every Pod in one tree the same exists, so
// the GET happens once per (kind, name).
func appendPodRefs(podNode *model.ResourceNode, podObj map[string]any, namespace string, exists existsFn) {
	for _, r := range collectPodRefs(podObj) {
		status := ""
		if r.required && exists != nil && !exists(r.kind, r.name) {
			status = model.MissingRefStatus
		}
		podNode.Children = append(podNode.Children, &model.ResourceNode{
			Name:      r.name,
			Kind:      r.kind,
			Namespace: namespace,
			Status:    status,
			Group:     "refs",
		})
	}
}

// collectPodRefs returns the Pod's distinct references in emit order:
// ServiceAccount, ConfigMap, Secret, PersistentVolumeClaim.
func collectPodRefs(podObj map[string]any) []refEntry {
	spec, _ := podObj["spec"].(map[string]any)
	if spec == nil {
		return nil
	}

	// Per-bucket ordered slices preserve stable emit order while the seen map
	// dedupes by (kind, name) and lets a later required reference upgrade an
	// earlier optional one.
	seen := map[refKey]int{}
	var sas, cms, secrets, pvcs []refEntry

	add := func(kind, name string, optional bool) {
		if name == "" {
			return
		}
		k := refKey{kind: kind, name: name}
		var bucket *[]refEntry
		switch kind {
		case "ServiceAccount":
			bucket = &sas
		case "ConfigMap":
			bucket = &cms
		case "Secret":
			bucket = &secrets
		case "PersistentVolumeClaim":
			bucket = &pvcs
		default:
			return
		}
		if idx, ok := seen[k]; ok {
			if !optional {
				(*bucket)[idx].required = true
			}
			return
		}
		seen[k] = len(*bucket)
		*bucket = append(*bucket, refEntry{kind: kind, name: name, required: !optional})
	}

	// ServiceAccount: skip if automountServiceAccountToken is explicitly false.
	// Empty serviceAccountName defaults to "default". Always required.
	if automount, ok := spec["automountServiceAccountToken"].(bool); !ok || automount {
		saName, _ := spec["serviceAccountName"].(string)
		if saName == "" {
			saName = "default"
		}
		add("ServiceAccount", saName, false)
	}

	// imagePullSecrets are required at pull time. Treat as non-optional.
	if pull, ok := spec["imagePullSecrets"].([]any); ok {
		for _, p := range pull {
			if m, ok := p.(map[string]any); ok {
				if n, _ := m["name"].(string); n != "" {
					add("Secret", n, false)
				}
			}
		}
	}

	for _, key := range []string{"initContainers", "containers", "ephemeralContainers"} {
		containers, _ := spec[key].([]any)
		for _, c := range containers {
			cMap, ok := c.(map[string]any)
			if !ok {
				continue
			}
			collectContainerRefs(cMap, add)
		}
	}

	if vols, ok := spec["volumes"].([]any); ok {
		for _, v := range vols {
			vMap, ok := v.(map[string]any)
			if !ok {
				continue
			}
			collectVolumeRefs(vMap, add)
		}
	}

	refs := make([]refEntry, 0, len(sas)+len(cms)+len(secrets)+len(pvcs))
	for _, bucket := range [][]refEntry{sas, cms, secrets, pvcs} {
		refs = append(refs, bucket...)
	}
	return refs
}

func collectContainerRefs(c map[string]any, add func(kind, name string, optional bool)) {
	if env, ok := c["env"].([]any); ok {
		for _, e := range env {
			eMap, ok := e.(map[string]any)
			if !ok {
				continue
			}
			vf, _ := eMap["valueFrom"].(map[string]any)
			if vf == nil {
				continue
			}
			if sk, ok := vf["secretKeyRef"].(map[string]any); ok {
				name, _ := sk["name"].(string)
				opt, _ := sk["optional"].(bool)
				add("Secret", name, opt)
			}
			if ck, ok := vf["configMapKeyRef"].(map[string]any); ok {
				name, _ := ck["name"].(string)
				opt, _ := ck["optional"].(bool)
				add("ConfigMap", name, opt)
			}
		}
	}
	if envFrom, ok := c["envFrom"].([]any); ok {
		for _, e := range envFrom {
			eMap, ok := e.(map[string]any)
			if !ok {
				continue
			}
			if sr, ok := eMap["secretRef"].(map[string]any); ok {
				name, _ := sr["name"].(string)
				opt, _ := sr["optional"].(bool)
				add("Secret", name, opt)
			}
			if cr, ok := eMap["configMapRef"].(map[string]any); ok {
				name, _ := cr["name"].(string)
				opt, _ := cr["optional"].(bool)
				add("ConfigMap", name, opt)
			}
		}
	}
}

func collectVolumeRefs(v map[string]any, add func(kind, name string, optional bool)) {
	// SecretVolumeSource uses secretName. SecretProjection uses name.
	if s, ok := v["secret"].(map[string]any); ok {
		name, _ := s["secretName"].(string)
		opt, _ := s["optional"].(bool)
		add("Secret", name, opt)
	}
	if cm, ok := v["configMap"].(map[string]any); ok {
		name, _ := cm["name"].(string)
		opt, _ := cm["optional"].(bool)
		add("ConfigMap", name, opt)
	}
	if pvc, ok := v["persistentVolumeClaim"].(map[string]any); ok {
		// PVC volume source has no optional concept — always required.
		name, _ := pvc["claimName"].(string)
		add("PersistentVolumeClaim", name, false)
	}
	if proj, ok := v["projected"].(map[string]any); ok {
		if sources, ok := proj["sources"].([]any); ok {
			for _, src := range sources {
				sMap, ok := src.(map[string]any)
				if !ok {
					continue
				}
				if s, ok := sMap["secret"].(map[string]any); ok {
					name, _ := s["name"].(string)
					opt, _ := s["optional"].(bool)
					add("Secret", name, opt)
				}
				if cm, ok := sMap["configMap"].(map[string]any); ok {
					name, _ := cm["name"].(string)
					opt, _ := cm["optional"].(bool)
					add("ConfigMap", name, opt)
				}
				// Skip serviceAccountToken and downwardAPI sources.
			}
		}
	}
}

var refGVRs = map[string]schema.GroupVersionResource{
	"Secret":                {Group: "", Version: "v1", Resource: "secrets"},
	"ConfigMap":             {Group: "", Version: "v1", Resource: "configmaps"},
	"PersistentVolumeClaim": {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	"ServiceAccount":        {Group: "", Version: "v1", Resource: "serviceaccounts"},
}

// refPrefetchLimit bounds warm's concurrent GETs so a release with hundreds of
// refs does not flood the apiserver.
const refPrefetchLimit = 8

// refChecker resolves ref existence in one namespace and caches every answer.
// ctx carries no timeout of its own: pass one with a deadline, or a slow
// apiserver blocks indefinitely. cache is not safe for concurrent use.
type refChecker struct {
	ctx       context.Context
	dyn       dynamic.Interface
	namespace string
	cache     map[refKey]bool
}

func newRefChecker(ctx context.Context, dynClient dynamic.Interface, namespace string) *refChecker {
	return &refChecker{ctx: ctx, dyn: dynClient, namespace: namespace, cache: map[refKey]bool{}}
}

func (r *refChecker) exists(kind, name string) bool {
	k := refKey{kind: kind, name: name}
	if v, ok := r.cache[k]; ok {
		return v
	}
	v := r.lookup(k)
	r.cache[k] = v

	return v
}

// lookup treats every error but IsNotFound as "exists", so a transient RBAC or
// network failure does not false-flag a ref.
func (r *refChecker) lookup(k refKey) bool {
	gvr, ok := refGVRs[k.kind]
	if !ok {
		return true
	}
	_, err := r.dyn.Resource(gvr).Namespace(r.namespace).Get(r.ctx, k.name, metav1.GetOptions{})

	return err == nil || !apierrors.IsNotFound(err)
}

// warm resolves every required ref of pods up front, in parallel. Serially the
// tree build paid one round trip per distinct ref, which dominates its runtime
// on a release with many workloads.
func (r *refChecker) warm(pods []unstructured.Unstructured) {
	var todo []refKey
	seen := map[refKey]bool{}
	for i := range pods {
		for _, ref := range collectPodRefs(pods[i].Object) {
			k := refKey{kind: ref.kind, name: ref.name}
			if !ref.required || seen[k] {
				continue
			}
			seen[k] = true
			if _, cached := r.cache[k]; !cached {
				todo = append(todo, k)
			}
		}
	}
	if len(todo) == 0 {
		return
	}

	results := make([]bool, len(todo))
	var g errgroup.Group
	g.SetLimit(refPrefetchLimit)
	for i, k := range todo {
		g.Go(func() error {
			results[i] = r.lookup(k)

			return nil
		})
	}
	_ = g.Wait()

	// Written from one goroutine only, after every worker has finished.
	for i, k := range todo {
		r.cache[k] = results[i]
	}
}

func newRefExistsFn(ctx context.Context, dynClient dynamic.Interface, namespace string) existsFn {
	return newRefChecker(ctx, dynClient, namespace).exists
}
