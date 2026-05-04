package k8s

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// At large scale (e.g. a Deployment with N replicas) the same Secret will be
// emitted under each Pod and existence-checked once per pod. Cross-pod dedup is
// intentionally deferred — if it ever becomes a perf issue, batch a LIST per
// kind at GetResourceTree level.
func appendPodRefs(podNode *model.ResourceNode, podObj map[string]any, namespace string, exists existsFn) {
	spec, _ := podObj["spec"].(map[string]any)
	if spec == nil {
		return
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

	// imagePullSecrets are required at pull time; treat as non-optional.
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

	for _, bucket := range [][]refEntry{sas, cms, secrets, pvcs} {
		for _, r := range bucket {
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
	// SecretVolumeSource uses secretName; SecretProjection uses name.
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

// newRefExistsFn returns an existsFn that resolves Secret/ConfigMap/PVC/SA
// existence via the dynamic client in the given namespace. The returned
// closure caches results so repeated lookups for the same (kind, name) within
// a single tree build cost one GET. Errors other than IsNotFound are treated
// as "exists" so transient RBAC/network failures don't false-flag refs.
//
// The provided ctx is reused for every kube GET the closure issues — it does
// not impose its own timeout, so callers should pass a context with a
// deadline or cancellation to avoid indefinite blocking on a slow apiserver.
//
// The cache map is not safe for concurrent use; the closure assumes
// sequential calls within a single tree build (which is how
// build*Tree paths invoke it today).
func newRefExistsFn(ctx context.Context, dynClient dynamic.Interface, namespace string) existsFn {
	gvrFor := map[string]schema.GroupVersionResource{
		"Secret":                {Group: "", Version: "v1", Resource: "secrets"},
		"ConfigMap":             {Group: "", Version: "v1", Resource: "configmaps"},
		"PersistentVolumeClaim": {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		"ServiceAccount":        {Group: "", Version: "v1", Resource: "serviceaccounts"},
	}
	cache := map[refKey]bool{}
	return func(kind, name string) bool {
		k := refKey{kind: kind, name: name}
		if v, ok := cache[k]; ok {
			return v
		}
		gvr, ok := gvrFor[kind]
		if !ok {
			cache[k] = true
			return true
		}
		_, err := dynClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		exists := err == nil || !apierrors.IsNotFound(err)
		cache[k] = exists
		return exists
	}
}
