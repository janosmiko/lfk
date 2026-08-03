package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
)

// writeSecureFile writes data to path with owner-only (0600) permissions,
// truncating any existing file. Unlike os.WriteFile it also tightens the mode
// of a pre-existing file, so a resource export that may contain Secret data
// (base64 tokens, TLS keys) never stays world-readable.
func writeSecureFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// copyYAMLToClipboard fetches the YAML for the selected resource and sends it for clipboard copy.
//
// Every fetch routes through scheduleK8sCall (KindYAMLFetch) so it shares the
// bounded worker pool, priority lanes, and gen-based cancellation with the rest
// of the K8s reads. The task names are deliberately distinct from the "YAML: …"
// preview path so a copy never coalesces onto an in-flight preview fetch (which
// would deliver the copy a nil result and silently drop it).
func (m Model) copyYAMLToClipboard() tea.Cmd {
	// Synthetic security items (e.g., __security_affected_resource__) have
	// no YAML; short-circuit to avoid "unknown resource type" warnings.
	if onSecurityView(&m) {
		return nil
	}
	kctx := m.effectiveContext()
	ns := m.resolveNamespace()
	target := bgtaskTarget(kctx, ns)

	switch m.nav.Level {
	case model.LevelResources:
		rt := m.nav.ResourceType
		// Gate on visible selection, not raw hasSelection() — selected rows
		// can be filtered out of view, in which case we want the single-item
		// (cursor) path, not an empty multi-doc fetch.
		if items := m.selectedItemsList(); len(items) > 0 {
			type fetchTarget struct {
				ns, name, ctx string
			}
			targets := make([]fetchTarget, len(items))
			for i, it := range items {
				itemNs := ns
				if it.Namespace != "" {
					itemNs = it.Namespace
				}
				// Per-item cluster routing for union mode: each row carries
				// its source cluster on it.ClusterName. ClusterName is empty
				// in non-union mode so this falls back to kctx.
				itemCtx := kctx
				if it.ClusterName != "" {
					itemCtx = it.ClusterName
				}
				targets[i] = fetchTarget{ns: itemNs, name: it.Name, ctx: itemCtx}
			}
			return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
				fmt.Sprintf("Copy YAML (%d items)", len(targets)), target,
				func(ctx context.Context) tea.Msg {
					docs := make([]string, 0, len(targets))
					var failures []string
					for _, t := range targets {
						content, err := m.client.GetResourceYAML(ctx, t.ctx, t.ns, rt, t.name)
						if err != nil {
							failures = append(failures, fmt.Sprintf("%s/%s: %v", t.ns, t.name, err))
							continue
						}
						docs = append(docs, strings.TrimRight(content, "\n"))
					}
					return buildBulkYAMLClipboardMsg(docs, failures, len(targets))
				})
		}
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		itemCtx := kctx
		if sel.ClusterName != "" {
			itemCtx = sel.ClusterName
		}
		return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
			"Copy YAML: "+name, target,
			func(ctx context.Context) tea.Msg {
				content, err := m.client.GetResourceYAML(ctx, itemCtx, itemNs, rt, name)
				return yamlClipboardMsg{content: content, count: 1, err: err}
			})
	case model.LevelOwned:
		// Bulk path mirrors LevelResources: gate on visible selection so a
		// selection that's been filtered out of view falls through to the
		// cursor branch instead of dispatching an empty fetch. Per-item Kind
		// dispatch (Pod -> GetPodYAML; others -> resolveOwnedResourceType +
		// GetResourceYAML) is resolved before the closure runs to keep the
		// goroutine off the model.
		if items := m.selectedItemsList(); len(items) > 0 {
			type fetchTarget struct {
				ns, name string
				isPod    bool
				rt       model.ResourceTypeEntry
				resolved bool
				kind     string
			}
			targets := make([]fetchTarget, len(items))
			for i, it := range items {
				itemNs := ns
				if it.Namespace != "" {
					itemNs = it.Namespace
				}
				t := fetchTarget{ns: itemNs, name: it.Name, kind: it.Kind, isPod: it.Kind == "Pod"}
				if !t.isPod {
					t.rt, t.resolved = m.resolveOwnedResourceType(&items[i])
				}
				targets[i] = t
			}
			return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
				fmt.Sprintf("Copy YAML (%d items)", len(targets)), target,
				func(ctx context.Context) tea.Msg {
					docs := make([]string, 0, len(targets))
					var failures []string
					for _, t := range targets {
						var (
							content string
							err     error
						)
						switch {
						case t.isPod:
							content, err = m.client.GetPodYAML(ctx, kctx, t.ns, t.name)
						case t.resolved:
							content, err = m.client.GetResourceYAML(ctx, kctx, t.ns, t.rt, t.name)
						default:
							err = fmt.Errorf("unknown resource type: %s", t.kind)
						}
						if err != nil {
							failures = append(failures, fmt.Sprintf("%s/%s: %v", t.ns, t.name, err))
							continue
						}
						docs = append(docs, strings.TrimRight(content, "\n"))
					}
					return buildBulkYAMLClipboardMsg(docs, failures, len(targets))
				})
		}
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		if sel.Kind == "Pod" {
			return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
				"Copy YAML: "+name, target,
				func(ctx context.Context) tea.Msg {
					content, err := m.client.GetPodYAML(ctx, kctx, itemNs, name)
					return yamlClipboardMsg{content: content, count: 1, err: err}
				})
		}
		rt, ok := m.resolveOwnedResourceType(sel)
		if !ok {
			return func() tea.Msg {
				return yamlClipboardMsg{err: fmt.Errorf("unknown resource type: %s", sel.Kind)}
			}
		}
		return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
			"Copy YAML: "+name, target,
			func(ctx context.Context) tea.Msg {
				content, err := m.client.GetResourceYAML(ctx, kctx, itemNs, rt, name)
				return yamlClipboardMsg{content: content, count: 1, err: err}
			})
	case model.LevelContainers:
		podName := m.nav.OwnedName
		// Bulk path: if the user has selected N containers in this Pod,
		// fetch the Pod manifest once and extract just those container
		// spec blocks. Falls through to the whole-Pod cursor path when
		// nothing is selected (back-compat with existing single-cursor
		// behavior).
		if items := m.selectedItemsList(); len(items) > 0 {
			names := make([]string, len(items))
			for i, it := range items {
				names[i] = it.Name
			}
			return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
				"Copy YAML: "+podName, target,
				func(ctx context.Context) tea.Msg {
					content, err := m.client.GetPodYAML(ctx, kctx, ns, podName)
					if err != nil {
						return yamlClipboardMsg{err: fmt.Errorf("%s/%s: %w", ns, podName, err)}
					}
					out, err := ExtractContainerBlocksYAML(content, names)
					if err != nil {
						return yamlClipboardMsg{err: fmt.Errorf("%s/%s: %w", ns, podName, err)}
					}
					return yamlClipboardMsg{content: out, count: len(names)}
				})
		}
		return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
			"Copy YAML: "+podName, target,
			func(ctx context.Context) tea.Msg {
				content, err := m.client.GetPodYAML(ctx, kctx, ns, podName)
				return yamlClipboardMsg{content: content, count: 1, err: err}
			})
	}
	return nil
}

