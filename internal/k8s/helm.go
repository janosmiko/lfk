package k8s

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"

	"sigs.k8s.io/yaml"
)

// --- Helm helpers ---

// GetHelmReleases lists Helm releases by finding secrets with owner=helm label.
func (c *Client) GetHelmReleases(ctx context.Context, contextName, namespace string) ([]model.Item, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	listOpts := metav1.ListOptions{
		LabelSelector: "owner=helm",
	}

	type secretInfo struct {
		name      string
		namespace string
		labels    map[string]string
		data      map[string][]byte
		created   time.Time
	}

	var secrets []secretInfo

	if namespace == "" {
		list, listErr := cs.CoreV1().Secrets("").List(ctx, listOpts)
		if listErr != nil {
			return nil, fmt.Errorf("listing helm secrets: %w", listErr)
		}
		for _, s := range list.Items {
			secrets = append(secrets, secretInfo{
				name:      s.Name,
				namespace: s.Namespace,
				labels:    s.Labels,
				data:      s.Data,
				created:   s.CreationTimestamp.Time,
			})
		}
	} else {
		list, listErr := cs.CoreV1().Secrets(namespace).List(ctx, listOpts)
		if listErr != nil {
			return nil, fmt.Errorf("listing helm secrets: %w", listErr)
		}
		for _, s := range list.Items {
			secrets = append(secrets, secretInfo{
				name:      s.Name,
				namespace: s.Namespace,
				labels:    s.Labels,
				data:      s.Data,
				created:   s.CreationTimestamp.Time,
			})
		}
	}

	// Group by release name, keep only latest version.
	type releaseInfo struct {
		name      string
		namespace string
		status    string
		version   string
		data      map[string][]byte
		created   time.Time
	}

	releases := make(map[string]*releaseInfo)
	for _, s := range secrets {
		relName := s.labels["name"]
		relStatus := s.labels["status"]
		relVersion := s.labels["version"]
		if relName == "" {
			continue
		}

		key := s.namespace + "/" + relName
		existing, ok := releases[key]
		if !ok || s.created.After(existing.created) {
			releases[key] = &releaseInfo{
				name:      relName,
				namespace: s.namespace,
				status:    relStatus,
				version:   relVersion,
				data:      s.data,
				created:   s.created,
			}
		}
	}

	items := make([]model.Item, 0, len(releases))
	for _, rel := range releases {
		status := rel.status
		// Capitalize first letter for display.
		if len(status) > 0 {
			status = strings.ToUpper(status[:1]) + status[1:]
		}

		ti := model.Item{
			Name:      rel.name,
			Kind:      "HelmRelease",
			Status:    status,
			Age:       formatAge(time.Since(rel.created)),
			CreatedAt: rel.created,
			Extra:     "v" + rel.version,
		}
		if namespace == "" {
			ti.Namespace = rel.namespace
		}
		ti.Columns = buildHelmReleaseColumns(rel.data, rel.name, status)
		items = append(items, ti)
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// buildHelmReleaseColumns builds the detail columns for a helm release list item.
// It tries to decode the gzipped release blob stored in the secret data; on
// failure it logs a warning and returns an empty slice so the list still renders.
func buildHelmReleaseColumns(data map[string][]byte, relName, displayStatus string) []model.KeyValue {
	raw, ok := data["release"]
	if !ok || len(raw) == 0 {
		return nil
	}
	info, err := decodeHelmReleaseSecret(raw)
	if err != nil {
		logger.Warn("Helm: failed to decode release blob; using label-only data", "release", relName, "error", err)
		return nil
	}
	cols := []model.KeyValue{
		{Key: "Chart", Value: info.ChartName},
		{Key: "Chart Version", Value: info.ChartVersion},
		{Key: "App Version", Value: info.AppVersion},
		{Key: "Revision", Value: strconv.Itoa(info.Revision)},
		{Key: "Status", Value: displayStatus},
	}
	if !info.LastDeployed.IsZero() {
		cols = append(cols, model.KeyValue{Key: "Last Deployed", Value: formatAge(time.Since(info.LastDeployed))})
	}
	if desc := stripControlChars(info.Description); desc != "" {
		cols = append(cols, model.KeyValue{Key: "Description", Value: desc})
	}
	return cols
}

// stripControlChars removes ASCII control characters (except tab) from a
// string. Used to sanitize free-text fields sourced from Kubernetes secret
// data before rendering them in the TUI, so a release with embedded ANSI
// escapes or other control sequences cannot corrupt the terminal state.
func stripControlChars(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 0x20 || r == '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// GetHelmReleaseYAML returns a summary YAML for a Helm release.
func (c *Client) GetHelmReleaseYAML(ctx context.Context, contextName, namespace, name string) (string, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return "", err
	}

	listOpts := metav1.ListOptions{
		LabelSelector: "owner=helm,name=" + name,
	}
	secretList, err := cs.CoreV1().Secrets(namespace).List(ctx, listOpts)
	if err != nil {
		return "", fmt.Errorf("listing helm secrets: %w", err)
	}

	if len(secretList.Items) == 0 {
		return "", fmt.Errorf("no helm release found for %s", name)
	}

	// Find the latest version.
	latest := secretList.Items[0]
	for _, s := range secretList.Items[1:] {
		if s.CreationTimestamp.After(latest.CreationTimestamp.Time) {
			latest = s
		}
	}

	// Build a summary (not the compressed data).
	info := map[string]any{
		"name":      latest.Labels["name"],
		"namespace": latest.Namespace,
		"version":   latest.Labels["version"],
		"status":    latest.Labels["status"],
		"created":   latest.CreationTimestamp.Format(time.RFC3339),
		"modified":  latest.Labels["modifiedAt"],
		"secret":    latest.Name,
	}

	data, err := yaml.Marshal(info)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *Client) getHelmManagedResources(ctx context.Context, contextName, namespace, releaseName string) ([]model.Item, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	// Primary discovery path: parse the rendered manifest stored inside the
	// latest helm release secret. This covers every kind the chart actually
	// installs (including cluster-scoped and custom resources) regardless of
	// whether chart authors set instance labels uniformly.
	if items, ok := c.collectHelmResourcesFromManifest(ctx, cs, namespace, releaseName); ok {
		return items, nil
	}

	// Fallback: label-based discovery for legacy releases whose manifest is
	// missing or undecodable. This preserves backwards compatibility with the
	// pre-manifest path.
	return c.collectHelmResourcesByLabels(ctx, cs, namespace, releaseName), nil
}

// collectHelmResourcesFromManifest reads the latest helm release secret for
// releaseName, decodes it, parses its manifest, and returns one Item per
// resource declared in the manifest. Known workload kinds in the release's
// namespace are enriched with live ready counts by merging with the output of
// the existing collectHelm* helpers. The second return value is false when no
// usable manifest could be loaded, signalling the caller to fall back to
// label-based discovery.
func (c *Client) collectHelmResourcesFromManifest(ctx context.Context, cs kubernetes.Interface, namespace, releaseName string) ([]model.Item, bool) {
	secret, ok := findLatestHelmReleaseSecret(ctx, cs, namespace, releaseName)
	if !ok {
		return nil, false
	}

	raw, ok := secret.Data["release"]
	if !ok || len(raw) == 0 {
		return nil, false
	}
	info, err := decodeHelmReleaseSecret(raw)
	if err != nil {
		logger.Warn("Helm: failed to decode release blob; falling back to label-based discovery", "release", releaseName, "error", err)
		return nil, false
	}
	if info.Manifest == "" {
		return nil, false
	}
	refs, err := parseHelmManifest(info.Manifest)
	if err != nil {
		logger.Warn("Helm: failed to parse release manifest; falling back to label-based discovery", "release", releaseName, "error", err)
		return nil, false
	}
	if len(refs) == 0 {
		return nil, false
	}

	items, mergeIndex := buildItemsFromManifestRefs(refs)

	// Enrich workload kinds with live status (Ready column) by merging with
	// the existing label-based collectors, matching on Kind+Name.
	enrichHelmWorkloadStatus(ctx, cs, namespace, releaseName, items, mergeIndex)

	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Name < items[j].Name
	})
	return items, true
}

