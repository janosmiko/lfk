package k8s

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/logger"
)

// unknownWave is the sentinel used for resources whose sync-wave is genuinely
// unknown — Missing resources we never fetch, per-resource fetch errors, and
// unparseable annotations. Rendered as "?" in the overlay. A missing annotation
// is NOT unknownWave: it lands at 0, matching ArgoCD's default sync-wave.
const unknownWave = math.MinInt

// SyncWaveTimeline is the immutable result of one fetch against an ArgoCD
// Application. The renderer reads it; nothing in the data layer mutates it
// after Build returns.
type SyncWaveTimeline struct {
	AppName       string
	AppNamespace  string
	Phases        []SyncWavePhase
	LastOperation *SyncOperationSummary
	LivePhase     string // operationState.phase: "" | "Running" | "Succeeded" | "Failed" | "Error" | "Terminating"
	Revision      string // last operation revision (short)
	FetchedAt     time.Time
	// Loading is true between the fast skeleton fetch and the slow wave-
	// annotation fan-out. The renderer surfaces this as "Loading wave map…"
	// in the header and the app layer chains a full fetch when Loading is
	// true. Always false on a result from GetSyncWaveTimeline (full path).
	Loading bool
}

// SyncWavePhase groups buckets of resources under a single ArgoCD phase
// name. All standard ArgoCD phases are always emitted in fixed order, even
// when empty — the renderer annotates empty phases with "(none in last
// operation)" so the operator can see the full pipeline at a glance.
type SyncWavePhase struct {
	Name  string // "PreSync" | "Sync" | "PostSync" | "SyncFail" | "PostSyncFail" | "PreDelete" | "PostDelete"
	Waves []SyncWaveBucket
}

// SyncWaveBucket groups resources that share the same wave number within
// a phase. Wave == unknownWave renders as "wave ?".
type SyncWaveBucket struct {
	Wave      int
	Resources []SyncWaveResource
}

// SyncWaveResource is a single managed resource or hook under a wave.
type SyncWaveResource struct {
	Group, Version, Kind, Namespace, Name string
	SyncStatus                            string // "Synced" | "OutOfSync" | "Missing" | "Unknown"
	HealthStatus                          string // "Healthy" | "Progressing" | "Degraded" | "Suspended" | "Missing" | ""
	HookPhase                             string // hooks only: "Running" | "Succeeded" | "Failed" | "Error" | "Terminating"
	OpStatus                              string // last operation: "Synced" | "Running" | "Failed" | "" — from operationState.syncResult.resources[]
	Message                               string // operationState.syncResult.resources[].message — surfaced on Failed/Error
	IsHook                                bool
}

// SyncOperationSummary is the header summary of the last completed (or
// currently-running) sync operation.
type SyncOperationSummary struct {
	Phase      string // Succeeded | Failed | Error | Running | Terminating
	Message    string
	StartedAt  time.Time
	FinishedAt time.Time // zero when Running
	Revision   string
}

// kindPluralOverrides handles kinds whose plural is not formed by the
// default rules below (lowercase + simple suffix handling). Keep this
// list short — covers only kinds that appear in typical ArgoCD apps.
var kindPluralOverrides = map[string]string{
	"Endpoints":      "endpoints",
	"NetworkPolicy":  "networkpolicies",
	"PodSecurity":    "podsecurities",
	"Ingress":        "ingresses",
	"PriorityClass":  "priorityclasses",
	"StorageClass":   "storageclasses",
	"VolumeSnapshot": "volumesnapshots",
}

// resolveSyncWaveGVR maps (group, version, kind) → GroupVersionResource
// using a small override table for irregular plurals plus a default
// pluralizer (...y → ...ies, ...s → ...ses, default → ...s). Empty
// version defaults to "v1" so legacy `status.resources` entries that
// only carry a Kind still resolve.
func resolveSyncWaveGVR(group, version, kind string) schema.GroupVersionResource {
	resource := pluralFor(kind)
	if version == "" {
		version = "v1"
	}
	return schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
}

func pluralFor(kind string) string {
	if p, ok := kindPluralOverrides[kind]; ok {
		return p
	}
	low := strings.ToLower(kind)
	switch {
	case strings.HasSuffix(low, "y"):
		return strings.TrimSuffix(low, "y") + "ies"
	case strings.HasSuffix(low, "s"):
		return low + "es"
	default:
		return low + "s"
	}
}

// waveResourceWithWave pairs a SyncWaveResource with the wave it belongs
// to. Used internally by the builder before bucketization; never exported.
type waveResourceWithWave struct {
	wave int
	res  SyncWaveResource
}

