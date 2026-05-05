package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// effectiveHeadroom returns the headroom multiplier to apply, snapping
// 0 (or negative — defensive) to model.DefaultRightsizingHeadroom so a
// caller that hasn't migrated to the new signature still gets a sane
// recommendation rather than a multiply-by-zero footgun.
func effectiveHeadroom(h float64) float64 {
	if h <= 0 {
		return model.DefaultRightsizingHeadroom
	}
	return h
}

// AvailableRightsizingStrategies returns the strategies usable for the
// given workload + cluster context. Snapshot is always present (the
// metrics-server fallback path that produced any number is the safety
// net). VPA appears iff a VerticalPodAutoscaler with a matching
// targetRef exists in the namespace. Prometheus strategies appear iff
// model.ConfigMonitoring carries a Prometheus endpoint for this
// context (or the "_global" fallback). The returned slice preserves
// the priority order from model.AllRightsizingStrategies so the caller
// can pick the head as the default and walk the slice on </>.
func (c *Client) AvailableRightsizingStrategies(ctx context.Context, contextName, namespace, kind, name string) []model.RightsizingStrategy {
	available := make(map[model.RightsizingStrategy]bool, len(model.AllRightsizingStrategies))
	available[model.StrategySnapshot] = true

	if vpa, err := c.findVPA(ctx, contextName, namespace, kind, name); err == nil && vpa != nil {
		available[model.StrategyVPA] = true
	}

	if hasPrometheusConfigured(contextName) {
		available[model.StrategyPromMax1D] = true
		available[model.StrategyPromAvg1D] = true
		available[model.StrategyPromP957D] = true
	}

	out := make([]model.RightsizingStrategy, 0, len(model.AllRightsizingStrategies))
	for _, s := range model.AllRightsizingStrategies {
		if available[s] {
			out = append(out, s)
		}
	}
	return out
}

// hasPrometheusConfigured reports whether ConfigMonitoring carries a
// usable Prometheus endpoint for the given context (or the "_global"
// fallback). Centralised so the right-sizing strategy picker and the
// Prometheus query path agree on what "configured" means.
func hasPrometheusConfigured(contextName string) bool {
	cfg := model.ConfigMonitoring
	if cfg == nil {
		return false
	}
	mc, ok := cfg[contextName]
	if !ok {
		mc, ok = cfg["_global"]
	}
	if !ok {
		return false
	}
	return mc.Prometheus != nil
}

// GetRightsizing builds a per-container recommendation payload for
// the given workload using the requested strategy + headroom factor.
// Empty strategy defaults to StrategySnapshot for backward
// compatibility; headroom == 0 defaults to
// model.DefaultRightsizingHeadroom so callers that haven't migrated
// to the new signature still get a sane recommendation.
//
// The pipeline is:
//
//  1. Validate the kind is in the supported set (Pod/Deployment/
//     StatefulSet/DaemonSet/Job/CronJob).
//  2. Read the pod spec to seed Current{Request,Limit} so the
//     renderer can compute deltas.
//  3. Always read live metrics-server usage and write it to the
//     Usage column — so even VPA / Prometheus rows show the user
//     what the container is currently consuming.
//  4. Layer the per-strategy SUGGESTION column on top, multiplying
//     the source value by the effective headroom.
//
// Returns an error for kinds outside the in-scope set so the
// action menu's gate isn't bypassed by malformed callers.
func (c *Client) GetRightsizing(ctx context.Context, contextName, namespace, kind, name string, strategy model.RightsizingStrategy, headroom float64) (*model.Rightsizing, error) {
	switch kind {
	case "Pod", "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob":
		// supported
	default:
		return nil, fmt.Errorf("rightsizing not supported for kind %q", kind)
	}
	if strategy == "" {
		strategy = model.StrategySnapshot
	}
	headroom = effectiveHeadroom(headroom)

	containers, err := c.podSpecContainers(ctx, contextName, namespace, kind, name)
	if err != nil {
		return nil, err
	}

	pods, err := c.resolvePodsForWorkload(ctx, contextName, namespace, kind, name)
	if err != nil {
		// Pod resolution failure is fatal — without pod names we can
		// neither aggregate metrics nor count for the header strip.
		return nil, err
	}

	out := &model.Rightsizing{
		Strategy:            strategy,
		Source:              strategy.HumanLabel(),
		AvailableStrategies: c.AvailableRightsizingStrategies(ctx, contextName, namespace, kind, name),
		Headroom:            headroom,
		PodCount:            len(pods),
		Containers:          make([]model.ContainerRec, 0, len(containers)),
	}
	for _, sc := range containers {
		rec := model.ContainerRec{Name: sc.Name}
		if v, ok := sc.Resources.Requests[corev1.ResourceCPU]; ok {
			rec.CPU.CurrentRequest = v.String()
		}
		if v, ok := sc.Resources.Limits[corev1.ResourceCPU]; ok {
			rec.CPU.CurrentLimit = v.String()
		}
		if v, ok := sc.Resources.Requests[corev1.ResourceMemory]; ok {
			rec.Mem.CurrentRequest = v.String()
		}
		if v, ok := sc.Resources.Limits[corev1.ResourceMemory]; ok {
			rec.Mem.CurrentLimit = v.String()
		}
		out.Containers = append(out.Containers, rec)
	}

	// Always populate the live USAGE column from metrics-server when
	// reachable, regardless of strategy. The user benefits from seeing
	// "what is this actually using now" even if the SUGGESTION column
	// comes from VPA history or a Prometheus window.
	c.applyLiveUsage(ctx, contextName, namespace, pods, out)

	// Dispatch to the per-strategy SUGGESTION builder. Each strategy
	// fills RecommendedRequest / RecommendedLimit (and bounds for VPA);
	// the Usage column is already populated above.
	switch strategy {
	case model.StrategyVPA:
		c.applyVPAStrategy(ctx, contextName, namespace, kind, name, headroom, out)
	case model.StrategyPromMax1D, model.StrategyPromAvg1D, model.StrategyPromP957D:
		c.applyPrometheusStrategy(ctx, contextName, namespace, pods, strategy, headroom, out)
	case model.StrategySnapshot:
		c.applySnapshotStrategy(ctx, contextName, namespace, pods, headroom, out)
	}

	return out, nil
}