// collectHelmResourcesByLabels is the legacy label-based discovery path,
// retained verbatim as a fallback for releases whose manifest field is
// missing, empty, or undecodable.
func (c *Client) collectHelmResourcesByLabels(ctx context.Context, cs kubernetes.Interface, namespace, releaseName string) []model.Item {
	labelSelectors := []string{
		"app.kubernetes.io/instance=" + releaseName,
		"release=" + releaseName,
	}

	seen := make(map[string]bool)
	var items []model.Item

	for _, labelSelector := range labelSelectors {
		logger.Debug("Helm: listing managed resources (label fallback)", "selector", labelSelector, "namespace", namespace)
		opts := metav1.ListOptions{LabelSelector: labelSelector}

		collectHelmDeployments(&items, seen, cs, ctx, namespace, opts)
		collectHelmStatefulSets(&items, seen, cs, ctx, namespace, opts)
		collectHelmDaemonSets(&items, seen, cs, ctx, namespace, opts)
		collectHelmSimpleResources(&items, seen, cs, ctx, namespace, opts)
	}

	if len(items) == 0 {
		logger.Info("Helm: no managed resources found for release", "release", releaseName, "namespace", namespace)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Name < items[j].Name
	})
	return items
}

// findLatestHelmReleaseSecret returns the newest helm release secret for the
// given release name in namespace, or ok=false if none exists. The latest is
// determined by CreationTimestamp.
func findLatestHelmReleaseSecret(ctx context.Context, cs kubernetes.Interface, namespace, releaseName string) (corev1.Secret, bool) {
	opts := metav1.ListOptions{LabelSelector: "owner=helm,name=" + releaseName}
	list, err := cs.CoreV1().Secrets(namespace).List(ctx, opts)
	if err != nil || len(list.Items) == 0 {
		return corev1.Secret{}, false
	}
	latest := list.Items[0]
	for _, s := range list.Items[1:] {
		if s.CreationTimestamp.After(latest.CreationTimestamp.Time) {
			latest = s
		}
	}
	return latest, true
}