// phaseRenderOrder is the fixed render order for every standard ArgoCD
// hook phase. Phases not in this slice are dropped (defends against typo'd
// `syncPhase` values from upstream data). Every phase in this slice is
// always emitted, even with no resources — the renderer annotates the
// empty ones so operators can see the full pipeline.
var phaseRenderOrder = []string{"PreSync", "Sync", "PostSync", "SyncFail", "PostSyncFail", "PreDelete", "PostDelete"}

// groupSyncWaveResources turns the flat per-phase input into the bucketized
// SyncWavePhase output: ascending wave numbers (unknownWave last), and
// resources within a bucket sorted by (Group, Kind, Namespace, Name).
// All standard phases in phaseRenderOrder are emitted in fixed order,
// even when empty — the renderer annotates the empty ones.
func groupSyncWaveResources(byPhase map[string][]waveResourceWithWave) []SyncWavePhase {
	out := make([]SyncWavePhase, 0, len(phaseRenderOrder))
	for _, phaseName := range phaseRenderOrder {
		entries := byPhase[phaseName]
		bucketsByWave := map[int][]SyncWaveResource{}
		for _, e := range entries {
			bucketsByWave[e.wave] = append(bucketsByWave[e.wave], e.res)
		}
		waves := make([]int, 0, len(bucketsByWave))
		for w := range bucketsByWave {
			waves = append(waves, w)
		}
		sort.Slice(waves, func(i, j int) bool {
			a, b := waves[i], waves[j]
			// unknownWave (math.MinInt) sorts last, not first.
			switch {
			case a == unknownWave && b != unknownWave:
				return false
			case b == unknownWave && a != unknownWave:
				return true
			default:
				return a < b
			}
		})
		buckets := make([]SyncWaveBucket, 0, len(waves))
		for _, w := range waves {
			rs := bucketsByWave[w]
			sort.Slice(rs, func(i, j int) bool {
				return resourceSortKey(rs[i]) < resourceSortKey(rs[j])
			})
			buckets = append(buckets, SyncWaveBucket{Wave: w, Resources: rs})
		}
		out = append(out, SyncWavePhase{Name: phaseName, Waves: buckets})
	}
	return out
}

// resourceSortKey produces a stable comparison key for sorting resources
// inside a bucket. Newline keeps the segments unambiguous.
func resourceSortKey(r SyncWaveResource) string {
	return r.Group + "\n" + r.Kind + "\n" + r.Namespace + "\n" + r.Name
}

// parseHookResources reads operationState.syncResult.resources[] and
// returns hook entries grouped by syncPhase. Non-hook entries (no
// hookType) are skipped. Wave is unknownWave for hooks since they aren't
// wave-bucketed; the renderer ignores wave for hook rows.
func parseHookResources(syncResult map[string]any) map[string][]waveResourceWithWave {
	if syncResult == nil {
		return nil
	}
	resources, ok := syncResult["resources"].([]any)
	if !ok {
		return nil
	}
	out := map[string][]waveResourceWithWave{}
	for _, r := range resources {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		hookType, _ := rm["hookType"].(string)
		if hookType == "" {
			continue
		}
		phase, _ := rm["syncPhase"].(string)
		if phase == "" {
			phase = hookType
		}
		entry := SyncWaveResource{IsHook: true}
		entry.Group, _ = rm["group"].(string)
		entry.Version, _ = rm["version"].(string)
		entry.Kind, _ = rm["kind"].(string)
		entry.Namespace, _ = rm["namespace"].(string)
		entry.Name, _ = rm["name"].(string)
		entry.OpStatus, _ = rm["status"].(string)
		entry.Message, _ = rm["message"].(string)
		entry.HookPhase, _ = rm["hookPhase"].(string)
		out[phase] = append(out[phase], waveResourceWithWave{wave: unknownWave, res: entry})
	}
	return out
}

// overlayOpStatus walks managed resources and, for each one whose
// (group, kind, namespace, name) matches a non-hook entry in
// syncResult.resources[], sets OpStatus and Message. Returns a new slice;
// the input slice is not mutated.
func overlayOpStatus(managed []SyncWaveResource, syncResult map[string]any) []SyncWaveResource {
	out := append([]SyncWaveResource(nil), managed...)
	if syncResult == nil {
		return out
	}
	resources, ok := syncResult["resources"].([]any)
	if !ok {
		return out
	}
	type opKey struct{ group, kind, ns, name string }
	index := map[opKey]map[string]any{}
	for _, r := range resources {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if hookType, _ := rm["hookType"].(string); hookType != "" {
			continue
		}
		k := opKey{}
		k.group, _ = rm["group"].(string)
		k.kind, _ = rm["kind"].(string)
		k.ns, _ = rm["namespace"].(string)
		k.name, _ = rm["name"].(string)
		index[k] = rm
	}
	for i := range out {
		k := opKey{group: out[i].Group, kind: out[i].Kind, ns: out[i].Namespace, name: out[i].Name}
		if rm, ok := index[k]; ok {
			out[i].OpStatus, _ = rm["status"].(string)
			out[i].Message, _ = rm["message"].(string)
		}
	}
	return out
}