// applyLiveUsage reads metrics-server PodMetrics for the workload's
// pods and writes the per-container peak (max across pods) into the
// Usage column. Plumbs the snapshot window into out.Window when the
// active strategy hasn't already set one. Tolerates a missing
// metrics-server: leaves Usage empty and the renderer dashes the cell.
func (c *Client) applyLiveUsage(ctx context.Context, contextName, namespace string, pods []string, out *model.Rightsizing) {
	if len(pods) == 0 {
		return
	}
	dyn, err := c.dynamicForContext(contextName)
	if err != nil {
		return
	}

	type maxUsage struct {
		cpuMilli int64
		memBytes int64
	}
	agg := make(map[string]*maxUsage)

	for _, gvr := range c.metricsGVR("pods") {
		list, err := dyn.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		wanted := make(map[string]bool, len(pods))
		for _, p := range pods {
			wanted[p] = true
		}
		for i := range list.Items {
			if !wanted[list.Items[i].GetName()] {
				continue
			}
			if out.Window == "" {
				if w, ok, _ := unstructured.NestedString(list.Items[i].Object, "window"); ok && w != "" {
					out.Window = w
				}
			}
			byContainer, err := parsePodMetricsByContainer(&list.Items[i])
			if err != nil {
				continue
			}
			for cname, u := range byContainer {
				cur := agg[cname]
				if cur == nil {
					cur = &maxUsage{}
					agg[cname] = cur
				}
				if u.CPUMilli > cur.cpuMilli {
					cur.cpuMilli = u.CPUMilli
				}
				if u.MemBytes > cur.memBytes {
					cur.memBytes = u.MemBytes
				}
			}
		}
		break
	}

	for i := range out.Containers {
		cr := &out.Containers[i]
		u, ok := agg[cr.Name]
		if !ok {
			continue
		}
		if u.cpuMilli > 0 {
			cr.CPU.Usage = SnapCPUMilliToCanonical(u.cpuMilli)
		}
		if u.memBytes > 0 {
			cr.Mem.Usage = SnapMemBytesToCanonical(u.memBytes)
		}
	}
}