// copyYAMLForScope is the scope-driven sibling of copyYAMLToClipboard.
// It builds a YAML fetch tea.Cmd from the supplied items instead of
// re-reading the live selection — used by the copy-as picker to honor
// the snapshot captured at picker-open time. Returns nil for an empty
// scope at any level. LevelContainers extracts each container spec
// block from the Pod YAML.
//
// Bulk paths are resilient to per-item failures: if some of the N
// requested fetches fail (RBAC, 404 on a row that was just deleted by
// a controller, transient network blip), the successes are still
// concatenated and copied. The per-item errors are joined into the
// returned yamlClipboardMsg.err so updateYamlClipboard can surface
// them — but only after the successful payload has been recorded so
// the user gets a partial copy rather than an empty clipboard.
// Without this every "select N → Y" interaction was all-or-nothing,
// turning a single broken row into "nothing copied" across the batch.
func (m Model) copyYAMLForScope(scope []model.Item) tea.Cmd {
	if len(scope) == 0 {
		return nil
	}
	kctx := m.nav.Context
	ns := m.resolveNamespace()
	target := bgtaskTarget(kctx, ns)

	switch m.nav.Level {
	case model.LevelResources:
		rt := m.nav.ResourceType
		type fetchTarget struct {
			ns, name string
		}
		targets := make([]fetchTarget, len(scope))
		for i, it := range scope {
			itemNs := ns
			if it.Namespace != "" {
				itemNs = it.Namespace
			}
			targets[i] = fetchTarget{ns: itemNs, name: it.Name}
		}
		return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
			fmt.Sprintf("Copy YAML (%d items)", len(targets)), target,
			func(ctx context.Context) tea.Msg {
				docs := make([]string, 0, len(targets))
				var failures []string
				for _, t := range targets {
					content, err := m.client.GetResourceYAML(ctx, kctx, t.ns, rt, t.name)
					if err != nil {
						failures = append(failures, fmt.Sprintf("%s/%s: %v", t.ns, t.name, err))
						continue
					}
					docs = append(docs, strings.TrimRight(content, "\n"))
				}
				return buildBulkYAMLClipboardMsg(docs, failures, len(targets))
			})
	case model.LevelOwned:
		type fetchTarget struct {
			ns, name string
			isPod    bool
			rt       model.ResourceTypeEntry
			resolved bool
			kind     string
		}
		targets := make([]fetchTarget, len(scope))
		for i, it := range scope {
			itemNs := ns
			if it.Namespace != "" {
				itemNs = it.Namespace
			}
			t := fetchTarget{ns: itemNs, name: it.Name, kind: it.Kind, isPod: it.Kind == "Pod"}
			if !t.isPod {
				t.rt, t.resolved = m.resolveOwnedResourceType(&scope[i])
			}
			targets[i] = t
		}
		return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
			fmt.Sprintf("Copy YAML (%d items)", len(targets)), target,
			func(ctx context.Context) tea.Msg {
				docs := make([]string, 0, len(targets))
				var failures []string
				for _, t := range targets {
					var (
						content string
						err     error
					)
					switch {
					case t.isPod:
						content, err = m.client.GetPodYAML(ctx, kctx, t.ns, t.name)
					case t.resolved:
						content, err = m.client.GetResourceYAML(ctx, kctx, t.ns, t.rt, t.name)
					default:
						err = fmt.Errorf("unknown resource type: %s", t.kind)
					}
					if err != nil {
						failures = append(failures, fmt.Sprintf("%s/%s: %v", t.ns, t.name, err))
						continue
					}
					docs = append(docs, strings.TrimRight(content, "\n"))
				}
				return buildBulkYAMLClipboardMsg(docs, failures, len(targets))
			})
	case model.LevelContainers:
		podName := m.nav.OwnedName
		names := make([]string, len(scope))
		for i, it := range scope {
			names[i] = it.Name
		}
		return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
			"Copy YAML: "+podName, target,
			func(ctx context.Context) tea.Msg {
				content, err := m.client.GetPodYAML(ctx, kctx, ns, podName)
				if err != nil {
					return yamlClipboardMsg{err: fmt.Errorf("%s/%s: %w", ns, podName, err)}
				}
				out, err := ExtractContainerBlocksYAML(content, names)
				if err != nil {
					return yamlClipboardMsg{err: fmt.Errorf("%s/%s: %w", ns, podName, err)}
				}
				return yamlClipboardMsg{content: out, count: len(names)}
			})
	}
	return nil
}

