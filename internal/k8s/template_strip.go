// Package k8s — template_strip.go
// Turns a live object's YAML into a manifest that applies cleanly somewhere
// else. Two layers: a generic strip every kind gets, and a per-kind strip for
// the kinds whose server-set spec fields are unambiguous. Kinds without a
// per-kind entry get the generic strip only — leaving a field in is recoverable,
// removing a meaningful one is not.
package k8s

import (
	"errors"
	"fmt"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// errTemplateNotAnObject is returned for input that is not a single YAML
// mapping — a list, a scalar, or an empty document.
var errTemplateNotAnObject = errors.New("not a Kubernetes object")

// injectedTokenVolumePrefix is the name the API server gives the projected
// ServiceAccount token volume it injects into every Pod.
const injectedTokenVolumePrefix = "kube-api-access-"

// metadataServerFields are set by the API server on every object regardless of
// kind, and are rejected or ignored on create.
var metadataServerFields = []string{
	"uid",
	"resourceVersion",
	"generation",
	"creationTimestamp",
	"managedFields",
	"selfLink",
	"ownerReferences",
	"deletionTimestamp",
	"deletionGracePeriodSeconds",
	// A template carrying a finalizer creates an object that cannot be deleted
	// until some controller removes it — never, if that controller is absent
	// in the target cluster.
	"finalizers",
	// Dropped so the template is portable: without it, `kubectl apply -n <ns>`
	// decides where the object lands instead of the namespace it was read from.
	"namespace",
}

// annotationsAlwaysDropped are written by kubectl or a controller, never by the
// author of the manifest.
var annotationsAlwaysDropped = []string{
	"kubectl.kubernetes.io/last-applied-configuration",
	"deployment.kubernetes.io/revision",
}

// pvcServerAnnotations record the outcome of binding a PVC to a PV. Carrying
// them to a new namespace claims a binding that does not exist there.
var pvcServerAnnotations = []string{
	"pv.kubernetes.io/bind-completed",
	"pv.kubernetes.io/bound-by-controller",
	"volume.beta.kubernetes.io/storage-provisioner",
	"volume.kubernetes.io/storage-provisioner",
	"volume.kubernetes.io/selected-node",
}

// secretKind is the one kind whose values are redacted rather than kept.
const secretKind = "Secret"

// TemplateRedactsValues reports whether StripToTemplate blanks the values of
// this kind, so an exporter can tell the user before they paste an empty
// template. Tied to the switch in StripToTemplate by the shared constant.
func TemplateRedactsValues(kind string) bool {
	return kind == secretKind
}

// redactSecretValues blanks every value under data and stringData while keeping
// the keys and the type. A template is a shape, not a payload: all three export
// destinations persist, and a template directory tends to end up in dotfiles.
// The keys and the type are what make the template useful — they say which
// entries to fill in and what kind of Secret this is. Users who want the live
// values already have the copy and YAML-view paths.
func redactSecretValues(obj map[string]any) {
	for _, field := range []string{"data", "stringData"} {
		for key := range childMap(obj, field) {
			childMap(obj, field)[key] = ""
		}
	}
}

// StripToTemplate rewrites a live object's YAML into a reusable template:
// status and the server-set metadata go, then the server-set spec fields for
// the kinds that have them. The result is meant to apply unchanged into an
// empty namespace.
func StripToTemplate(doc string) (string, error) {
	var obj map[string]any
	if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
		return "", fmt.Errorf("parsing YAML: %w", err)
	}
	if len(obj) == 0 {
		return "", errTemplateNotAnObject
	}

	delete(obj, "status")
	stripObjectMeta(obj, annotationsAlwaysDropped)
	stripEmbeddedTemplateMeta(obj)
	stripControllerLabels(obj)
	stripHelmOwnership(obj)
	stripVendorRuntimeAnnotations(obj)

	kind, _ := obj["kind"].(string)
	switch kind {
	case "Pod":
		stripPodSpec(childMap(obj, "spec"))
	case "Service":
		deleteKeys(childMap(obj, "spec"), "clusterIP", "clusterIPs", "ipFamilies", "ipFamilyPolicy")
	case "PersistentVolumeClaim":
		deleteKeys(childMap(obj, "spec"), "volumeName")
		stripAnnotations(obj, pvcServerAnnotations)
	case "Job":
		stripJob(obj)
	case secretKind:
		redactSecretValues(obj)
	}

	out, err := marshalResourceYAML(obj)
	if err != nil {
		return "", fmt.Errorf("marshalling to YAML: %w", err)
	}
	return reorderYAMLFields(out), nil
}

// stripObjectMeta removes the server-set metadata fields and the given
// annotations from the object's top-level metadata.
func stripObjectMeta(obj map[string]any, dropAnnotations []string) {
	md := childMap(obj, "metadata")
	if md == nil {
		return
	}
	deleteKeys(md, metadataServerFields...)
	deleteKeys(childMap(md, "annotations"), dropAnnotations...)
	pruneEmptyChild(md, "annotations")
}