// applyVPAStrategy looks up a matching VPA and layers its
// recommendations (target × headroom + lower/upper bounds) onto out.
// Missing CRD or no-match leaves the SUGGESTION column empty — the
// user picked VPA explicitly, so we don't silently fall through to a
// different algorithm. Other errors are logged but leave out
// unchanged.
//
// At headroom == 1.0 the SUGGESTION matches the raw VPA target;
// higher values pad the recommendation above what the recommender
// proposes (useful when the user wants extra safety margin on top of
// VPA's own historical analysis).
func (c *Client) applyVPAStrategy(ctx context.Context, contextName, namespace, kind, name string, headroom float64, out *model.Rightsizing) {
	vpa, err := c.findVPA(ctx, contextName, namespace, kind, name)
	if err != nil {
		logger.Info("rightsizing: VPA lookup failed",
			"context", contextName, "namespace", namespace, "kind", kind, "name", name, "err", err)
		return
	}
	if vpa == nil {
		return
	}
	applyVPARecommendations(out, vpa, headroom)
}

// applySnapshotStrategy is the metrics-server "current usage ×
// headroom" path. Reuses applyMetricsRecommendations which preserves
// the spec's request:limit ratio when scaling the limit.
func (c *Client) applySnapshotStrategy(ctx context.Context, contextName, namespace string, pods []string, headroom float64, out *model.Rightsizing) {
	c.applyMetricsRecommendations(ctx, contextName, namespace, pods, headroom, out)
}

// applyPrometheusStrategy is implemented in client_rightsizing_prom.go
// (a separate file to keep the strategy dispatch surface here under
// the file-length cap). It runs a per-container PromQL query for the
// requested window/aggregation and writes the result into the
// SUGGESTION column.

// podSpecContainers returns the container list from the workload's
// pod spec template. CronJob is two levels deep (.spec.jobTemplate
// .spec.template.spec.containers); Pod has it directly on .spec.
//
// Init containers are intentionally excluded — their resource needs
// are different from steady-state recommendations (out of v1 scope).
//
//nolint:unparam // contextName is the right-sizing API contract — real callers pass the active context; tests use the fake's "test-ctx".
func (c *Client) podSpecContainers(ctx context.Context, contextName, namespace, kind, name string) ([]corev1.Container, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "Pod":
		p, err := cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting pod %s/%s: %w", namespace, name, err)
		}
		return p.Spec.Containers, nil
	case "Deployment":
		d, err := cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting deployment %s/%s: %w", namespace, name, err)
		}
		return d.Spec.Template.Spec.Containers, nil
	case "StatefulSet":
		s, err := cs.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting statefulset %s/%s: %w", namespace, name, err)
		}
		return s.Spec.Template.Spec.Containers, nil
	case "DaemonSet":
		d, err := cs.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting daemonset %s/%s: %w", namespace, name, err)
		}
		return d.Spec.Template.Spec.Containers, nil
	case "Job":
		j, err := cs.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting job %s/%s: %w", namespace, name, err)
		}
		return j.Spec.Template.Spec.Containers, nil
	case "CronJob":
		cj, err := cs.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting cronjob %s/%s: %w", namespace, name, err)
		}
		return cj.Spec.JobTemplate.Spec.Template.Spec.Containers, nil
	}
	return nil, fmt.Errorf("podSpecContainers: unsupported kind %q", kind)
}

func vpaGVRs() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"},
		{Group: "autoscaling.k8s.io", Version: "v1beta2", Resource: "verticalpodautoscalers"},
	}
}

// findVPA returns the first VPA in the namespace whose targetRef
// matches (kind, name). Returns (nil, nil) when no VPA matches OR
// the CRD is not installed (NoMatchError / NotFound) — caller treats
// both as "fall through to metrics path." Other errors (RBAC denials,
// network timeouts, server errors) are propagated so the caller can
// surface or log them rather than silently swallowing them.
func (c *Client) findVPA(ctx context.Context, contextName, namespace, kind, name string) (*unstructured.Unstructured, error) {
	dyn, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}
	for _, gvr := range vpaGVRs() {
		list, err := dyn.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			// CRD or version not installed — try the next GVR.
			if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
				continue
			}
			// Real error (RBAC, network, server) — propagate so the
			// caller can surface or log it. Not silently swallowed.
			return nil, fmt.Errorf("listing VPAs in %s: %w", namespace, err)
		}
		for i := range list.Items {
			ref, found, err := unstructured.NestedMap(list.Items[i].Object, "spec", "targetRef")
			if err != nil || !found {
				continue
			}
			refKind, _ := ref["kind"].(string)
			refName, _ := ref["name"].(string)
			if refKind == kind && refName == name {
				return &list.Items[i], nil
			}
		}
	}
	return nil, nil
}