// buildBulkYAMLClipboardMsg assembles a yamlClipboardMsg for a bulk-fetch
// path with per-item failure tolerance. docs holds the successful YAML
// payloads (joined with the standard \n---\n multi-doc separator);
// failures carries the "<ns>/<name>: <err>" lines for each fetch that
// errored out. total is the original target count.
//
// Behavior:
//   - All N succeeded → success message (count == N, no err).
//   - All N failed → error-only message (no content, single err with
//     every failure joined).
//   - Mixed → partial-success message: content is the successes,
//     count is len(docs), err is the joined failures so the status
//     line can render "copied K/N, M failed: ...".
//
// Without this, every per-item failure aborts the batch and leaves the
// clipboard untouched — which made "select N → Y → YAML" look broken
// whenever a single row (e.g. a 404 from a controller-driven delete
// racing the fetch, or an RBAC gap on one item) tripped the loop.
func buildBulkYAMLClipboardMsg(docs, failures []string, total int) yamlClipboardMsg {
	if len(docs) == 0 {
		return yamlClipboardMsg{
			err: fmt.Errorf("all %d fetches failed: %s", total, strings.Join(failures, "; ")),
		}
	}
	msg := yamlClipboardMsg{
		content: strings.Join(docs, "\n---\n") + "\n",
		count:   len(docs),
	}
	if len(failures) > 0 {
		msg.err = fmt.Errorf("copied %d/%d, %d failed: %s", len(docs), total, len(failures), strings.Join(failures, "; "))
	}
	return msg
}