// parseManagedResources extracts SyncWaveResource entries from
// status.resources[]. Entries that aren't maps are skipped silently —
// upstream data is permissive so we are too.
func parseManagedResources(resources []any) []SyncWaveResource {
	out := make([]SyncWaveResource, 0, len(resources))
	for _, r := range resources {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		entry := SyncWaveResource{IsHook: false}
		entry.Group, _ = rm["group"].(string)
		entry.Version, _ = rm["version"].(string)
		entry.Kind, _ = rm["kind"].(string)
		entry.Namespace, _ = rm["namespace"].(string)
		entry.Name, _ = rm["name"].(string)
		entry.SyncStatus, _ = rm["status"].(string)
		if hm, ok := rm["health"].(map[string]any); ok {
			entry.HealthStatus, _ = hm["status"].(string)
		}
		out = append(out, entry)
	}
	return out
}

// parseOperationSummary reads status.operationState. Returns nil for nil
// input. Revision is truncated to 7 chars for header display.
func parseOperationSummary(op map[string]any) *SyncOperationSummary {
	if op == nil {
		return nil
	}
	out := &SyncOperationSummary{}
	out.Phase, _ = op["phase"].(string)
	out.Message, _ = op["message"].(string)
	if s, ok := op["startedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			out.StartedAt = t
		}
	}
	if s, ok := op["finishedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			out.FinishedAt = t
		}
	}
	if syncResult, ok := op["syncResult"].(map[string]any); ok {
		if rev, ok := syncResult["revision"].(string); ok {
			if len(rev) > 7 {
				rev = rev[:7]
			}
			out.Revision = rev
		}
	}
	return out
}

// fetchWaveConcurrency caps the number of in-flight GETs when reading
// per-resource sync-wave annotations. Eight is enough to be useful on a
// large app without overwhelming the apiserver from a single user session.
const fetchWaveConcurrency = 8

// fetchWaveAnnotations fans out per-resource GETs to read each managed
// resource's argocd.argoproj.io/sync-wave annotation. Missing-status
// resources are skipped (we know they aren't in the cluster) and stay at
// unknownWave. A missing annotation lands at 0 (ArgoCD's default).
// Per-resource errors are logged and the resource lands at unknownWave;
// only context cancellation propagates.
func (c *Client) fetchWaveAnnotations(ctx context.Context, contextName string, in []SyncWaveResource) ([]waveResourceWithWave, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	out := make([]waveResourceWithWave, len(in))
	for i, r := range in {
		out[i] = waveResourceWithWave{wave: unknownWave, res: r}
	}

	sem := make(chan struct{}, fetchWaveConcurrency)
	var wg sync.WaitGroup
	for i, r := range in {
		if r.SyncStatus == "Missing" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, r SyncWaveResource) {
			defer wg.Done()
			defer func() { <-sem }()
			gvr := resolveSyncWaveGVR(r.Group, r.Version, r.Kind)
			obj, getErr := dynClient.Resource(gvr).Namespace(r.Namespace).Get(ctx, r.Name, metav1.GetOptions{})
			if getErr != nil {
				logger.Debug("sync-wave fetch error", "kind", r.Kind, "name", r.Name, "error", getErr)
				return
			}
			ann, _, _ := unstructured.NestedStringMap(obj.Object, "metadata", "annotations")
			raw, ok := ann["argocd.argoproj.io/sync-wave"]
			if !ok {
				// ArgoCD treats a missing annotation as wave 0 (default).
				out[i].wave = 0
				return
			}
			w, err := strconv.Atoi(raw)
			if err != nil {
				// Unparseable annotation: keep pre-seeded unknownWave.
				return
			}
			out[i].wave = w
		}(i, r)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	return out, nil
}

// fetchSyncWaveApplication issues the Application GET that both the
// skeleton and full pipelines share. Returned as *unstructured for the
// helpers below.
func (c *Client) fetchSyncWaveApplication(ctx context.Context, contextName, namespace, appName string) (*unstructured.Unstructured, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}
	appGVR := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	app, err := dynClient.Resource(appGVR).Namespace(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting application %s: %w", appName, err)
	}
	return app, nil
}