// applyMetricsRecommendations is the snapshot-strategy SUGGESTION
// builder. Aggregates per-container max usage across the workload's
// pods, applies the headroom factor, snaps to canonical k8s units,
// and preserves the spec's request:limit ratio for RecommendedLimit.
// Skips containers that already have a recommendation from a higher-
// priority path (preserves backward compat with the legacy "VPA-then-
// fallback" behavior).
func (c *Client) applyMetricsRecommendations(ctx context.Context, contextName, namespace string, pods []string, headroom float64, out *model.Rightsizing) {
	if len(pods) == 0 {
		return
	}
	dyn, err := c.dynamicForContext(contextName)
	if err != nil {
		return
	}

	type maxUsage struct {
		cpuMilli int64
		memBytes int64
	}
	agg := make(map[string]*maxUsage)

	for _, gvr := range c.metricsGVR("pods") {
		list, err := dyn.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		wanted := make(map[string]bool, len(pods))
		for _, p := range pods {
			wanted[p] = true
		}
		for i := range list.Items {
			if !wanted[list.Items[i].GetName()] {
				continue
			}
			byContainer, err := parsePodMetricsByContainer(&list.Items[i])
			if err != nil {
				continue
			}
			for cname, u := range byContainer {
				cur := agg[cname]
				if cur == nil {
					cur = &maxUsage{}
					agg[cname] = cur
				}
				if u.CPUMilli > cur.cpuMilli {
					cur.cpuMilli = u.CPUMilli
				}
				if u.MemBytes > cur.memBytes {
					cur.memBytes = u.MemBytes
				}
			}
		}
		break // Successful list — don't try the next GVR
	}

	for i := range out.Containers {
		cr := &out.Containers[i]
		u, ok := agg[cr.Name]
		if !ok {
			continue
		}
		if cr.CPU.RecommendedRequest == "" && u.cpuMilli > 0 {
			rec := SnapCPUMilliToCanonical(int64(float64(u.cpuMilli) * headroom))
			cr.CPU.RecommendedRequest = rec
			cr.CPU.RecommendedLimit = scaleLimitFromRatio(cr.CPU.CurrentRequest, cr.CPU.CurrentLimit, rec)
		}
		if cr.Mem.RecommendedRequest == "" && u.memBytes > 0 {
			rec := SnapMemBytesToCanonical(int64(float64(u.memBytes) * headroom))
			cr.Mem.RecommendedRequest = rec
			cr.Mem.RecommendedLimit = scaleLimitFromRatio(cr.Mem.CurrentRequest, cr.Mem.CurrentLimit, rec)
		}
	}
}

// applyVPARecommendations layers VPA target × headroom +
// lowerBound / upperBound onto the already-seeded result. VPA returns
// ONE number per resource (no separate limit), so RecommendedLimit
// comes from the spec's request:limit ratio. The Source/Strategy
// fields are owned by the caller (set from the requested strategy
// upfront), not flipped here.
//
// The lower/upper bounds are NOT scaled by headroom — they are the
// recommender's safety boundaries (anything outside should not run
// long-term) and are only meaningful as VPA's own raw signal.
func applyVPARecommendations(out *model.Rightsizing, vpa *unstructured.Unstructured, headroom float64) {
	recs, found, err := unstructured.NestedSlice(vpa.Object, "status", "recommendation", "containerRecommendations")
	if err != nil || !found {
		return
	}
	byName := make(map[string]map[string]any, len(recs))
	for _, r := range recs {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		name, _ := rm["containerName"].(string)
		if name == "" {
			continue
		}
		byName[name] = rm
	}
	for i := range out.Containers {
		rm, ok := byName[out.Containers[i].Name]
		if !ok {
			continue
		}
		layerVPAResource(&out.Containers[i].CPU, rm, "cpu", headroom)
		layerVPAResource(&out.Containers[i].Mem, rm, "memory", headroom)
	}
}

// layerVPAResource updates a single ResourceRec from a VPA rec map's
// target × headroom + lowerBound / upperBound for the given resource
// key ("cpu" or "memory"). RecommendedLimit is derived from the spec
// ratio applied to the post-headroom request.
//
// The bounds are written through verbatim — they describe VPA's own
// confidence interval, not a recommendation the user should pad.
func layerVPAResource(rec *model.ResourceRec, rm map[string]any, resKey string, headroom float64) {
	if target, ok := rm["target"].(map[string]any); ok {
		if v, ok := target[resKey]; ok {
			rec.RecommendedRequest = scaleQuantityByHeadroom(fmt.Sprintf("%v", v), headroom)
		}
	}
	if lb, ok := rm["lowerBound"].(map[string]any); ok {
		if v, ok := lb[resKey]; ok {
			rec.LowerBound = fmt.Sprintf("%v", v)
		}
	}
	if ub, ok := rm["upperBound"].(map[string]any); ok {
		if v, ok := ub[resKey]; ok {
			rec.UpperBound = fmt.Sprintf("%v", v)
		}
	}
	rec.RecommendedLimit = scaleLimitFromRatio(rec.CurrentRequest, rec.CurrentLimit, rec.RecommendedRequest)
}