// exportResourceToFile saves the selected resource YAML to a file.
func (m Model) exportResourceToFile() tea.Cmd {
	// Synthetic security items have no YAML to export.
	if onSecurityView(&m) {
		return nil
	}
	kctx := m.effectiveContext()
	ns := m.resolveNamespace()

	var fetchYAML func(ctx context.Context) (string, string, error) // returns (yaml, kindForFilename, error)
	var exportName string

	switch m.nav.Level {
	case model.LevelResources:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		rt := m.nav.ResourceType
		name := sel.Name
		exportName = name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		itemCtx := kctx
		if sel.ClusterName != "" {
			itemCtx = sel.ClusterName
		}
		kind := strings.ToLower(rt.Kind)
		fetchYAML = func(ctx context.Context) (string, string, error) {
			content, err := m.client.GetResourceYAML(ctx, itemCtx, itemNs, rt, name)
			return content, kind, err
		}
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		name := sel.Name
		exportName = name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		itemCtx := kctx
		if sel.ClusterName != "" {
			itemCtx = sel.ClusterName
		}
		if sel.Kind == "Pod" {
			fetchYAML = func(ctx context.Context) (string, string, error) {
				content, err := m.client.GetPodYAML(ctx, itemCtx, itemNs, name)
				return content, "pod", err
			}
		} else {
			rt, ok := m.resolveOwnedResourceType(sel)
			if !ok {
				return func() tea.Msg {
					return exportDoneMsg{err: fmt.Errorf("unknown resource type: %s", sel.Kind)}
				}
			}
			kind := strings.ToLower(rt.Kind)
			fetchYAML = func(ctx context.Context) (string, string, error) {
				content, err := m.client.GetResourceYAML(ctx, itemCtx, itemNs, rt, name)
				return content, kind, err
			}
		}
	case model.LevelContainers:
		podName := m.nav.OwnedName
		exportName = podName
		fetchYAML = func(ctx context.Context) (string, string, error) {
			content, err := m.client.GetPodYAML(ctx, kctx, ns, podName)
			return content, "pod", err
		}
	default:
		return nil
	}

	return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
		"Export YAML: "+exportName, bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			yaml, kind, err := fetchYAML(ctx)
			if err != nil {
				return exportDoneMsg{err: fmt.Errorf("fetching resource: %w", err)}
			}

			// Build filename: <kind>_<name>.yaml
			var name string
			switch m.nav.Level {
			case model.LevelContainers:
				name = m.nav.OwnedName
			default:
				sel := m.selectedMiddleItem()
				if sel != nil {
					name = sel.Name
				}
			}
			sanitized := strings.ReplaceAll(name, "/", "_")
			filename := fmt.Sprintf("%s_%s.yaml", kind, sanitized)

			if err := writeSecureFile(filename, []byte(yaml)); err != nil {
				return exportDoneMsg{err: fmt.Errorf("writing file: %w", err)}
			}

			abs, _ := filepath.Abs(filename)
			if abs == "" {
				abs = filename
			}
			return exportDoneMsg{path: abs}
		})
}