// buildSyncWaveTimelineFromApp assembles a SyncWaveTimeline from an
// already-fetched Application. When wavesByIndex is nil (skeleton path),
// every managed resource lands at unknownWave and Loading is set to true.
// When wavesByIndex is non-nil, len(wavesByIndex) MUST equal the number
// of parsed managed resources — those wave numbers are applied 1:1 and
// Loading is false.
func buildSyncWaveTimelineFromApp(app *unstructured.Unstructured, appName, namespace string, wavesByIndex []int) *SyncWaveTimeline {
	tl := &SyncWaveTimeline{
		AppName:      appName,
		AppNamespace: namespace,
		FetchedAt:    time.Now(),
		Loading:      wavesByIndex == nil,
	}
	statusMap, _ := app.Object["status"].(map[string]any)
	if statusMap == nil {
		// Always emit the seven standard phases, even when the
		// Application has no status yet — the Sync Wave overlay's
		// pinned sidebar walks tl.Phases to render the phase pipeline,
		// and a nil slice would leave the user staring at an empty
		// frame. The phases are empty (no waves), which the renderer
		// surfaces as "(none in last operation)".
		tl.Phases = groupSyncWaveResources(map[string][]waveResourceWithWave{})
		return tl
	}

	opState, _ := statusMap["operationState"].(map[string]any)
	tl.LastOperation = parseOperationSummary(opState)
	if tl.LastOperation != nil {
		tl.LivePhase = tl.LastOperation.Phase
		tl.Revision = tl.LastOperation.Revision
	}

	statusResources, _ := statusMap["resources"].([]any)
	managed := parseManagedResources(statusResources)

	var syncResult map[string]any
	if opState != nil {
		syncResult, _ = opState["syncResult"].(map[string]any)
	}
	managed = overlayOpStatus(managed, syncResult)

	// Build the (resource, wave) pairs. Skeleton path: all unknownWave.
	// Full path: caller-supplied wave numbers indexed by managed[].
	withWaves := make([]waveResourceWithWave, len(managed))
	for i, r := range managed {
		w := unknownWave
		if wavesByIndex != nil && i < len(wavesByIndex) {
			w = wavesByIndex[i]
		}
		withWaves[i] = waveResourceWithWave{wave: w, res: r}
	}

	byPhase := map[string][]waveResourceWithWave{}
	byPhase["Sync"] = append(byPhase["Sync"], withWaves...)
	for phase, hooks := range parseHookResources(syncResult) {
		byPhase[phase] = append(byPhase[phase], hooks...)
	}
	tl.Phases = groupSyncWaveResources(byPhase)
	return tl
}

// GetSyncWaveTimelineSkeleton is the fast Phase 1 entry: Application GET +
// parse only. All managed resources land at unknownWave so the overlay
// can render the phase pipeline immediately while the slow per-resource
// annotation fan-out runs in the background. The returned timeline has
// Loading set to true; the renderer surfaces this as "Loading wave map…"
// in the header.
func (c *Client) GetSyncWaveTimelineSkeleton(ctx context.Context, contextName, namespace, appName string) (*SyncWaveTimeline, error) {
	app, err := c.fetchSyncWaveApplication(ctx, contextName, namespace, appName)
	if err != nil {
		return nil, err
	}
	return buildSyncWaveTimelineFromApp(app, appName, namespace, nil), nil
}

// GetSyncWaveTimeline reads an ArgoCD Application and assembles a
// SyncWaveTimeline ready for rendering. Per-resource annotation fetches
// run in parallel under fetchWaveConcurrency. Per-resource fetch errors
// are non-fatal — affected resources land in the unknownWave bucket.
// The Application GET itself is fatal (NotFound, etc.). Loading is
// always false on the result.
func (c *Client) GetSyncWaveTimeline(ctx context.Context, contextName, namespace, appName string) (*SyncWaveTimeline, error) {
	app, err := c.fetchSyncWaveApplication(ctx, contextName, namespace, appName)
	if err != nil {
		return nil, err
	}

	// Reparse managed resources here to drive the wave fetch — we need
	// them in the same order the helper will see them so wavesByIndex
	// aligns 1:1.
	statusMap, _ := app.Object["status"].(map[string]any)
	var managed []SyncWaveResource
	if statusMap != nil {
		statusResources, _ := statusMap["resources"].([]any)
		managed = parseManagedResources(statusResources)
	}

	withWaves, fetchErr := c.fetchWaveAnnotations(ctx, contextName, managed)
	if fetchErr != nil {
		return nil, fmt.Errorf("fetching wave annotations: %w", fetchErr)
	}
	wavesByIndex := make([]int, len(withWaves))
	for i, w := range withWaves {
		wavesByIndex[i] = w.wave
	}

	return buildSyncWaveTimelineFromApp(app, appName, namespace, wavesByIndex), nil
}