// scaleQuantityByHeadroom multiplies a k8s canonical quantity string
// (e.g. "60m", "200Mi") by `headroom` and snaps back to canonical
// form. Returns the input unchanged when headroom == 1 (avoids
// pointless re-snapping that could shift "60m" to "60m" via parser
// round-trip — same answer, but unnecessary work) or when the string
// fails to parse (defensive — the VPA payload always parses, but
// silent passthrough beats panicking on a future schema change).
func scaleQuantityByHeadroom(q string, headroom float64) string {
	if q == "" || headroom == 1 {
		return q
	}
	parsed, err := resource.ParseQuantity(q)
	if err != nil {
		return q
	}
	if isMemoryQuantity(q) {
		// MilliValue() for memory returns bytes×1000; convert back to
		// bytes before scaling so SnapMemBytesToCanonical sees the right
		// unit.
		bytes := parsed.MilliValue() / 1000
		return SnapMemBytesToCanonical(int64(float64(bytes) * headroom))
	}
	return SnapCPUMilliToCanonical(int64(float64(parsed.MilliValue()) * headroom))
}

// scaleLimitFromRatio returns the recommended limit that preserves
// the spec's request:limit ratio. Empty string when any input is
// empty/zero or unparseable — callers treat that as "don't recommend
// a limit; user didn't have one."
func scaleLimitFromRatio(currentRequest, currentLimit, recommendedRequest string) string {
	if currentRequest == "" || currentLimit == "" || recommendedRequest == "" {
		return ""
	}
	cr, err := resource.ParseQuantity(currentRequest)
	if err != nil || cr.MilliValue() == 0 {
		return ""
	}
	cl, err := resource.ParseQuantity(currentLimit)
	if err != nil {
		return ""
	}
	rr, err := resource.ParseQuantity(recommendedRequest)
	if err != nil {
		return ""
	}
	ratio := float64(cl.MilliValue()) / float64(cr.MilliValue())
	scaled := int64(float64(rr.MilliValue()) * ratio)
	if isMemoryQuantity(currentRequest) {
		// MilliValue for memory returns bytes×1000; convert back to bytes for the snap.
		return SnapMemBytesToCanonical(scaled / 1000)
	}
	return SnapCPUMilliToCanonical(scaled)
}

// isMemoryQuantity returns true when s carries a memory unit suffix
// (Ki/Mi/Gi/Ti/Pi/Ei or K/M/G/T/P/E). CPU values use bare
// "Nm"/"N"/"N.M" forms with no unit suffix, so this is a reliable
// disambiguation in lfk's right-sizing context (callers always pass
// canonical k8s strings from a real spec).
func isMemoryQuantity(s string) bool {
	for _, sfx := range []string{"Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "K", "M", "G", "T", "P", "E"} {
		if len(s) >= len(sfx) && s[len(s)-len(sfx):] == sfx {
			return true
		}
	}
	return false
}

// SnapCPUMilliToCanonical / SnapMemBytesToCanonical duplicate the
// snapping logic from internal/ui/quantity_math.go. The k8s package
// can't import internal/ui (would invert the architecture's data ->
// presentation direction), so the helpers live in both places. Keep
// in sync; they're only ~15 lines each.
func SnapCPUMilliToCanonical(milli int64) string {
	if milli <= 0 {
		return "0"
	}
	snapped := ((milli + 9) / 10) * 10
	if snapped >= 1000 && snapped%1000 == 0 {
		return fmt.Sprintf("%d", snapped/1000)
	}
	return fmt.Sprintf("%dm", snapped)
}

func SnapMemBytesToCanonical(bytes int64) string {
	if bytes <= 0 {
		return "0"
	}
	const mi = 1024 * 1024
	mibs := (bytes + mi - 1) / mi
	return fmt.Sprintf("%dMi", mibs)
}