// buildItemsFromManifestRefs converts parsed manifest refs into model.Items.
// Instead of setting per-kind icons, it populates Columns with KIND and
// APIVERSION key-value pairs so the explorer renders them as table columns
// (matching the ArgoCD Application children pattern).
//
// Item.Name is always the raw Kubernetes resource name (no namespace prefix)
// because it is forwarded to the dynamic client as a resource name when the
// user loads YAML, and the API server rejects any name containing '/'.
// Cross-namespace visibility is provided by Item.Namespace, which the explorer
// surfaces in its own NAMESPACE column whenever any item carries one.
//
// The second return value is an index keyed by Kind+Namespace+Name that lets
// the caller match items to live resources for status enrichment.
func buildItemsFromManifestRefs(refs []ManifestResourceRef) ([]model.Item, map[string]int) {
	items := make([]model.Item, 0, len(refs))
	index := make(map[string]int, len(refs))
	for _, ref := range refs {
		items = append(items, model.Item{
			Name:      ref.Name,
			Namespace: ref.Namespace,
			Kind:      ref.Kind,
			Extra:     ref.APIVersion,
			Columns: []model.KeyValue{
				{Key: "KIND", Value: ref.Kind},
				{Key: "APIVERSION", Value: ref.APIVersion},
			},
		})
		index[helmRefKey(ref.Kind, ref.Namespace, ref.Name)] = len(items) - 1
	}
	return items, index
}

