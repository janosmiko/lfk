package k8s

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ConstraintRow is one entry in a "what constrains this object" report: a
// policy or scheduling rule from one source that already blocks the target
// or narrows its headroom.
type ConstraintRow struct {
	Source    string // "Quota" | "LimitRange" | "PDB" | "PriorityClass" | "Node" | "Webhook"
	Kind      string
	Namespace string
	Name      string
	Detail    string
	Headroom  string
	Blocking  bool
}

// ConstraintReport is the aggregated result of DetectConstraints.
// Skipped names the sources a failed list omitted — see DetectConstraints.
type ConstraintReport struct {
	Rows    []ConstraintRow
	Skipped []string
}

// ContainerRequest holds one container's resource requests/limits as raw
// quantity strings, matching the string-quantity arithmetic
// GetNamespaceQuotas already uses.
type ContainerRequest struct {
	Name     string
	Requests map[string]string
	Limits   map[string]string
}

// NodeSelectorRequirement mirrors corev1.NodeSelectorRequirement without
// the JSON tags, read out of a workload's raw object.
type NodeSelectorRequirement struct {
	Key      string
	Operator string // In, NotIn, Exists, DoesNotExist, Gt, Lt
	Values   []string
}

// NodeSelectorTerm is one OR-branch of a required node affinity match.
// Every MatchExpressions entry must match (AND). Preferred terms are
// dropped upstream — they never block scheduling.
type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement
}

// ConstraintTarget is the pod-shaped part of an object (or its pod
// template) every constraint source is evaluated against.
type ConstraintTarget struct {
	Namespace         string
	Labels            map[string]string // the object's own metadata.labels
	PodLabels         map[string]string // pod template labels (workloads) or the pod's own labels
	NodeSelector      map[string]string
	Affinity          []NodeSelectorTerm
	Tolerations       []corev1.Toleration
	Containers        []ContainerRequest
	PriorityClassName string
	GVR               schema.GroupVersionResource
	Kind              string
	// PodName is the object's own name, set only when Kind is Pod — the
	// preemption check reads a specific pod's status and events, which a
	// workload's pod template has no single instance of.
	PodName string
}

// TargetFromRaw reads spec.template (a workload controller) or spec (a
// bare Pod) off the raw object. Namespace, Kind and GVR come from the
// caller's resource type entry, not from the object itself.
func TargetFromRaw(raw map[string]any) ConstraintTarget {
	return targetFromRaw(raw)
}

func targetFromRaw(raw map[string]any) ConstraintTarget {
	meta, _ := raw["metadata"].(map[string]any)
	podSpec, podMeta := podSpecAndMetaFromRaw(raw)

	var podName string
	if kind, _ := raw["kind"].(string); kind == "Pod" {
		podName = stringField(meta, "name")
	}

	return ConstraintTarget{
		Namespace:         stringField(meta, "namespace"),
		Labels:            stringMapFromRaw(meta["labels"]),
		PodLabels:         stringMapFromRaw(podMeta["labels"]),
		NodeSelector:      stringMapFromRaw(podSpec["nodeSelector"]),
		PodName:           podName,
		Affinity:          nodeAffinityTermsFromRaw(podSpec["affinity"]),
		Tolerations:       tolerationsFromRaw(podSpec["tolerations"]),
		Containers:        containerRequestsFromRaw(podSpec["containers"]),
		PriorityClassName: stringField(podSpec, "priorityClassName"),
	}
}

// podSpecAndMetaFromRaw picks the Pod spec/metadata for a bare Pod, or the
// pod template's for anything else.
func podSpecAndMetaFromRaw(raw map[string]any) (spec, meta map[string]any) {
	kind, _ := raw["kind"].(string)
	if kind == "Pod" {
		spec, _ = raw["spec"].(map[string]any)
		meta, _ = raw["metadata"].(map[string]any)
		return spec, meta
	}
	topSpec, _ := raw["spec"].(map[string]any)
	tmpl, _ := topSpec["template"].(map[string]any)
	spec, _ = tmpl["spec"].(map[string]any)
	meta, _ = tmpl["metadata"].(map[string]any)
	return spec, meta
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func rawSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func rawMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// stringMapFromRaw drops any value that isn't itself a string — a
// well-formed label/selector map never holds anything else.
func stringMapFromRaw(v any) map[string]string {
	src := rawMap(v)
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, val := range src {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	return out
}

// nodeAffinityTermsFromRaw reads
// affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms.
func nodeAffinityTermsFromRaw(v any) []NodeSelectorTerm {
	nodeAffinity := rawMap(rawMap(v)["nodeAffinity"])
	required := rawMap(nodeAffinity["requiredDuringSchedulingIgnoredDuringExecution"])
	rawTerms := rawSlice(required["nodeSelectorTerms"])
	if len(rawTerms) == 0 {
		return nil
	}
	out := make([]NodeSelectorTerm, 0, len(rawTerms))
	for _, rt := range rawTerms {
		termMap := rawMap(rt)
		exprs := rawSlice(termMap["matchExpressions"])
		term := NodeSelectorTerm{MatchExpressions: make([]NodeSelectorRequirement, 0, len(exprs))}
		for _, e := range exprs {
			em := rawMap(e)
			req := NodeSelectorRequirement{
				Key:      stringField(em, "key"),
				Operator: stringField(em, "operator"),
			}
			for _, val := range rawSlice(em["values"]) {
				if s, ok := val.(string); ok {
					req.Values = append(req.Values, s)
				}
			}
			term.MatchExpressions = append(term.MatchExpressions, req)
		}
		out = append(out, term)
	}
	return out
}

func tolerationsFromRaw(v any) []corev1.Toleration {
	rawTolerations := rawSlice(v)
	if len(rawTolerations) == 0 {
		return nil
	}
	out := make([]corev1.Toleration, 0, len(rawTolerations))
	for _, rt := range rawTolerations {
		tm := rawMap(rt)
		out = append(out, corev1.Toleration{
			Key:      stringField(tm, "key"),
			Operator: corev1.TolerationOperator(stringField(tm, "operator")),
			Value:    stringField(tm, "value"),
			Effect:   corev1.TaintEffect(stringField(tm, "effect")),
		})
	}
	return out
}

func containerRequestsFromRaw(v any) []ContainerRequest {
	rawContainers := rawSlice(v)
	if len(rawContainers) == 0 {
		return nil
	}
	out := make([]ContainerRequest, 0, len(rawContainers))
	for _, rc := range rawContainers {
		cm := rawMap(rc)
		resources := rawMap(cm["resources"])
		out = append(out, ContainerRequest{
			Name:     stringField(cm, "name"),
			Requests: quantityMapFromRaw(resources["requests"]),
			Limits:   quantityMapFromRaw(resources["limits"]),
		})
	}
	return out
}

// quantityMapFromRaw stringifies float64 values — the dynamic client
// decodes a unitless quantity (e.g. `cpu: 2`) as JSON number, not string.
func quantityMapFromRaw(v any) map[string]string {
	src := rawMap(v)
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, val := range src {
		switch t := val.(type) {
		case string:
			out[k] = t
		case float64:
			out[k] = strconv.FormatFloat(t, 'f', -1, 64)
		}
	}
	return out
}