// stripAnnotations removes annotations from the object's top-level metadata.
func stripAnnotations(obj map[string]any, keys []string) {
	md := childMap(obj, "metadata")
	deleteKeys(childMap(md, "annotations"), keys...)
	pruneEmptyChild(md, "annotations")
}

// stripEmbeddedTemplateMeta drops the creationTimestamp the serializer emits
// inside an embedded template. It is never author-written and renders as the
// confusing `creationTimestamp: null`.
func stripEmbeddedTemplateMeta(obj map[string]any) {
	for _, tmpl := range embeddedTemplates(obj) {
		md := childMap(tmpl, "metadata")
		deleteKeys(md, "creationTimestamp")
		pruneEmptyChild(tmpl, "metadata")
	}
}

// embeddedTemplates returns every embedded template of a workload object that
// carries an ObjectMeta of its own: spec.template for
// Deployment/StatefulSet/DaemonSet/ReplicaSet/Job, and for CronJob both
// spec.jobTemplate (the JobTemplateSpec) and the pod template nested inside it.
func embeddedTemplates(obj map[string]any) []map[string]any {
	spec := childMap(obj, "spec")
	var out []map[string]any
	if t := childMap(spec, "template"); t != nil {
		out = append(out, t)
	}
	jobTmpl := childMap(spec, "jobTemplate")
	if jobTmpl != nil {
		out = append(out, jobTmpl)
	}
	if t := childMap(childMap(jobTmpl, "spec"), "template"); t != nil {
		out = append(out, t)
	}
	return out
}

// stripPodSpec removes the fields the scheduler and the ServiceAccount
// admission controller write into a live Pod: the node it landed on, the
// deprecated serviceAccount mirror of serviceAccountName, and the injected
// projected token volume together with the volumeMounts that reference it.
func stripPodSpec(spec map[string]any) {
	if spec == nil {
		return
	}
	deleteKeys(spec, "nodeName", "serviceAccount")

	volumes, _ := spec["volumes"].([]any)
	kept := make([]any, 0, len(volumes))
	injected := make(map[string]bool)
	for _, v := range volumes {
		vol, ok := v.(map[string]any)
		if ok && isInjectedTokenVolume(vol) {
			name, _ := vol["name"].(string)
			injected[name] = true
			continue
		}
		kept = append(kept, v)
	}
	if len(injected) == 0 {
		return
	}
	if len(kept) == 0 {
		delete(spec, "volumes")
	} else {
		spec["volumes"] = kept
	}
	for _, key := range []string{"containers", "initContainers", "ephemeralContainers"} {
		dropVolumeMounts(spec, key, injected)
	}
}

// isInjectedTokenVolume reports whether a volume is the API server's projected
// ServiceAccount token. Both the generated name and the projected
// serviceAccountToken source must match, so a user volume that merely borrows
// the name prefix is left alone.
func isInjectedTokenVolume(vol map[string]any) bool {
	name, _ := vol["name"].(string)
	if !strings.HasPrefix(name, injectedTokenVolumePrefix) {
		return false
	}
	sources, _ := childMap(vol, "projected")["sources"].([]any)
	for _, s := range sources {
		src, ok := s.(map[string]any)
		if ok && src["serviceAccountToken"] != nil {
			return true
		}
	}
	return false
}

// dropVolumeMounts removes volumeMounts naming any of the dropped volumes from
// every container in spec[key]. A mount without its volume is rejected on create.
func dropVolumeMounts(spec map[string]any, key string, dropped map[string]bool) {
	list, _ := spec[key].([]any)
	for _, c := range list {
		container, ok := c.(map[string]any)
		if !ok {
			continue
		}
		mounts, _ := container["volumeMounts"].([]any)
		kept := make([]any, 0, len(mounts))
		for _, m := range mounts {
			mount, ok := m.(map[string]any)
			if ok {
				if name, _ := mount["name"].(string); dropped[name] {
					continue
				}
			}
			kept = append(kept, m)
		}
		if len(kept) == 0 {
			delete(container, "volumeMounts")
			continue
		}
		container["volumeMounts"] = kept
	}
}

// stripJob removes the selector the Job controller generates from the Job's UID.
// Keeping it without manualSelector makes the API server reject the create. The
// matching labels are covered by controllerGeneratedLabels.
func stripJob(obj map[string]any) {
	delete(childMap(obj, "spec"), "selector")
}

// childMap returns parent[key] as a map, or nil when absent or another type.
func childMap(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return nil
	}
	m, _ := parent[key].(map[string]any)
	return m
}

func deleteKeys(m map[string]any, keys ...string) {
	if m == nil {
		return
	}
	for _, k := range keys {
		delete(m, k)
	}
}

// pruneEmptyChild removes parent[key] when the strip emptied it, so the output
// carries no `annotations: {}` noise.
func pruneEmptyChild(parent map[string]any, key string) {
	if child := childMap(parent, key); child != nil && len(child) == 0 {
		delete(parent, key)
	}
}