// helmRefKey builds a stable merge key for a manifest ref or a live resource.
// Namespace is included so cross-namespace resources with colliding names
// (same kind + same name in different namespaces) merge correctly.
func helmRefKey(kind, namespace, name string) string {
	return kind + "/" + namespace + "/" + name
}

// enrichHelmWorkloadStatus merges live ready counts into manifest-derived
// items for known workload kinds in the release's namespace. It reuses the
// existing collectHelm* helpers so any fields they populate (Ready, Age,
// Status, CreatedAt) are carried over without duplicating the logic. The
// mergeIndex is keyed by Kind+Namespace+Name against the canonical ref
// identity so the display-name transform applied to the Name field does not
// break matching.
func enrichHelmWorkloadStatus(ctx context.Context, cs kubernetes.Interface, namespace, releaseName string, items []model.Item, mergeIndex map[string]int) {
	if len(mergeIndex) == 0 {
		return
	}
	// Collect live workloads in the release namespace using every known
	// selector. The final empty selector picks up workloads that are in the
	// manifest but don't carry instance labels — exactly the Cilium-style
	// case we're fixing.
	labelSelectors := []string{
		"app.kubernetes.io/instance=" + releaseName,
		"release=" + releaseName,
		"",
	}

	seen := make(map[string]bool)
	var live []model.Item
	for _, selector := range labelSelectors {
		opts := metav1.ListOptions{LabelSelector: selector}
		collectHelmDeployments(&live, seen, cs, ctx, namespace, opts)
		collectHelmStatefulSets(&live, seen, cs, ctx, namespace, opts)
		collectHelmDaemonSets(&live, seen, cs, ctx, namespace, opts)
	}
	if len(live) == 0 {
		return
	}
	for _, l := range live {
		// collectHelm* helpers fetch from `namespace`, so every live item is
		// in that namespace even though Item.Namespace is left unset.
		key := helmRefKey(l.Kind, namespace, l.Name)
		idx, ok := mergeIndex[key]
		if !ok {
			continue
		}
		items[idx].Ready = l.Ready
		if l.Status != "" {
			items[idx].Status = l.Status
		}
		if l.Age != "" {
			items[idx].Age = l.Age
		}
		if !l.CreatedAt.IsZero() {
			items[idx].CreatedAt = l.CreatedAt
		}
	}
}

// collectHelmDeployments lists Deployments matching opts and appends them (with status/ready) to items.
func collectHelmDeployments(items *[]model.Item, seen map[string]bool, cs kubernetes.Interface, ctx context.Context, namespace string, opts metav1.ListOptions) {
	depList, err := cs.AppsV1().Deployments(namespace).List(ctx, opts)
	if err != nil {
		return
	}
	for _, d := range depList.Items {
		key := "Deployment/" + d.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		var replicas int32 = 1
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		ti := model.Item{Name: d.Name, Kind: "Deployment"}
		if d.Status.AvailableReplicas == replicas {
			ti.Status = "Available"
		} else {
			ti.Status = "Progressing"
		}
		ti.Ready = fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, replicas)
		ti.CreatedAt = d.CreationTimestamp.Time
		ti.Age = formatAge(time.Since(d.CreationTimestamp.Time))
		*items = append(*items, ti)
	}
}

// collectHelmStatefulSets lists StatefulSets matching opts and appends them (with ready) to items.
func collectHelmStatefulSets(items *[]model.Item, seen map[string]bool, cs kubernetes.Interface, ctx context.Context, namespace string, opts metav1.ListOptions) {
	ssList, err := cs.AppsV1().StatefulSets(namespace).List(ctx, opts)
	if err != nil {
		return
	}
	for _, ss := range ssList.Items {
		key := "StatefulSet/" + ss.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		var replicas int32 = 1
		if ss.Spec.Replicas != nil {
			replicas = *ss.Spec.Replicas
		}
		ti := model.Item{Name: ss.Name, Kind: "StatefulSet"}
		ti.Ready = fmt.Sprintf("%d/%d", ss.Status.ReadyReplicas, replicas)
		ti.CreatedAt = ss.CreationTimestamp.Time
		ti.Age = formatAge(time.Since(ss.CreationTimestamp.Time))
		*items = append(*items, ti)
	}
}

// collectHelmDaemonSets lists DaemonSets matching opts and appends them (with ready) to items.
func collectHelmDaemonSets(items *[]model.Item, seen map[string]bool, cs kubernetes.Interface, ctx context.Context, namespace string, opts metav1.ListOptions) {
	dsList, err := cs.AppsV1().DaemonSets(namespace).List(ctx, opts)
	if err != nil {
		return
	}
	for _, ds := range dsList.Items {
		key := "DaemonSet/" + ds.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		ti := model.Item{Name: ds.Name, Kind: "DaemonSet"}
		ti.Ready = fmt.Sprintf("%d/%d", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
		ti.CreatedAt = ds.CreationTimestamp.Time
		ti.Age = formatAge(time.Since(ds.CreationTimestamp.Time))
		*items = append(*items, ti)
	}
}

// collectHelmSimpleResources lists Services, ConfigMaps, Secrets,
// ServiceAccounts, and Ingresses matching opts and appends them to items.
func collectHelmSimpleResources(items *[]model.Item, seen map[string]bool, cs kubernetes.Interface, ctx context.Context, namespace string, opts metav1.ListOptions) {
	// Services
	if svcList, err := cs.CoreV1().Services(namespace).List(ctx, opts); err == nil {
		for _, s := range svcList.Items {
			key := "Service/" + s.Name
			if seen[key] {
				continue
			}
			seen[key] = true
			*items = append(*items, model.Item{
				Name:      s.Name,
				Kind:      "Service",
				Status:    "Active",
				CreatedAt: s.CreationTimestamp.Time,
				Age:       formatAge(time.Since(s.CreationTimestamp.Time)),
			})
		}
	}

	// ConfigMaps
	if cmList, err := cs.CoreV1().ConfigMaps(namespace).List(ctx, opts); err == nil {
		for _, cm := range cmList.Items {
			appendIfUnseenItem(items, seen, "ConfigMap", cm.Name, cm.CreationTimestamp.Time)
		}
	}

	// Secrets (non-helm-release)
	if secList, err := cs.CoreV1().Secrets(namespace).List(ctx, opts); err == nil {
		for _, s := range secList.Items {
			if s.Labels["owner"] == "helm" {
				continue // skip helm release secrets
			}
			appendIfUnseenItem(items, seen, "Secret", s.Name, s.CreationTimestamp.Time)
		}
	}

	// ServiceAccounts
	if saList, err := cs.CoreV1().ServiceAccounts(namespace).List(ctx, opts); err == nil {
		for _, sa := range saList.Items {
			appendIfUnseenItem(items, seen, "ServiceAccount", sa.Name, sa.CreationTimestamp.Time)
		}
	}

	// Ingresses
	if ingList, err := cs.NetworkingV1().Ingresses(namespace).List(ctx, opts); err == nil {
		for _, ing := range ingList.Items {
			appendIfUnseenItem(items, seen, "Ingress", ing.Name, ing.CreationTimestamp.Time)
		}
	}
}

// appendIfUnseenItem appends an item with no namespace to items if the kind/name key has not been seen.
func appendIfUnseenItem(items *[]model.Item, seen map[string]bool, kind, name string, createdAt time.Time) {
	key := kind + "/" + name
	if seen[key] {
		return
	}
	seen[key] = true
	*items = append(*items, model.Item{
		Name:      name,
		Kind:      kind,
		CreatedAt: createdAt,
		Age:       formatAge(time.Since(createdAt)),
	})
}
